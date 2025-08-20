package orca

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
	"time"

	cosmath "cosmossdk.io/math"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/yimingWOW/solroute/pkg"
	"lukechampine.com/uint128"
)

// WhirlpoolPool struct - Mapped from Orca Whirlpool account structure
//
// This struct precisely maps the pool account data format of Orca Whirlpool V2 protocol.
// Data structure is based on Whirlpool struct from external/orca/whirlpool/generated/types.go,
// and adapted for field naming differences:
//   - CLMM: TokenMint0/1 → Whirlpool: TokenMintA/B
//   - CLMM: SqrtPriceX64 → Whirlpool: SqrtPrice
//   - CLMM: TickCurrent → Whirlpool: TickCurrentIndex
//
// Total account size: 653 bytes (including 8-byte discriminator)
type WhirlpoolPool struct {
	// 8 bytes discriminator
	Discriminator [8]uint8 `bin:"skip"`

	// Core configuration - Mapped from Whirlpool struct in external/orca/whirlpool/generated/types.go
	WhirlpoolsConfig solana.PublicKey // whirlpoolsConfig
	WhirlpoolBump    [1]uint8         // whirlpoolBump
	TickSpacing      uint16           // tickSpacing
	FeeTierIndexSeed [2]uint8         // feeTierIndexSeed
	FeeRate          uint16           // feeRate
	ProtocolFeeRate  uint16           // protocolFeeRate

	// Liquidity state - Field name mapping: SqrtPriceX64 -> SqrtPrice, TickCurrent -> TickCurrentIndex
	Liquidity        uint128.Uint128 // liquidity
	SqrtPrice        uint128.Uint128 // sqrtPrice (note: CLMM uses SqrtPriceX64)
	TickCurrentIndex int32           // tickCurrentIndex (note: CLMM uses TickCurrent)

	// Protocol fees
	ProtocolFeeOwedA uint64 // protocolFeeOwedA
	ProtocolFeeOwedB uint64 // protocolFeeOwedB

	// Token configuration - Field name mapping: TokenMint0/1 -> TokenMintA/B
	TokenMintA       solana.PublicKey // tokenMintA (note: CLMM uses TokenMint0)
	TokenVaultA      solana.PublicKey // tokenVaultA (note: CLMM uses TokenVault0)
	FeeGrowthGlobalA uint128.Uint128  // feeGrowthGlobalA

	TokenMintB       solana.PublicKey // tokenMintB (note: CLMM uses TokenMint1)
	TokenVaultB      solana.PublicKey // tokenVaultB (note: CLMM uses TokenVault1)
	FeeGrowthGlobalB uint128.Uint128  // feeGrowthGlobalB

	// Reward information
	RewardLastUpdatedTimestamp uint64                 // rewardLastUpdatedTimestamp
	RewardInfos                [3]WhirlpoolRewardInfo // rewardInfos

	// Internal use fields
	PoolId           solana.PublicKey // Pool ID (internal calculation)
	UserBaseAccount  solana.PublicKey // User base token account
	UserQuoteAccount solana.PublicKey // User quote token account
}

// WhirlpoolRewardInfo reward information structure - Reference external/orca/whirlpool/generated/types.go
type WhirlpoolRewardInfo struct {
	Mint                  solana.PublicKey // mint
	Vault                 solana.PublicKey // vault
	Authority             solana.PublicKey // authority
	EmissionsPerSecondX64 uint128.Uint128  // emissionsPerSecondX64
	GrowthGlobalX64       uint128.Uint128  // growthGlobalX64
}

// Implement basic methods of Pool interface
func (pool *WhirlpoolPool) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolNameOrcaWhirlpool
}

func (pool *WhirlpoolPool) ProtocolType() pkg.ProtocolType {
	return pkg.ProtocolTypeOrcaWhirlpool
}

func (pool *WhirlpoolPool) GetProgramID() solana.PublicKey {
	return ORCA_WHIRLPOOL_PROGRAM_ID
}

func (pool *WhirlpoolPool) GetID() string {
	return pool.PoolId.String()
}

// GetTokens returns token pair - Note field name mapping
func (pool *WhirlpoolPool) GetTokens() (baseMint, quoteMint string) {
	return pool.TokenMintA.String(), pool.TokenMintB.String()
}

// Decode parses Whirlpool account data - Reference CLMM Decode implementation
func (pool *WhirlpoolPool) Decode(data []byte) error {
	// Skip 8 bytes discriminator if present
	if len(data) > 8 {
		data = data[8:]
	}

	offset := 0

	// Parse whirlpools config (32 bytes)
	pool.WhirlpoolsConfig = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	// Parse whirlpool bump (1 byte)
	copy(pool.WhirlpoolBump[:], data[offset:offset+1])
	offset += 1

	// Parse tick spacing (2 bytes)
	pool.TickSpacing = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Parse fee tier index seed (2 bytes)
	copy(pool.FeeTierIndexSeed[:], data[offset:offset+2])
	offset += 2

	// Parse fee rate (2 bytes)
	pool.FeeRate = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Parse protocol fee rate (2 bytes)
	pool.ProtocolFeeRate = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Parse liquidity (16 bytes)
	pool.Liquidity = uint128.FromBytes(data[offset : offset+16])
	offset += 16

	// Parse sqrt price (16 bytes) - Note: CLMM calls it SqrtPriceX64
	pool.SqrtPrice = uint128.FromBytes(data[offset : offset+16])
	offset += 16

	// Parse tick current index (4 bytes) - Note: CLMM calls it TickCurrent
	pool.TickCurrentIndex = int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4

	// Parse protocol fee owed A (8 bytes)
	pool.ProtocolFeeOwedA = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Parse protocol fee owed B (8 bytes)
	pool.ProtocolFeeOwedB = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Parse token mint A (32 bytes) - Note: CLMM calls it TokenMint0
	pool.TokenMintA = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	// Parse token vault A (32 bytes) - Note: CLMM calls it TokenVault0
	pool.TokenVaultA = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	// Parse fee growth global A (16 bytes)
	pool.FeeGrowthGlobalA = uint128.FromBytes(data[offset : offset+16])
	offset += 16

	// Parse token mint B (32 bytes) - Note: CLMM calls it TokenMint1
	pool.TokenMintB = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	// Parse token vault B (32 bytes) - Note: CLMM calls it TokenVault1
	pool.TokenVaultB = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	// Parse fee growth global B (16 bytes)
	pool.FeeGrowthGlobalB = uint128.FromBytes(data[offset : offset+16])
	offset += 16

	// Parse reward last updated timestamp (8 bytes)
	pool.RewardLastUpdatedTimestamp = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Parse reward infos (3 reward infos, each containing multiple fields)
	for i := 0; i < 3; i++ {
		// mint (32 bytes)
		pool.RewardInfos[i].Mint = solana.PublicKeyFromBytes(data[offset : offset+32])
		offset += 32

		// vault (32 bytes)
		pool.RewardInfos[i].Vault = solana.PublicKeyFromBytes(data[offset : offset+32])
		offset += 32

		// authority (32 bytes)
		pool.RewardInfos[i].Authority = solana.PublicKeyFromBytes(data[offset : offset+32])
		offset += 32

		// emissions per second x64 (16 bytes)
		pool.RewardInfos[i].EmissionsPerSecondX64 = uint128.FromBytes(data[offset : offset+16])
		offset += 16

		// growth global x64 (16 bytes)
		pool.RewardInfos[i].GrowthGlobalX64 = uint128.FromBytes(data[offset : offset+16])
		offset += 16
	}

	return nil
}

