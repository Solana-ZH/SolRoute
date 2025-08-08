package pump

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"cosmossdk.io/math"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/yimingWOW/solroute/pkg"
	"github.com/yimingWOW/solroute/utils"
)

const (
	// PoolDataSize represents the expected size of pool data in bytes
	PoolDataSize = 211

	// DefaultSpan represents the default span value for the pool
	DefaultSpan = 300

	// BaseMintOffset represents the offset for BaseMint in the pool data
	BaseMintOffset = 43

	// QuoteMintOffset represents the offset for QuoteMint in the pool data
	QuoteMintOffset = BaseMintOffset + 32

	// DefaultFeeRate represents the default fee rate for swaps (0.25%)
	DefaultFeeRate = 0.00250
)

// PumpAMMPool represents an AMM pool for the Pump protocol
type PumpAMMPool struct {
	Discriminator         [8]uint8 `bin:"skip"`
	PoolBump              uint8
	Index                 uint16
	Creator               solana.PublicKey
	BaseMint              solana.PublicKey
	QuoteMint             solana.PublicKey
	LpMint                solana.PublicKey
	PoolBaseTokenAccount  solana.PublicKey
	PoolQuoteTokenAccount solana.PublicKey
	LpSupply              uint64
	CoinCreator           solana.PublicKey

	PoolId           solana.PublicKey
	BaseAmount       math.Int
	QuoteAmount      math.Int
	UserBaseAccount  solana.PublicKey
	UserQuoteAccount solana.PublicKey
}

func (pool *PumpAMMPool) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolNamePumpAmm
}

func (pool *PumpAMMPool) ProtocolType() pkg.ProtocolType {
	return pkg.ProtocolTypePumpAmm
}

func (pool *PumpAMMPool) GetProgramID() solana.PublicKey {
	return PumpSwapProgramID
}

// Span returns the default span value for the pool
func (p *PumpAMMPool) Span() uint64 {
	return uint64(DefaultSpan)
}

// Offset returns the byte offset for a given field in the pool data
func (p *PumpAMMPool) Offset(value string) uint64 {
	switch value {
	case "BaseMint":
		return BaseMintOffset
	case "QuoteMint":
		return QuoteMintOffset
	default:
		return 0
	}
}

// Decode decodes the pool data from bytes
func (p *PumpAMMPool) Decode(data []byte) error {
	if len(data) < PoolDataSize {
		return fmt.Errorf("data too short: expected %d bytes, got %d", PoolDataSize, len(data))
	}
	dec := bin.NewBinDecoder(data)
	return dec.Decode(p)
}

// ParsePoolData parses the raw pool data into a PumpAMMPool struct
func ParsePoolData(data []byte) (*PumpAMMPool, error) {
	if len(data) < PoolDataSize {
		return nil, fmt.Errorf("data too short: expected %d bytes, got %d", PoolDataSize, len(data))
	}

	layout := &PumpAMMPool{}
	// Parse structure
	discriminator := [8]byte{}
	copy(discriminator[:], data[:8])
	layout.PoolBump = uint8(data[8])
	layout.Index = binary.LittleEndian.Uint16(data[9:11])

	offset := 11
	layout.Creator = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	layout.BaseMint = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	layout.QuoteMint = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	layout.LpMint = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	layout.PoolBaseTokenAccount = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	layout.PoolQuoteTokenAccount = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32
	layout.LpSupply = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	if len(data[offset:]) > 32 {
		layout.CoinCreator = solana.PublicKeyFromBytes(data[offset : offset+32])
	} else {
		layout.CoinCreator = solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	}

	return layout, nil
}

func (l *PumpAMMPool) GetID() string {
	return l.PoolId.String()
}

func (l *PumpAMMPool) GetTokens() (string, string) {
	return l.BaseMint.String(), l.QuoteMint.String()
}

func (s *PumpAMMPool) BuildSwapInstructions(
	ctx context.Context,
	solClient *rpc.Client,
	user solana.PublicKey,
	inputMint string,
	inputAmount math.Int,
	minOut math.Int,
) ([]solana.Instruction, error) {
	if inputMint == s.BaseMint.String() {
		return s.buyInAMMPool(user, s, inputAmount, minOut)
	} else {
		return s.sellInAMMPool(user, s, inputAmount, minOut)
	}
}

