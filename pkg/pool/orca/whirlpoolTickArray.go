package orca

import (
	"fmt"
	"math"
	"math/big"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"lukechampine.com/uint128"
)

// WhirlpoolTickArrayBitmapExtensionType - Whirlpool version of tick array bitmap extension
type WhirlpoolTickArrayBitmapExtensionType struct {
	PoolId                  solana.PublicKey
	PositiveTickArrayBitmap [][]uint64
	NegativeTickArrayBitmap [][]uint64
}

// WhirlpoolTickArray - Whirlpool version of tick array, based on CLMM but uses Whirlpool-specific size
type WhirlpoolTickArray struct {
	_                    [8]byte              `bin:"skip"`         // padding
	PoolId               solana.PublicKey     `bin:"fixed"`        // 32 bytes
	StartTickIndex       int32                `bin:"le"`           // 4 bytes
	Ticks                []WhirlpoolTickState `bin:"array,len=88"` // TICK_ARRAY_SIZE=88 for Whirlpool
	InitializedTickCount uint8                // 1 byte
	_                    [115]byte            `bin:"skip"` // padding
}

// WhirlpoolTickState - Whirlpool version of tick state, similar structure to CLMM but fields may differ
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

// Decode parses Whirlpool tick array data
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

// Whirlpool version utility functions - Copied from CLMM implementation with adjusted parameters

// getTickCount returns the number of ticks in tick array - Whirlpool uses 88 instead of 60
func getWhirlpoolTickCount(tickSpacing int64) int64 {
	return tickSpacing * TICK_ARRAY_SIZE // TICK_ARRAY_SIZE = 88 for Whirlpool
}

// getTickArrayStartIndex gets the start index of tick array
func getWhirlpoolTickArrayStartIndex(tick int64, tickSpacing int64) int64 {
	return tick - tick%getWhirlpoolTickCount(tickSpacing)
}

// GetWhirlpoolTickArrayStartIndexByTick gets tick array start index by tick (exported version)
func GetWhirlpoolTickArrayStartIndexByTick(tickIndex int64, tickSpacing int64) int64 {
	return getWhirlpoolTickArrayStartIndexByTick(tickIndex, tickSpacing)
}

// getWhirlpoolTickArrayStartIndexByTick gets tick array start index by tick
func getWhirlpoolTickArrayStartIndexByTick(tickIndex int64, tickSpacing int64) int64 {
	ticksInArray := getWhirlpoolTickCount(tickSpacing)
	start := math.Floor(float64(tickIndex) / float64(ticksInArray))
	return int64(start * float64(ticksInArray))
}

// maxTickInTickarrayBitmap Whirlpool version of maximum tick
func maxWhirlpoolTickInTickarrayBitmap(tickSpacing int64) int64 {
	return TICK_ARRAY_BITMAP_SIZE * getWhirlpoolTickCount(tickSpacing)
}

// TickArrayOffsetInBitmap calculates tick array offset in bitmap
func WhirlpoolTickArrayOffsetInBitmap(tickArrayStartIndex int64, tickSpacing int64) int64 {
	m := abs(tickArrayStartIndex)
	tickArrayOffsetInBitmap := m / getWhirlpoolTickCount(tickSpacing)

	if tickArrayStartIndex < 0 && m != 0 {
		tickArrayOffsetInBitmap = TICK_ARRAY_BITMAP_SIZE - tickArrayOffsetInBitmap
	}

	return tickArrayOffsetInBitmap
}

// abs returns the absolute value of integer
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// getFirstInitializedWhirlpoolTickArray - Whirlpool version of getting first initialized tick array
func (pool *WhirlpoolPool) getFirstInitializedWhirlpoolTickArray(zeroForOne bool, exTickArrayBitmap *WhirlpoolTickArrayBitmapExtensionType) (int64, solana.PublicKey, error) {
	// 1. Calculate start index of tick array containing current tick
	startIndex := getWhirlpoolTickArrayStartIndexByTick(int64(pool.TickCurrentIndex), int64(pool.TickSpacing))

	// 2. For simplified implementation, temporarily return calculated start index
	// TODO: Implement complete bitmap lookup logic, refer to CLMM implementation

	// 3. Construct tick array address (using real PDA derivation)
	tickArrayPDA, err := DeriveWhirlpoolTickArrayPDA(pool.PoolId, startIndex)
	if err != nil {
		return 0, solana.PublicKey{}, fmt.Errorf("failed to derive tick array PDA: %w", err)
	}

	return startIndex, tickArrayPDA, nil
}

// isOverflowDefaultWhirlpoolTickarrayBitmap checks if exceeding default bitmap range
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

// whirlpoolTickRange gets Whirlpool tick range
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

// Whirlpool version bitmap operation functions - Reuse CLMM logic

