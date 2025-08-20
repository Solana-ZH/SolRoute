package protocol

import (
	"context"
	"fmt"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/yimingWOW/solroute/pkg"
	"github.com/yimingWOW/solroute/pkg/pool/raydium"
	"github.com/yimingWOW/solroute/pkg/sol"
)

type RaydiumClmmProtocol struct {
	SolClient *sol.Client
}

func NewRaydiumClmm(solClient *sol.Client) *RaydiumClmmProtocol {
	return &RaydiumClmmProtocol{
		SolClient: solClient,
	}
}

func (p *RaydiumClmmProtocol) FetchPoolsByPair(ctx context.Context, baseMint string, quoteMint string) ([]pkg.Pool, error) {
	accounts := make([]*rpc.KeyedAccount, 0)
	programAccounts, err := p.getCLMMPoolAccountsByTokenPair(ctx, baseMint, quoteMint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pools with base token %s: %w", baseMint, err)
	}
	accounts = append(accounts, programAccounts...)
	programAccounts, err = p.getCLMMPoolAccountsByTokenPair(ctx, quoteMint, baseMint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pools with base token %s: %w", quoteMint, err)
	}
	accounts = append(accounts, programAccounts...)

	res := make([]pkg.Pool, 0)
	for _, v := range accounts {
		data := v.Account.Data.GetBinary()
		layout := &raydium.CLMMPool{}
		if err := layout.Decode(data); err != nil {
			continue
		}
		layout.PoolId = v.Pubkey

		ammConfigData, err := p.SolClient.RpcClient.GetAccountInfo(ctx, layout.AmmConfig)
		if err != nil {
			continue
		}
		feeRate, err := parseAmmConfig(ammConfigData.Value.Data.GetBinary())
		if err != nil {
			continue
		}
		layout.FeeRate = feeRate

		exBitmapAddress, _, err := raydium.GetPdaExBitmapAccount(raydium.RAYDIUM_CLMM_PROGRAM_ID, layout.PoolId)
		if err != nil {
			continue
		}
		layout.ExBitmapAddress = exBitmapAddress

		// Check if pool has Swap functionality enabled, only add to results if enabled
		if !layout.IsSwapEnabled() {
			continue
		}

		// We don't set user accounts here, leave it to BuildSwapInstructions to handle
		res = append(res, layout)
	}
	return res, nil
}

func (p *RaydiumClmmProtocol) getCLMMPoolAccountsByTokenPair(ctx context.Context, baseMint string, quoteMint string) (rpc.GetProgramAccountsResult, error) {
	baseKey, err := solana.PublicKeyFromBase58(baseMint)
	if err != nil {
		return nil, fmt.Errorf("invalid base mint address: %w", err)
	}
	quoteKey, err := solana.PublicKeyFromBase58(quoteMint)
	if err != nil {
		return nil, fmt.Errorf("invalid quote mint address: %w", err)
	}

	var knownPoolLayout raydium.CLMMPool
	result, err := p.SolClient.RpcClient.GetProgramAccountsWithOpts(ctx, raydium.RAYDIUM_CLMM_PROGRAM_ID, &rpc.GetProgramAccountsOpts{
		Filters: []rpc.RPCFilter{
			{
				DataSize: uint64(knownPoolLayout.Span()),
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: knownPoolLayout.Offset("TokenMint0"),
					Bytes:  baseKey.Bytes(),
				},
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: knownPoolLayout.Offset("TokenMint1"),
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

func (r *RaydiumClmmProtocol) FetchPoolByID(ctx context.Context, poolId string) (pkg.Pool, error) {
	poolIdKey, err := solana.PublicKeyFromBase58(poolId)
	if err != nil {
		return nil, fmt.Errorf("invalid pool id: %w", err)
	}
	account, err := r.SolClient.RpcClient.GetAccountInfo(ctx, poolIdKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get pool account %s: %w", poolId, err)
	}

	data := account.Value.Data.GetBinary()
	layout := &raydium.CLMMPool{}
	if err := layout.Decode(data); err != nil {
		return nil, fmt.Errorf("failed to decode pool data for %s: %w", poolId, err)
	}
	layout.PoolId = poolIdKey

	// Check if pool has Swap functionality enabled
	if !layout.IsSwapEnabled() {
		return nil, fmt.Errorf("pool %s has swap functionality disabled", poolId)
	}

	return layout, nil
}

func parseAmmConfig(data []byte) (uint32, error) {
	var ammConfig AmmConfig
	if err := ammConfig.Decode(data); err != nil {
		return 0, fmt.Errorf("failed to decode amm config: %w", err)
	}
	return ammConfig.TradeFeeRate, nil
}

type AmmConfig struct {
	Bump            uint8
	Index           uint16
	Owner           solana.PublicKey
	ProtocolFeeRate uint32
	TradeFeeRate    uint32
	TickSpacing     uint16
	FundFeeRate     uint32
	PaddingU32      uint32
	FundOwner       solana.PublicKey
	Padding         [3]uint64
}

func (l *AmmConfig) Decode(data []byte) error {
	// Skip 8 bytes discriminator if present
	if len(data) > 8 {
		data = data[8:]
	}

	dec := bin.NewBinDecoder(data)
	return dec.Decode(l)
}