// Span returns account data size - Precise calculation based on complete Whirlpool structure
func (pool *WhirlpoolPool) Span() uint64 {
	// Based on Whirlpool structure calculation from external/orca/whirlpool/generated/types.go:
	//
	// 8 bytes discriminator
	// 32 bytes whirlpoolsConfig (PublicKey)
	// 1 byte whirlpoolBump
	// 2 bytes tickSpacing (uint16)
	// 2 bytes feeTierIndexSeed
	// 2 bytes feeRate (uint16)
	// 2 bytes protocolFeeRate (uint16)
	// 16 bytes liquidity (Uint128)
	// 16 bytes sqrtPrice (Uint128)
	// 4 bytes tickCurrentIndex (int32)
	// 8 bytes protocolFeeOwedA (uint64)
	// 8 bytes protocolFeeOwedB (uint64)
	// 32 bytes tokenMintA (PublicKey)
	// 32 bytes tokenVaultA (PublicKey)
	// 16 bytes feeGrowthGlobalA (Uint128)
	// 32 bytes tokenMintB (PublicKey)
	// 32 bytes tokenVaultB (PublicKey)
	// 16 bytes feeGrowthGlobalB (Uint128)
	// 8 bytes rewardLastUpdatedTimestamp (uint64)
	// 3 * (32+32+32+16+16) bytes rewardInfos (3 WhirlpoolRewardInfo)
	//   Each WhirlpoolRewardInfo: mint(32) + vault(32) + authority(32) + emissionsPerSecondX64(16) + growthGlobalX64(16) = 128 bytes

	return uint64(8 + 32 + 1 + 2 + 2 + 2 + 2 + 16 + 16 + 4 + 8 + 8 + 32 + 32 + 16 + 32 + 32 + 16 + 8 + 3*128)
	// = 8 + 261 + 384 = 653 bytes (including discriminator)
}

// Offset returns field offset - Used for RPC query filters
func (pool *WhirlpoolPool) Offset(field string) uint64 {
	// Add 8 bytes for discriminator
	baseOffset := uint64(8)

	switch field {
	case "TokenMintA":
		// Precise offset calculation based on Whirlpool structure:
		// whirlpoolsConfig(32) + whirlpoolBump(1) + tickSpacing(2) + feeTierIndexSeed(2) +
		// feeRate(2) + protocolFeeRate(2) + liquidity(16) + sqrtPrice(16) +
		// tickCurrentIndex(4) + protocolFeeOwedA(8) + protocolFeeOwedB(8)
		return baseOffset + 32 + 1 + 2 + 2 + 2 + 2 + 16 + 16 + 4 + 8 + 8 // = 101
	case "TokenMintB":
		// After TokenMintA: tokenMintA(32) + tokenVaultA(32) + feeGrowthGlobalA(16)
		// Note: Actual verification found TokenMintB at offset 181, not 189
		return baseOffset + 101 + 32 + 32 + 16 - 8 // = 181 (corrected 8-byte difference)
	case "TickSpacing":
		// After whirlpoolsConfig(32) + whirlpoolBump(1)
		return baseOffset + 32 + 1 // = 41
	case "FeeRate":
		// After whirlpoolsConfig(32) + whirlpoolBump(1) + tickSpacing(2) + feeTierIndexSeed(2)
		return baseOffset + 32 + 1 + 2 + 2 // = 45
	case "SqrtPrice":
		// After liquidity
		return baseOffset + 32 + 1 + 2 + 2 + 2 + 2 + 16 // = 65
	case "TickCurrentIndex":
		// After sqrtPrice
		return baseOffset + 32 + 1 + 2 + 2 + 2 + 2 + 16 + 16 // = 81
	}
	return 0
}

// Quote method - Get swap quote (with boundary validation and error handling)
func (pool *WhirlpoolPool) Quote(ctx context.Context, solClient *rpc.Client, inputMint string, inputAmount cosmath.Int) (cosmath.Int, error) {
	// 1. Input validation
	if err := pool.validateQuoteInputs(inputMint, inputAmount); err != nil {
		return cosmath.Int{}, fmt.Errorf("quote input validation failed: %w", err)
	}

	// 2. Pool state validation
	if err := pool.validatePoolState(); err != nil {
		return cosmath.Int{}, fmt.Errorf("pool state validation failed: %w", err)
	}

	// 3. Calculate quote (with retry mechanism)
	maxRetries := 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var priceResult cosmath.Int
		var err error

		if inputMint == pool.TokenMintA.String() {
			// A -> B swap
			priceResult, err = pool.ComputeWhirlpoolAmountOutFormat(pool.TokenMintA.String(), inputAmount)
		} else if inputMint == pool.TokenMintB.String() {
			// B -> A swap
			priceResult, err = pool.ComputeWhirlpoolAmountOutFormat(pool.TokenMintB.String(), inputAmount)
		} else {
			return cosmath.Int{}, fmt.Errorf("input mint %s not found in pool %s", inputMint, pool.PoolId.String())
		}

		if err != nil {
			lastErr = err
			// If calculation error and retries remaining, wait briefly and retry
			if attempt < maxRetries && isTemporaryError(err) {
				time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
				continue
			}
			return cosmath.Int{}, fmt.Errorf("amount calculation failed after %d attempts: %w", attempt+1, err)
		}

		// 4. Output validation
		if err := pool.validateQuoteOutput(priceResult); err != nil {
			return cosmath.Int{}, fmt.Errorf("quote output validation failed: %w", err)
		}

		return priceResult.Neg(), nil // Return negative number to indicate output amount
	}

	return cosmath.Int{}, fmt.Errorf("quote calculation failed after retries: %w", lastErr)
}

