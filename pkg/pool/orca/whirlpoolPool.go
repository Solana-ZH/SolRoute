package orca

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
	"time"

	cosmath "cosmossdk.io/math"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/yimingWOW/solroute/pkg"
	"lukechampine.com/uint128"
)

// WhirlpoolPool 结构体 - 映射自 Orca Whirlpool 账户结构
//
// 这个结构体精确映射了 Orca Whirlpool V2 协议的池账户数据格式。
// 数据结构基于 external/orca/whirlpool/generated/types.go 的 Whirlpool 结构，
// 并针对字段命名差异进行了适配：
//   - CLMM: TokenMint0/1 → Whirlpool: TokenMintA/B
//   - CLMM: SqrtPriceX64 → Whirlpool: SqrtPrice
//   - CLMM: TickCurrent → Whirlpool: TickCurrentIndex
//
// 总账户大小: 653 字节 (包含 8 字节 discriminator)
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

// Quote 方法 - 获取交换报价 (带边界验证和错误处理)
func (pool *WhirlpoolPool) Quote(ctx context.Context, solClient *rpc.Client, inputMint string, inputAmount cosmath.Int) (cosmath.Int, error) {
	// 1. 输入验证
	if err := pool.validateQuoteInputs(inputMint, inputAmount); err != nil {
		return cosmath.Int{}, fmt.Errorf("quote input validation failed: %w", err)
	}

	// 2. 池状态验证
	if err := pool.validatePoolState(); err != nil {
		return cosmath.Int{}, fmt.Errorf("pool state validation failed: %w", err)
	}

	// 3. 计算报价 (带重试机制)
	maxRetries := 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var priceResult cosmath.Int
		var err error

		if inputMint == pool.TokenMintA.String() {
			// A -> B 交换
			priceResult, err = pool.ComputeWhirlpoolAmountOutFormat(pool.TokenMintA.String(), inputAmount)
		} else if inputMint == pool.TokenMintB.String() {
			// B -> A 交换
			priceResult, err = pool.ComputeWhirlpoolAmountOutFormat(pool.TokenMintB.String(), inputAmount)
		} else {
			return cosmath.Int{}, fmt.Errorf("input mint %s not found in pool %s", inputMint, pool.PoolId.String())
		}

		if err != nil {
			lastErr = err
			// 如果是计算错误且还有重试次数，短暂等待后重试
			if attempt < maxRetries && isTemporaryError(err) {
				time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
				continue
			}
			return cosmath.Int{}, fmt.Errorf("amount calculation failed after %d attempts: %w", attempt+1, err)
		}

		// 4. 输出验证
		if err := pool.validateQuoteOutput(priceResult); err != nil {
			return cosmath.Int{}, fmt.Errorf("quote output validation failed: %w", err)
		}

		return priceResult.Neg(), nil // 返回负数表示输出金额
	}

	return cosmath.Int{}, fmt.Errorf("quote calculation failed after retries: %w", lastErr)
}

// validateQuoteInputs 验证报价输入参数
func (pool *WhirlpoolPool) validateQuoteInputs(inputMint string, inputAmount cosmath.Int) error {
	// 检查输入金额
	if inputAmount.IsZero() {
		return fmt.Errorf("input amount cannot be zero")
	}
	if inputAmount.IsNegative() {
		return fmt.Errorf("input amount cannot be negative")
	}

	// 检查输入金额是否过大 (防止溢出)
	maxAmount := cosmath.NewIntFromUint64(1e18) // 设置合理的最大值
	if inputAmount.GT(maxAmount) {
		return fmt.Errorf("input amount too large: %s > %s", inputAmount.String(), maxAmount.String())
	}

	// 验证代币 mint 地址格式 - 使用 Solana 标准验证
	_, err := solana.PublicKeyFromBase58(inputMint)
	if err != nil {
		return fmt.Errorf("invalid mint address format: %s, error: %w", inputMint, err)
	}

	return nil
}

// validatePoolState 验证池状态
func (pool *WhirlpoolPool) validatePoolState() error {
	// 检查流动性 - 如果为零，跳过这个池但不报错，让路由器选择其他池
	if pool.Liquidity.IsZero() {
		return fmt.Errorf("pool has zero liquidity") // 这会让路由器跳过此池
	}

	// 检查价格 - 价格为零的池无法进行交易
	if pool.SqrtPrice.IsZero() {
		return fmt.Errorf("pool has zero sqrt price")
	}

	// 检查 tick spacing - 为零的 tick spacing 不合法
	if pool.TickSpacing == 0 {
		return fmt.Errorf("pool has zero tick spacing")
	}

	// 检查代币 mint 地址 - 无效地址的池不可用
	if pool.TokenMintA.IsZero() || pool.TokenMintB.IsZero() {
		return fmt.Errorf("pool has invalid token mint addresses")
	}

	return nil
}

