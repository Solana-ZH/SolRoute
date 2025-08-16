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

// OrcaWhirlpoolProtocol 实现 Protocol 接口
type OrcaWhirlpoolProtocol struct {
	SolClient *sol.Client
}

// NewOrcaWhirlpool 创建新的 Orca Whirlpool 协议实例
func NewOrcaWhirlpool(solClient *sol.Client) *OrcaWhirlpoolProtocol {
	return &OrcaWhirlpoolProtocol{
		SolClient: solClient,
	}
}

// FetchPoolsByPair 根据代币对获取 Whirlpool 池列表
// 参考 raydiumClmm.go 的实现，调整字段名映射
func (p *OrcaWhirlpoolProtocol) FetchPoolsByPair(ctx context.Context, baseMint string, quoteMint string) ([]pkg.Pool, error) {
	accounts := make([]*rpc.KeyedAccount, 0)

	// 查询 baseMint -> quoteMint 的池
	programAccounts, err := p.getWhirlpoolAccountsByTokenPair(ctx, baseMint, quoteMint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pools with base token %s: %w", baseMint, err)
	}
	accounts = append(accounts, programAccounts...)

	// 查询 quoteMint -> baseMint 的池
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

		// TODO: 如果需要获取其他配置信息（如费率等），在此处添加

		res = append(res, layout)
	}
	return res, nil
}

// getWhirlpoolAccountsByTokenPair 查询指定代币对的 Whirlpool 账户
// 参考 raydiumClmm.go 的 getCLMMPoolAccountsByTokenPair 方法
func (p *OrcaWhirlpoolProtocol) getWhirlpoolAccountsByTokenPair(ctx context.Context, baseMint string, quoteMint string) (rpc.GetProgramAccountsResult, error) {
	baseKey, err := solana.PublicKeyFromBase58(baseMint)
	if err != nil {
		return nil, fmt.Errorf("invalid base mint address: %w", err)
	}
	quoteKey, err := solana.PublicKeyFromBase58(quoteMint)
	if err != nil {
		return nil, fmt.Errorf("invalid quote mint address: %w", err)
	}

	var knownPoolLayout orca.WhirlpoolPool
	result, err := p.SolClient.RpcClient.GetProgramAccountsWithOpts(ctx, orca.ORCA_WHIRLPOOL_PROGRAM_ID, &rpc.GetProgramAccountsOpts{
		Filters: []rpc.RPCFilter{
			{
				DataSize: uint64(knownPoolLayout.Span()),
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: knownPoolLayout.Offset("TokenMintA"), // 注意：CLMM 用 TokenMint0
					Bytes:  baseKey.Bytes(),
				},
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: knownPoolLayout.Offset("TokenMintB"), // 注意：CLMM 用 TokenMint1
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

// FetchPoolByID 根据池 ID 获取单个 Whirlpool 池
// 参考 raydiumClmm.go 的实现
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
