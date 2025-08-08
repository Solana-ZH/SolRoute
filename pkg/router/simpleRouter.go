package router

import (
	"context"
	"fmt"
	"log"

	"cosmossdk.io/math"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/yimingWOW/solroute/pkg"
)

type SimpleRouter struct {
	protocols []pkg.Protocol
}

func NewSimpleRouter(protocols ...pkg.Protocol) *SimpleRouter {
	return &SimpleRouter{protocols: protocols}
}

func (r *SimpleRouter) QueryAllPools(ctx context.Context, baseMint, quoteMint string) ([]pkg.Pool, error) {
	var allPools []pkg.Pool
	for _, proto := range r.protocols {
		pools, err := proto.FetchPoolsByPair(ctx, baseMint, quoteMint)
		if err != nil {
			continue
		}
		allPools = append(allPools, pools...)
	}
	return allPools, nil
}

func (r *SimpleRouter) GetBestPool(
	ctx context.Context,
	solClient *rpc.Client,
	tokenIn, tokenOut string,
	amountIn math.Int,
) (pkg.Pool, math.Int, error) {
	var best pkg.Pool
	maxOut := math.NewInt(0)

	for _, p := range r.protocols {
		pools, err := p.FetchPoolsByPair(ctx, tokenIn, tokenOut)
		if err != nil {
			log.Printf("error fetching pools: %v", err)
			continue
		}

		for _, pool := range pools {
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
	}

	if best == nil {
		return nil, math.ZeroInt(), fmt.Errorf("no route found")
	}

	return best, maxOut, nil
}
