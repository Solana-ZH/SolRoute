package protocol

import (
	"context"
	"fmt"

	"github.com/Solana-ZH/solroute/pkg"
	"github.com/Solana-ZH/solroute/pkg/pool/pump"
	"github.com/Solana-ZH/solroute/pkg/sol"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type PumpAmmProtocol struct {
	SolClient *sol.Client
}

func NewPumpAmm(solClient *sol.Client) *PumpAmmProtocol {
	return &PumpAmmProtocol{
		SolClient: solClient,
	}
}

func (p *PumpAmmProtocol) FetchPoolsByPair(ctx context.Context, baseMint string, quoteMint string) ([]pkg.Pool, error) {
	programAccounts := rpc.GetProgramAccountsResult{}
	data, err := p.getPumpAMMPoolAccountsByTokenPair(ctx, baseMint, quoteMint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pools with base token %s: %w", baseMint, err)
	}
	programAccounts = append(programAccounts, data...)
	data, err = p.getPumpAMMPoolAccountsByTokenPair(ctx, quoteMint, baseMint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pools with base token %s: %w", quoteMint, err)
	}
	programAccounts = append(programAccounts, data...)

	res := make([]pkg.Pool, 0)
	for _, v := range programAccounts {
		layout, err := pump.ParsePoolData(v.Account.Data.GetBinary())
		if err != nil {
			continue
		}
		layout.PoolId = v.Pubkey
		res = append(res, layout)
	}
	return res, nil
}

func (p *PumpAmmProtocol) getPumpAMMPoolAccountsByTokenPair(ctx context.Context, baseMint string, quoteMint string) (rpc.GetProgramAccountsResult, error) {
	var layout pump.PumpAMMPool
	baseMintPubkey, err := solana.PublicKeyFromBase58(baseMint)
	if err != nil {
		return nil, fmt.Errorf("invalid base mint address: %w", err)
	}
	quoteMintPubkey, err := solana.PublicKeyFromBase58(quoteMint)
	if err != nil {
		return nil, fmt.Errorf("invalid quote mint address: %w", err)
	}

	return p.SolClient.RpcClient.GetProgramAccountsWithOpts(ctx, pump.PumpSwapProgramID, &rpc.GetProgramAccountsOpts{
		Filters: []rpc.RPCFilter{
			{
				DataSize: layout.Span(),
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: layout.Offset("BaseMint"),
					Bytes:  baseMintPubkey.Bytes(),
				},
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: layout.Offset("QuoteMint"),
					Bytes:  quoteMintPubkey.Bytes(),
				},
			},
		},
	})
}

func (p *PumpAmmProtocol) FetchPoolByID(ctx context.Context, poolId string) (pkg.Pool, error) {
	poolPubkey, err := solana.PublicKeyFromBase58(poolId)
	if err != nil {
		return nil, fmt.Errorf("invalid pool ID: %w", err)
	}

	account, err := p.SolClient.RpcClient.GetAccountInfo(ctx, poolPubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to get pool account %s: %w", poolId, err)
	}

	layout, err := pump.ParsePoolData(account.Value.Data.GetBinary())
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool data for pool %s: %w", poolId, err)
	}
	layout.PoolId = poolPubkey
	return layout, nil
}
