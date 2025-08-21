// Package raydium implements the Raydium AMM pool functionality for Solana blockchain
package raydium

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"unsafe"

	"cosmossdk.io/math"
	cosmath "cosmossdk.io/math"
	"github.com/Solana-ZH/solroute/pkg"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"lukechampine.com/uint128"
)

// AMMPool represents a Raydium AMM liquidity pool with all its parameters and state
type AMMPool struct {
	// Pool status and configuration
	Status                 uint64
	Nonce                  uint64
	MaxOrder               uint64
	Depth                  uint64
	BaseDecimal            uint64
	QuoteDecimal           uint64
	State                  uint64
	ResetFlag              uint64
	MinSize                uint64
	VolMaxCutRatio         uint64
	AmountWaveRatio        uint64
	BaseLotSize            uint64
	QuoteLotSize           uint64
	MinPriceMultiplier     uint64
	MaxPriceMultiplier     uint64
	SystemDecimalValue     uint64
	MinSeparateNumerator   uint64
	MinSeparateDenominator uint64
	TradeFeeNumerator      uint64
	TradeFeeDenominator    uint64
	PnlNumerator           uint64
	PnlDenominator         uint64
	SwapFeeNumerator       uint64
	SwapFeeDenominator     uint64

	// Pool state and PnL tracking
	BaseNeedTakePnl     uint64
	QuoteNeedTakePnl    uint64
	QuoteTotalPnl       uint64
	BaseTotalPnl        uint64
	PoolOpenTime        uint64
	PunishPcAmount      uint64
	PunishCoinAmount    uint64
	OrderbookToInitTime uint64

	// Swap related amounts
	SwapBaseInAmount   uint128.Uint128
	SwapQuoteOutAmount uint128.Uint128
	SwapBase2QuoteFee  uint64
	SwapQuoteInAmount  uint128.Uint128
	SwapBaseOutAmount  uint128.Uint128
	SwapQuote2BaseFee  uint64

	// Pool accounts
	BaseVault       solana.PublicKey
	QuoteVault      solana.PublicKey
	BaseMint        solana.PublicKey
	QuoteMint       solana.PublicKey
	LpMint          solana.PublicKey
	OpenOrders      solana.PublicKey
	MarketId        solana.PublicKey
	MarketProgramId solana.PublicKey
	TargetOrders    solana.PublicKey
	WithdrawQueue   solana.PublicKey
	LpVault         solana.PublicKey
	Owner           solana.PublicKey
	LpReserve       uint64
	Padding         [3]uint64

	// Market related accounts
	PoolId           solana.PublicKey
	Authority        solana.PublicKey
	MarketAuthority  solana.PublicKey
	MarketBaseVault  solana.PublicKey
	MarketQuoteVault solana.PublicKey
	MarketBids       solana.PublicKey
	MarketAsks       solana.PublicKey
	MarketEventQueue solana.PublicKey

	// Pool balances
	BaseAmount       cosmath.Int
	QuoteAmount      cosmath.Int
	BaseReserve      cosmath.Int
	QuoteReserve     cosmath.Int
	UserBaseAccount  solana.PublicKey
	UserQuoteAccount solana.PublicKey
}

func (pool *AMMPool) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolNameRaydiumAmm
}

func (pool *AMMPool) ProtocolType() pkg.ProtocolType {
	return pkg.ProtocolTypeRaydiumAmm
}

func (pool *AMMPool) GetProgramID() solana.PublicKey {
	return RAYDIUM_AMM_PROGRAM_ID
}

func (l *AMMPool) Span() uint64 {
	return 752
}

func (l *AMMPool) Offset(value string) uint64 {
	fieldType, found := reflect.TypeOf(*l).FieldByName(value)
	if !found {
		return 0
	}
	return uint64(fieldType.Offset)
}

func (l *AMMPool) DecodeBase64(data string) error {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return err
	}
	return l.Decode(decoded)
}