func (s *PumpAMMPool) buyInAMMPool(userAddr solana.PublicKey, pool *PumpAMMPool,
	maxInputAmountWithDecimals math.Int, outAmountWithDecimals math.Int) ([]solana.Instruction, error) {
	// Initialize instruction array
	instrs := []solana.Instruction{}

	inst := BuySwapInstruction{
		BaseAmountOut:    outAmountWithDecimals.Uint64(),
		MaxQuoteAmountIn: maxInputAmountWithDecimals.Uint64(),
	}
	if pool.CoinCreator == solana.MustPublicKeyFromBase58("11111111111111111111111111111111") {
		inst.AccountMetaSlice = make(solana.AccountMetaSlice, 17)
	} else {
		inst.AccountMetaSlice = make(solana.AccountMetaSlice, 19)
	}

	inst.BaseVariant = bin.BaseVariant{
		Impl: inst,
	}
	// Ensure correct Token Program address
	inst.AccountMetaSlice[0] = solana.NewAccountMeta(pool.PoolId, false, false)
	inst.AccountMetaSlice[1] = solana.NewAccountMeta(userAddr, true, true)
	inst.AccountMetaSlice[2] = solana.NewAccountMeta(PumpGlobalConfig, false, false)
	inst.AccountMetaSlice[3] = solana.NewAccountMeta(pool.BaseMint, false, false)
	inst.AccountMetaSlice[4] = solana.NewAccountMeta(pool.QuoteMint, false, false)
	inst.AccountMetaSlice[5] = solana.NewAccountMeta(pool.UserBaseAccount, true, false)
	inst.AccountMetaSlice[6] = solana.NewAccountMeta(pool.UserQuoteAccount, true, false)
	inst.AccountMetaSlice[7] = solana.NewAccountMeta(pool.PoolBaseTokenAccount, true, false)
	inst.AccountMetaSlice[8] = solana.NewAccountMeta(pool.PoolQuoteTokenAccount, true, false)
	inst.AccountMetaSlice[9] = solana.NewAccountMeta(PumpProtocolFeeRecipient, false, false)
	inst.AccountMetaSlice[10] = solana.NewAccountMeta(PumpProtocolFeeRecipientTokenAccount, true, false)
	tokenProgramID := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	inst.AccountMetaSlice[11] = solana.NewAccountMeta(tokenProgramID, false, false)
	inst.AccountMetaSlice[12] = solana.NewAccountMeta(tokenProgramID, false, false)
	inst.AccountMetaSlice[13] = solana.NewAccountMeta(solana.MustPublicKeyFromBase58("11111111111111111111111111111111"), false, false)
	inst.AccountMetaSlice[14] = solana.NewAccountMeta(solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"), false, false)
	inst.AccountMetaSlice[15] = solana.NewAccountMeta(solana.MustPublicKeyFromBase58("GS4CU59F31iL7aR2Q8zVS8DRrcRnXX1yjQ66TqNVQnaR"), false, false)
	inst.AccountMetaSlice[16] = solana.NewAccountMeta(PumpSwapProgramID, false, false)
	if pool.CoinCreator != solana.MustPublicKeyFromBase58("11111111111111111111111111111111") {
		ata, err := GetCoinCreatorVaultATA(pool.CoinCreator)
		if err != nil {
			return nil, fmt.Errorf("failed to get coin creator vault ata: %w", err)
		}
		inst.AccountMetaSlice[17] = solana.NewAccountMeta(ata, true, false)
		authority, err := GetCoinCreatorVaultAuthority(pool.CoinCreator)
		if err != nil {
			return nil, fmt.Errorf("failed to get coin creator vault authority: %w", err)
		}
		inst.AccountMetaSlice[18] = solana.NewAccountMeta(authority, false, false)
	}
	instrs = append(instrs, &inst)

	return instrs, nil
}

func (s *PumpAMMPool) sellInAMMPool(userAddr solana.PublicKey,
	pool *PumpAMMPool, baseAmountIn math.Int, minQuoteAmountOut math.Int) ([]solana.Instruction, error) {
	instrs := []solana.Instruction{}

	inst := SellSwapInstruction{
		BaseAmountIn:      baseAmountIn.Uint64(),
		MinQuoteAmountOut: minQuoteAmountOut.Uint64(),
	}
	if pool.CoinCreator == solana.MustPublicKeyFromBase58("11111111111111111111111111111111") {
		inst.AccountMetaSlice = make(solana.AccountMetaSlice, 17)
	} else {
		inst.AccountMetaSlice = make(solana.AccountMetaSlice, 19)
	}
	inst.BaseVariant = bin.BaseVariant{
		Impl: inst,
	}
	// 确保使用正确的 Token Program 地址
	inst.AccountMetaSlice[0] = solana.NewAccountMeta(pool.PoolId, false, false)
	inst.AccountMetaSlice[1] = solana.NewAccountMeta(userAddr, true, true)
	inst.AccountMetaSlice[2] = solana.NewAccountMeta(PumpGlobalConfig, false, false)
	inst.AccountMetaSlice[3] = solana.NewAccountMeta(pool.BaseMint, false, false)
	inst.AccountMetaSlice[4] = solana.NewAccountMeta(pool.QuoteMint, false, false)
	inst.AccountMetaSlice[5] = solana.NewAccountMeta(pool.UserBaseAccount, true, false)
	inst.AccountMetaSlice[6] = solana.NewAccountMeta(pool.UserQuoteAccount, true, false)
	inst.AccountMetaSlice[7] = solana.NewAccountMeta(pool.PoolBaseTokenAccount, true, false)
	inst.AccountMetaSlice[8] = solana.NewAccountMeta(pool.PoolQuoteTokenAccount, true, false)
	inst.AccountMetaSlice[9] = solana.NewAccountMeta(PumpProtocolFeeRecipient, false, false)
	inst.AccountMetaSlice[10] = solana.NewAccountMeta(PumpProtocolFeeRecipientTokenAccount, true, false)
	tokenProgramID := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	inst.AccountMetaSlice[11] = solana.NewAccountMeta(tokenProgramID, false, false)
	inst.AccountMetaSlice[12] = solana.NewAccountMeta(tokenProgramID, false, false)
	inst.AccountMetaSlice[13] = solana.NewAccountMeta(solana.MustPublicKeyFromBase58("11111111111111111111111111111111"), false, false)
	inst.AccountMetaSlice[14] = solana.NewAccountMeta(solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"), false, false)
	inst.AccountMetaSlice[15] = solana.NewAccountMeta(solana.MustPublicKeyFromBase58("GS4CU59F31iL7aR2Q8zVS8DRrcRnXX1yjQ66TqNVQnaR"), false, false)
	inst.AccountMetaSlice[16] = solana.NewAccountMeta(PumpSwapProgramID, false, false)
	if pool.CoinCreator != solana.MustPublicKeyFromBase58("11111111111111111111111111111111") {
		ata, err := GetCoinCreatorVaultATA(pool.CoinCreator)
		if err != nil {
			return nil, fmt.Errorf("failed to get coin creator vault ata: %w", err)
		}
		inst.AccountMetaSlice[17] = solana.NewAccountMeta(ata, false, false)
		authority, err := GetCoinCreatorVaultAuthority(pool.CoinCreator)
		if err != nil {
			return nil, fmt.Errorf("failed to get coin creator vault authority: %w", err)
		}
		inst.AccountMetaSlice[18] = solana.NewAccountMeta(authority, false, false)
	}
	instrs = append(instrs, &inst)

	return instrs, nil
}

type BuySwapInstruction struct {
	bin.BaseVariant
	BaseAmountOut           uint64
	MaxQuoteAmountIn        uint64
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

func (inst *BuySwapInstruction) ProgramID() solana.PublicKey {
	return PumpSwapProgramID
}

func (inst *BuySwapInstruction) Accounts() (out []*solana.AccountMeta) {
	return inst.Impl.(solana.AccountsGettable).GetAccounts()
}

func (inst *BuySwapInstruction) Data() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write discriminator for swap instruction
	namespace := "global"
	name := "buy"
	discriminator := utils.GetDiscriminator(namespace, name)
	if _, err := buf.Write(discriminator); err != nil {
		return nil, fmt.Errorf("failed to write discriminator: %w", err)
	}

	// Write amount
	if err := bin.NewBorshEncoder(buf).WriteUint64(inst.BaseAmountOut, binary.LittleEndian); err != nil {
		return nil, fmt.Errorf("failed to encode amount: %w", err)
	}

	// Write other amount threshold
	if err := bin.NewBorshEncoder(buf).WriteUint64(inst.MaxQuoteAmountIn, binary.LittleEndian); err != nil {
		return nil, fmt.Errorf("failed to encode other amount threshold: %w", err)
	}

	return buf.Bytes(), nil
}

type SellSwapInstruction struct {
	bin.BaseVariant
	BaseAmountIn            uint64
	MinQuoteAmountOut       uint64
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

func (inst *SellSwapInstruction) ProgramID() solana.PublicKey {
	return PumpSwapProgramID
}

func (inst *SellSwapInstruction) Accounts() (out []*solana.AccountMeta) {
	return inst.Impl.(solana.AccountsGettable).GetAccounts()
}

func (inst *SellSwapInstruction) Data() ([]byte, error) {

	// 手动构建指令数据
	buf := new(bytes.Buffer)

	// Write discriminator for swap instruction
	namespace := "global"
	name := "sell"
	discriminator := utils.GetDiscriminator(namespace, name)
	if _, err := buf.Write(discriminator); err != nil {
		return nil, fmt.Errorf("failed to write discriminator: %w", err)
	}

	// Write amount
	if err := bin.NewBorshEncoder(buf).WriteUint64(inst.BaseAmountIn, binary.LittleEndian); err != nil {
		return nil, fmt.Errorf("failed to encode amount: %w", err)
	}

	// Write other amount threshold
	if err := bin.NewBorshEncoder(buf).WriteUint64(inst.MinQuoteAmountOut, binary.LittleEndian); err != nil {
		return nil, fmt.Errorf("failed to encode other amount threshold: %w", err)
	}

	return buf.Bytes(), nil
}

func (pool *PumpAMMPool) Quote(ctx context.Context, solClient *rpc.Client, inputMint string, inputAmount math.Int) (math.Int, error) {
	// update pool data first
	accounts := make([]solana.PublicKey, 0)
	accounts = append(accounts, pool.PoolBaseTokenAccount)
	accounts = append(accounts, pool.PoolQuoteTokenAccount)
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
		if pool.PoolBaseTokenAccount.String() == accountKey {
			amountBytes := result.Data.GetBinary()[64:72]
			amountUint := binary.LittleEndian.Uint64(amountBytes)
			amount := math.NewIntFromUint64(amountUint)
			pool.BaseAmount = amount
		} else {
			amountBytes := result.Data.GetBinary()[64:72]
			amountUint := binary.LittleEndian.Uint64(amountBytes)
			amount := math.NewIntFromUint64(amountUint)
			pool.QuoteAmount = amount
		}
	}

	feeRate := 1 - DefaultFeeRate
	feeMultiplier := math.NewInt(int64(feeRate * float64(BaseDecimalInt)))

	// Calculate k = baseAmount * quoteAmount
	k := pool.BaseAmount.Mul(pool.QuoteAmount)

	if inputMint == pool.BaseMint.String() {
		// Calculate newBase = baseAmount + amountWithFee
		newBase := pool.BaseAmount.Add(inputAmount.Mul(feeMultiplier).Quo(BaseDecimal))
		// Calculate newQuote = k / newBase
		newQuote := k.Quo(newBase)
		priceBaseToQuote := pool.QuoteAmount.Sub(newQuote)
		return priceBaseToQuote, nil
	} else {
		// Calculate newQuote = quoteAmount + amountWithFee
		newQuote := pool.QuoteAmount.Add(inputAmount.Mul(feeMultiplier).Quo(BaseDecimal))
		// Calculate newBase = k / newQuote
		newBase := k.Quo(newQuote)
		priceQuoteToBase := pool.BaseAmount.Sub(newBase)
		return priceQuoteToBase, nil
	}
}
