package meteora

import (
	"context"
	"fmt"
	"math/big"
	"unsafe"

	"github.com/gagliardetto/solana-go"
	"github.com/yimingWOW/solroute/pkg"
	"github.com/yimingWOW/solroute/pkg/sol"
)

// MeteoraDlmmPool represents a Meteora DLMM (Dynamic Liquidity Market Maker) pool
// This struct contains all the pool parameters, state, and runtime data
type MeteoraDlmmPool struct {
	Discriminator [8]uint8 `bin:"borsh"`
	parameters    struct {
		baseFactor               uint16   `bin:"borsh"`
		filterPeriod             uint16   `bin:"borsh"`
		decayPeriod              uint16   `bin:"borsh"`
		reductionFactor          uint16   `bin:"borsh"`
		variableFeeControl       uint32   `bin:"borsh"`
		maxVolatilityAccumulator uint32   `bin:"borsh"`
		minBinId                 int32    `bin:"borsh"`
		maxBinId                 int32    `bin:"borsh"`
		protocolShare            uint16   `bin:"borsh"`
		baseFeePowerFactor       uint8    `bin:"borsh"`
		padding                  [5]uint8 `bin:"borsh"`
	} `bin:"borsh"`
	vParameters struct {
		volatilityAccumulator uint32   `bin:"borsh"`
		volatilityReference   uint32   `bin:"borsh"`
		indexReference        int32    `bin:"borsh"`
		padding               [4]uint8 `bin:"borsh"`
		lastUpdateTimestamp   int64    `bin:"borsh"`
		padding1              [8]uint8 `bin:"borsh"`
	} `bin:"borsh"`
	bumpSeed                [1]uint8         `bin:"borsh"`
	binStepSeed             [2]uint8         `bin:"borsh"`
	pairType                uint8            `bin:"borsh"`
	activeId                int32            `bin:"borsh"`
	binStep                 uint16           `bin:"borsh"`
	status                  uint8            `bin:"borsh"`
	requireBaseFactorSeed   uint8            `bin:"borsh"`
	baseFactorSeed          [2]uint8         `bin:"borsh"`
	activationType          uint8            `bin:"borsh"`
	creatorPoolOnOffControl uint8            `bin:"borsh"`
	TokenXMint              solana.PublicKey `bin:"borsh"`
	TokenYMint              solana.PublicKey `bin:"borsh"`
	reserveX                solana.PublicKey `bin:"borsh"`
	reserveY                solana.PublicKey `bin:"borsh"`
	protocolFee             struct {
		amountX uint64 `bin:"borsh"`
		amountY uint64 `bin:"borsh"`
	} `bin:"borsh"`
	padding1    [32]uint8 `bin:"borsh"`
	rewardInfos [2]struct {
		mint                                      solana.PublicKey `bin:"borsh"`
		vault                                     solana.PublicKey `bin:"borsh"`
		funder                                    solana.PublicKey `bin:"borsh"`
		rewardDuration                            int64            `bin:"borsh"`
		rewardDurationEnd                         int64            `bin:"borsh"`
		rewardRate                                int64            `bin:"borsh"`
		lastUpdateTime                            int64            `bin:"borsh"`
		cumulativeSecondsWithEmptyLiquidityReward int64            `bin:"borsh"`
	} `bin:"borsh"`
	oracle                   solana.PublicKey `bin:"borsh"`
	binArrayBitmap           [16]uint64       `bin:"borsh"`
	lastUpdatedAt            int64            `bin:"borsh"`
	padding2                 [32]uint8        `bin:"borsh"`
	preActivationSwapAddress solana.PublicKey `bin:"borsh"`
	baseKey                  solana.PublicKey `bin:"borsh"`
	activationPoint          uint64           `bin:"borsh"`
	preActivationDuration    uint64           `bin:"borsh"`
	padding3                 [8]uint8         `bin:"borsh"`
	padding4                 uint64           `bin:"borsh"`
	creator                  solana.PublicKey `bin:"borsh"`
	tokenMintXProgramFlag    uint8            `bin:"borsh"`
	tokenMintYProgramFlag    uint8            `bin:"borsh"`
	reserved                 [22]uint8        `bin:"borsh"`
	_                        [16]uint8        `bin:"borsh"` // padding to ensure 904 bytes total size

	// Runtime fields (not part of on-chain data)
	PoolId             solana.PublicKey
	BinArrays          map[string]BinArray // key: binArrayPubkey
	BitmapExtensionKey solana.PublicKey
	bitmapExtension    *BinArrayBitmapExtension
	Clock              sol.Clock
	orgActiveId        int32
	UserBaseAccount    solana.PublicKey
	UserQuoteAccount   solana.PublicKey
}