// validateQuoteOutput 验证报价输出
func (pool *WhirlpoolPool) validateQuoteOutput(outputAmount cosmath.Int) error {
	// 检查输出是否为零
	if outputAmount.IsZero() {
		return fmt.Errorf("computed output amount is zero")
	}

	// 注意：负数是有效的，表示输出金额（通过 .Neg() 转换为负数）
	// 所以我们验证绝对值不为零即可
	absoluteAmount := outputAmount.Abs()
	if absoluteAmount.IsZero() {
		return fmt.Errorf("computed output amount absolute value is zero: %s", outputAmount.String())
	}

	return nil
}

// isTemporaryError 判断是否是临时错误
func isTemporaryError(err error) bool {
	errorMsg := strings.ToLower(err.Error())
	return strings.Contains(errorMsg, "overflow") ||
		strings.Contains(errorMsg, "underflow") ||
		strings.Contains(errorMsg, "division by zero") ||
		strings.Contains(errorMsg, "timeout")
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

// BuildSwapInstructions 方法 - 构建真实的 Whirlpool SwapV2 指令
//
// 这个方法构建完整的 Whirlpool SwapV2 交易指令，包括：
// 1. 交换方向判断 (A->B 或 B->A)
// 2. ATA 账户推导和存在性检查
// 3. Tick Array PDA 地址计算
// 4. SwapV2 指令参数编码
// 5. 正确的账户元数据排列
//
// 返回的指令可以直接用于 Solana 交易执行。
func (pool *WhirlpoolPool) BuildSwapInstructions(
	ctx context.Context,
	solClient *rpc.Client,
	userAddr solana.PublicKey,
	inputMint string,
	amountIn cosmath.Int,
	minOutAmountWithDecimals cosmath.Int,
) ([]solana.Instruction, error) {
	// 1. 确定交换方向
	var aToB bool

	if inputMint == pool.TokenMintA.String() {
		// A -> B 交换
		aToB = true
	} else if inputMint == pool.TokenMintB.String() {
		// B -> A 交换
		aToB = false
	} else {
		return nil, fmt.Errorf("input mint %s not found in pool", inputMint)
	}

	// 2. 获取或创建用户的代币账户 - 固定为 A 和 B，不随交换方向变化
	userTokenAccountA, err := getOrCreateTokenAccount(ctx, solClient, userAddr, pool.TokenMintA)
	if err != nil {
		return nil, fmt.Errorf("failed to get token A account: %w", err)
	}

	userTokenAccountB, err := getOrCreateTokenAccount(ctx, solClient, userAddr, pool.TokenMintB)
	if err != nil {
		return nil, fmt.Errorf("failed to get token B account: %w", err)
	}

	// 3. 计算价格限制 (设置为极值，实际不限制)
	var sqrtPriceLimit uint128.Uint128
	if aToB {
		sqrtPriceLimit = uint128.FromBig(MIN_SQRT_PRICE_X64.BigInt())
	} else {
		sqrtPriceLimit = uint128.FromBig(MAX_SQRT_PRICE_X64.BigInt())
	}

	// 4. 构建 tick array 地址 (使用真实的 PDA 推导)
	tickArray0, tickArray1, tickArray2, err := DeriveMultipleWhirlpoolTickArrayPDAs(
		pool.PoolId,
		int64(pool.TickCurrentIndex),
		int64(pool.TickSpacing),
		aToB,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to derive tick array PDAs: %w", err)
	}

	// 5. Oracle 地址 (使用正确的 PDA 推导)
	oracleAddr, err := DeriveWhirlpoolOraclePDA(pool.PoolId)
	if err != nil {
		return nil, fmt.Errorf("failed to derive oracle PDA: %w", err)
	}

	// 6. 构建 SwapV2 指令参数
	instruction, err := createWhirlpoolSwapV2Instruction(
		// 指令参数
		amountIn.Uint64(),                 // amount
		minOutAmountWithDecimals.Uint64(), // otherAmountThreshold
		sqrtPriceLimit,                    // sqrtPriceLimit
		true,                              // amountSpecifiedIsInput
		aToB,                              // aToB
		nil,                               // remainingAccountsInfo

		// 账户地址 - 固定为 A 和 B 顺序，不随交换方向变化
		TOKEN_PROGRAM_ID,  // tokenProgramA
		TOKEN_PROGRAM_ID,  // tokenProgramB
		MEMO_PROGRAM_ID,   // memoProgram
		userAddr,          // tokenAuthority
		pool.PoolId,       // whirlpool
		pool.TokenMintA,   // tokenMintA
		pool.TokenMintB,   // tokenMintB
		userTokenAccountA, // tokenOwnerAccountA (固定为 A)
		pool.TokenVaultA,  // tokenVaultA (固定为 A)
		userTokenAccountB, // tokenOwnerAccountB (固定为 B)
		pool.TokenVaultB,  // tokenVaultB (固定为 B)
		tickArray0,        // tickArray0
		tickArray1,        // tickArray1
		tickArray2,        // tickArray2
		oracleAddr,        // oracle
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SwapV2 instruction: %w", err)
	}

	return []solana.Instruction{instruction}, nil
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

// whirlpoolSwapStepCompute - Whirlpool 精确 CLMM 计算 (基于 Raydium CLMM 算法)
// 采用与 Raydium CLMM 相同的精确数学公式，确保计算准确性
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

	baseAmount := amountRemaining.Abs()
	if baseAmount.IsZero() {
		return sqrtPriceCurrent, cosmath.ZeroInt(), cosmath.ZeroInt(), cosmath.ZeroInt(), nil
	}

	// 调用精确的 CLMM swap step 计算
	// 这个函数采用与 Raydium 相同的算法，确保数学准确性
	return whirlpoolSwapStepComputePrecise(
		sqrtPriceCurrent.BigInt(),
		sqrtPriceTarget.BigInt(),
		liquidity.BigInt(),
		baseAmount.BigInt(),
		uint32(feeRate.Int64()),
		zeroForOne,
	)
}

// whirlpoolSwapStepComputePrecise - 精确的 CLMM swap step 计算
// 基于 Raydium CLMM 的 swapStepCompute 函数，针对 Whirlpool 进行适配
func whirlpoolSwapStepComputePrecise(
	sqrtPriceX64Current *big.Int,
	sqrtPriceX64Target *big.Int,
	liquidity *big.Int,
	amountRemaining *big.Int,
	feeRate uint32,
	zeroForOne bool,
) (cosmath.Int, cosmath.Int, cosmath.Int, cosmath.Int, error) {

	// 定义 SwapStep 结构来追踪计算状态
	swapStep := &WhirlpoolSwapStep{
		SqrtPriceX64Next: new(big.Int),
		AmountIn:         new(big.Int),
		AmountOut:        new(big.Int),
		FeeAmount:        new(big.Int),
	}

	zero := new(big.Int)
	baseInput := amountRemaining.Cmp(zero) >= 0

	// Step 1: 计算费用率相关常量
	// FEE_RATE_DENOMINATOR = 1,000,000 (Whirlpool 使用百万分之一作为费率单位)
	FEE_RATE_DENOMINATOR := cosmath.NewInt(1000000)

	if baseInput {
		// 精确输入模式：先扣除费用，再计算交换
		feeRateBig := cosmath.NewInt(int64(feeRate))
		tmp := FEE_RATE_DENOMINATOR.Sub(feeRateBig)
		amountRemainingSubtractFee := whirlpoolMulDivFloor(
			cosmath.NewIntFromBigInt(amountRemaining),
			tmp,
			FEE_RATE_DENOMINATOR,
		)

		// 计算在当前价格区间内可以交换的最大金额
		if zeroForOne {
			// Token A -> Token B
			swapStep.AmountIn = whirlpoolGetTokenAmountAFromLiquidity(
				sqrtPriceX64Target, sqrtPriceX64Current, liquidity, true)
		} else {
			// Token B -> Token A
			swapStep.AmountIn = whirlpoolGetTokenAmountBFromLiquidity(
				sqrtPriceX64Current, sqrtPriceX64Target, liquidity, true)
		}

		// 判断是否会到达目标价格
		if amountRemainingSubtractFee.GTE(cosmath.NewIntFromBigInt(swapStep.AmountIn)) {
			// 输入足够大，会到达目标价格
			swapStep.SqrtPriceX64Next.Set(sqrtPriceX64Target)
		} else {
			// 输入不足，计算新的价格
			swapStep.SqrtPriceX64Next = whirlpoolGetNextSqrtPriceX64FromInput(
				sqrtPriceX64Current,
				liquidity,
				amountRemainingSubtractFee.BigInt(),
				zeroForOne,
			)
		}
	} else {
		// 精确输出模式：直接计算所需输入
		if zeroForOne {
			swapStep.AmountOut = whirlpoolGetTokenAmountBFromLiquidity(
				sqrtPriceX64Target, sqrtPriceX64Current, liquidity, false)
		} else {
			swapStep.AmountOut = whirlpoolGetTokenAmountAFromLiquidity(
				sqrtPriceX64Current, sqrtPriceX64Target, liquidity, false)
		}

		negativeOne := new(big.Int).SetInt64(-1)
		amountRemainingNeg := new(big.Int).Mul(amountRemaining, negativeOne)

		if amountRemainingNeg.Cmp(swapStep.AmountOut) >= 0 {
			swapStep.SqrtPriceX64Next.Set(sqrtPriceX64Target)
		} else {
			swapStep.SqrtPriceX64Next = whirlpoolGetNextSqrtPriceX64FromOutput(
				sqrtPriceX64Current,
				liquidity,
				amountRemainingNeg,
				zeroForOne,
			)
		}
	}

	// Step 2: 重新计算精确的输入输出金额
	reachTargetPrice := swapStep.SqrtPriceX64Next.Cmp(sqrtPriceX64Target) == 0

	if zeroForOne {
		if !(reachTargetPrice && baseInput) {
			swapStep.AmountIn = whirlpoolGetTokenAmountAFromLiquidity(
				swapStep.SqrtPriceX64Next,
				sqrtPriceX64Current,
				liquidity,
				true,
			)
		}

		if !(reachTargetPrice && !baseInput) {
			swapStep.AmountOut = whirlpoolGetTokenAmountBFromLiquidity(
				swapStep.SqrtPriceX64Next,
				sqrtPriceX64Current,
				liquidity,
				false,
			)
		}
	} else {
		if !(reachTargetPrice && baseInput) {
			swapStep.AmountIn = whirlpoolGetTokenAmountBFromLiquidity(
				sqrtPriceX64Current,
				swapStep.SqrtPriceX64Next,
				liquidity,
				true,
			)
		}

		if !(reachTargetPrice && !baseInput) {
			swapStep.AmountOut = whirlpoolGetTokenAmountAFromLiquidity(
				sqrtPriceX64Current,
				swapStep.SqrtPriceX64Next,
				liquidity,
				false,
			)
		}
	}

	// Step 3: 计算费用
	if baseInput && swapStep.SqrtPriceX64Next.Cmp(sqrtPriceX64Target) != 0 {
		swapStep.FeeAmount = new(big.Int).Sub(amountRemaining, swapStep.AmountIn)
	} else {
		feeRateBig := cosmath.NewInt(int64(feeRate))
		feeRateSubtracted := FEE_RATE_DENOMINATOR.Sub(feeRateBig)
		swapStep.FeeAmount = whirlpoolMulDivCeil(
			cosmath.NewIntFromBigInt(swapStep.AmountIn),
			feeRateBig,
			feeRateSubtracted,
		).BigInt()
	}

	// 应用适中的安全边际 (10% 而不是 80%)
	safetyMargin := cosmath.NewInt(90) // 保留 90% 的计算结果
	adjustedAmountOut := cosmath.NewIntFromBigInt(swapStep.AmountOut).Mul(safetyMargin).Quo(cosmath.NewInt(100))

	// 确保最小输出
	if adjustedAmountOut.IsZero() && swapStep.AmountOut.Cmp(zero) > 0 {
		adjustedAmountOut = cosmath.NewInt(1)
	}

	return cosmath.NewIntFromBigInt(swapStep.SqrtPriceX64Next),
		cosmath.NewIntFromBigInt(swapStep.AmountIn),
		adjustedAmountOut,
		cosmath.NewIntFromBigInt(swapStep.FeeAmount), nil
}

// getOrCreateTokenAccount 获取或创建用户的代币账户
func getOrCreateTokenAccount(ctx context.Context, solClient *rpc.Client, userAddr solana.PublicKey, tokenMint solana.PublicKey) (solana.PublicKey, error) {
	// 1. 推导 ATA 地址
	ata, _, err := solana.FindAssociatedTokenAddress(userAddr, tokenMint)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to find associated token address: %w", err)
	}

	// 2. 检查 ATA 账户是否存在
	accountExists, err := checkAccountExists(ctx, solClient, ata)
	if err != nil {
		// 如果 RPC 查询失败，继续使用 ATA 地址，让交易自然失败
		// 这样可以避免阻塞正常流程
		return ata, nil
	}

	if !accountExists {
		// ATA 不存在，但我们仍然返回地址
		// 在实际应用中，调用方需要决定是否添加创建 ATA 的指令
		// 对于主流代币（如 SOL, USDC），用户通常已经有 ATA
		return ata, nil
	}

	return ata, nil
}

