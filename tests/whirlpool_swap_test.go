package tests

import (
	"context"
	"os"
	"testing"

	"cosmossdk.io/math"
	"github.com/Solana-ZH/solroute/pkg/protocol"
	"github.com/Solana-ZH/solroute/pkg/router"
	"github.com/Solana-ZH/solroute/pkg/sol"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Token addresses
	usdcTokenAddr = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	// Swap parameters
	defaultAmountIn = 1000000 // 1 sol (9 decimals) - same as main.go
	slippageBps     = 100     // 1% slippage protection
)

type TestSuite struct {
	ctx        context.Context
	privateKey solana.PrivateKey
	solClient  *sol.Client
	router     *router.SimpleRouter
}

// setupTestSuite initializes test environment and creates Solana client
func setupTestSuite(t *testing.T) *TestSuite {
	// Get private key from environment variable
	privateKeyStr := os.Getenv("SOLANA_PRIVATE_KEY")
	require.NotEmpty(t, privateKeyStr, "SOLANA_PRIVATE_KEY environment variable is required")

	privateKey := solana.MustPrivateKeyFromBase58(privateKeyStr)
	t.Logf("PublicKey: %v", privateKey.PublicKey())

	ctx := context.Background()

	// Get RPC endpoints from environment variables
	mainnetRPC := os.Getenv("SOLANA_RPC_URL")
	if mainnetRPC == "" {
		mainnetRPC = "https://api.mainnet-beta.solana.com"
	}

	mainnetWSRPC := os.Getenv("SOLANA_WS_RPC_URL")
	if mainnetWSRPC == "" {
		mainnetWSRPC = "wss://api.mainnet-beta.solana.com"
	}

	solClient, err := sol.NewClient(ctx, mainnetRPC, mainnetWSRPC)
	require.NoError(t, err, "Failed to create solana client")

	// Initialize router with Orca Whirlpool protocol (same as main.go)
	testRouter := router.NewSimpleRouter(
		protocol.NewOrcaWhirlpool(solClient),
	)

	return &TestSuite{
		ctx:        ctx,
		privateKey: privateKey,
		solClient:  solClient,
		router:     testRouter,
	}
}

// teardownTestSuite cleans up resources after testing
func (ts *TestSuite) teardownTestSuite() {
	if ts.solClient != nil {
		ts.solClient.Close()
	}
}

// setupTokenAccounts prepares WSOL and USDC token accounts
func (ts *TestSuite) setupTokenAccounts(t *testing.T) solana.PublicKey {
	// Check WSOL balance and cover if necessary
	balance, err := ts.solClient.GetUserTokenBalance(ts.ctx, ts.privateKey.PublicKey(), sol.WSOL)
	require.NoError(t, err, "Failed to get user token balance")
	t.Logf("User WSOL balance: %v", balance)

	if balance < 10000000 {
		err = ts.solClient.CoverWsol(ts.ctx, ts.privateKey, 10000000)
		require.NoError(t, err, "Failed to cover wsol")
	}

	// Get or create USDC token account
	tokenAccount, err := ts.solClient.SelectOrCreateSPLTokenAccount(ts.ctx, ts.privateKey, solana.MustPublicKeyFromBase58(usdcTokenAddr))
	require.NoError(t, err, "Failed to get USDC token account")
	t.Logf("USDC token account: %v", tokenAccount.String())

	return tokenAccount
}

// TestQueryPoolAndSwap is the main test function that replicates main.go logic
func TestQueryPoolAndSwap(t *testing.T) {
	// Setup test environment
	ts := setupTestSuite(t)
	defer ts.teardownTestSuite()

	// Setup token accounts
	usdcTokenAccount := ts.setupTokenAccounts(t)
	assert.NotEqual(t, solana.PublicKey{}, usdcTokenAccount, "USDC token account should not be empty")

	// Query available pools
	pools, err := ts.router.QueryAllPools(ts.ctx, usdcTokenAddr, sol.WSOL.String())
	require.NoError(t, err, "Failed to query all pools")
	require.NotEmpty(t, pools, "Should find at least one pool")

	for _, pool := range pools {
		t.Logf("Found pool: %v", pool.GetID())
	}

	// Find best pool for the swap
	amountIn := math.NewInt(defaultAmountIn)
	bestPool, amountOut, err := ts.router.GetBestPool(ts.ctx, ts.solClient.RpcClient, sol.WSOL.String(), usdcTokenAddr, amountIn)
	require.NoError(t, err, "Failed to get best pool")
	require.NotNil(t, bestPool, "Best pool should not be nil")
	require.True(t, amountOut.GT(math.ZeroInt()), "Amount out should be greater than zero")

	t.Logf("Selected best pool: %v", bestPool.GetID())
	t.Logf("Expected output amount: %v", amountOut)

	// Calculate minimum output amount with slippage
	minAmountOut := amountOut.Mul(math.NewInt(10000 - slippageBps)).Quo(math.NewInt(10000))
	t.Logf("Amount out: %s, Min amount out: %s (slippage: %d bps)", amountOut.String(), minAmountOut.String(), slippageBps)

	// Build swap instructions (swapping WSOL for USDC)
	instructions, err := bestPool.BuildSwapInstructions(ts.ctx, ts.solClient.RpcClient,
		ts.privateKey.PublicKey(), sol.WSOL.String(), amountIn, minAmountOut)
	require.NoError(t, err, "Failed to build swap instructions")
	require.NotEmpty(t, instructions, "Should generate at least one instruction")

	t.Logf("Generated swap instructions count: %v", len(instructions))

	// Prepare transaction components
	signers := []solana.PrivateKey{ts.privateKey}
	res, err := ts.solClient.RpcClient.GetLatestBlockhash(ts.ctx, rpc.CommitmentFinalized)
	require.NoError(t, err, "Failed to get blockhash")

	// Send transaction (this will execute the actual swap)
	sig, err := ts.solClient.SendTx(ts.ctx, res.Value.Blockhash, signers, instructions, true)
	require.NoError(t, err, "Failed to send transaction")
	require.NotEmpty(t, sig, "Transaction signature should not be empty")

	t.Logf("Transaction successful: https://solscan.io/tx/%v", sig)
}