func (pool *MeteoraDlmmPool) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolNameMeteoraDlmm
}

func (pool *MeteoraDlmmPool) ProtocolType() pkg.ProtocolType {
	return pkg.ProtocolTypeMeteoraDlmm
}

func (pool *MeteoraDlmmPool) GetProgramID() solana.PublicKey {
	return MeteoraProgramID
}

// GetID returns the pool ID as a string
func (pool *MeteoraDlmmPool) GetID() string {
	return pool.PoolId.String()
}

// GetTokens returns the token mint addresses as strings
func (pool *MeteoraDlmmPool) GetTokens() (string, string) {
	return pool.TokenXMint.String(), pool.TokenYMint.String()
}

// Span returns the size of the pool struct in bytes
func (pool *MeteoraDlmmPool) Span() uint64 {
	return uint64(unsafe.Sizeof(*pool))
}

// Offset returns the byte offset of a specific field in the pool data
func (pool *MeteoraDlmmPool) Offset(field string) uint64 {
	switch field {
	case "TokenYMint":
		return 120
	case "TokenXMint":
		return 88
	default:
		return 0
	}
}

// Decode deserializes binary data into the pool structure
func (pool *MeteoraDlmmPool) Decode(data []byte) error {
	// Manual parsing for first few fields
	offset := 8 // Skip discriminator
	pool.parameters.baseFactor = uint16(data[offset]) | uint16(data[offset+1])<<8
	offset += 2

	pool.parameters.filterPeriod = uint16(data[offset]) | uint16(data[offset+1])<<8
	offset += 2

	pool.parameters.decayPeriod = uint16(data[offset]) | uint16(data[offset+1])<<8
	offset += 2

	pool.parameters.reductionFactor = uint16(data[offset]) | uint16(data[offset+1])<<8
	offset += 2

	pool.parameters.variableFeeControl = uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24
	offset += 4

	pool.parameters.maxVolatilityAccumulator = uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24
	offset += 4

	pool.parameters.minBinId = int32(uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24)
	offset += 4

	pool.parameters.maxBinId = int32(uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24)
	offset += 4

	pool.parameters.protocolShare = uint16(data[offset]) | uint16(data[offset+1])<<8
	offset += 2

	pool.parameters.baseFeePowerFactor = data[offset]
	offset += 1

	// Skip padding
	offset += 5

	// Parse vParameters
	pool.vParameters.volatilityAccumulator = uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24
	offset += 4

	pool.vParameters.volatilityReference = uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24
	offset += 4

	pool.vParameters.indexReference = int32(uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24)
	offset += 4

	// Skip padding
	offset += 4

	pool.vParameters.lastUpdateTimestamp = int64(uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 | uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56)
	offset += 8

	// Skip padding
	offset += 8

	// Parse bumpSeed and binStepSeed
	pool.bumpSeed[0] = data[offset]
	offset += 1

	pool.binStepSeed[0] = data[offset]
	pool.binStepSeed[1] = data[offset+1]
	offset += 2

	// Parse pairType and activeId
	pool.pairType = data[offset]
	offset += 1

	pool.activeId = int32(uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24)
	offset += 4

	// Parse binStep and status
	pool.binStep = uint16(data[offset]) | uint16(data[offset+1])<<8
	offset += 2

	pool.status = data[offset]
	offset += 1

	// Parse requireBaseFactorSeed and baseFactorSeed
	pool.requireBaseFactorSeed = data[offset]
	offset += 1

	pool.baseFactorSeed[0] = data[offset]
	pool.baseFactorSeed[1] = data[offset+1]
	offset += 2

	// Parse activationType and creatorPoolOnOffControl
	pool.activationType = data[offset]
	offset += 1

	pool.creatorPoolOnOffControl = data[offset]
	offset += 1

	// Parse TokenXMint
	copy(pool.TokenXMint[:], data[offset:offset+32])
	offset += 32

	// Parse TokenYMint
	copy(pool.TokenYMint[:], data[offset:offset+32])
	offset += 32

	// Parse reserveX
	copy(pool.reserveX[:], data[offset:offset+32])
	offset += 32

	// Parse reserveY
	copy(pool.reserveY[:], data[offset:offset+32])
	offset += 32

	// Parse protocolFee
	pool.protocolFee.amountX = uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 | uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56
	offset += 8

	pool.protocolFee.amountY = uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 | uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56
	offset += 8

	// Skip padding1
	offset += 32

	// Parse rewardInfos
	for i := 0; i < 2; i++ {
		copy(pool.rewardInfos[i].mint[:], data[offset:offset+32])
		offset += 32

		copy(pool.rewardInfos[i].vault[:], data[offset:offset+32])
		offset += 32

		copy(pool.rewardInfos[i].funder[:], data[offset:offset+32])
		offset += 32

		pool.rewardInfos[i].rewardDuration = int64(uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 | uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56)
		offset += 8

		pool.rewardInfos[i].rewardDurationEnd = int64(uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 | uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56)
		offset += 8

		pool.rewardInfos[i].rewardRate = int64(uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 | uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56)
		offset += 8

		pool.rewardInfos[i].lastUpdateTime = int64(uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 | uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56)
		offset += 8

		pool.rewardInfos[i].cumulativeSecondsWithEmptyLiquidityReward = int64(uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 | uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56)
		offset += 8
	}

	// Adjust offset to match the correct oracle position
	offset = 552

	// Parse oracle
	copy(pool.oracle[:], data[offset:offset+32])
	offset += 32

	// Parse binArrayBitmap
	for i := 0; i < 16; i++ {
		pool.binArrayBitmap[i] = uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 | uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56
		offset += 8
	}

	// Parse lastUpdatedAt
	pool.lastUpdatedAt = int64(uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 | uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56)
	offset += 8

	// Skip padding2
	offset += 32

	// Parse preActivationSwapAddress
	copy(pool.preActivationSwapAddress[:], data[offset:offset+32])
	offset += 32

	// Parse baseKey
	copy(pool.baseKey[:], data[offset:offset+32])
	offset += 32

	// Parse activationPoint
	pool.activationPoint = uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 | uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56
	offset += 8

	// Parse preActivationDuration
	pool.preActivationDuration = uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 | uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56
	offset += 8

	// Skip padding3
	offset += 8

	// Parse padding4
	pool.padding4 = uint64(data[offset]) | uint64(data[offset+1])<<8 | uint64(data[offset+2])<<16 | uint64(data[offset+3])<<24 | uint64(data[offset+4])<<32 | uint64(data[offset+5])<<40 | uint64(data[offset+6])<<48 | uint64(data[offset+7])<<56
	offset += 8

	// Parse creator
	copy(pool.creator[:], data[offset:offset+32])
	offset += 32

	// Parse tokenMintXProgramFlag and tokenMintYProgramFlag
	pool.tokenMintXProgramFlag = data[offset]
	offset += 1

	pool.tokenMintYProgramFlag = data[offset]
	offset += 1

	// Parse reserved
	copy(pool.reserved[:], data[offset:offset+22])
	offset += 22

	// Skip final padding
	offset += 16

	return nil
}

