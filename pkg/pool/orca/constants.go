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

// Tick Array Configuration - 参考 Orca Whirlpool 规范
const (
	TICK_ARRAY_SIZE                 = 88  // Whirlpool 使用 88 而不是 CLMM 的 60
	TickSize                        = 168 // Tick 大小保持相同
	TICK_ARRAY_BITMAP_SIZE          = 512 // 位图大小保持相同
	MAX_TICK                        = 443636
	MIN_TICK                        = -443636
	EXTENSION_TICKARRAY_BITMAP_SIZE = 14
	U64Resolution                   = 64
)

// Price Constants - 复用 CLMM 的价格限制常量
var (
	MIN_SQRT_PRICE_X64    = math.NewIntFromBigInt(big.NewInt(4295048016))
	MAX_SQRT_PRICE_X64, _ = math.NewIntFromString("79226673521066979257578248091")
	FEE_RATE_DENOMINATOR  = math.NewInt(int64(1000000))
)

// Liquidity Constants - Whirlpool 可能有不同的费用结构
var (
	LIQUIDITY_FEES_NUMERATOR   = math.NewInt(25)
	LIQUIDITY_FEES_DENOMINATOR = math.NewInt(10000)
)

// Seeds and Discriminators - Whirlpool 特有的种子和判别器
var (
	// Whirlpool 账户种子
	WHIRLPOOL_SEED = "whirlpool"

	// Whirlpool Swap 指令判别器 (从 IDL 中获取)
	SwapDiscriminator = []byte{248, 198, 158, 145, 225, 117, 135, 200}
	// Whirlpool Swap V2 指令判别器 (从 IDL 中获取)
	SwapV2Discriminator = []byte{43, 4, 237, 11, 26, 201, 30, 98} // 需要从实际 IDL 验证

	// 其他常用种子
	TICK_ARRAY_SEED = "tick_array"
	POSITION_SEED   = "position"
)

// Whirlpool 特有常量
const (
	// Whirlpool 账户数据大小 (653 字节包含 discriminator)
	WHIRLPOOL_SIZE = 653

	// Whirlpool 支持的 tick spacing 列表
	TICK_SPACING_STABLE   = 1   // 稳定币对
	TICK_SPACING_STANDARD = 64  // 标准代币对
	TICK_SPACING_VOLATILE = 128 // 波动性代币对
)

// 数学计算常量
var (
	// Q64 格式常量 (2^64)
	Q64 = math.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 64))

	// Q128 格式常量 (2^128)
	Q128 = math.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 128))

	// 零值常量
	ZERO_INT = math.NewInt(0)
	ONE_INT  = math.NewInt(1)
)
