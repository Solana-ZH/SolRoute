package orca

import (
	"fmt"
	"math"
	"math/big"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"lukechampine.com/uint128"
)

// WhirlpoolTickArrayBitmapExtensionType - Whirlpool 版本的 tick array bitmap 扩展
type WhirlpoolTickArrayBitmapExtensionType struct {
	PoolId                  solana.PublicKey
	PositiveTickArrayBitmap [][]uint64
	NegativeTickArrayBitmap [][]uint64
}

// WhirlpoolTickArray - Whirlpool 版本的 tick array，参考 CLMM 但使用 Whirlpool 特有的大小
type WhirlpoolTickArray struct {
	_                    [8]byte              `bin:"skip"`         // padding
	PoolId               solana.PublicKey     `bin:"fixed"`        // 32 bytes
	StartTickIndex       int32                `bin:"le"`           // 4 bytes
	Ticks                []WhirlpoolTickState `bin:"array,len=88"` // TICK_ARRAY_SIZE=88 for Whirlpool
	InitializedTickCount uint8                // 1 byte
	_                    [115]byte            `bin:"skip"` // padding
}

// WhirlpoolTickState - Whirlpool 版本的 tick state，结构类似 CLMM 但字段可能有差异
type WhirlpoolTickState struct {
	Tick                    int32              `bin:"le"`   // 4 bytes
	LiquidityNet            int64              `bin:"le"`   // 8 bytes
	_                       [8]byte            `bin:"skip"` // skip high 8 bytes
	LiquidityGross          uint128.Uint128    `bin:"le"`   // 16 bytes
	FeeGrowthOutsideX64A    uint128.Uint128    `bin:"le"`   // 16 bytes
	FeeGrowthOutsideX64B    uint128.Uint128    `bin:"le"`   // 16 bytes
	RewardGrowthsOutsideX64 [3]uint128.Uint128 `bin:"le"`   // 48 bytes
	_                       [52]byte           `bin:"skip"` // padding
}

// Decode 解析 Whirlpool tick array 数据
func (t *WhirlpoolTickArray) Decode(data []byte) error {
	decoder := bin.NewBinDecoder(data)

	// Decode initial padding
	var padding [8]byte
	err := decoder.Decode(&padding)
	if err != nil {
		return fmt.Errorf("failed to decode padding: %w", err)
	}

	// Decode pool ID
	err = decoder.Decode(&t.PoolId)
	if err != nil {
		return fmt.Errorf("failed to decode pool ID: %w", err)
	}

	// Decode start tick index
	err = decoder.Decode(&t.StartTickIndex)
	if err != nil {
		return fmt.Errorf("failed to decode start tick index: %w", err)
	}

	// Decode ticks array
	t.Ticks = make([]WhirlpoolTickState, TICK_ARRAY_SIZE)
	for i := 0; i < TICK_ARRAY_SIZE; i++ {
		err = decoder.Decode(&t.Ticks[i])
		if err != nil {
			return fmt.Errorf("failed to decode tick %d: %w", i, err)
		}
	}

	// Decode initialized tick count
	err = decoder.Decode(&t.InitializedTickCount)
	if err != nil {
		return fmt.Errorf("failed to decode initialized tick count: %w", err)
	}

	return nil
}

// Whirlpool 版本的工具函数 - 复制自 CLMM 实现并调整参数

// getTickCount 返回 tick array 中 tick 的数量 - Whirlpool 使用 88 而不是 60
func getWhirlpoolTickCount(tickSpacing int64) int64 {
	return tickSpacing * TICK_ARRAY_SIZE // TICK_ARRAY_SIZE = 88 for Whirlpool
}

// getTickArrayStartIndex 获取 tick array 的起始索引
func getWhirlpoolTickArrayStartIndex(tick int64, tickSpacing int64) int64 {
	return tick - tick%getWhirlpoolTickCount(tickSpacing)
}

// GetWhirlpoolTickArrayStartIndexByTick 根据 tick 获取 tick array 起始索引（导出版本）
func GetWhirlpoolTickArrayStartIndexByTick(tickIndex int64, tickSpacing int64) int64 {
	return getWhirlpoolTickArrayStartIndexByTick(tickIndex, tickSpacing)
}

// getWhirlpoolTickArrayStartIndexByTick 根据 tick 获取 tick array 起始索引
func getWhirlpoolTickArrayStartIndexByTick(tickIndex int64, tickSpacing int64) int64 {
	ticksInArray := getWhirlpoolTickCount(tickSpacing)
	start := math.Floor(float64(tickIndex) / float64(ticksInArray))
	return int64(start * float64(ticksInArray))
}

// maxTickInTickarrayBitmap Whirlpool 版本的最大 tick
func maxWhirlpoolTickInTickarrayBitmap(tickSpacing int64) int64 {
	return TICK_ARRAY_BITMAP_SIZE * getWhirlpoolTickCount(tickSpacing)
}