// MergeWhirlpoolTickArrayBitmap merges tick array bitmap
func MergeWhirlpoolTickArrayBitmap(bns []uint64) *big.Int {
	result := new(big.Int)

	// Iterate through array
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

// IsZero checks if big.Int is zero
func IsZero(bitNum int, data *big.Int) bool {
	return data.Sign() == 0
}

// TrailingZeros calculates the number of trailing zeros
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

// LeadingZeros calculates the number of leading zeros
func LeadingZeros(bitNum int, data *big.Int) *int {
	if IsZero(bitNum, data) {
		return nil
	}

	// Get position of highest bit
	bitLen := data.BitLen()

	if bitLen == 0 {
		return nil
	}

	// Calculate leading zeros
	leadingZeros := bitNum - bitLen
	if leadingZeros < 0 {
		leadingZeros = 0
	}

	return &leadingZeros
}

// MostSignificantBit gets the most significant bit
func MostSignificantBit(bitNum int, data *big.Int) *int {
	// Check if zero
	if IsZero(bitNum, data) {
		return nil
	}
	// Return number of leading zeros
	return LeadingZeros(bitNum, data)
}

// DeriveWhirlpoolTickArrayPDA derives PDA address for Whirlpool tick array
// Based on Whirlpool source code implementation: seeds = ["tick_array", whirlpool_pubkey, start_tick_index.to_string()]
func DeriveWhirlpoolTickArrayPDA(whirlpoolPubkey solana.PublicKey, startTickIndex int64) (solana.PublicKey, error) {
	// Convert start_tick_index to string byte array, consistent with Whirlpool source code
	// Source code: start_tick_index.to_string().as_bytes()
	startTickIndexStr := fmt.Sprintf("%d", startTickIndex)
	startTickIndexBytes := []byte(startTickIndexStr)

	// Build seeds
	seeds := [][]byte{
		[]byte(TICK_ARRAY_SEED), // "tick_array"
		whirlpoolPubkey.Bytes(), // whirlpool address (32 bytes)
		startTickIndexBytes,     // start_tick_index string bytes
	}

	// Derive PDA
	pda, _, err := solana.FindProgramAddress(seeds, ORCA_WHIRLPOOL_PROGRAM_ID)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to find program address for tick array: %w", err)
	}

	return pda, nil
}

// DeriveMultipleWhirlpoolTickArrayPDAs derives multiple tick array PDA addresses
// Used for tick_array0, tick_array1, tick_array2 needed in swap instructions
// Implements correct tick array sequence calculation based on Whirlpool source code
func DeriveMultipleWhirlpoolTickArrayPDAs(whirlpoolPubkey solana.PublicKey, currentTick int64, tickSpacing int64, aToB bool) (tickArray0, tickArray1, tickArray2 solana.PublicKey, err error) {
	// 1. Based on Whirlpool source code get_start_tick_indexes function implementation
	tickCurrentIndex := int32(currentTick)
	tickSpacingI32 := int32(tickSpacing)
	ticksInArray := TICK_ARRAY_SIZE * tickSpacingI32 // TICK_ARRAY_SIZE = 88

	// 2. Calculate start_tick_index_base (floor division)
	startTickIndexBase := floorDivision(tickCurrentIndex, ticksInArray) * ticksInArray

	// 3. Calculate offset based on swap direction
	var offsets []int32
	if aToB {
		// A -> B: price decreases, need current and previous tick arrays
		offsets = []int32{0, -1, -2}
	} else {
		// B -> A: price increases, need current and subsequent tick arrays
		// Check if already crossed to next tick array
		shifted := tickCurrentIndex+tickSpacingI32 >= startTickIndexBase+ticksInArray
		if shifted {
			offsets = []int32{1, 2, 3}
		} else {
			offsets = []int32{0, 1, 2}
		}
	}

	// 4. Derive three tick array PDAs
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

// floorDivision implements integer division (floor), consistent with floor_division in Whirlpool source code
func floorDivision(dividend, divisor int32) int32 {
	if (dividend < 0) != (divisor < 0) && dividend%divisor != 0 {
		return dividend/divisor - 1
	}
	return dividend / divisor
}

// DeriveWhirlpoolOraclePDA derives PDA address for Whirlpool Oracle
// Based on Solana PDA derivation rules: seeds = ["oracle", whirlpool_pubkey]
func DeriveWhirlpoolOraclePDA(whirlpoolPubkey solana.PublicKey) (solana.PublicKey, error) {
	// Build seeds
	seeds := [][]byte{
		[]byte("oracle"),        // "oracle"
		whirlpoolPubkey.Bytes(), // whirlpool address (32 bytes)
	}

	// Derive PDA
	pda, _, err := solana.FindProgramAddress(seeds, ORCA_WHIRLPOOL_PROGRAM_ID)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to find program address for oracle: %w", err)
	}

	return pda, nil
}