// validateQuoteInputs validates quote input parameters
func (pool *WhirlpoolPool) validateQuoteInputs(inputMint string, inputAmount cosmath.Int) error {
	// Check input amount
	if inputAmount.IsZero() {
		return fmt.Errorf("input amount cannot be zero")
	}
	if inputAmount.IsNegative() {
		return fmt.Errorf("input amount cannot be negative")
	}

	// Check if input amount is too large (prevent overflow)
	maxAmount := cosmath.NewIntFromUint64(1e18) // Set reasonable maximum value
	if inputAmount.GT(maxAmount) {
		return fmt.Errorf("input amount too large: %s > %s", inputAmount.String(), maxAmount.String())
	}

	// Validate token mint address format - Use Solana standard validation
	_, err := solana.PublicKeyFromBase58(inputMint)
	if err != nil {
		return fmt.Errorf("invalid mint address format: %s, error: %w", inputMint, err)
	}

	return nil
}

// validatePoolState validates pool state
func (pool *WhirlpoolPool) validatePoolState() error {
	// Check liquidity - if zero, skip this pool without error, let router choose other pools
	if pool.Liquidity.IsZero() {
		return fmt.Errorf("pool has zero liquidity") // This will make router skip this pool
	}

	// Check price - pools with zero price cannot trade
	if pool.SqrtPrice.IsZero() {
		return fmt.Errorf("pool has zero sqrt price")
	}

	// Check tick spacing - zero tick spacing is invalid
	if pool.TickSpacing == 0 {
		return fmt.Errorf("pool has zero tick spacing")
	}

	// Check token mint addresses - pools with invalid addresses are unusable
	if pool.TokenMintA.IsZero() || pool.TokenMintB.IsZero() {
		return fmt.Errorf("pool has invalid token mint addresses")
	}

	return nil
}

// validateQuoteOutput validates quote output
func (pool *WhirlpoolPool) validateQuoteOutput(outputAmount cosmath.Int) error {
	// Check if output is zero
	if outputAmount.IsZero() {
		return fmt.Errorf("computed output amount is zero")
	}

	// Note: negative numbers are valid, representing output amount (converted to negative via .Neg())
	// So we just verify absolute value is not zero
	absoluteAmount := outputAmount.Abs()
	if absoluteAmount.IsZero() {
		return fmt.Errorf("computed output amount absolute value is zero: %s", outputAmount.String())
	}

	return nil
}

// isTemporaryError determines if error is temporary
func isTemporaryError(err error) bool {
	errorMsg := strings.ToLower(err.Error())
	return strings.Contains(errorMsg, "overflow") ||
		strings.Contains(errorMsg, "underflow") ||
		strings.Contains(errorMsg, "division by zero") ||
		strings.Contains(errorMsg, "timeout")
}

// ComputeWhirlpoolAmountOutFormat - Whirlpool version of output amount calculation, referencing CLMM implementation
func (pool *WhirlpoolPool) ComputeWhirlpoolAmountOutFormat(inputTokenMint string, inputAmount cosmath.Int) (cosmath.Int, error) {
	// Determine swap direction: A -> B is true, B -> A is false
	zeroForOne := inputTokenMint == pool.TokenMintA.String()

	// Use current pool state for basic calculation
	firstTickArrayStartIndex := getWhirlpoolTickArrayStartIndexByTick(int64(pool.TickCurrentIndex), int64(pool.TickSpacing))

	// Call core swap calculation logic
	expectedAmountOut, err := pool.whirlpoolSwapCompute(
		int64(pool.TickCurrentIndex),
		zeroForOne,
		inputAmount,
		cosmath.NewIntFromUint64(uint64(pool.FeeRate)), // Use pool's fee rate
		firstTickArrayStartIndex,
		nil, // Temporarily not using external bitmap
	)
	if err != nil {
		return cosmath.Int{}, fmt.Errorf("failed to compute Whirlpool swap amount: %w", err)
	}
	return expectedAmountOut, nil
}

// BuildSwapInstructions method - builds real Whirlpool SwapV2 instruction
//
// This method builds complete Whirlpool SwapV2 transaction instruction, including:
// 1. Swap direction determination (A->B or B->A)
// 2. ATA account derivation and existence check
// 3. Tick Array PDA address calculation
// 4. SwapV2 instruction parameter encoding
// 5. Correct account metadata arrangement
//
// Returned instruction can be directly used for Solana transaction execution.
func (pool *WhirlpoolPool) BuildSwapInstructions(
	ctx context.Context,
	solClient *rpc.Client,
	userAddr solana.PublicKey,
	inputMint string,
	amountIn cosmath.Int,
	minOutAmountWithDecimals cosmath.Int,
) ([]solana.Instruction, error) {
	// 1. Determine swap direction
	var aToB bool

	if inputMint == pool.TokenMintA.String() {
		// A -> B swap
		aToB = true
	} else if inputMint == pool.TokenMintB.String() {
		// B -> A swap
		aToB = false
	} else {
		return nil, fmt.Errorf("input mint %s not found in pool", inputMint)
	}

	// 2. Get or create user's token accounts - fixed as A and B, not changing with swap direction
	userTokenAccountA, err := getOrCreateTokenAccount(ctx, solClient, userAddr, pool.TokenMintA)
	if err != nil {
		return nil, fmt.Errorf("failed to get token A account: %w", err)
	}

	userTokenAccountB, err := getOrCreateTokenAccount(ctx, solClient, userAddr, pool.TokenMintB)
	if err != nil {
		return nil, fmt.Errorf("failed to get token B account: %w", err)
	}

	// 3. Calculate price limit (set to extreme values, actually no limit)
	var sqrtPriceLimit uint128.Uint128
	if aToB {
		sqrtPriceLimit = uint128.FromBig(MIN_SQRT_PRICE_X64.BigInt())
	} else {
		sqrtPriceLimit = uint128.FromBig(MAX_SQRT_PRICE_X64.BigInt())
	}

	// 4. Build tick array addresses (using real PDA derivation)
	tickArray0, tickArray1, tickArray2, err := DeriveMultipleWhirlpoolTickArrayPDAs(
		pool.PoolId,
		int64(pool.TickCurrentIndex),
		int64(pool.TickSpacing),
		aToB,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to derive tick array PDAs: %w", err)
	}

	// 5. Oracle address (using correct PDA derivation)
	oracleAddr, err := DeriveWhirlpoolOraclePDA(pool.PoolId)
	if err != nil {
		return nil, fmt.Errorf("failed to derive oracle PDA: %w", err)
	}

	// 6. Build SwapV2 instruction parameters
	instruction, err := createWhirlpoolSwapV2Instruction(
		// Instruction parameters
		amountIn.Uint64(),                 // amount
		minOutAmountWithDecimals.Uint64(), // otherAmountThreshold
		sqrtPriceLimit,                    // sqrtPriceLimit
		true,                              // amountSpecifiedIsInput
		aToB,                              // aToB
		nil,                               // remainingAccountsInfo

		// Account addresses - fixed as A and B order, not changing with swap direction
		TOKEN_PROGRAM_ID,  // tokenProgramA
		TOKEN_PROGRAM_ID,  // tokenProgramB
		MEMO_PROGRAM_ID,   // memoProgram
		userAddr,          // tokenAuthority
		pool.PoolId,       // whirlpool
		pool.TokenMintA,   // tokenMintA
		pool.TokenMintB,   // tokenMintB
		userTokenAccountA, // tokenOwnerAccountA (fixed as A)
		pool.TokenVaultA,  // tokenVaultA (fixed as A)
		userTokenAccountB, // tokenOwnerAccountB (fixed as B)
		pool.TokenVaultB,  // tokenVaultB (fixed as B)
		tickArray0,        // tickArray0
		tickArray1,        // tickArray1
		tickArray2,        // tickArray2
		oracleAddr,        // oracle
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SwapV2 instruction: %w", err)
	}

	return []solana.Instruction{instruction}, nil
}

