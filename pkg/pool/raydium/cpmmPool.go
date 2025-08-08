package raydium

import (
	"context"
	"encoding/binary"
	"fmt"

	"cosmossdk.io/math"
	cosmath "cosmossdk.io/math"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/yimingWOW/solroute/pkg"
)

// CPMMPool represents the on-chain pool state
type CPMMPool struct {
	AmmConfig          solana.PublicKey // 32 bytes
	PoolCreator        solana.PublicKey // 32 bytes
	Token0Vault        solana.PublicKey // 32 bytes
	Token1Vault        solana.PublicKey // 32 bytes
	LpMint             solana.PublicKey // 32 bytes
	Token0Mint         solana.PublicKey // 32 bytes
	Token1Mint         solana.PublicKey // 32 bytes
	Token0Program      solana.PublicKey // 32 bytes
	Token1Program      solana.PublicKey // 32 bytes
	ObservationKey     solana.PublicKey // 32 bytes
	AuthBump           uint8            // 1 byte
	Status             uint8            // 1 byte
	LpMintDecimals     uint8            // 1 byte
	Mint0Decimals      uint8            // 1 byte
	Mint1Decimals      uint8            // 1 byte
	_padding1          [3]uint8         // 3 bytes padding
	LpSupply           uint64           // 8 bytes
	ProtocolFeesToken0 uint64           // 8 bytes
	ProtocolFeesToken1 uint64           // 8 bytes
	FundFeesToken0     uint64           // 8 bytes
	FundFeesToken1     uint64           // 8 bytes
	OpenTime           uint64           // 8 bytes
	_padding2          [32]uint64       // 256 bytes padding

	PoolId           solana.PublicKey
	UserBaseAccount  solana.PublicKey
	UserQuoteAccount solana.PublicKey
	BaseAmount       cosmath.Int
	QuoteAmount      cosmath.Int
	BaseReserve      cosmath.Int
	QuoteReserve     cosmath.Int
	BaseDecimal      uint64
	QuoteDecimal     uint64
	BaseNeedTakePnl  uint64
	QuoteNeedTakePnl uint64
}

func (pool *CPMMPool) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolNameRaydiumCpmm
}

func (pool *CPMMPool) ProtocolType() pkg.ProtocolType {
	return pkg.ProtocolTypeRaydiumCpmm
}

func (pool *CPMMPool) GetProgramID() solana.PublicKey {
	return RAYDIUM_CPMM_PROGRAM_ID
}

func (p *CPMMPool) Decode(data []byte) error {
	if len(data) > 8 {
		data = data[8:]
	}

	dec := bin.NewBinDecoder(data)
	return dec.Decode(p)
}

func (p *CPMMPool) Span() uint64 {
	return 584 // Total size in bytes (including discriminator)
}

func (p *CPMMPool) Offset(field string) uint64 {
	switch field {
	case "Token0Mint":
		return 8 + 32*5 // discriminator + 5 pubkeys
	case "Token1Mint":
		return 8 + 32*6 // discriminator + 6 pubkeys
	default:
		return 0
	}
}

func (pool *CPMMPool) GetID() string {
	return pool.PoolId.String()
}

func (pool *CPMMPool) GetTokens() (string, string) {
	return pool.Token0Mint.String(), pool.Token1Mint.String()
}

func (pool *CPMMPool) BuildSwapInstructions(
	ctx context.Context,
	solClient *rpc.Client,
	userAddr solana.PublicKey,
	inputMint string,
	amountIn math.Int,
	minOutAmountWithDecimals math.Int,
) ([]solana.Instruction, error) {

	// 初始化指令数组
	instrs := []solana.Instruction{}

	var inputValueMint solana.PublicKey
	if inputMint == pool.Token0Mint.String() {
		inputValueMint = pool.Token0Mint
	} else {
		inputValueMint = pool.Token1Mint
	}

	var fromAccount solana.PublicKey
	var toAccount solana.PublicKey
	if inputValueMint.String() == pool.Token0Mint.String() {
		fromAccount = pool.UserBaseAccount
		toAccount = pool.UserQuoteAccount
	} else {
		fromAccount = pool.UserQuoteAccount
		toAccount = pool.UserBaseAccount
	}

	// 创建 swap 指令
	swapInst := CPMMSwapInstruction{
		InAmount:         amountIn.Uint64(),
		MinimumOutAmount: minOutAmountWithDecimals.Uint64(),
		AccountMetaSlice: make(solana.AccountMetaSlice, 13),
	}
	swapInst.BaseVariant = bin.BaseVariant{
		Impl: swapInst,
	}

	// Get the authority PDA
	authority, _, err := getAuthorityPDA()
	if err != nil {
		return nil, fmt.Errorf("failed to get authority PDA: %v", err)
	}
	// 设置账户
	swapInst.AccountMetaSlice[0] = solana.NewAccountMeta(userAddr, true, true)                // payer
	swapInst.AccountMetaSlice[1] = solana.NewAccountMeta(authority, false, false)             // authority
	swapInst.AccountMetaSlice[2] = solana.NewAccountMeta(pool.AmmConfig, false, false)        // amm_config
	swapInst.AccountMetaSlice[3] = solana.NewAccountMeta(pool.PoolId, true, false)            // pool_state
	swapInst.AccountMetaSlice[4] = solana.NewAccountMeta(fromAccount, true, false)            // input_token_account
	swapInst.AccountMetaSlice[5] = solana.NewAccountMeta(toAccount, true, false)              // output_token_account
	swapInst.AccountMetaSlice[6] = solana.NewAccountMeta(pool.Token0Vault, true, false)       // input_vault
	swapInst.AccountMetaSlice[7] = solana.NewAccountMeta(pool.Token1Vault, true, false)       // output_vault
	swapInst.AccountMetaSlice[8] = solana.NewAccountMeta(solana.TokenProgramID, false, false) // input_token_program
	swapInst.AccountMetaSlice[9] = solana.NewAccountMeta(solana.TokenProgramID, false, false) // output_token_program
	swapInst.AccountMetaSlice[10] = solana.NewAccountMeta(pool.Token0Mint, false, false)      // input_token_mint
	swapInst.AccountMetaSlice[11] = solana.NewAccountMeta(pool.Token1Mint, false, false)      // output_token_mint
	swapInst.AccountMetaSlice[12] = solana.NewAccountMeta(pool.ObservationKey, true, false)   // observation_state
	instrs = append(instrs, &swapInst)

	return instrs, nil
}

