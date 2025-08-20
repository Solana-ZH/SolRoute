package protocol

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/yimingWOW/solroute/pkg"
	"github.com/yimingWOW/solroute/pkg/pool/orca"
	"github.com/yimingWOW/solroute/pkg/sol"
)

// OrcaWhirlpoolProtocol implements Protocol interface, providing Orca Whirlpool V2 protocol support
//
// Orca Whirlpool is a concentrated liquidity-based automated market maker (CLMM) protocol,
// supporting capital efficiency optimized liquidity provision and trading.
//
// Program ID: whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc
//
// Main features:
// - Concentrated liquidity management
// - Multi-tier fee structure
// - Tick-based price mechanism
// - SwapV2 instruction support
type OrcaWhirlpoolProtocol struct {
	SolClient *sol.Client
}

// NewOrcaWhirlpool creates a new Orca Whirlpool protocol instance
//
// Parameters:
//   - solClient: Solana client for blockchain interaction
//
// Returns:
//   - *OrcaWhirlpoolProtocol: protocol instance
func NewOrcaWhirlpool(solClient *sol.Client) *OrcaWhirlpoolProtocol {
	return &OrcaWhirlpoolProtocol{
		SolClient: solClient,
	}
}

// FetchPoolsByPair gets Whirlpool pool list by token pair
// Reference raydiumClmm.go implementation, adjust field name mapping
func (p *OrcaWhirlpoolProtocol) FetchPoolsByPair(ctx context.Context, baseMint string, quoteMint string) ([]pkg.Pool, error) {
	accounts := make([]*rpc.KeyedAccount, 0)

	// Query pools for baseMint -> quoteMint
	programAccounts, err := p.getWhirlpoolAccountsByTokenPair(ctx, baseMint, quoteMint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pools with base token %s: %w", baseMint, err)
	}
	accounts = append(accounts, programAccounts...)

	// Query pools for quoteMint -> baseMint
	programAccounts, err = p.getWhirlpoolAccountsByTokenPair(ctx, quoteMint, baseMint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pools with base token %s: %w", quoteMint, err)
	}
	accounts = append(accounts, programAccounts...)

	res := make([]pkg.Pool, 0)
	for _, v := range accounts {
		data := v.Account.Data.GetBinary()
		layout := &orca.WhirlpoolPool{}
		if err := layout.Decode(data); err != nil {
			continue
		}
		layout.PoolId = v.Pubkey

		// TODO: Add here if need to get other configuration info (like fee rates)

		res = append(res, layout)
	}
	return res, nil
}

// getWhirlpoolAccountsByTokenPair queries Whirlpool accounts for specified token pair
// Reference getCLMMPoolAccountsByTokenPair method from raydiumClmm.go
func (p *OrcaWhirlpoolProtocol) getWhirlpoolAccountsByTokenPair(ctx context.Context, baseMint string, quoteMint string) (rpc.GetProgramAccountsResult, error) {
	baseKey, err := solana.PublicKeyFromBase58(baseMint)
	if err != nil {
		return nil, fmt.Errorf("invalid base mint address: %w", err)
	}
	quoteKey, err := solana.PublicKeyFromBase58(quoteMint)
	if err != nil {
		return nil, fmt.Errorf("invalid quote mint address: %w", err)
	}

	// Whirlpool account discriminator (from external/orca/whirlpool/generated/discriminators.go)
	whirlpoolDiscriminator := [8]byte{63, 149, 209, 12, 225, 128, 99, 9}

	var knownPoolLayout orca.WhirlpoolPool
	result, err := p.SolClient.RpcClient.GetProgramAccountsWithOpts(ctx, orca.ORCA_WHIRLPOOL_PROGRAM_ID, &rpc.GetProgramAccountsOpts{
		Filters: []rpc.RPCFilter{
			{
				// First filter Whirlpool discriminator (ensure only querying Whirlpool accounts)
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: 0, // Discriminator at beginning of account data
					Bytes:  whirlpoolDiscriminator[:],
				},
			},
			{
				DataSize: uint64(knownPoolLayout.Span()),
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: knownPoolLayout.Offset("TokenMintA"), // Note: CLMM uses TokenMint0
					Bytes:  baseKey.Bytes(),
				},
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: knownPoolLayout.Offset("TokenMintB"), // Note: CLMM uses TokenMint1
					Bytes:  quoteKey.Bytes(),
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get pools: %w", err)
	}

	return result, nil
}

// FetchPoolByID gets single Whirlpool pool by pool ID
// Reference raydiumClmm.go implementation
func (p *OrcaWhirlpoolProtocol) FetchPoolByID(ctx context.Context, poolId string) (pkg.Pool, error) {
	poolIdKey, err := solana.PublicKeyFromBase58(poolId)
	if err != nil {
		return nil, fmt.Errorf("invalid pool id: %w", err)
	}

	account, err := p.SolClient.RpcClient.GetAccountInfo(ctx, poolIdKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get pool account %s: %w", poolId, err)
	}

	data := account.Value.Data.GetBinary()
	layout := &orca.WhirlpoolPool{}
	if err := layout.Decode(data); err != nil {
		return nil, fmt.Errorf("failed to decode pool data for %s: %w", poolId, err)
	}
	layout.PoolId = poolIdKey

	return layout, nil
}
