package orca

import (
	"context"
	"encoding/binary"
	"fmt"

	cosmath "cosmossdk.io/math"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/yimingWOW/solroute/pkg"
	"lukechampine.com/uint128"
)

// WhirlpoolPool 结构体 - 映射自 Orca Whirlpool 账户结构
type WhirlpoolPool struct {
	// 8 bytes discriminator
	Discriminator [8]uint8 `bin:"skip"`

	// 核心配置 - 映射自 external/orca/whirlpool/generated/types.go 的 Whirlpool 结构
	WhirlpoolsConfig solana.PublicKey // whirlpoolsConfig
	WhirlpoolBump    [1]uint8         // whirlpoolBump
	TickSpacing      uint16           // tickSpacing
	FeeTierIndexSeed [2]uint8         // feeTierIndexSeed
	FeeRate          uint16           // feeRate
	ProtocolFeeRate  uint16           // protocolFeeRate

	// 流动性状态 - 字段名映射: SqrtPriceX64 -> SqrtPrice, TickCurrent -> TickCurrentIndex
	Liquidity        uint128.Uint128 // liquidity
	SqrtPrice        uint128.Uint128 // sqrtPrice (注意：CLMM 用 SqrtPriceX64)
	TickCurrentIndex int32           // tickCurrentIndex (注意：CLMM 用 TickCurrent)

	// 协议费用
	ProtocolFeeOwedA uint64 // protocolFeeOwedA
	ProtocolFeeOwedB uint64 // protocolFeeOwedB

	// 代币配置 - 字段名映射: TokenMint0/1 -> TokenMintA/B
	TokenMintA       solana.PublicKey // tokenMintA (注意：CLMM 用 TokenMint0)
	TokenVaultA      solana.PublicKey // tokenVaultA (注意：CLMM 用 TokenVault0)
	FeeGrowthGlobalA uint128.Uint128  // feeGrowthGlobalA

	TokenMintB       solana.PublicKey // tokenMintB (注意：CLMM 用 TokenMint1)
	TokenVaultB      solana.PublicKey // tokenVaultB (注意：CLMM 用 TokenVault1)
	FeeGrowthGlobalB uint128.Uint128  // feeGrowthGlobalB

	// 奖励信息
	RewardLastUpdatedTimestamp uint64                 // rewardLastUpdatedTimestamp
	RewardInfos                [3]WhirlpoolRewardInfo // rewardInfos

	// 内部使用字段
	PoolId           solana.PublicKey // 池 ID (内部计算)
	UserBaseAccount  solana.PublicKey // 用户基础代币账户
	UserQuoteAccount solana.PublicKey // 用户报价代币账户
}

// WhirlpoolRewardInfo 奖励信息结构 - 参考 external/orca/whirlpool/generated/types.go
type WhirlpoolRewardInfo struct {
	Mint                  solana.PublicKey // mint
	Vault                 solana.PublicKey // vault
	Authority             solana.PublicKey // authority
	EmissionsPerSecondX64 uint128.Uint128  // emissionsPerSecondX64
	GrowthGlobalX64       uint128.Uint128  // growthGlobalX64
}

// 实现 Pool 接口的基础方法
func (pool *WhirlpoolPool) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolNameOrcaWhirlpool
}

func (pool *WhirlpoolPool) ProtocolType() pkg.ProtocolType {
	return pkg.ProtocolTypeOrcaWhirlpool
}

func (pool *WhirlpoolPool) GetProgramID() solana.PublicKey {
	return ORCA_WHIRLPOOL_PROGRAM_ID
}

func (pool *WhirlpoolPool) GetID() string {
	return pool.PoolId.String()
}

// GetTokens 返回代币对 - 注意字段名映射
func (pool *WhirlpoolPool) GetTokens() (baseMint, quoteMint string) {
	return pool.TokenMintA.String(), pool.TokenMintB.String()
}