// ComputeFee calculates the fee for a given amount using ceiling division
func (pool *MeteoraDlmmPool) ComputeFee(amount uint64) (uint64, error) {
	// Get total fee rate
	totalFeeRate, err := pool.GetTotalFee()
	if err != nil {
		return 0, fmt.Errorf("failed to get total fee: %w", err)
	}

	// Calculate denominator: FEE_PRECISION - totalFeeRate
	feePrecision := new(big.Int).SetUint64(FeePrecision)
	denominator := new(big.Int).Sub(feePrecision, totalFeeRate)
	if denominator.Sign() <= 0 {
		return 0, fmt.Errorf("denominator overflow or zero: feePrecision=%v, totalFeeRate=%v", feePrecision, totalFeeRate)
	}

	// Ceiling division calculation
	// fee = (amount * totalFeeRate + denominator - 1) / denominator

	// 1. amount * totalFeeRate
	amountBig := new(big.Int).SetUint64(amount)
	fee := new(big.Int).Mul(amountBig, totalFeeRate)

	// 2. + denominator
	fee.Add(fee, denominator)

	// 3. - 1
	fee.Sub(fee, big.NewInt(1))

	// 4. / denominator
	fee.Div(fee, denominator)

	// Check if result exceeds uint64 range
	if !fee.IsUint64() {
		return 0, fmt.Errorf("fee exceeds uint64 range")
	}

	return fee.Uint64(), nil
}

