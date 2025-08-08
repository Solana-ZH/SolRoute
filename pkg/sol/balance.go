package sol

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func (t *Client) GetUserTokenBalance(ctx context.Context, userAddr solana.PublicKey, tokenMint solana.PublicKey) (uint64, error) {
	acc, err := t.RpcClient.GetTokenAccountsByOwner(ctx, userAddr,
		&rpc.GetTokenAccountsConfig{Mint: tokenMint.ToPointer()},
		&rpc.GetTokenAccountsOpts{
			Encoding: "jsonParsed",
		},
	)
	if err != nil {
		return 0, err
	}
	if len(acc.Value) == 0 {
		return 0, errors.New("no token account found")
	}

	tokenAccount, err := t.RpcClient.GetTokenAccountBalance(ctx, acc.Value[0].Pubkey, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, fmt.Errorf("failed to get token account balance: %v", err)
	}
	tokenAmt, err := strconv.ParseUint(tokenAccount.Value.Amount, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse token amount: %w", err)
	}

	return tokenAmt, nil
}
