package protocol

import (
	"context"
	"fmt"

	"github.com/Solana-ZH/solroute/pkg"
	"github.com/Solana-ZH/solroute/pkg/pool/raydium"
	"github.com/Solana-ZH/solroute/pkg/sol"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// RaydiumCpmmProtocol represents the Raydium CPMM protocol implementation
type RaydiumCpmmProtocol struct {
	SolClient *sol.Client
}

// NewRaydiumCpmm creates a new instance of RaydiumCpmmProtocol
func NewRaydiumCpmm(solClient *sol.Client) *RaydiumCpmmProtocol {
	return &RaydiumCpmmProtocol{
		SolClient: solClient,
	}
}

// FetchPoolsByPair retrieves all pools for a given token pair
func (p *RaydiumCpmmProtocol) FetchPoolsByPair(ctx context.Context, baseMint string, quoteMint string) ([]pkg.Pool, error) {
	// Fetch pools with baseMint as token0
	programAccounts, err := p.getCPMMPoolAccountsByTokenPair(ctx, baseMint, quoteMint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pools with base token %s: %w", baseMint, err)
	}

	pools := make([]pkg.Pool, 0)
	for _, account := range programAccounts {
		data := account.Account.Data.GetBinary()
		pool := &raydium.CPMMPool{}
		if err := pool.Decode(data); err != nil {
			continue
		}
		pool.PoolId = account.Pubkey
		pools = append(pools, pool)
	}

	// Fetch pools with quoteMint as token0
	programAccounts, err = p.getCPMMPoolAccountsByTokenPair(ctx, quoteMint, baseMint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pools with base token %s: %w", quoteMint, err)
	}

	for _, account := range programAccounts {
		data := account.Account.Data.GetBinary()
		pool := &raydium.CPMMPool{}
		if err := pool.Decode(data); err != nil {
			continue
		}
		pool.PoolId = account.Pubkey
		pools = append(pools, pool)
	}

	return pools, nil
}

// getCPMMPoolAccountsByTokenPair retrieves CPMM pool accounts for a given token pair
func (p *RaydiumCpmmProtocol) getCPMMPoolAccountsByTokenPair(ctx context.Context, baseMint string, quoteMint string) (rpc.GetProgramAccountsResult, error) {
	baseKey, err := solana.PublicKeyFromBase58(baseMint)
	if err != nil {
		return nil, fmt.Errorf("invalid base mint address: %w", err)
	}

	quoteKey, err := solana.PublicKeyFromBase58(quoteMint)
	if err != nil {
		return nil, fmt.Errorf("invalid quote mint address: %w", err)
	}

	var layout raydium.CPMMPool
	filters := []rpc.RPCFilter{
		{
			DataSize: 637,
		},
		{
			Memcmp: &rpc.RPCFilterMemcmp{
				Offset: layout.Offset("Token0Mint"),
				Bytes:  baseKey.Bytes(),
			},
		},
		{
			Memcmp: &rpc.RPCFilterMemcmp{
				Offset: layout.Offset("Token1Mint"),
				Bytes:  quoteKey.Bytes(),
			},
		},
	}

	result, err := p.SolClient.RpcClient.GetProgramAccountsWithOpts(ctx, raydium.RAYDIUM_CPMM_PROGRAM_ID, &rpc.GetProgramAccountsOpts{
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get pools: %w", err)
	}

	return result, nil
}

// FetchPoolByID retrieves a CPMM pool by its ID
func (p *RaydiumCpmmProtocol) FetchPoolByID(ctx context.Context, poolID string) (pkg.Pool, error) {
	account, err := p.SolClient.RpcClient.GetAccountInfo(ctx, solana.MustPublicKeyFromBase58(poolID))
	if err != nil {
		return nil, fmt.Errorf("failed to get pool account %s: %w", poolID, err)
	}

	pool := &raydium.CPMMPool{}
	if err := pool.Decode(account.Value.Data.GetBinary()); err != nil {
		return nil, fmt.Errorf("failed to decode pool data for %s: %w", poolID, err)
	}
	pool.PoolId = solana.MustPublicKeyFromBase58(poolID)

	return pool, nil
}