// UpdateClock fetches and updates the current clock information
func (pool *MeteoraDlmmPool) UpdateClock(ctx context.Context, client *sol.Client) error {
	clock, err := client.GetClock(ctx)
	if err != nil {
		return fmt.Errorf("failed to get clock: %w", err)
	}
	pool.Clock = *clock
	return nil
}

// GetBinArrayForSwap retrieves bin arrays needed for swap operations
func (pool *MeteoraDlmmPool) GetBinArrayForSwap(ctx context.Context, client *sol.Client) error {
	if pool.BinArrays == nil {
		pool.BinArrays = make(map[string]BinArray) // Initialize bin array map
	}

	// Get active bin array public keys for both positive and negative orders
	var activeBinArrayPubkeys []solana.PublicKey

	positiveOrderActiveBinArrayPubkeys, err := pool.GetBinArrayPubkeysForSwap(true, 4)
	if err != nil {
		return fmt.Errorf("failed to get positive order bin array pubkeys: %w", err)
	}
	activeBinArrayPubkeys = append(activeBinArrayPubkeys, positiveOrderActiveBinArrayPubkeys...)

	negativeOrderActiveBinArrayPubkeys, err := pool.GetBinArrayPubkeysForSwap(false, 4)
	if err != nil {
		return fmt.Errorf("failed to get negative order bin array pubkeys: %w", err)
	}
	activeBinArrayPubkeys = append(activeBinArrayPubkeys, negativeOrderActiveBinArrayPubkeys...)

	// Fetch all bin array accounts in batch
	results, err := client.RpcClient.GetMultipleAccounts(ctx, activeBinArrayPubkeys...)
	if err != nil {
		return fmt.Errorf("batch request failed: %w", err)
	}

	// Parse and store bin arrays
	for i, result := range results.Value {
		if result == nil {
			// Skip nil results (account doesn't exist)
			continue
		}
		accountKey := activeBinArrayPubkeys[i].String()
		binArray, err := ParseBinArray(result.Data.GetBinary())
		if err != nil {
			return fmt.Errorf("failed to parse bin array for account %s: %w", accountKey, err)
		}
		pool.BinArrays[accountKey] = binArray
	}
	return nil
}
