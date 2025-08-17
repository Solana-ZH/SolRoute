package main

import (
	"context"
	"log"

	"cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/yimingWOW/solroute/pkg/protocol"
	"github.com/yimingWOW/solroute/pkg/router"
	"github.com/yimingWOW/solroute/pkg/sol"
)

const (
	privateKeyStr = ""
	// RPC endpoints
	mainnetRPC   = ""
	mainnetWSRPC = ""

	// Token addresses
	usdcTokenAddr = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	// Swap parameters
	defaultAmountIn = 1000000000 // 1 sol (9 decimals)
	slippageBps     = 100        // 1% slippage
)

func main() {
	// TODO: Initialize private key from environment or config file
	privateKey := solana.MustPrivateKeyFromBase58(privateKeyStr)
	log.Printf("PublicKey: %v", privateKey.PublicKey())

	ctx := context.Background()
	solClient, err := sol.NewClient(ctx, mainnetRPC, mainnetWSRPC)
	if err != nil {
		log.Fatalf("Failed to create solana client: %v", err)
	}
	defer solClient.Close()

	// check balance first
	balance, err := solClient.GetUserTokenBalance(ctx, privateKey.PublicKey(), sol.WSOL)
	if err != nil {
		log.Fatalf("Failed to get user token balance: %v", err)
	}
	log.Printf("User token balance: %v", balance)
	if balance < 10000000 {
		err = solClient.CoverWsol(ctx, privateKey, 10000000)
		if err != nil {
			log.Fatalf("Failed to cover wsol: %v", err)
		}
	}

	tokenAccount, err := solClient.SelectOrCreateSPLTokenAccount(ctx, privateKey, solana.MustPublicKeyFromBase58(usdcTokenAddr))
	if err != nil {
		log.Fatalf("Failed to get user token balance: %v", err)
	}
	log.Printf("USDC token account: %v", tokenAccount.String())

	router := router.NewSimpleRouter(
		protocol.NewPumpAmm(solClient),
		protocol.NewRaydiumAmm(solClient),
		protocol.NewRaydiumClmm(solClient),
		protocol.NewRaydiumCpmm(solClient),
		protocol.NewMeteoraDlmm(solClient),
		protocol.NewOrcaWhirlpool(solClient),
	)

	// Query available pools
	pools, err := router.QueryAllPools(ctx, usdcTokenAddr, sol.WSOL.String())
	if err != nil {
		log.Fatalf("Failed to query all pools: %v", err)
	}
	for _, pool := range pools {
		log.Printf("Found pool: %v", pool.GetID())
	}

	// Find best pool for the swap
	amountIn := math.NewInt(defaultAmountIn)
	bestPool, amountOut, err := router.GetBestPool(ctx, solClient.RpcClient, sol.WSOL.String(), usdcTokenAddr, amountIn)
	if err != nil {
		log.Fatalf("Failed to get best pool: %v", err)
	}
	log.Printf("Selected best pool: %v", bestPool.GetID())
	log.Printf("Expected output amount: %v", amountOut)

	// Calculate minimum output amount with slippage
	minAmountOut := amountOut.Mul(math.NewInt(10000 - slippageBps)).Quo(math.NewInt(10000))
	log.Printf("Amount out: %s, Min amount out: %s (slippage: %d bps)", amountOut.String(), minAmountOut.String(), slippageBps)

	// Build swap instructions (注意: 我们是用 WSOL 换 USDC)
	instructions, err := bestPool.BuildSwapInstructions(ctx, solClient.RpcClient,
		privateKey.PublicKey(), sol.WSOL.String(), amountIn, minAmountOut)
	if err != nil {
		log.Fatalf("Failed to build swap instructions: %v", err)
	}
	log.Printf("Generated swap instructions: %v", instructions)

	// Prepare transaction
	signers := []solana.PrivateKey{privateKey}
	res, err := solClient.RpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		log.Fatalf("Failed to get blockhash: %v", err)
	}

	// Send transaction
	sig, err := solClient.SendTx(ctx, res.Value.Blockhash, signers, instructions, true)
	if err != nil {
		log.Fatalf("Failed to send transaction: %v", err)
	}
	log.Printf("Transaction successful: https://solscan.io/tx/%v", sig)
}