func (l *AMMPool) Decode(data []byte) error {
	if len(data) < 752 {
		return fmt.Errorf("data too short: expected 752 bytes, got %d", len(data))
	}

	offset := 0

	// Parse all uint64 fields
	l.Status = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.Nonce = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.MaxOrder = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.Depth = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.BaseDecimal = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.QuoteDecimal = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.State = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.ResetFlag = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.MinSize = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.VolMaxCutRatio = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.AmountWaveRatio = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.BaseLotSize = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.QuoteLotSize = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.MinPriceMultiplier = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.MaxPriceMultiplier = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.SystemDecimalValue = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.MinSeparateNumerator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.MinSeparateDenominator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.TradeFeeNumerator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.TradeFeeDenominator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.PnlNumerator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.PnlDenominator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.SwapFeeNumerator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.SwapFeeDenominator = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.BaseNeedTakePnl = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.QuoteNeedTakePnl = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.QuoteTotalPnl = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.BaseTotalPnl = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.PoolOpenTime = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.PunishPcAmount = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.PunishCoinAmount = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.OrderbookToInitTime = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Parse uint128 fields (16 bytes each)
	l.SwapBaseInAmount = uint128.FromBytes(data[offset : offset+16])
	offset += 16
	l.SwapQuoteOutAmount = uint128.FromBytes(data[offset : offset+16])
	offset += 16
	l.SwapBase2QuoteFee = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	l.SwapQuoteInAmount = uint128.FromBytes(data[offset : offset+16])
	offset += 16
	l.SwapBaseOutAmount = uint128.FromBytes(data[offset : offset+16])
	offset += 16
	l.SwapQuote2BaseFee = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Parse PublicKey fields (32 bytes each)
	l.BaseVault = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	l.QuoteVault = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	l.BaseMint = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	l.QuoteMint = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	l.LpMint = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	l.OpenOrders = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	l.MarketId = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	l.MarketProgramId = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	l.TargetOrders = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	l.WithdrawQueue = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	l.LpVault = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	l.Owner = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	// Parse remaining fields
	l.LpReserve = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	// Copy padding as uint64 values
	for i := 0; i < 3; i++ {
		l.Padding[i] = binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8
	}

	return nil
}

type MarketStateLayoutV3 struct {
	AccountFlag            [5]byte
	Padding                [8]byte
	OwnAddress             solana.PublicKey
	VaultSignerNonce       uint64
	BaseMint               solana.PublicKey
	QuoteMint              solana.PublicKey
	BaseVault              solana.PublicKey
	BaseDepositsTotal      uint64
	BaseFeesAccrued        uint64
	QuoteVault             solana.PublicKey
	QuoteDepositsTotal     uint64
	QuoteFeesAccrued       uint64
	QuoteDustThreshold     uint64
	RequestQueue           solana.PublicKey
	EventQueue             solana.PublicKey
	Bids                   solana.PublicKey
	Asks                   solana.PublicKey
	BaseLotSize            uint64
	QuoteLotSize           uint64
	FeeRateBps             uint64
	ReferrerRebatesAccrued uint64
	PaddingEnd             [7]byte
}

func (l MarketStateLayoutV3) Span() uint64 {
	return uint64(unsafe.Sizeof(l)) - 4 // Blame golang
}

func (l *MarketStateLayoutV3) DecodeBase64(data string) error {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return err
	}
	return l.Decode(decoded)
}

func (l *MarketStateLayoutV3) Decode(data []byte) error {
	err := bin.UnmarshalBorsh(&l, data)
	return err
}

func (l *MarketStateLayoutV3) Offset(value string) uint64 {
	fieldType, found := reflect.TypeOf(*l).FieldByName(value)
	if !found {
		return 0
	}
	return uint64(fieldType.Offset)
}

// Print outputs the pool information in a structured JSON format
func (l *MarketStateLayoutV3) Print() {
	poolInfo, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal pool info: %v", err)
		return
	}
	log.Printf("Pool Information:\n%s", string(poolInfo))
}

// GetID returns the pool ID
func (p *AMMPool) GetID() string {
	return p.PoolId.String()
}