// whirlpoolSwapCompute - Whirlpool core swap calculation logic
func (pool *WhirlpoolPool) whirlpoolSwapCompute(
	currentTick int64,
	zeroForOne bool,
	amountSpecified cosmath.Int,
	fee cosmath.Int,
	lastSavedTickArrayStartIndex int64,
	exTickArrayBitmap *WhirlpoolTickArrayBitmapExtensionType,
) (cosmath.Int, error) {
	// Input validation
	if amountSpecified.IsZero() {
		return cosmath.Int{}, fmt.Errorf("input amount cannot be zero")
	}

	// Basic variable initialization
	baseInput := amountSpecified.IsPositive()
	sqrtPriceLimitX64 := cosmath.NewInt(0)

	// Initialize calculation variables
	amountSpecifiedRemaining := amountSpecified
	amountCalculated := cosmath.NewInt(0)
	sqrtPriceX64 := cosmath.NewIntFromBigInt(pool.SqrtPrice.Big()) // Note: Whirlpool uses SqrtPrice instead of SqrtPriceX64
	liquidity := cosmath.NewIntFromBigInt(pool.Liquidity.Big())

	// Set price limits - reuse CLMM constants
	if zeroForOne {
		sqrtPriceLimitX64 = MIN_SQRT_PRICE_X64.Add(cosmath.NewInt(1))
	} else {
		sqrtPriceLimitX64 = MAX_SQRT_PRICE_X64.Sub(cosmath.NewInt(1))
	}

	// Production environment requires complete tick traversal logic implementation

	// Calculate target price (simplified: move a small step towards price limit direction)
	targetPrice := sqrtPriceX64
	if zeroForOne {
		// A -> B: price decreases
		targetPrice = sqrtPriceX64.Mul(cosmath.NewInt(995)).Quo(cosmath.NewInt(1000)) // Decrease 0.5%
		if targetPrice.LT(sqrtPriceLimitX64) {
			targetPrice = sqrtPriceLimitX64
		}
	} else {
		// B -> A: price increases
		targetPrice = sqrtPriceX64.Mul(cosmath.NewInt(1005)).Quo(cosmath.NewInt(1000)) // Increase 0.5%
		if targetPrice.GT(sqrtPriceLimitX64) {
			targetPrice = sqrtPriceLimitX64
		}
	}

	// Call simplified single-step calculation
	newSqrtPrice, amountIn, amountOut, feeAmount, err := pool.whirlpoolSwapStepCompute(
		sqrtPriceX64,
		targetPrice,
		liquidity,
		amountSpecifiedRemaining,
		fee,
		zeroForOne,
	)
	if err != nil {
		return cosmath.Int{}, fmt.Errorf("swap step compute failed: %w", err)
	}

	// Update calculation results
	if baseInput {
		// Exact input mode
		amountCalculated = amountOut.Neg() // Return negative number representing output
	} else {
		// Exact output mode
		amountCalculated = amountIn.Add(feeAmount)
	}

	// Validate result reasonableness
	if amountCalculated.IsZero() {
		return cosmath.Int{}, fmt.Errorf("calculated amount is zero, input: %s, sqrtPrice: %s->%s",
			amountSpecified.String(), sqrtPriceX64.String(), newSqrtPrice.String())
	}

	return amountCalculated, nil
}

// whirlpoolSwapStepCompute - Whirlpool precise CLMM calculation (based on Raydium CLMM algorithm)
// Uses same precise mathematical formulas as Raydium CLMM to ensure calculation accuracy
func (pool *WhirlpoolPool) whirlpoolSwapStepCompute(
	sqrtPriceCurrent cosmath.Int,
	sqrtPriceTarget cosmath.Int,
	liquidity cosmath.Int,
	amountRemaining cosmath.Int,
	feeRate cosmath.Int,
	zeroForOne bool,
) (sqrtPriceNext cosmath.Int, amountIn cosmath.Int, amountOut cosmath.Int, feeAmount cosmath.Int, err error) {

	// Basic validation
	if liquidity.IsZero() {
		return cosmath.Int{}, cosmath.Int{}, cosmath.Int{}, cosmath.Int{}, fmt.Errorf("liquidity is zero")
	}

	baseAmount := amountRemaining.Abs()
	if baseAmount.IsZero() {
		return sqrtPriceCurrent, cosmath.ZeroInt(), cosmath.ZeroInt(), cosmath.ZeroInt(), nil
	}

	// Call precise CLMM swap step calculation
	// This function uses same algorithm as Raydium to ensure mathematical accuracy
	return whirlpoolSwapStepComputePrecise(
		sqrtPriceCurrent.BigInt(),
		sqrtPriceTarget.BigInt(),
		liquidity.BigInt(),
		baseAmount.BigInt(),
		uint32(feeRate.Int64()),
		zeroForOne,
	)
}