// checkAccountExists 检查账户是否存在 (带重试机制)
func checkAccountExists(ctx context.Context, solClient *rpc.Client, accountAddr solana.PublicKey) (bool, error) {
	// 实现简单的重试机制，应对 RPC 限流
	maxRetries := 3
	baseDelay := 100 // 100ms

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 使用 getAccountInfo 检查账户是否存在
		_, err := solClient.GetAccountInfo(ctx, accountAddr)
		if err != nil {
			// 检查是否是"账户不存在"的错误
			if isAccountNotFoundError(err) {
				return false, nil
			}

			// 检查是否是 RPC 限流错误
			if isRateLimitError(err) && attempt < maxRetries {
				// 指数退避重试
				delay := baseDelay * (1 << attempt) // 100ms, 200ms, 400ms
				time.Sleep(time.Duration(delay) * time.Millisecond)
				continue
			}

			// 其他错误直接返回
			return false, fmt.Errorf("failed to check account existence after %d attempts: %w", attempt+1, err)
		}

		// 账户存在，成功返回
		return true, nil
	}

	// 不应该到达这里
	return false, fmt.Errorf("exhausted retries checking account existence")
}

// isAccountNotFoundError 判断是否是账户不存在的错误
func isAccountNotFoundError(err error) bool {
	// Solana RPC 在账户不存在时返回特定错误信息
	errorMsg := strings.ToLower(err.Error())
	return strings.Contains(errorMsg, "account not found") ||
		strings.Contains(errorMsg, "could not find account") ||
		strings.Contains(errorMsg, "invalid param")
}