// TestQueryPoolsOnly tests pool discovery without executing swap
func TestQueryPoolsOnly(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.teardownTestSuite()

	// Query available pools without executing swap
	pools, err := ts.router.QueryAllPools(ts.ctx, usdcTokenAddr, sol.WSOL.String())
	require.NoError(t, err, "Failed to query all pools")

	t.Logf("Total pools found: %d", len(pools))

	for i, pool := range pools {
		t.Logf("Pool %d: %v", i+1, pool.GetID())
	}

	// Verify we found pools
	assert.NotEmpty(t, pools, "Should discover at least one pool for USDC/WSOL pair")
}

// TestGetBestQuote tests the quote functionality without executing swap
func TestGetBestQuote(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.teardownTestSuite()

	// Query pools first
	pools, err := ts.router.QueryAllPools(ts.ctx, usdcTokenAddr, sol.WSOL.String())
	require.NoError(t, err, "Failed to query all pools")
	require.NotEmpty(t, pools, "Should find at least one pool")

	// Test quote functionality
	amountIn := math.NewInt(defaultAmountIn)
	bestPool, amountOut, err := ts.router.GetBestPool(ts.ctx, ts.solClient.RpcClient, sol.WSOL.String(), usdcTokenAddr, amountIn)
	require.NoError(t, err, "Failed to get best pool")
	require.NotNil(t, bestPool, "Best pool should not be nil")

	t.Logf("Best pool ID: %v", bestPool.GetID())
	t.Logf("Input amount: %v WSOL", amountIn)
	t.Logf("Expected output: %v USDC", amountOut)

	// Validate quote makes sense
	assert.True(t, amountOut.GT(math.ZeroInt()), "Output amount should be positive")

	// Calculate slippage protection
	minAmountOut := amountOut.Mul(math.NewInt(10000 - slippageBps)).Quo(math.NewInt(10000))
	assert.True(t, minAmountOut.GT(math.ZeroInt()), "Min amount out should be positive")
	assert.True(t, minAmountOut.LT(amountOut), "Min amount out should be less than expected amount")
}

// TestInstructionGeneration tests swap instruction building without sending transaction
func TestInstructionGeneration(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.teardownTestSuite()

	// Setup token accounts
	_ = ts.setupTokenAccounts(t)

	// Get best pool and quote
	amountIn := math.NewInt(defaultAmountIn)
	bestPool, amountOut, err := ts.router.GetBestPool(ts.ctx, ts.solClient.RpcClient, sol.WSOL.String(), usdcTokenAddr, amountIn)
	require.NoError(t, err, "Failed to get best pool")
	require.NotNil(t, bestPool, "Best pool should not be nil")

	// Calculate minimum output with slippage
	minAmountOut := amountOut.Mul(math.NewInt(10000 - slippageBps)).Quo(math.NewInt(10000))

	// Build swap instructions
	instructions, err := bestPool.BuildSwapInstructions(ts.ctx, ts.solClient.RpcClient,
		ts.privateKey.PublicKey(), sol.WSOL.String(), amountIn, minAmountOut)
	require.NoError(t, err, "Failed to build swap instructions")
	require.NotEmpty(t, instructions, "Should generate instructions")

	t.Logf("Generated %d instructions for swap", len(instructions))

	// Validate instructions
	for i, instr := range instructions {
		assert.NotNil(t, instr, "Instruction %d should not be nil", i)
		t.Logf("Instruction %d: Program ID %v, %d accounts", i, instr.ProgramID(), len(instr.Accounts()))
	}
}
