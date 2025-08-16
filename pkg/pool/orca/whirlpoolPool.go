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

// Whirlpool 程序 ID
var ORCA_WHIRLPOOL_PROGRAM_ID = solana.MustPublicKeyFromBase58("whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc")

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

// Span 返回账户数据大小 - 需要根据实际 Whirlpool 账户大小调整
func (pool *WhirlpoolPool) Span() uint64 {
	// TODO: 需要根据实际 Whirlpool 账户大小计算
	// 暂时返回一个估算值，后续需要精确计算
	return uint64(653) // 估算值，需要验证
}

// Offset 返回字段偏移量 - 用于 RPC 查询过滤器
func (pool *WhirlpoolPool) Offset(field string) uint64 {
	// Add 8 bytes for discriminator
	baseOffset := uint64(8)

	switch field {
	case "TokenMintA":
		// whirlpoolsConfig(32) + whirlpoolBump(1) + tickSpacing(2) + feeTierIndexSeed(2) +
		// feeRate(2) + protocolFeeRate(2) + liquidity(16) + sqrtPrice(16) +
		// tickCurrentIndex(4) + protocolFeeOwedA(8) + protocolFeeOwedB(8)
		return baseOffset + 32 + 1 + 2 + 2 + 2 + 2 + 16 + 16 + 4 + 8 + 8 // = 93
	case "TokenMintB":
		// TokenMintA(32) + tokenVaultA(32) + feeGrowthGlobalA(16) 之后
		return baseOffset + 93 + 32 + 32 + 16 // = 173
	}
	return 0
}

// Quote 方法 - 获取交换报价 (基础实现，返回虚拟报价用于测试)
func (pool *WhirlpoolPool) Quote(ctx context.Context, solClient *rpc.Client, inputMint string, inputAmount cosmath.Int) (cosmath.Int, error) {
	// TODO: 实现真正的报价计算逻辑，参考 CLMM 的实现
	// 现在返回一个基础的虚拟报价用于测试框架

	// 检查输入代币是哪个
	var isAtoB bool
	if inputMint == pool.TokenMintA.String() {
		isAtoB = true
	} else if inputMint == pool.TokenMintB.String() {
		isAtoB = false
	} else {
		return cosmath.Int{}, fmt.Errorf("input mint %s not found in pool", inputMint)
	}

	// 简单的虚拟计算：假设 1:1000 的汇率 (仅用于测试)
	// 实际实现需要根据池的流动性、价格等计算
	if isAtoB {
		// A -> B, 假设 B 的价值更高
		outputAmount := inputAmount.Mul(cosmath.NewInt(1000))
		return outputAmount.Neg(), nil // 返回负数表示输出金额
	} else {
		// B -> A, 假设 A 的价值更低
		outputAmount := inputAmount.Quo(cosmath.NewInt(1000))
		return outputAmount.Neg(), nil // 返回负数表示输出金额
	}
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
