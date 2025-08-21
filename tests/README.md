# SolRoute Testing Suite

This test suite is based on the complete logic of `main.go` and provides comprehensive testing for pool query and swap functionalities.

## Test Structure

### Core Test Functions

1.  **TestQueryPoolAndSwap** - The main test function, which fully replicates the logic of `main.go`.
    *   Sets up the test environment and client connection.
    *   Checks and prepares WSOL/USDC token accounts.
    *   Queries for available liquidity pools.
    *   Gets the best quote and route.
    *   Builds the swap instruction.
    *   Sends the actual transaction.
2.  **TestQueryPoolsOnly** - Tests only the pool discovery feature.
    *   Verifies that pools for the USDC/WSOL pair can be discovered.
    *   Does not execute an actual transaction.
3.  **TestGetBestQuote** - Tests the quoting functionality.
    *   Gets the best trade route.
    *   Verifies quote calculation and slippage protection.
    *   Does not send a transaction.
4.  **TestInstructionGeneration** - Tests instruction building.
    *   Verifies the correct generation of the swap instruction.
    *   Checks the instruction structure and parameters.
    *   Does not send a transaction.

### Test Suite Structure

*   **TestSuite** - The test suite struct, which includes:
    *   `ctx` - Context
    *   `privateKey` - Private key
    *   `solClient` - Solana client
    *   `router` - Router instance
*   **setupTestSuite()** - Initializes the test environment.
*   **teardownTestSuite()** - Cleans up resources.
*   **setupTokenAccounts()** - Prepares token accounts.

## Running Tests

### Environment Setup

Set the necessary environment variables:

```bash
# Required
export SOLANA_PRIVATE_KEY="your_private_key"

# Optional (with default values)
export SOLANA_RPC_URL="https://api.mainnet-beta.solana.com"
export SOLANA_WS_RPC_URL="wss://api.mainnet-beta.solana.com"
```

Or on Windows:

```cmd
set SOLANA_PRIVATE_KEY=your_private_key
set SOLANA_RPC_URL=https://api.mainnet-beta.solana.com
set SOLANA_WS_RPC_URL=wss://api.mainnet-beta.solana.com
```

### Run Commands

```bash
# Run all tests
go test ./tests

# Run a specific test
go test ./tests -run TestQueryPoolsOnly

# Run tests with verbose output
go test -v ./tests

# Run only the main swap test (will execute a real transaction)
go test -v ./tests -run TestQueryPoolAndSwap
```

### Test Scenarios

1.  **Full Swap Test** (`TestQueryPoolAndSwap`)
    *   For a real transaction, change the last parameter of the `SendTx` call in the test class to `false` (default is `true`).
    *   Executes a real WSOL → USDC swap.
    *   Requires the wallet to have a sufficient SOL balance.
    *   Will incur real transaction fees.
2.  **Safe Tests** (`TestQueryPoolsOnly`, `TestGetBestQuote`, `TestInstructionGeneration`)
    *   Only tests functional logic without executing actual transactions.
    *   Suitable for development and debugging phases.

## Test Configuration

*   **Default Transaction Amount**: 1,000,000 lamports (1 SOL)
*   **Slippage Protection**: 100 bps (1%)
*   **Token Pair**: WSOL/USDC
*   **Supported Protocol**: Orca Whirlpool V2

## Important Notes

1.  **Network Connection**: Tests require a stable connection to the mainnet RPC.
2.  **Balance Requirement**: The wallet needs enough SOL for transaction fees and the test swap.
3.  **Real Transactions**: `TestQueryPoolAndSwap` will execute a real transaction.
4.  **Protocol Selection**: Currently, only Orca Whirlpool is tested. This can be adjusted as needed.

## Troubleshooting

*   **Connection Failed**: Check the RPC endpoint and your network connection.
*   **Insufficient Balance**: Ensure the wallet has enough SOL.
*   **Incorrect Private Key**: Verify the `SOLANA_PRIVATE_KEY` environment variable is set correctly.
*   **Pool Discovery Failed**: Check the network connection and the protocol's status.