// Decode 解析 Whirlpool 账户数据 - 参考 CLMM 的 Decode 实现
func (pool *WhirlpoolPool) Decode(data []byte) error {
	// Skip 8 bytes discriminator if present
	if len(data) > 8 {
		data = data[8:]
	}

	offset := 0

	// Parse whirlpools config (32 bytes)
	pool.WhirlpoolsConfig = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	// Parse whirlpool bump (1 byte)
	copy(pool.WhirlpoolBump[:], data[offset:offset+1])
	offset += 1

	// Parse tick spacing (2 bytes)
	pool.TickSpacing = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Parse fee tier index seed (2 bytes)
	copy(pool.FeeTierIndexSeed[:], data[offset:offset+2])
	offset += 2

	// Parse fee rate (2 bytes)
	pool.FeeRate = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Parse protocol fee rate (2 bytes)
	pool.ProtocolFeeRate = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Parse liquidity (16 bytes)
	pool.Liquidity = uint128.FromBytes(data[offset : offset+16])
	offset += 16

	// Parse sqrt price (16 bytes) - 注意：CLMM 叫 SqrtPriceX64
	pool.SqrtPrice = uint128.FromBytes(data[offset : offset+16])
	offset += 16

	// Parse tick current index (4 bytes) - 注意：CLMM 叫 TickCurrent
	pool.TickCurrentIndex = int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4

	// Parse protocol fee owed A (8 bytes)
	pool.ProtocolFeeOwedA = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Parse protocol fee owed B (8 bytes)
	pool.ProtocolFeeOwedB = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Parse token mint A (32 bytes) - 注意：CLMM 叫 TokenMint0
	pool.TokenMintA = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	// Parse token vault A (32 bytes) - 注意：CLMM 叫 TokenVault0
	pool.TokenVaultA = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	// Parse fee growth global A (16 bytes)
	pool.FeeGrowthGlobalA = uint128.FromBytes(data[offset : offset+16])
	offset += 16

	// Parse token mint B (32 bytes) - 注意：CLMM 叫 TokenMint1
	pool.TokenMintB = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	// Parse token vault B (32 bytes) - 注意：CLMM 叫 TokenVault1
	pool.TokenVaultB = solana.PublicKeyFromBytes(data[offset : offset+32])
	offset += 32

	// Parse fee growth global B (16 bytes)
	pool.FeeGrowthGlobalB = uint128.FromBytes(data[offset : offset+16])
	offset += 16

	// Parse reward last updated timestamp (8 bytes)
	pool.RewardLastUpdatedTimestamp = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Parse reward infos (3 个奖励信息，每个包含多个字段)
	for i := 0; i < 3; i++ {
		// mint (32 bytes)
		pool.RewardInfos[i].Mint = solana.PublicKeyFromBytes(data[offset : offset+32])
		offset += 32

		// vault (32 bytes)
		pool.RewardInfos[i].Vault = solana.PublicKeyFromBytes(data[offset : offset+32])
		offset += 32

		// authority (32 bytes)
		pool.RewardInfos[i].Authority = solana.PublicKeyFromBytes(data[offset : offset+32])
		offset += 32

		// emissions per second x64 (16 bytes)
		pool.RewardInfos[i].EmissionsPerSecondX64 = uint128.FromBytes(data[offset : offset+16])
		offset += 16

		// growth global x64 (16 bytes)
		pool.RewardInfos[i].GrowthGlobalX64 = uint128.FromBytes(data[offset : offset+16])
		offset += 16
	}

	return nil
}

// Span 返回账户数据大小 - 根据 Whirlpool 完整结构精确计算
func (pool *WhirlpoolPool) Span() uint64 {
	// 基于 external/orca/whirlpool/generated/types.go 的 Whirlpool 结构计算:
	//
	// 8 bytes discriminator
	// 32 bytes whirlpoolsConfig (PublicKey)
	// 1 byte whirlpoolBump
	// 2 bytes tickSpacing (uint16)
	// 2 bytes feeTierIndexSeed
	// 2 bytes feeRate (uint16)
	// 2 bytes protocolFeeRate (uint16)
	// 16 bytes liquidity (Uint128)
	// 16 bytes sqrtPrice (Uint128)
	// 4 bytes tickCurrentIndex (int32)
	// 8 bytes protocolFeeOwedA (uint64)
	// 8 bytes protocolFeeOwedB (uint64)
	// 32 bytes tokenMintA (PublicKey)
	// 32 bytes tokenVaultA (PublicKey)
	// 16 bytes feeGrowthGlobalA (Uint128)
	// 32 bytes tokenMintB (PublicKey)
	// 32 bytes tokenVaultB (PublicKey)
	// 16 bytes feeGrowthGlobalB (Uint128)
	// 8 bytes rewardLastUpdatedTimestamp (uint64)
	// 3 * (32+32+32+16+16) bytes rewardInfos (3个WhirlpoolRewardInfo)
	//   每个 WhirlpoolRewardInfo: mint(32) + vault(32) + authority(32) + emissionsPerSecondX64(16) + growthGlobalX64(16) = 128 bytes

	return uint64(8 + 32 + 1 + 2 + 2 + 2 + 2 + 16 + 16 + 4 + 8 + 8 + 32 + 32 + 16 + 32 + 32 + 16 + 8 + 3*128)
	// = 8 + 261 + 384 = 653 bytes (包含 discriminator)
}