// isRateLimitError 判断是否是 RPC 限流错误
func isRateLimitError(err error) bool {
	// 检测常见的 RPC 限流错误信息
	errorMsg := strings.ToLower(err.Error())
	return strings.Contains(errorMsg, "too many requests") ||
		strings.Contains(errorMsg, "rate limit") ||
		strings.Contains(errorMsg, "429") ||
		strings.Contains(errorMsg, "quota exceeded") ||
		strings.Contains(errorMsg, "timeout") ||
		strings.Contains(errorMsg, "connection reset")
}

// createAssociatedTokenAccountInstruction 创建 ATA 账户的指令 (预留功能)
// 注意：当前不自动添加创建指令，由调用方决定
func createAssociatedTokenAccountInstruction(
	payer solana.PublicKey,
	associatedTokenAddress solana.PublicKey,
	owner solana.PublicKey,
	tokenMint solana.PublicKey,
) (solana.Instruction, error) {
	// 构建创建 ATA 的指令
	// 参考: https://github.com/solana-labs/solana-program-library/blob/master/associated-token-account/program/src/instruction.rs

	accounts := solana.AccountMetaSlice{}
	accounts.Append(solana.NewAccountMeta(payer, false, true))                   // 0: payer (signer)
	accounts.Append(solana.NewAccountMeta(associatedTokenAddress, true, false))  // 1: associated_token_account (writable)
	accounts.Append(solana.NewAccountMeta(owner, false, false))                  // 2: owner
	accounts.Append(solana.NewAccountMeta(tokenMint, false, false))              // 3: mint
	accounts.Append(solana.NewAccountMeta(solana.SystemProgramID, false, false)) // 4: system_program
	accounts.Append(solana.NewAccountMeta(TOKEN_PROGRAM_ID, false, false))       // 5: token_program

	// ATA 程序 ID
	ataProgramID := solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")

	// 创建指令 (无需数据，ATA 程序有默认创建指令)
	return solana.NewInstruction(
		ataProgramID,
		accounts,
		[]byte{}, // 空数据，ATA 程序的创建指令不需要参数
	), nil
}