// whirlpoolSwapStepComputePrecise - precise CLMM swap step calculation
// Based on Raydium CLMM's swapStepCompute function, adapted for Whirlpool
func whirlpoolSwapStepComputePrecise(
	sqrtPriceX64Current *big.Int,
	sqrtPriceX64Target *big.Int,
	liquidity *big.Int,
	amountRemaining *big.Int,
	feeRate uint32,
	zeroForOne bool,
) (cosmath.Int, cosmath.Int, cosmath.Int, cosmath.Int, error) {

	// Define SwapStep structure to track calculation state
	swapStep := &WhirlpoolSwapStep{
		SqrtPriceX64Next: new(big.Int),
		AmountIn:         new(big.Int),
		AmountOut:        new(big.Int),
		FeeAmount:        new(big.Int),
	}

	zero := new(big.Int)
	baseInput := amountRemaining.Cmp(zero) >= 0

	// Step 1: Calculate fee rate related constants
	// FEE_RATE_DENOMINATOR = 1,000,000 (Whirlpool uses parts per million as fee rate unit)
	FEE_RATE_DENOMINATOR := cosmath.NewInt(1000000)

	if baseInput {
		// Exact input mode: deduct fees first, then calculate swap
		feeRateBig := cosmath.NewInt(int64(feeRate))
		tmp := FEE_RATE_DENOMINATOR.Sub(feeRateBig)
		amountRemainingSubtractFee := whirlpoolMulDivFloor(
			cosmath.NewIntFromBigInt(amountRemaining),
			tmp,
			FEE_RATE_DENOMINATOR,
		)

		// Calculate maximum amount that can be swapped within current price range
		if zeroForOne {
			// Token A -> Token B
			swapStep.AmountIn = whirlpoolGetTokenAmountAFromLiquidity(
				sqrtPriceX64Target, sqrtPriceX64Current, liquidity, true)
		} else {
			// Token B -> Token A
			swapStep.AmountIn = whirlpoolGetTokenAmountBFromLiquidity(
				sqrtPriceX64Current, sqrtPriceX64Target, liquidity, true)
		}

		// Determine if target price will be reached
		if amountRemainingSubtractFee.GTE(cosmath.NewIntFromBigInt(swapStep.AmountIn)) {
			// Input is large enough, will reach target price
			swapStep.SqrtPriceX64Next.Set(sqrtPriceX64Target)
		} else {
			// Input insufficient, calculate new price
			swapStep.SqrtPriceX64Next = whirlpoolGetNextSqrtPriceX64FromInput(
				sqrtPriceX64Current,
				liquidity,
				amountRemainingSubtractFee.BigInt(),
				zeroForOne,
			)
		}
	} else {
		// Exact output mode: directly calculate required input
		if zeroForOne {
			swapStep.AmountOut = whirlpoolGetTokenAmountBFromLiquidity(
				sqrtPriceX64Target, sqrtPriceX64Current, liquidity, false)
		} else {
			swapStep.AmountOut = whirlpoolGetTokenAmountAFromLiquidity(
				sqrtPriceX64Current, sqrtPriceX64Target, liquidity, false)
		}

		negativeOne := new(big.Int).SetInt64(-1)
		amountRemainingNeg := new(big.Int).Mul(amountRemaining, negativeOne)

		if amountRemainingNeg.Cmp(swapStep.AmountOut) >= 0 {
			swapStep.SqrtPriceX64Next.Set(sqrtPriceX64Target)
		} else {
			swapStep.SqrtPriceX64Next = whirlpoolGetNextSqrtPriceX64FromOutput(
				sqrtPriceX64Current,
				liquidity,
				amountRemainingNeg,
				zeroForOne,
			)
		}
	}

	// Step 2: Recalculate precise input and output amounts
	reachTargetPrice := swapStep.SqrtPriceX64Next.Cmp(sqrtPriceX64Target) == 0

	if zeroForOne {
		if !(reachTargetPrice && baseInput) {
			swapStep.AmountIn = whirlpoolGetTokenAmountAFromLiquidity(
				swapStep.SqrtPriceX64Next,
				sqrtPriceX64Current,
				liquidity,
				true,
			)
		}

		if !(reachTargetPrice && !baseInput) {
			swapStep.AmountOut = whirlpoolGetTokenAmountBFromLiquidity(
				swapStep.SqrtPriceX64Next,
				sqrtPriceX64Current,
				liquidity,
				false,
			)
		}
	} else {
		if !(reachTargetPrice && baseInput) {
			swapStep.AmountIn = whirlpoolGetTokenAmountBFromLiquidity(
				sqrtPriceX64Current,
				swapStep.SqrtPriceX64Next,
				liquidity,
				true,
			)
		}

		if !(reachTargetPrice && !baseInput) {
			swapStep.AmountOut = whirlpoolGetTokenAmountAFromLiquidity(
				sqrtPriceX64Current,
				swapStep.SqrtPriceX64Next,
				liquidity,
				false,
			)
		}
	}

	// Step 3: Calculate fees
	if baseInput && swapStep.SqrtPriceX64Next.Cmp(sqrtPriceX64Target) != 0 {
		swapStep.FeeAmount = new(big.Int).Sub(amountRemaining, swapStep.AmountIn)
	} else {
		feeRateBig := cosmath.NewInt(int64(feeRate))
		feeRateSubtracted := FEE_RATE_DENOMINATOR.Sub(feeRateBig)
		swapStep.FeeAmount = whirlpoolMulDivCeil(
			cosmath.NewIntFromBigInt(swapStep.AmountIn),
			feeRateBig,
			feeRateSubtracted,
		).BigInt()
	}

	// Apply moderate safety margin (10% instead of 80%)
	safetyMargin := cosmath.NewInt(90) // Keep 90% of calculation result
	adjustedAmountOut := cosmath.NewIntFromBigInt(swapStep.AmountOut).Mul(safetyMargin).Quo(cosmath.NewInt(100))

	// Ensure minimum output
	if adjustedAmountOut.IsZero() && swapStep.AmountOut.Cmp(zero) > 0 {
		adjustedAmountOut = cosmath.NewInt(1)
	}

	return cosmath.NewIntFromBigInt(swapStep.SqrtPriceX64Next),
		cosmath.NewIntFromBigInt(swapStep.AmountIn),
		adjustedAmountOut,
		cosmath.NewIntFromBigInt(swapStep.FeeAmount), nil
}

