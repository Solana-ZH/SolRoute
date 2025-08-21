package pump

import (
	"fmt"

	"github.com/Solana-ZH/solroute/pkg/sol"
	"github.com/gagliardetto/solana-go"
)

const (
	// CreatorVaultSeed is used for deriving the vault authority PDA
	CreatorVaultSeed = "creator_vault"
)

// GetCoinCreatorVaultAuthority derives the Program Derived Address (PDA) for the coin creator's vault authority
func GetCoinCreatorVaultAuthority(coinCreator solana.PublicKey) (solana.PublicKey, error) {
	if coinCreator.IsZero() {
		return solana.PublicKey{}, fmt.Errorf("invalid coin creator public key")
	}

	seeds := [][]byte{
		[]byte(CreatorVaultSeed),
		coinCreator.Bytes(),
	}

	pda, _, err := solana.FindProgramAddress(seeds, PumpSwapProgramID)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to find program address: %w", err)
	}

	return pda, nil
}

// GetCoinCreatorVaultATA derives the Associated Token Account (ATA) for the coin creator's vault authority
func GetCoinCreatorVaultATA(coinCreator solana.PublicKey) (solana.PublicKey, error) {
	if coinCreator.IsZero() {
		return solana.PublicKey{}, fmt.Errorf("invalid coin creator public key")
	}

	creatorVaultAuthority, err := GetCoinCreatorVaultAuthority(coinCreator)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to get vault authority: %w", err)
	}

	ata, _, err := solana.FindAssociatedTokenAddress(
		creatorVaultAuthority, // owner
		sol.WSOL,              // mint
	)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to find associated token address: %w", err)
	}

	return ata, nil
}
