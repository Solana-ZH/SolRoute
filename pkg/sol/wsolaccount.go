package sol

import (
	"context"
	"log"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

func (t *Client) CoverWsol(ctx context.Context, privateKey solana.PrivateKey, amount int64) error {
	var signers []solana.PrivateKey
	signers = append(signers, privateKey)

	allInstrs := make([]solana.Instruction, 0)
	user := privateKey.PublicKey()

	acc, err := t.RpcClient.GetTokenAccountsByOwner(ctx, user,
		&rpc.GetTokenAccountsConfig{Mint: WSOL.ToPointer()},
		&rpc.GetTokenAccountsOpts{
			Encoding: "jsonParsed",
		},
	)
	if err != nil {
		log.Printf("GetTokenAccountsByOwner err: %v", err)
		return err
	}
	if len(acc.Value) == 0 {
		createAtaInst, err := associatedtokenaccount.NewCreateInstruction(
			user,
			user,
			WSOL,
		).ValidateAndBuild()
		if err != nil {
			return err
		}
		allInstrs = append(allInstrs, createAtaInst)
	}

	wsolAccount, _, err := solana.FindAssociatedTokenAddress(user, WSOL)
	if err != nil {
		log.Printf("FindAssociatedTokenAddress err: %v", err)
		return err
	}

	transferInst, err := system.NewTransferInstruction(
		uint64(amount),
		user,
		wsolAccount,
	).ValidateAndBuild()
	if err != nil {
		log.Printf("NewTransferInstruction err: %v", err)
		return err
	}
	allInstrs = append(allInstrs, transferInst)

	// Add SyncNative instruction for WSOL
	syncNativeInst, err := token.NewSyncNativeInstruction(
		wsolAccount,
	).ValidateAndBuild()
	if err != nil {
		return err
	}
	allInstrs = append(allInstrs, syncNativeInst)

	recent, err := t.RpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		log.Printf("GetLatestBlockhash err: %v\n", err)
		return err
	}
	_, err = t.SendTx(ctx, recent.Value.Blockhash, signers, allInstrs, false)
	if err != nil {
		log.Printf("Failed to send transaction: %v\n", err)
		return err
	}
	return nil
}

func (t *Client) CloseWsol(ctx context.Context, privateKey solana.PrivateKey) error {
	var signers []solana.PrivateKey
	signers = append(signers, privateKey)
	user := privateKey.PublicKey()
	insts := make([]solana.Instruction, 0)

	wsolAccount, _, err := solana.FindAssociatedTokenAddress(user, WSOL)
	if err != nil {
		log.Printf("FindAssociatedTokenAddress err: %v", err)
		return err
	}
	closeInst, err := token.NewCloseAccountInstruction(
		wsolAccount,
		user,
		user,
		[]solana.PublicKey{},
	).ValidateAndBuild()
	if err != nil {
		log.Printf("CloseAccountInstruction err: %v\n", err)
		return err
	}
	insts = append(insts, closeInst)

	recent, err := t.RpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		log.Printf("GetLatestBlockhash err: %v\n", err)
		return err
	}
	_, err = t.SendTx(ctx, recent.Value.Blockhash, signers, insts, false)
	if err != nil {
		log.Printf("Failed to send transaction: %v\n", err)
		return err
	}
	return nil
}