// CPMMSwapInstruction represents the data for a CPMM swap instruction
type CPMMSwapInstruction struct {
	bin.BaseVariant
	InAmount                uint64
	MinimumOutAmount        uint64
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

func (inst *CPMMSwapInstruction) ProgramID() solana.PublicKey {
	return RAYDIUM_CPMM_PROGRAM_ID
}

func (inst *CPMMSwapInstruction) Accounts() (out []*solana.AccountMeta) {
	return inst.AccountMetaSlice
}

func (inst *CPMMSwapInstruction) Data() ([]byte, error) {
	// Use a single method to encode data to avoid conflicts
	data := make([]byte, 8+8+8) // discriminator(8) + amount_in(8) + minimum_amount_out(8)

	// Write the correct discriminator for swapBaseInput
	copy(data[0:8], SwapBaseInputDiscriminator)

	// Write amount_in
	binary.LittleEndian.PutUint64(data[8:16], inst.InAmount)

	// Write minimum_amount_out
	binary.LittleEndian.PutUint64(data[16:24], inst.MinimumOutAmount)

	return data, nil
}

// Add a helper function to get the authority PDA
func getAuthorityPDA() (solana.PublicKey, uint8, error) {
	seeds := [][]byte{
		[]byte(AUTH_SEED),
	}
	authority, bump, err := solana.FindProgramAddress(seeds, RAYDIUM_CPMM_PROGRAM_ID)
	if err != nil {
		return solana.PublicKey{}, 0, fmt.Errorf("failed to find authority PDA: %v", err)
	}
	return authority, bump, nil
}

func (pool *CPMMPool) Quote(ctx context.Context, solClient *rpc.Client, inputMint string, inputAmount math.Int) (math.Int, error) {
	// update pool data first
	accounts := make([]solana.PublicKey, 0)
	accounts = append(accounts, pool.Token0Vault)
	accounts = append(accounts, pool.Token1Vault)
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
		if pool.Token0Vault.String() == accountKey {
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

	pool.BaseReserve = pool.BaseAmount.Sub(math.NewInt(int64(pool.BaseNeedTakePnl)))
	pool.QuoteReserve = pool.QuoteAmount.Sub(math.NewInt(int64(pool.QuoteNeedTakePnl)))

	// Set reserves based on direction
	reserves := []math.Int{pool.BaseReserve, pool.QuoteReserve}
	mintDecimals := []int{int(pool.BaseDecimal), int(pool.QuoteDecimal)}

	// Determine input side
	input := "base"
	if inputMint == pool.Token1Mint.String() {
		input = "quote"
	}

	// If input is quote, reverse reserves and decimals
	if input == "quote" {
		reserves[0], reserves[1] = reserves[1], reserves[0]
		mintDecimals[0], mintDecimals[1] = mintDecimals[1], mintDecimals[0]
	}

	reserveIn := reserves[0]
	reserveOut := reserves[1]

	// Initialize output values
	amountOutRaw := math.ZeroInt()
	feeRaw := math.ZeroInt()

	// If amountIn is not zero, calculate amountOut
	if !inputAmount.IsZero() {
		// Calculate fee
		feeRaw = inputAmount.Mul(LIQUIDITY_FEES_NUMERATOR).Quo(LIQUIDITY_FEES_DENOMINATOR)

		// Calculate amountInWithFee
		amountInWithFee := inputAmount.Sub(feeRaw)

		// Calculate output amount using constant product formula
		denominator := reserveIn.Add(amountInWithFee)
		amountOutRaw = reserveOut.Mul(amountInWithFee).Quo(denominator)
	}
	return amountOutRaw, nil
}