// TickArrayOffsetInBitmap 计算 tick array 在 bitmap 中的偏移
func WhirlpoolTickArrayOffsetInBitmap(tickArrayStartIndex int64, tickSpacing int64) int64 {
	m := abs(tickArrayStartIndex)
	tickArrayOffsetInBitmap := m / getWhirlpoolTickCount(tickSpacing)

	if tickArrayStartIndex < 0 && m != 0 {
		tickArrayOffsetInBitmap = TICK_ARRAY_BITMAP_SIZE - tickArrayOffsetInBitmap
	}

	return tickArrayOffsetInBitmap
}

// abs 返回整数的绝对值
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// getFirstInitializedWhirlpoolTickArray - Whirlpool 版本的第一个初始化 tick array 获取
func (pool *WhirlpoolPool) getFirstInitializedWhirlpoolTickArray(zeroForOne bool, exTickArrayBitmap *WhirlpoolTickArrayBitmapExtensionType) (int64, solana.PublicKey, error) {
	// 1. 计算当前 tick 所在的 tick array 起始索引
	startIndex := getWhirlpoolTickArrayStartIndexByTick(int64(pool.TickCurrentIndex), int64(pool.TickSpacing))

	// 2. 为简化实现，暂时返回计算出的起始索引
	// TODO: 实现完整的 bitmap 查找逻辑，参考 CLMM 的实现

	// 3. 构造 tick array 地址（使用真实的 PDA 推导）
	tickArrayPDA, err := DeriveWhirlpoolTickArrayPDA(pool.PoolId, startIndex)
	if err != nil {
		return 0, solana.PublicKey{}, fmt.Errorf("failed to derive tick array PDA: %w", err)
	}

	return startIndex, tickArrayPDA, nil
}

// isOverflowDefaultWhirlpoolTickarrayBitmap 检查是否超出默认 bitmap 范围
func isOverflowDefaultWhirlpoolTickarrayBitmap(tickSpacing int64, tickarrayStartIndexs []int64) bool {
	tickRange := whirlpoolTickRange(tickSpacing)
	maxTickBoundary := tickRange.maxTickBoundary
	minTickBoundary := tickRange.minTickBoundary

	for _, tickIndex := range tickarrayStartIndexs {
		if tickIndex >= maxTickBoundary || tickIndex < minTickBoundary {
			return true
		}
	}
	return false
}

// whirlpoolTickRange 获取 Whirlpool tick 范围
func whirlpoolTickRange(tickSpacing int64) struct {
	maxTickBoundary int64
	minTickBoundary int64
} {
	maxTickBoundary := maxWhirlpoolTickInTickarrayBitmap(tickSpacing)
	minTickBoundary := -maxTickBoundary

	if maxTickBoundary > MAX_TICK {
		maxTickBoundary = getWhirlpoolTickArrayStartIndex(MAX_TICK, tickSpacing) + getWhirlpoolTickCount(tickSpacing)
	}
	if minTickBoundary < MIN_TICK {
		minTickBoundary = getWhirlpoolTickArrayStartIndex(MIN_TICK, tickSpacing)
	}
	return struct {
		maxTickBoundary int64
		minTickBoundary int64
	}{
		maxTickBoundary: maxTickBoundary,
		minTickBoundary: minTickBoundary,
	}
}

// Whirlpool 版本的 bitmap 操作函数 - 复用 CLMM 的逻辑

// MergeWhirlpoolTickArrayBitmap 合并 tick array bitmap
func MergeWhirlpoolTickArrayBitmap(bns []uint64) *big.Int {
	result := new(big.Int)

	// 遍历数组
	for i, bn := range bns {
		// Convert uint64 to big.Int
		bnBig := new(big.Int).SetUint64(bn)

		// Shift by 64 * i bits
		bnBig.Lsh(bnBig, uint(64*i))

		// OR with result
		result.Or(result, bnBig)
	}

	return result
}

// IsZero 检查 big.Int 是否为零
func IsZero(bitNum int, data *big.Int) bool {
	return data.Sign() == 0
}

// TrailingZeros 计算尾随零的数量
func TrailingZeros(bitNum int, data *big.Int) *int {
	if IsZero(bitNum, data) {
		return nil
	}

	count := 0
	temp := new(big.Int).Set(data)

	for temp.Bit(count) == 0 {
		count++
		if count >= bitNum {
			return nil
		}
	}

	return &count
}

// LeadingZeros 计算前导零的数量
func LeadingZeros(bitNum int, data *big.Int) *int {
	if IsZero(bitNum, data) {
		return nil
	}

	// 获取最高位的位置
	bitLen := data.BitLen()

	if bitLen == 0 {
		return nil
	}

	// 计算前导零
	leadingZeros := bitNum - bitLen
	if leadingZeros < 0 {
		leadingZeros = 0
	}

	return &leadingZeros
}