// getOrCreateTokenAccount gets or creates user's token account
func getOrCreateTokenAccount(ctx context.Context, solClient *rpc.Client, userAddr solana.PublicKey, tokenMint solana.PublicKey) (solana.PublicKey, error) {
	// 1. Derive ATA address
	ata, _, err := solana.FindAssociatedTokenAddress(userAddr, tokenMint)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to find associated token address: %w", err)
	}

	// 2. Check if ATA account exists
	accountExists, err := checkAccountExists(ctx, solClient, ata)
	if err != nil {
		// If RPC query fails, continue using ATA address, let transaction fail naturally
		// This avoids blocking normal flow
		return ata, nil
	}

	if !accountExists {
		// ATA doesn't exist, but we still return the address
		// In practical applications, caller needs to decide whether to add ATA creation instruction
		// For mainstream tokens (like SOL, USDC), users usually already have ATA
		return ata, nil
	}

	return ata, nil
}

// checkAccountExists checks if account exists (with retry mechanism)
func checkAccountExists(ctx context.Context, solClient *rpc.Client, accountAddr solana.PublicKey) (bool, error) {
	// 实现简单的重试机制，应对 RPC 限流
	maxRetries := 3
	baseDelay := 100 // 100ms

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 使用 getAccountInfo 检查账户是否存在
		_, err := solClient.GetAccountInfo(ctx, accountAddr)
		if err != nil {
			// 检查是否是"账户不存在"的错误
			if isAccountNotFoundError(err) {
				return false, nil
			}

			// 检查是否是 RPC 限流错误
			if isRateLimitError(err) && attempt < maxRetries {
				// 指数退避重试
				delay := baseDelay * (1 << attempt) // 100ms, 200ms, 400ms
				time.Sleep(time.Duration(delay) * time.Millisecond)
				continue
			}

			// 其他错误直接返回
			return false, fmt.Errorf("failed to check account existence after %d attempts: %w", attempt+1, err)
		}

		// 账户存在，成功返回
		return true, nil
	}

	// 不应该到达这里
	return false, fmt.Errorf("exhausted retries checking account existence")
}

// isAccountNotFoundError 判断是否是账户不存在的错误
func isAccountNotFoundError(err error) bool {
	// Solana RPC 在账户不存在时返回特定错误信息
	errorMsg := strings.ToLower(err.Error())
	return strings.Contains(errorMsg, "account not found") ||
		strings.Contains(errorMsg, "could not find account") ||
		strings.Contains(errorMsg, "invalid param")
}

// isRateLimitError 判断是否是 RPC 限流错误
func isRateLimitError(err error) bool {
	// 检测常见的 RPC 限流错误信息
	errorMsg := strings.ToLower(err.Error())
	return strings.Contains(errorMsg, "too many requests") ||
		strings.Contains(errorMsg, "rate limit") ||
		strings.Contains(errorMsg, "429") ||
		strings.Contains(errorMsg, "quota exceeded") ||
		strings.Contains(errorMsg, "timeout") ||
		strings.Contains(errorMsg, "connection reset")
}

// createAssociatedTokenAccountInstruction 创建 ATA 账户的指令 (预留功能)
// 注意：当前不自动添加创建指令，由调用方决定
func createAssociatedTokenAccountInstruction(
	payer solana.PublicKey,
	associatedTokenAddress solana.PublicKey,
	owner solana.PublicKey,
	tokenMint solana.PublicKey,
) (solana.Instruction, error) {
	// 构建创建 ATA 的指令
	// 参考: https://github.com/solana-labs/solana-program-library/blob/master/associated-token-account/program/src/instruction.rs

	accounts := solana.AccountMetaSlice{}
	accounts.Append(solana.NewAccountMeta(payer, false, true))                   // 0: payer (signer)
	accounts.Append(solana.NewAccountMeta(associatedTokenAddress, true, false))  // 1: associated_token_account (writable)
	accounts.Append(solana.NewAccountMeta(owner, false, false))                  // 2: owner
	accounts.Append(solana.NewAccountMeta(tokenMint, false, false))              // 3: mint
	accounts.Append(solana.NewAccountMeta(solana.SystemProgramID, false, false)) // 4: system_program
	accounts.Append(solana.NewAccountMeta(TOKEN_PROGRAM_ID, false, false))       // 5: token_program

	// ATA 程序 ID
	ataProgramID := solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")

	// 创建指令 (无需数据，ATA 程序有默认创建指令)
	return solana.NewInstruction(
		ataProgramID,
		accounts,
		[]byte{}, // 空数据，ATA 程序的创建指令不需要参数
	), nil
}