// GetTokens returns the base and quote token mints
func (p *AMMPool) GetTokens() (baseMint, quoteMint string) {
	return p.BaseMint.String(), p.QuoteMint.String()
}

// Quote calculates the expected output amount for a given input amount
// It takes into account the current pool reserves and fees
func (p *AMMPool) Quote(
	ctx context.Context,
	solClient *rpc.Client,
	inputMint string,
	inputAmount cosmath.Int,
) (cosmath.Int, error) {
	// update pool data first
	accounts := make([]solana.PublicKey, 0)
	accounts = append(accounts, p.BaseVault)
	accounts = append(accounts, p.QuoteVault)
	results, err := solClient.GetMultipleAccountsWithOpts(ctx,
		accounts,
		&rpc.GetMultipleAccountsOpts{
			Commitment: rpc.CommitmentProcessed,
		},
	)
	if err != nil {
		return math.NewInt(0), fmt.Errorf("batch request failed: %v", err)
	}
	for i, result := range results.Value {
		if result == nil {
			return math.NewInt(0), fmt.Errorf("result is nil, account: %v", accounts[i].String())
		}
		accountKey := accounts[i].String()
		if p.BaseVault.String() == accountKey {
			amountBytes := result.Data.GetBinary()[64:72]
			amountUint := binary.LittleEndian.Uint64(amountBytes)
			amount := math.NewIntFromUint64(amountUint)
			p.BaseAmount = amount
		} else {
			amountBytes := result.Data.GetBinary()[64:72]
			amountUint := binary.LittleEndian.Uint64(amountBytes)
			amount := math.NewIntFromUint64(amountUint)
			p.QuoteAmount = amount
		}
	}

	// Calculate effective reserves by subtracting pending PnL
	p.BaseReserve = p.BaseAmount.Sub(cosmath.NewInt(int64(p.BaseNeedTakePnl)))
	p.QuoteReserve = p.QuoteAmount.Sub(cosmath.NewInt(int64(p.QuoteNeedTakePnl)))

	// Set reserves and decimals based on swap direction
	reserves := []cosmath.Int{p.BaseReserve, p.QuoteReserve}
	mintDecimals := []int{int(p.BaseDecimal), int(p.QuoteDecimal)}

	// Swap reserves if input is quote token
	if inputMint == p.QuoteMint.String() {
		reserves[0], reserves[1] = reserves[1], reserves[0]
		mintDecimals[0], mintDecimals[1] = mintDecimals[1], mintDecimals[0]
	}

	reserveIn := reserves[0]
	reserveOut := reserves[1]

	// Initialize output values
	amountOutRaw := cosmath.ZeroInt()
	feeRaw := cosmath.ZeroInt()

	// Calculate output amount if input is non-zero
	if !inputAmount.IsZero() {
		// Calculate fee based on input amount
		feeRaw = inputAmount.Mul(LIQUIDITY_FEES_NUMERATOR).Quo(LIQUIDITY_FEES_DENOMINATOR)

		// Calculate amount after fee
		amountInWithFee := inputAmount.Sub(feeRaw)

		// Calculate output using constant product formula: x * y = k
		denominator := reserveIn.Add(amountInWithFee)
		amountOutRaw = reserveOut.Mul(amountInWithFee).Quo(denominator)
	}
	return amountOutRaw, nil
}