// MostSignificantBit 获取最高有效位
func MostSignificantBit(bitNum int, data *big.Int) *int {
	// 检查是否为零
	if IsZero(bitNum, data) {
		return nil
	}
	// 返回前导零的数量
	return LeadingZeros(bitNum, data)
}

// DeriveWhirlpoolTickArrayPDA 推导 Whirlpool tick array 的 PDA 地址
// 基于 Whirlpool 源码实现：种子 = ["tick_array", whirlpool_pubkey, start_tick_index.to_string()]
func DeriveWhirlpoolTickArrayPDA(whirlpoolPubkey solana.PublicKey, startTickIndex int64) (solana.PublicKey, error) {
	// 将 start_tick_index 转换为字符串字节数组，与 Whirlpool 源码保持一致
	// 源码：start_tick_index.to_string().as_bytes()
	startTickIndexStr := fmt.Sprintf("%d", startTickIndex)
	startTickIndexBytes := []byte(startTickIndexStr)

	// 构建种子
	seeds := [][]byte{
		[]byte(TICK_ARRAY_SEED), // "tick_array"
		whirlpoolPubkey.Bytes(), // whirlpool 地址 (32 bytes)
		startTickIndexBytes,     // start_tick_index 字符串字节
	}

	// 推导 PDA
	pda, _, err := solana.FindProgramAddress(seeds, ORCA_WHIRLPOOL_PROGRAM_ID)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to find program address for tick array: %w", err)
	}

	return pda, nil
}

// DeriveMultipleWhirlpoolTickArrayPDAs 推导多个 tick array PDA 地址
// 用于交换指令中需要的 tick_array0, tick_array1, tick_array2
// 根据 Whirlpool 源码实现正确的 tick array 序列计算
func DeriveMultipleWhirlpoolTickArrayPDAs(whirlpoolPubkey solana.PublicKey, currentTick int64, tickSpacing int64, aToB bool) (tickArray0, tickArray1, tickArray2 solana.PublicKey, err error) {
	// 1. 基于 Whirlpool 源码的 get_start_tick_indexes 函数实现
	tickCurrentIndex := int32(currentTick)
	tickSpacingI32 := int32(tickSpacing)
	ticksInArray := TICK_ARRAY_SIZE * tickSpacingI32 // TICK_ARRAY_SIZE = 88

	// 2. 计算 start_tick_index_base（向下整除）
	startTickIndexBase := floorDivision(tickCurrentIndex, ticksInArray) * ticksInArray

	// 3. 根据交换方向计算偏移量
	var offsets []int32
	if aToB {
		// A -> B: 价格下降，需要当前及之前的 tick arrays
		offsets = []int32{0, -1, -2}
	} else {
		// B -> A: 价格上升，需要当前及之后的 tick arrays
		// 检查是否已跨越到下一个 tick array
		shifted := tickCurrentIndex+tickSpacingI32 >= startTickIndexBase+ticksInArray
		if shifted {
			offsets = []int32{1, 2, 3}
		} else {
			offsets = []int32{0, 1, 2}
		}
	}

	// 4. 推导三个 tick array PDA
	startIndex0 := startTickIndexBase + offsets[0]*ticksInArray
	tickArray0, err = DeriveWhirlpoolTickArrayPDA(whirlpoolPubkey, int64(startIndex0))
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("failed to derive tick_array0: %w", err)
	}

	startIndex1 := startTickIndexBase + offsets[1]*ticksInArray
	tickArray1, err = DeriveWhirlpoolTickArrayPDA(whirlpoolPubkey, int64(startIndex1))
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("failed to derive tick_array1: %w", err)
	}

	startIndex2 := startTickIndexBase + offsets[2]*ticksInArray
	tickArray2, err = DeriveWhirlpoolTickArrayPDA(whirlpoolPubkey, int64(startIndex2))
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("failed to derive tick_array2: %w", err)
	}

	return tickArray0, tickArray1, tickArray2, nil
}

// floorDivision 实现整数除法（向下取整），与 Whirlpool 源码中的 floor_division 一致
func floorDivision(dividend, divisor int32) int32 {
	if (dividend < 0) != (divisor < 0) && dividend%divisor != 0 {
		return dividend/divisor - 1
	}
	return dividend / divisor
}

// DeriveWhirlpoolOraclePDA 推导 Whirlpool Oracle 的 PDA 地址
// 基于 Solana PDA 推导规则：种子 = ["oracle", whirlpool_pubkey]
func DeriveWhirlpoolOraclePDA(whirlpoolPubkey solana.PublicKey) (solana.PublicKey, error) {
	// 构建种子
	seeds := [][]byte{
		[]byte("oracle"),        // "oracle"
		whirlpoolPubkey.Bytes(), // whirlpool 地址 (32 bytes)
	}

	// 推导 PDA
	pda, _, err := solana.FindProgramAddress(seeds, ORCA_WHIRLPOOL_PROGRAM_ID)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to find program address for oracle: %w", err)
	}

	return pda, nil
}