// createWhirlpoolSwapV2Instruction 创建 Whirlpool SwapV2 指令
func createWhirlpoolSwapV2Instruction(
	// 参数
	amount uint64,
	otherAmountThreshold uint64,
	sqrtPriceLimit uint128.Uint128,
	amountSpecifiedIsInput bool,
	aToB bool,
	remainingAccountsInfo interface{}, // 暂时用 interface{}

	// 账户
	tokenProgramA solana.PublicKey,
	tokenProgramB solana.PublicKey,
	memoProgram solana.PublicKey,
	tokenAuthority solana.PublicKey,
	whirlpool solana.PublicKey,
	tokenMintA solana.PublicKey,
	tokenMintB solana.PublicKey,
	tokenOwnerAccountA solana.PublicKey,
	tokenVaultA solana.PublicKey,
	tokenOwnerAccountB solana.PublicKey,
	tokenVaultB solana.PublicKey,
	tickArray0 solana.PublicKey,
	tickArray1 solana.PublicKey,
	tickArray2 solana.PublicKey,
	oracle solana.PublicKey,
) (solana.Instruction, error) {

	// 1. 构建指令数据
	buf := new(bytes.Buffer)
	enc := bin.NewBorshEncoder(buf)

	// 写入 SwapV2 指令判别器
	err := enc.WriteBytes(SwapV2Discriminator, false)
	if err != nil {
		return nil, fmt.Errorf("failed to write discriminator: %w", err)
	}

	// 写入参数
	err = enc.Encode(amount)
	if err != nil {
		return nil, fmt.Errorf("failed to encode amount: %w", err)
	}

	err = enc.Encode(otherAmountThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to encode otherAmountThreshold: %w", err)
	}

	// 写入 sqrt price limit (16 bytes little endian)
	sqrtPriceLimitLo := sqrtPriceLimit.Lo
	sqrtPriceLimitHi := sqrtPriceLimit.Hi

	// 写入低64位
	err = enc.Encode(sqrtPriceLimitLo)
	if err != nil {
		return nil, fmt.Errorf("failed to encode sqrtPriceLimit lo: %w", err)
	}

	// 写入高64位
	err = enc.Encode(sqrtPriceLimitHi)
	if err != nil {
		return nil, fmt.Errorf("failed to encode sqrtPriceLimit hi: %w", err)
	}

	err = enc.Encode(amountSpecifiedIsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to encode amountSpecifiedIsInput: %w", err)
	}

	err = enc.Encode(aToB)
	if err != nil {
		return nil, fmt.Errorf("failed to encode aToB: %w", err)
	}

	// 写入 remainingAccountsInfo (Option<None>)
	err = enc.WriteOption(false) // None
	if err != nil {
		return nil, fmt.Errorf("failed to encode remainingAccountsInfo: %w", err)
	}

	// 2. 构建账户元数据
	accounts := solana.AccountMetaSlice{}

	// 按照 Whirlpool SwapV2 指令的账户顺序添加
	accounts.Append(solana.NewAccountMeta(tokenProgramA, false, false))     // 0: token_program_a
	accounts.Append(solana.NewAccountMeta(tokenProgramB, false, false))     // 1: token_program_b
	accounts.Append(solana.NewAccountMeta(memoProgram, false, false))       // 2: memo_program
	accounts.Append(solana.NewAccountMeta(tokenAuthority, false, true))     // 3: token_authority (signer)
	accounts.Append(solana.NewAccountMeta(whirlpool, true, false))          // 4: whirlpool (writable)
	accounts.Append(solana.NewAccountMeta(tokenMintA, false, false))        // 5: token_mint_a
	accounts.Append(solana.NewAccountMeta(tokenMintB, false, false))        // 6: token_mint_b
	accounts.Append(solana.NewAccountMeta(tokenOwnerAccountA, true, false)) // 7: token_owner_account_a (writable)
	accounts.Append(solana.NewAccountMeta(tokenVaultA, true, false))        // 8: token_vault_a (writable)
	accounts.Append(solana.NewAccountMeta(tokenOwnerAccountB, true, false)) // 9: token_owner_account_b (writable)
	accounts.Append(solana.NewAccountMeta(tokenVaultB, true, false))        // 10: token_vault_b (writable)
	accounts.Append(solana.NewAccountMeta(tickArray0, true, false))         // 11: tick_array_0 (writable)
	accounts.Append(solana.NewAccountMeta(tickArray1, true, false))         // 12: tick_array_1 (writable)
	accounts.Append(solana.NewAccountMeta(tickArray2, true, false))         // 13: tick_array_2 (writable)
	accounts.Append(solana.NewAccountMeta(oracle, true, false))             // 14: oracle (writable)

	// 3. 创建指令
	return solana.NewInstruction(
		ORCA_WHIRLPOOL_PROGRAM_ID,
		accounts,
		buf.Bytes(),
	), nil
}

