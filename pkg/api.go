package pkg

import (
	"context"

	"cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// ProtocolName represents the string name of AMM protocol
type ProtocolName string

const (
	ProtocolNameRaydiumAmm    ProtocolName = "raydium_amm"
	ProtocolNameRaydiumClmm   ProtocolName = "raydium_clmm"
	ProtocolNameRaydiumCpmm   ProtocolName = "raydium_cpmm"
	ProtocolNameMeteoraDlmm   ProtocolName = "meteora_dlmm"
	ProtocolNamePumpAmm       ProtocolName = "pump_amm"
	ProtocolNameOrcaWhirlpool ProtocolName = "orca_whirlpool"
)

// ProtocolType represents the numeric type of AMM protocol (matches contract enum)
type ProtocolType uint8

const (
	ProtocolTypeRaydiumAmm ProtocolType = iota
	ProtocolTypeRaydiumClmm
	ProtocolTypeRaydiumCpmm
	ProtocolTypeMeteoraDlmm
	ProtocolTypePumpAmm
	ProtocolTypeOrcaWhirlpool
)

type Pool interface {
	ProtocolName() ProtocolName
	ProtocolType() ProtocolType
	GetProgramID() solana.PublicKey
	GetID() string
	GetTokens() (baseMint, quoteMint string)
	Quote(ctx context.Context, solClient *rpc.Client, inputMint string, inputAmount math.Int) (math.Int, error)
	BuildSwapInstructions(
		ctx context.Context,
		solClient *rpc.Client,
		user solana.PublicKey,
		inputMint string,
		inputAmount math.Int,
		minOut math.Int,
	) ([]solana.Instruction, error)
}

type Protocol interface {
	FetchPoolsByPair(ctx context.Context, baseMint, quoteMint string) ([]Pool, error)
	FetchPoolByID(ctx context.Context, poolID string) (Pool, error)
}