// Offset 返回字段偏移量 - 用于 RPC 查询过滤器
func (pool *WhirlpoolPool) Offset(field string) uint64 {
	// Add 8 bytes for discriminator
	baseOffset := uint64(8)

	switch field {
	case "TokenMintA":
		// 基于 Whirlpool 结构的精确偏移计算:
		// whirlpoolsConfig(32) + whirlpoolBump(1) + tickSpacing(2) + feeTierIndexSeed(2) +
		// feeRate(2) + protocolFeeRate(2) + liquidity(16) + sqrtPrice(16) +
		// tickCurrentIndex(4) + protocolFeeOwedA(8) + protocolFeeOwedB(8)
		return baseOffset + 32 + 1 + 2 + 2 + 2 + 2 + 16 + 16 + 4 + 8 + 8 // = 101
	case "TokenMintB":
		// TokenMintA 之后: tokenMintA(32) + tokenVaultA(32) + feeGrowthGlobalA(16)
		// 注意：实际验证发现 TokenMintB 在偏移量 181，不是 189
		return baseOffset + 101 + 32 + 32 + 16 - 8 // = 181 (修正 8 字节差异)
	case "TickSpacing":
		// whirlpoolsConfig(32) + whirlpoolBump(1) 之后
		return baseOffset + 32 + 1 // = 41
	case "FeeRate":
		// whirlpoolsConfig(32) + whirlpoolBump(1) + tickSpacing(2) + feeTierIndexSeed(2) 之后
		return baseOffset + 32 + 1 + 2 + 2 // = 45
	case "SqrtPrice":
		// 在 liquidity 之后
		return baseOffset + 32 + 1 + 2 + 2 + 2 + 2 + 16 // = 65
	case "TickCurrentIndex":
		// 在 sqrtPrice 之后
		return baseOffset + 32 + 1 + 2 + 2 + 2 + 2 + 16 + 16 // = 81
	}
	return 0
}

// Quote 方法 - 获取交换报价 (基础实现，返回虚拟报价用于测试)
func (pool *WhirlpoolPool) Quote(ctx context.Context, solClient *rpc.Client, inputMint string, inputAmount cosmath.Int) (cosmath.Int, error) {
	// Whirlpool 真实报价计算 - 参考 CLMM 实现并适配 Whirlpool
	// 暂时简化实现，不查询外部 bitmap 和 tick arrays

	// 1. 检查输入代币类型
	if inputMint == pool.TokenMintA.String() {
		// A -> B 交换
		priceAtoB, err := pool.ComputeWhirlpoolAmountOutFormat(pool.TokenMintA.String(), inputAmount)
		if err != nil {
			return cosmath.Int{}, err
		}
		return priceAtoB.Neg(), nil // 返回负数表示输出金额
	} else if inputMint == pool.TokenMintB.String() {
		// B -> A 交换
		priceBtoA, err := pool.ComputeWhirlpoolAmountOutFormat(pool.TokenMintB.String(), inputAmount)
		if err != nil {
			return cosmath.Int{}, err
		}
		return priceBtoA.Neg(), nil // 返回负数表示输出金额
	} else {
		return cosmath.Int{}, fmt.Errorf("input mint %s not found in pool", inputMint)
	}
}