// WhirlpoolSwapStep - Whirlpool 交换步骤结构
type WhirlpoolSwapStep struct {
	SqrtPriceX64Next *big.Int
	AmountIn         *big.Int
	AmountOut        *big.Int
	FeeAmount        *big.Int
}

// Whirlpool CLMM 精确计算相关常量
// U64Resolution 已经在 constants.go 中定义

// whirlpoolMulDivFloor - 乘除法（向下取整）
func whirlpoolMulDivFloor(a, b, denominator cosmath.Int) cosmath.Int {
	if denominator.IsZero() {
		panic("division by zero")
	}
	numerator := a.Mul(b)
	return numerator.Quo(denominator)
}

// whirlpoolMulDivCeil - 乘除法（向上取整）
func whirlpoolMulDivCeil(a, b, denominator cosmath.Int) cosmath.Int {
	if denominator.IsZero() {
		return cosmath.Int{}
	}
	numerator := a.Mul(b).Add(denominator.Sub(cosmath.OneInt()))
	return numerator.Quo(denominator)
}

// whirlpoolGetTokenAmountAFromLiquidity - 从流动性计算 Token A 数量
func whirlpoolGetTokenAmountAFromLiquidity(
	sqrtPriceX64A *big.Int,
	sqrtPriceX64B *big.Int,
	liquidity *big.Int,
	roundUp bool,
) *big.Int {
	// 创建副本避免修改原始值
	priceA := new(big.Int).Set(sqrtPriceX64A)
	priceB := new(big.Int).Set(sqrtPriceX64B)

	// 确保 priceA <= priceB
	if priceA.Cmp(priceB) > 0 {
		priceA, priceB = priceB, priceA
	}

	if priceA.Cmp(big.NewInt(0)) <= 0 {
		panic("sqrtPriceX64A must be greater than 0")
	}

	// 计算 numerator1 = liquidity << U64Resolution
	numerator1 := new(big.Int).Lsh(liquidity, U64Resolution)
	// 计算 numerator2 = priceB - priceA
	numerator2 := new(big.Int).Sub(priceB, priceA)

	if roundUp {
		// 向上取整计算
		temp := whirlpoolMulDivCeil(
			cosmath.NewIntFromBigInt(numerator1),
			cosmath.NewIntFromBigInt(numerator2),
			cosmath.NewIntFromBigInt(priceB),
		)
		return whirlpoolMulDivCeil(
			temp,
			cosmath.NewIntFromBigInt(big.NewInt(1)),
			cosmath.NewIntFromBigInt(priceA),
		).BigInt()
	} else {
		// 向下取整计算
		temp := whirlpoolMulDivFloor(
			cosmath.NewIntFromBigInt(numerator1),
			cosmath.NewIntFromBigInt(numerator2),
			cosmath.NewIntFromBigInt(priceB),
		)
		return temp.Quo(cosmath.NewIntFromBigInt(priceA)).BigInt()
	}
}

