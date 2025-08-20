package orca

import (
	"math/big"

	"cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
)

// Program IDs
var (
	// Orca Whirlpool Program ID
	ORCA_WHIRLPOOL_PROGRAM_ID = solana.MustPublicKeyFromBase58("whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc")

	// Standard Solana Program IDs
	TOKEN_PROGRAM_ID      = solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	TOKEN_2022_PROGRAM_ID = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
	MEMO_PROGRAM_ID       = solana.MustPublicKeyFromBase58("MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr")
)

// Tick Array Configuration - Based on Orca Whirlpool specification
const (
	TICK_ARRAY_SIZE                 = 88  // Whirlpool uses 88 instead of CLMM's 60
	TickSize                        = 168 // Tick size remains the same
	TICK_ARRAY_BITMAP_SIZE          = 512 // Bitmap size remains the same
	MAX_TICK                        = 443636
	MIN_TICK                        = -443636
	EXTENSION_TICKARRAY_BITMAP_SIZE = 14
	U64Resolution                   = 64
)

// Price Constants - Reuse CLMM price limit constants
var (
	MIN_SQRT_PRICE_X64    = math.NewIntFromBigInt(big.NewInt(4295048016))
	MAX_SQRT_PRICE_X64, _ = math.NewIntFromString("79226673521066979257578248091")
	FEE_RATE_DENOMINATOR  = math.NewInt(int64(1000000))
)

// Liquidity Constants - Whirlpool may have different fee structure
var (
	LIQUIDITY_FEES_NUMERATOR   = math.NewInt(25)
	LIQUIDITY_FEES_DENOMINATOR = math.NewInt(10000)
)

// Seeds and Discriminators - Whirlpool-specific seeds and discriminators
var (
	// Whirlpool account seed
	WHIRLPOOL_SEED = "whirlpool"

	// Whirlpool Swap instruction discriminator (from IDL)
	SwapDiscriminator = []byte{248, 198, 158, 145, 225, 117, 135, 200}
	// Whirlpool Swap V2 instruction discriminator (from IDL)
	SwapV2Discriminator = []byte{43, 4, 237, 11, 26, 201, 30, 98} // Need to verify from actual IDL

	// Other common seeds
	TICK_ARRAY_SEED = "tick_array"
	POSITION_SEED   = "position"
)

// Whirlpool-specific constants
const (
	// Whirlpool account data size (653 bytes including discriminator)
	WHIRLPOOL_SIZE = 653

	// Whirlpool supported tick spacing list
	TICK_SPACING_STABLE   = 1   // Stable coin pairs
	TICK_SPACING_STANDARD = 64  // Standard token pairs
	TICK_SPACING_VOLATILE = 128 // Volatile token pairs
)

// Mathematical calculation constants
var (
	// Q64 format constant (2^64)
	Q64 = math.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 64))

	// Q128 format constant (2^128)
	Q128 = math.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 128))

	// Zero value constants
	ZERO_INT = math.NewInt(0)
	ONE_INT  = math.NewInt(1)
)