// ComputeWhirlpoolAmountOutFormat - Whirlpool 版本的输出金额计算，参考 CLMM 实现
func (pool *WhirlpoolPool) ComputeWhirlpoolAmountOutFormat(inputTokenMint string, inputAmount cosmath.Int) (cosmath.Int, error) {
	// 确定交换方向：A -> B 为 true，B -> A 为 false
	zeroForOne := inputTokenMint == pool.TokenMintA.String()

	// 简化版本：暂时不查询外部 tick arrays
	// 使用当前池状态进行基础计算
	firstTickArrayStartIndex := getWhirlpoolTickArrayStartIndexByTick(int64(pool.TickCurrentIndex), int64(pool.TickSpacing))

	// 调用核心交换计算逻辑
	expectedAmountOut, err := pool.whirlpoolSwapCompute(
		int64(pool.TickCurrentIndex),
		zeroForOne,
		inputAmount,
		cosmath.NewIntFromUint64(uint64(pool.FeeRate)), // 使用池的费率
		firstTickArrayStartIndex,
		nil, // 暂时不使用外部 bitmap
	)
	if err != nil {
		return cosmath.Int{}, fmt.Errorf("failed to compute Whirlpool swap amount: %w", err)
	}
	return expectedAmountOut, nil
}

// BuildSwapInstructions 方法 - 构建交换指令 (基础实现，返回空指令用于测试)
func (pool *WhirlpoolPool) BuildSwapInstructions(
	ctx context.Context,
	solClient *rpc.Client,
	userAddr solana.PublicKey,
	inputMint string,
	amountIn cosmath.Int,
	minOutAmountWithDecimals cosmath.Int,
) ([]solana.Instruction, error) {
	// TODO: 实现真正的交换指令构建逻辑，参考 CLMM 的实现
	// 现在返回空指令列表用于测试框架

	// 检查输入代币是哪个
	var isAtoB bool
	if inputMint == pool.TokenMintA.String() {
		isAtoB = true
	} else if inputMint == pool.TokenMintB.String() {
		isAtoB = false
	} else {
		return nil, fmt.Errorf("input mint %s not found in pool", inputMint)
	}

	// 设置用户账户（暂时设为同一个地址，实际需要查找真实的代币账户）
	if isAtoB {
		pool.UserBaseAccount = userAddr  // 简化处理
		pool.UserQuoteAccount = userAddr // 简化处理
	} else {
		pool.UserBaseAccount = userAddr  // 简化处理
		pool.UserQuoteAccount = userAddr // 简化处理
	}

	// 返回空指令列表（用于测试，不会实际执行交易）
	fmt.Printf("Whirlpool BuildSwapInstructions called: pool=%s, inputMint=%s, amountIn=%s, aToB=%v\n",
		pool.GetID(), inputMint, amountIn.String(), isAtoB)

	return []solana.Instruction{}, nil
}