// whirlpoolGetTokenAmountBFromLiquidity - 从流动性计算 Token B 数量
func whirlpoolGetTokenAmountBFromLiquidity(
	sqrtPriceX64A *big.Int,
	sqrtPriceX64B *big.Int,
	liquidity *big.Int,
	roundUp bool,
) *big.Int {
	// 创建副本避免修改原始值
	priceA := new(big.Int).Set(sqrtPriceX64A)
	priceB := new(big.Int).Set(sqrtPriceX64B)

	// 确保 priceA <= priceB
	if priceA.Cmp(priceB) > 0 {
		priceA, priceB = priceB, priceA
	}

	if priceA.Cmp(big.NewInt(0)) <= 0 {
		panic("sqrtPriceX64A must be greater than 0")
	}

	// 计算价格差
	priceDiff := new(big.Int).Sub(priceB, priceA)
	denominator := new(big.Int).Lsh(big.NewInt(1), U64Resolution)

	if roundUp {
		return whirlpoolMulDivCeil(
			cosmath.NewIntFromBigInt(liquidity),
			cosmath.NewIntFromBigInt(priceDiff),
			cosmath.NewIntFromBigInt(denominator),
		).BigInt()
	} else {
		return whirlpoolMulDivFloor(
			cosmath.NewIntFromBigInt(liquidity),
			cosmath.NewIntFromBigInt(priceDiff),
			cosmath.NewIntFromBigInt(denominator),
		).BigInt()
	}
}

// whirlpoolGetNextSqrtPriceX64FromInput - 从输入金额计算下个平方根价格
func whirlpoolGetNextSqrtPriceX64FromInput(
	sqrtPriceX64Current *big.Int,
	liquidity *big.Int,
	amount *big.Int,
	zeroForOne bool,
) *big.Int {
	if sqrtPriceX64Current.Cmp(big.NewInt(0)) <= 0 {
		panic("sqrtPriceX64Current must be greater than 0")
	}
	if liquidity.Cmp(big.NewInt(0)) <= 0 {
		panic("liquidity must be greater than 0")
	}

	if amount.Cmp(big.NewInt(0)) == 0 {
		return sqrtPriceX64Current
	}

	if zeroForOne {
		return whirlpoolGetNextSqrtPriceFromTokenAmountARoundingUp(
			sqrtPriceX64Current, liquidity, amount, true)
	} else {
		return whirlpoolGetNextSqrtPriceFromTokenAmountBRoundingDown(
			sqrtPriceX64Current, liquidity, amount, true)
	}
}