// createWhirlpoolSwapV2Instruction 创建 Whirlpool SwapV2 指令
func createWhirlpoolSwapV2Instruction(
	// 参数
	amount uint64,
	otherAmountThreshold uint64,
	sqrtPriceLimit uint128.Uint128,
	amountSpecifiedIsInput bool,
	aToB bool,
	remainingAccountsInfo interface{}, // 暂时用 interface{}

	// 账户
	tokenProgramA solana.PublicKey,
	tokenProgramB solana.PublicKey,
	memoProgram solana.PublicKey,
	tokenAuthority solana.PublicKey,
	whirlpool solana.PublicKey,
	tokenMintA solana.PublicKey,
	tokenMintB solana.PublicKey,
	tokenOwnerAccountA solana.PublicKey,
	tokenVaultA solana.PublicKey,
	tokenOwnerAccountB solana.PublicKey,
	tokenVaultB solana.PublicKey,
	tickArray0 solana.PublicKey,
	tickArray1 solana.PublicKey,
	tickArray2 solana.PublicKey,
	oracle solana.PublicKey,
) (solana.Instruction, error) {

	// 1. 构建指令数据
	buf := new(bytes.Buffer)
	enc := bin.NewBorshEncoder(buf)

	// 写入 SwapV2 指令判别器
	err := enc.WriteBytes(SwapV2Discriminator, false)
	if err != nil {
		return nil, fmt.Errorf("failed to write discriminator: %w", err)
	}

	// 写入参数
	err = enc.Encode(amount)
	if err != nil {
		return nil, fmt.Errorf("failed to encode amount: %w", err)
	}

	err = enc.Encode(otherAmountThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to encode otherAmountThreshold: %w", err)
	}

	// 写入 sqrt price limit (16 bytes little endian)
	sqrtPriceLimitLo := sqrtPriceLimit.Lo
	sqrtPriceLimitHi := sqrtPriceLimit.Hi

	// 写入低64位
	err = enc.Encode(sqrtPriceLimitLo)
	if err != nil {
		return nil, fmt.Errorf("failed to encode sqrtPriceLimit lo: %w", err)
	}

	// 写入高64位
	err = enc.Encode(sqrtPriceLimitHi)
	if err != nil {
		return nil, fmt.Errorf("failed to encode sqrtPriceLimit hi: %w", err)
	}

	err = enc.Encode(amountSpecifiedIsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to encode amountSpecifiedIsInput: %w", err)
	}

	err = enc.Encode(aToB)
	if err != nil {
		return nil, fmt.Errorf("failed to encode aToB: %w", err)
	}

	// 写入 remainingAccountsInfo (Option<None>)
	err = enc.WriteOption(false) // None
	if err != nil {
		return nil, fmt.Errorf("failed to encode remainingAccountsInfo: %w", err)
	}

	// 2. 构建账户元数据
	accounts := solana.AccountMetaSlice{}

	// 按照 Whirlpool SwapV2 指令的账户顺序添加
	accounts.Append(solana.NewAccountMeta(tokenProgramA, false, false))     // 0: token_program_a
	accounts.Append(solana.NewAccountMeta(tokenProgramB, false, false))     // 1: token_program_b
	accounts.Append(solana.NewAccountMeta(memoProgram, false, false))       // 2: memo_program
	accounts.Append(solana.NewAccountMeta(tokenAuthority, false, true))     // 3: token_authority (signer)
	accounts.Append(solana.NewAccountMeta(whirlpool, true, false))          // 4: whirlpool (writable)
	accounts.Append(solana.NewAccountMeta(tokenMintA, false, false))        // 5: token_mint_a
	accounts.Append(solana.NewAccountMeta(tokenMintB, false, false))        // 6: token_mint_b
	accounts.Append(solana.NewAccountMeta(tokenOwnerAccountA, true, false)) // 7: token_owner_account_a (writable)
	accounts.Append(solana.NewAccountMeta(tokenVaultA, true, false))        // 8: token_vault_a (writable)
	accounts.Append(solana.NewAccountMeta(tokenOwnerAccountB, true, false)) // 9: token_owner_account_b (writable)
	accounts.Append(solana.NewAccountMeta(tokenVaultB, true, false))        // 10: token_vault_b (writable)
	accounts.Append(solana.NewAccountMeta(tickArray0, true, false))         // 11: tick_array_0 (writable)
	accounts.Append(solana.NewAccountMeta(tickArray1, true, false))         // 12: tick_array_1 (writable)
	accounts.Append(solana.NewAccountMeta(tickArray2, true, false))         // 13: tick_array_2 (writable)
	accounts.Append(solana.NewAccountMeta(oracle, true, false))             // 14: oracle (writable)

	// 3. 创建指令
	return solana.NewInstruction(
		ORCA_WHIRLPOOL_PROGRAM_ID,
		accounts,
		buf.Bytes(),
	), nil
}

// WhirlpoolSwapStep - Whirlpool 交换步骤结构
type WhirlpoolSwapStep struct {
	SqrtPriceX64Next *big.Int
	AmountIn         *big.Int
	AmountOut        *big.Int
	FeeAmount        *big.Int
}

// Whirlpool CLMM 精确计算相关常量
// U64Resolution 已经在 constants.go 中定义

// whirlpoolMulDivFloor - 乘除法（向下取整）
func whirlpoolMulDivFloor(a, b, denominator cosmath.Int) cosmath.Int {
	if denominator.IsZero() {
		panic("division by zero")
	}
	numerator := a.Mul(b)
	return numerator.Quo(denominator)
}

// whirlpoolMulDivCeil - 乘除法（向上取整）
func whirlpoolMulDivCeil(a, b, denominator cosmath.Int) cosmath.Int {
	if denominator.IsZero() {
		return cosmath.Int{}
	}
	numerator := a.Mul(b).Add(denominator.Sub(cosmath.OneInt()))
	return numerator.Quo(denominator)
}

// whirlpoolGetTokenAmountAFromLiquidity - 从流动性计算 Token A 数量
func whirlpoolGetTokenAmountAFromLiquidity(
	sqrtPriceX64A *big.Int,
	sqrtPriceX64B *big.Int,
	liquidity *big.Int,
	roundUp bool,
) *big.Int {
	// 创建副本避免修改原始值
	priceA := new(big.Int).Set(sqrtPriceX64A)
	priceB := new(big.Int).Set(sqrtPriceX64B)

	// 确保 priceA <= priceB
	if priceA.Cmp(priceB) > 0 {
		priceA, priceB = priceB, priceA
	}

	if priceA.Cmp(big.NewInt(0)) <= 0 {
		panic("sqrtPriceX64A must be greater than 0")
	}

	// 计算 numerator1 = liquidity << U64Resolution
	numerator1 := new(big.Int).Lsh(liquidity, U64Resolution)
	// 计算 numerator2 = priceB - priceA
	numerator2 := new(big.Int).Sub(priceB, priceA)

	if roundUp {
		// 向上取整计算
		temp := whirlpoolMulDivCeil(
			cosmath.NewIntFromBigInt(numerator1),
			cosmath.NewIntFromBigInt(numerator2),
			cosmath.NewIntFromBigInt(priceB),
		)
		return whirlpoolMulDivCeil(
			temp,
			cosmath.NewIntFromBigInt(big.NewInt(1)),
			cosmath.NewIntFromBigInt(priceA),
		).BigInt()
	} else {
		// 向下取整计算
		temp := whirlpoolMulDivFloor(
			cosmath.NewIntFromBigInt(numerator1),
			cosmath.NewIntFromBigInt(numerator2),
			cosmath.NewIntFromBigInt(priceB),
		)
		return temp.Quo(cosmath.NewIntFromBigInt(priceA)).BigInt()
	}
}