// BuildSwapInstructions constructs the necessary instructions for executing a swap
// It handles both base-to-quote and quote-to-base swaps
func (pool *AMMPool) BuildSwapInstructions(
	ctx context.Context,
	solClient *rpc.Client,
	user solana.PublicKey,
	inputMint string,
	inputAmount cosmath.Int,
	minOut cosmath.Int,
) ([]solana.Instruction, error) {
	instrs := []solana.Instruction{}

	// Determine input token mint
	var inputValueMint solana.PublicKey
	if inputMint == pool.BaseMint.String() {
		inputValueMint = pool.BaseMint
	} else {
		inputValueMint = pool.QuoteMint
	}

	// Set up source and destination accounts based on swap direction
	var fromAccount, toAccount solana.PublicKey
	if inputValueMint.String() == pool.BaseMint.String() {
		fromAccount = pool.UserBaseAccount
		toAccount = pool.UserQuoteAccount
	} else {
		fromAccount = pool.UserQuoteAccount
		toAccount = pool.UserBaseAccount
	}

	// Create swap instruction
	inst := InSwapInstruction{
		InAmount:         inputAmount.Uint64(),
		MinimumOutAmount: minOut.Uint64(),
		AccountMetaSlice: make(solana.AccountMetaSlice, 18),
	}
	inst.BaseVariant = bin.BaseVariant{
		Impl: inst,
	}

	// Set up account metas for the swap instruction
	tokenProgramID := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	inst.AccountMetaSlice[0] = solana.NewAccountMeta(tokenProgramID, false, false)
	inst.AccountMetaSlice[1] = solana.NewAccountMeta(pool.PoolId, true, false)
	inst.AccountMetaSlice[2] = solana.NewAccountMeta(pool.Authority, false, false)
	inst.AccountMetaSlice[3] = solana.NewAccountMeta(pool.OpenOrders, true, false)
	inst.AccountMetaSlice[4] = solana.NewAccountMeta(pool.TargetOrders, true, false)
	inst.AccountMetaSlice[5] = solana.NewAccountMeta(pool.BaseVault, true, false)
	inst.AccountMetaSlice[6] = solana.NewAccountMeta(pool.QuoteVault, true, false)
	inst.AccountMetaSlice[7] = solana.NewAccountMeta(pool.MarketProgramId, false, false)
	inst.AccountMetaSlice[8] = solana.NewAccountMeta(pool.MarketId, true, false)
	inst.AccountMetaSlice[9] = solana.NewAccountMeta(pool.MarketBids, true, false)
	inst.AccountMetaSlice[10] = solana.NewAccountMeta(pool.MarketAsks, true, false)
	inst.AccountMetaSlice[11] = solana.NewAccountMeta(pool.MarketEventQueue, true, false)
	inst.AccountMetaSlice[12] = solana.NewAccountMeta(pool.MarketBaseVault, true, false)
	inst.AccountMetaSlice[13] = solana.NewAccountMeta(pool.MarketQuoteVault, true, false)
	inst.AccountMetaSlice[14] = solana.NewAccountMeta(pool.MarketAuthority, false, false)
	inst.AccountMetaSlice[15] = solana.NewAccountMeta(fromAccount, true, false)
	inst.AccountMetaSlice[16] = solana.NewAccountMeta(toAccount, true, false)
	inst.AccountMetaSlice[17] = solana.NewAccountMeta(user, true, true)

	instrs = append(instrs, &inst)
	return instrs, nil
}

type InSwapInstruction struct {
	bin.BaseVariant
	InAmount                uint64
	MinimumOutAmount        uint64
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

func (inst *InSwapInstruction) ProgramID() solana.PublicKey {
	return RAYDIUM_AMM_PROGRAM_ID
}

func (inst *InSwapInstruction) Accounts() (out []*solana.AccountMeta) {
	return inst.Impl.(solana.AccountsGettable).GetAccounts()
}

func (inst *InSwapInstruction) Data() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := bin.NewBorshEncoder(buf).Encode(inst); err != nil {
		return nil, fmt.Errorf("unable to encode instruction: %w", err)
	}
	return buf.Bytes(), nil
}

func (inst *InSwapInstruction) MarshalWithEncoder(encoder *bin.Encoder) (err error) {
	// Swap instruction is number 9
	err = encoder.WriteUint8(9)
	if err != nil {
		return err
	}
	err = encoder.WriteUint64(inst.InAmount, binary.LittleEndian)
	if err != nil {
		return err
	}
	err = encoder.WriteUint64(inst.MinimumOutAmount, binary.LittleEndian)
	if err != nil {
		return err
	}
	return nil
}