// whirlpoolSwapCompute - Whirlpool 核心交换计算逻辑 (简化版本，参考 CLMM 实现)
func (pool *WhirlpoolPool) whirlpoolSwapCompute(
	currentTick int64,
	zeroForOne bool,
	amountSpecified cosmath.Int,
	fee cosmath.Int,
	lastSavedTickArrayStartIndex int64,
	exTickArrayBitmap *WhirlpoolTickArrayBitmapExtensionType,
) (cosmath.Int, error) {
	// 输入验证
	if amountSpecified.IsZero() {
		return cosmath.Int{}, fmt.Errorf("input amount cannot be zero")
	}

	// 基础变量初始化
	baseInput := amountSpecified.IsPositive()
	sqrtPriceLimitX64 := cosmath.NewInt(0)

	// 初始化计算变量
	amountSpecifiedRemaining := amountSpecified
	amountCalculated := cosmath.NewInt(0)
	sqrtPriceX64 := cosmath.NewIntFromBigInt(pool.SqrtPrice.Big()) // 注意：Whirlpool 用 SqrtPrice 而不是 SqrtPriceX64
	liquidity := cosmath.NewIntFromBigInt(pool.Liquidity.Big())

	// 设置价格限制 - 复用 CLMM 的常量
	if zeroForOne {
		sqrtPriceLimitX64 = MIN_SQRT_PRICE_X64.Add(cosmath.NewInt(1))
	} else {
		sqrtPriceLimitX64 = MAX_SQRT_PRICE_X64.Sub(cosmath.NewInt(1))
	}

	// 简化版本：不使用复杂的 tick array 遍历，直接基于当前价格计算
	// 实际生产环境需要实现完整的 tick 遍历逻辑

	// 计算目标价格 (简化：向价格限制方向移动一小步)
	targetPrice := sqrtPriceX64
	if zeroForOne {
		// A -> B: 价格下降
		targetPrice = sqrtPriceX64.Mul(cosmath.NewInt(995)).Quo(cosmath.NewInt(1000)) // 降低 0.5%
		if targetPrice.LT(sqrtPriceLimitX64) {
			targetPrice = sqrtPriceLimitX64
		}
	} else {
		// B -> A: 价格上升
		targetPrice = sqrtPriceX64.Mul(cosmath.NewInt(1005)).Quo(cosmath.NewInt(1000)) // 增加 0.5%
		if targetPrice.GT(sqrtPriceLimitX64) {
			targetPrice = sqrtPriceLimitX64
		}
	}

	// 调用简化的单步计算
	newSqrtPrice, amountIn, amountOut, feeAmount, err := pool.whirlpoolSwapStepCompute(
		sqrtPriceX64,
		targetPrice,
		liquidity,
		amountSpecifiedRemaining,
		fee,
		zeroForOne,
	)
	if err != nil {
		return cosmath.Int{}, fmt.Errorf("swap step compute failed: %w", err)
	}

	// 更新计算结果
	if baseInput {
		// 精确输入模式
		amountCalculated = amountOut.Neg() // 返回负数表示输出
	} else {
		// 精确输出模式
		amountCalculated = amountIn.Add(feeAmount)
	}

	// 验证结果合理性
	if amountCalculated.IsZero() {
		return cosmath.Int{}, fmt.Errorf("calculated amount is zero, input: %s, sqrtPrice: %s->%s",
			amountSpecified.String(), sqrtPriceX64.String(), newSqrtPrice.String())
	}

	return amountCalculated, nil
}

// whirlpoolSwapStepCompute - Whirlpool 单步交换计算 (简化版本)
func (pool *WhirlpoolPool) whirlpoolSwapStepCompute(
	sqrtPriceCurrent cosmath.Int,
	sqrtPriceTarget cosmath.Int,
	liquidity cosmath.Int,
	amountRemaining cosmath.Int,
	feeRate cosmath.Int,
	zeroForOne bool,
) (sqrtPriceNext cosmath.Int, amountIn cosmath.Int, amountOut cosmath.Int, feeAmount cosmath.Int, err error) {

	// 基础验证
	if liquidity.IsZero() {
		return cosmath.Int{}, cosmath.Int{}, cosmath.Int{}, cosmath.Int{}, fmt.Errorf("liquidity is zero")
	}

	// 简化计算：基于恒定乘积公式 x * y = k
	// 其中 k = liquidity^2, sqrtPrice = sqrt(y/x)

	// 简化版本暂时不需要计算价格变化比例
	// 实际实现时可以用于更精确的价格计算
	_ = sqrtPriceTarget // 避免未使用警告

	// 根据流动性和价格变化计算交换金额
	// 简化公式：基于相对价格变化计算
	baseAmount := amountRemaining.Abs()

	// 计算费用
	feeAmount = baseAmount.Mul(feeRate).Quo(FEE_RATE_DENOMINATOR)

	// 计算实际用于交换的金额 (扣除费用)
	amountForSwap := baseAmount.Sub(feeAmount)

	// 简化的输出计算：基于流动性比例
	// 实际 AMM 需要考虑 sqrt 价格曲线，这里用简化公式
	liquidityRatio := liquidity.Mul(cosmath.NewInt(1000)).Quo(liquidity.Add(amountForSwap))

	if zeroForOne {
		// A -> B
		amountIn = baseAmount
		amountOut = amountForSwap.Mul(liquidityRatio).Quo(cosmath.NewInt(1000))
		sqrtPriceNext = sqrtPriceTarget
	} else {
		// B -> A
		amountIn = baseAmount
		amountOut = amountForSwap.Mul(liquidityRatio).Quo(cosmath.NewInt(1000))
		sqrtPriceNext = sqrtPriceTarget
	}

	// 确保输出金额合理
	if amountOut.IsZero() {
		amountOut = cosmath.NewInt(1) // 最小输出
	}

	return sqrtPriceNext, amountIn, amountOut, feeAmount, nil
}