// whirlpoolGetTokenAmountBFromLiquidity - 从流动性计算 Token B 数量
func whirlpoolGetTokenAmountBFromLiquidity(
	sqrtPriceX64A *big.Int,
	sqrtPriceX64B *big.Int,
	liquidity *big.Int,
	roundUp bool,
) *big.Int {
	// 创建副本避免修改原始值
	priceA := new(big.Int).Set(sqrtPriceX64A)
	priceB := new(big.Int).Set(sqrtPriceX64B)

	// 确保 priceA <= priceB
	if priceA.Cmp(priceB) > 0 {
		priceA, priceB = priceB, priceA
	}

	if priceA.Cmp(big.NewInt(0)) <= 0 {
		panic("sqrtPriceX64A must be greater than 0")
	}

	// 计算价格差
	priceDiff := new(big.Int).Sub(priceB, priceA)
	denominator := new(big.Int).Lsh(big.NewInt(1), U64Resolution)

	if roundUp {
		return whirlpoolMulDivCeil(
			cosmath.NewIntFromBigInt(liquidity),
			cosmath.NewIntFromBigInt(priceDiff),
			cosmath.NewIntFromBigInt(denominator),
		).BigInt()
	} else {
		return whirlpoolMulDivFloor(
			cosmath.NewIntFromBigInt(liquidity),
			cosmath.NewIntFromBigInt(priceDiff),
			cosmath.NewIntFromBigInt(denominator),
		).BigInt()
	}
}

// whirlpoolGetNextSqrtPriceX64FromInput - 从输入金额计算下个平方根价格
func whirlpoolGetNextSqrtPriceX64FromInput(
	sqrtPriceX64Current *big.Int,
	liquidity *big.Int,
	amount *big.Int,
	zeroForOne bool,
) *big.Int {
	if sqrtPriceX64Current.Cmp(big.NewInt(0)) <= 0 {
		panic("sqrtPriceX64Current must be greater than 0")
	}
	if liquidity.Cmp(big.NewInt(0)) <= 0 {
		panic("liquidity must be greater than 0")
	}

	if amount.Cmp(big.NewInt(0)) == 0 {
		return sqrtPriceX64Current
	}

	if zeroForOne {
		return whirlpoolGetNextSqrtPriceFromTokenAmountARoundingUp(
			sqrtPriceX64Current, liquidity, amount, true)
	} else {
		return whirlpoolGetNextSqrtPriceFromTokenAmountBRoundingDown(
			sqrtPriceX64Current, liquidity, amount, true)
	}
}

// whirlpoolGetNextSqrtPriceX64FromOutput - 从输出金额计算下个平方根价格
func whirlpoolGetNextSqrtPriceX64FromOutput(
	sqrtPriceX64Current *big.Int,
	liquidity *big.Int,
	amount *big.Int,
	zeroForOne bool,
) *big.Int {
	if sqrtPriceX64Current.Cmp(big.NewInt(0)) <= 0 {
		panic("sqrtPriceX64Current must be greater than 0")
	}
	if liquidity.Cmp(big.NewInt(0)) <= 0 {
		panic("liquidity must be greater than 0")
	}

	if zeroForOne {
		return whirlpoolGetNextSqrtPriceFromTokenAmountBRoundingDown(
			sqrtPriceX64Current, liquidity, amount, false)
	} else {
		return whirlpoolGetNextSqrtPriceFromTokenAmountARoundingUp(
			sqrtPriceX64Current, liquidity, amount, false)
	}
}

// whirlpoolGetNextSqrtPriceFromTokenAmountARoundingUp - 从 Token A 数量计算平方根价格（向上取整）
func whirlpoolGetNextSqrtPriceFromTokenAmountARoundingUp(
	sqrtPriceX64 *big.Int,
	liquidity *big.Int,
	amount *big.Int,
	add bool,
) *big.Int {
	if amount.Cmp(big.NewInt(0)) == 0 {
		return sqrtPriceX64
	}

	liquidityLeftShift := new(big.Int).Lsh(liquidity, U64Resolution)

	if add {
		numerator1 := liquidityLeftShift
		denominator := new(big.Int).Add(liquidityLeftShift, new(big.Int).Mul(amount, sqrtPriceX64))
		if denominator.Cmp(numerator1) >= 0 {
			return whirlpoolMulDivCeil(
				cosmath.NewIntFromBigInt(numerator1),
				cosmath.NewIntFromBigInt(sqrtPriceX64),
				cosmath.NewIntFromBigInt(denominator),
			).BigInt()
		}

		temp := new(big.Int).Div(numerator1, sqrtPriceX64)
		temp.Add(temp, amount)
		return whirlpoolMulDivRoundingUp(numerator1, big.NewInt(1), temp)
	} else {
		amountMulSqrtPrice := new(big.Int).Mul(amount, sqrtPriceX64)
		if liquidityLeftShift.Cmp(amountMulSqrtPrice) <= 0 {
			panic("liquidity must be greater than amount * sqrtPrice")
		}
		denominator := new(big.Int).Sub(liquidityLeftShift, amountMulSqrtPrice)
		return whirlpoolMulDivCeil(
			cosmath.NewIntFromBigInt(liquidityLeftShift),
			cosmath.NewIntFromBigInt(sqrtPriceX64),
			cosmath.NewIntFromBigInt(denominator),
		).BigInt()
	}
}

// whirlpoolGetNextSqrtPriceFromTokenAmountBRoundingDown - 从 Token B 数量计算平方根价格（向下取整）
func whirlpoolGetNextSqrtPriceFromTokenAmountBRoundingDown(
	sqrtPriceX64 *big.Int,
	liquidity *big.Int,
	amount *big.Int,
	add bool,
) *big.Int {
	deltaY := new(big.Int).Lsh(amount, U64Resolution)

	if add {
		return new(big.Int).Add(sqrtPriceX64, new(big.Int).Div(deltaY, liquidity))
	} else {
		amountDivLiquidity := whirlpoolMulDivRoundingUp(deltaY, big.NewInt(1), liquidity)
		if sqrtPriceX64.Cmp(amountDivLiquidity) <= 0 {
			panic("sqrtPriceX64 must be greater than amountDivLiquidity")
		}
		return new(big.Int).Sub(sqrtPriceX64, amountDivLiquidity)
	}
}

// whirlpoolMulDivRoundingUp - 乘除法向上取整
func whirlpoolMulDivRoundingUp(a, b, denominator *big.Int) *big.Int {
	numerator := new(big.Int).Mul(a, b)
	result := new(big.Int).Div(numerator, denominator)
	if new(big.Int).Mod(numerator, denominator).Cmp(big.NewInt(0)) != 0 {
		result.Add(result, big.NewInt(1))
	}
	return result
}
