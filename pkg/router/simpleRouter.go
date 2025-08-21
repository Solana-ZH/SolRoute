package router

import (
	"context"
	"fmt"
	"log"

	"cosmossdk.io/math"
	"github.com/Solana-ZH/solroute/pkg"
	"github.com/gagliardetto/solana-go/rpc"
)

type SimpleRouter struct {
	protocols []pkg.Protocol
	pools     []pkg.Pool
}

func NewSimpleRouter(protocols ...pkg.Protocol) *SimpleRouter {
	return &SimpleRouter{
		protocols: protocols,
		pools:     []pkg.Pool{},
	}
}

func (r *SimpleRouter) QueryAllPools(ctx context.Context, baseMint, quoteMint string) ([]pkg.Pool, error) {
	for _, proto := range r.protocols {
		pools, err := proto.FetchPoolsByPair(ctx, baseMint, quoteMint)
		if err != nil {
			continue
		}
		r.pools = append(r.pools, pools...)
	}
	return r.pools, nil
}

func (r *SimpleRouter) GetBestPool(ctx context.Context, solClient *rpc.Client, tokenIn, tokenOut string, amountIn math.Int) (pkg.Pool, math.Int, error) {
	var best pkg.Pool
	maxOut := math.NewInt(0)
	for _, pool := range r.pools {
		outAmount, err := pool.Quote(ctx, solClient, tokenIn, amountIn)
		if err != nil {
			log.Printf("error quoting: %v", err)
			continue
		}
		if outAmount.GT(maxOut) {
			maxOut = outAmount
			best = pool
		}
	}
	if best == nil {
		return nil, math.ZeroInt(), fmt.Errorf("no route found")
	}
	return best, maxOut, nil
}