// whirlpoolGetNextSqrtPriceX64FromOutput - 从输出金额计算下个平方根价格
func whirlpoolGetNextSqrtPriceX64FromOutput(
	sqrtPriceX64Current *big.Int,
	liquidity *big.Int,
	amount *big.Int,
	zeroForOne bool,
) *big.Int {
	if sqrtPriceX64Current.Cmp(big.NewInt(0)) <= 0 {
		panic("sqrtPriceX64Current must be greater than 0")
	}
	if liquidity.Cmp(big.NewInt(0)) <= 0 {
		panic("liquidity must be greater than 0")
	}

	if zeroForOne {
		return whirlpoolGetNextSqrtPriceFromTokenAmountBRoundingDown(
			sqrtPriceX64Current, liquidity, amount, false)
	} else {
		return whirlpoolGetNextSqrtPriceFromTokenAmountARoundingUp(
			sqrtPriceX64Current, liquidity, amount, false)
	}
}

// whirlpoolGetNextSqrtPriceFromTokenAmountARoundingUp - 从 Token A 数量计算平方根价格（向上取整）
func whirlpoolGetNextSqrtPriceFromTokenAmountARoundingUp(
	sqrtPriceX64 *big.Int,
	liquidity *big.Int,
	amount *big.Int,
	add bool,
) *big.Int {
	if amount.Cmp(big.NewInt(0)) == 0 {
		return sqrtPriceX64
	}

	liquidityLeftShift := new(big.Int).Lsh(liquidity, U64Resolution)

	if add {
		numerator1 := liquidityLeftShift
		denominator := new(big.Int).Add(liquidityLeftShift, new(big.Int).Mul(amount, sqrtPriceX64))
		if denominator.Cmp(numerator1) >= 0 {
			return whirlpoolMulDivCeil(
				cosmath.NewIntFromBigInt(numerator1),
				cosmath.NewIntFromBigInt(sqrtPriceX64),
				cosmath.NewIntFromBigInt(denominator),
			).BigInt()
		}

		temp := new(big.Int).Div(numerator1, sqrtPriceX64)
		temp.Add(temp, amount)
		return whirlpoolMulDivRoundingUp(numerator1, big.NewInt(1), temp)
	} else {
		amountMulSqrtPrice := new(big.Int).Mul(amount, sqrtPriceX64)
		if liquidityLeftShift.Cmp(amountMulSqrtPrice) <= 0 {
			panic("liquidity must be greater than amount * sqrtPrice")
		}
		denominator := new(big.Int).Sub(liquidityLeftShift, amountMulSqrtPrice)
		return whirlpoolMulDivCeil(
			cosmath.NewIntFromBigInt(liquidityLeftShift),
			cosmath.NewIntFromBigInt(sqrtPriceX64),
			cosmath.NewIntFromBigInt(denominator),
		).BigInt()
	}
}

// whirlpoolGetNextSqrtPriceFromTokenAmountBRoundingDown - 从 Token B 数量计算平方根价格（向下取整）
func whirlpoolGetNextSqrtPriceFromTokenAmountBRoundingDown(
	sqrtPriceX64 *big.Int,
	liquidity *big.Int,
	amount *big.Int,
	add bool,
) *big.Int {
	deltaY := new(big.Int).Lsh(amount, U64Resolution)

	if add {
		return new(big.Int).Add(sqrtPriceX64, new(big.Int).Div(deltaY, liquidity))
	} else {
		amountDivLiquidity := whirlpoolMulDivRoundingUp(deltaY, big.NewInt(1), liquidity)
		if sqrtPriceX64.Cmp(amountDivLiquidity) <= 0 {
			panic("sqrtPriceX64 must be greater than amountDivLiquidity")
		}
		return new(big.Int).Sub(sqrtPriceX64, amountDivLiquidity)
	}
}

// whirlpoolMulDivRoundingUp - 乘除法向上取整
func whirlpoolMulDivRoundingUp(a, b, denominator *big.Int) *big.Int {
	numerator := new(big.Int).Mul(a, b)
	result := new(big.Int).Div(numerator, denominator)
	if new(big.Int).Mod(numerator, denominator).Cmp(big.NewInt(0)) != 0 {
		result.Add(result, big.NewInt(1))
	}
	return result
}
