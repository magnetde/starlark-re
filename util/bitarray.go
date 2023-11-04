package util

import "math/bits"

const (
	blockw        = 32  // width of each block
	defaultFactor = 8   // default grouping of blocks
	minOptimLen   = 128 // minimum length for optimizing
)

// BitArray is a type that represents bitarrays and is optimized for rank and select queries.
// It is based on "Practical Implementation of Rank and Select Queries" (R. Gonzalez, V. Makinen, 2005).
type BitArray struct {
	data []uint32
	len  uint
	ones uint

	factor uint
	s      uint
	rs     []uint32
}

// Grow increases the capacity to guarantee space for `n` extra bits.
func (b *BitArray) Grow(n int) {
	b.ensureCap(b.len + uint(n))
}

// ensureCap guarantees space for `n` bits.
func (b *BitArray) ensureCap(n uint) {
	size := uint(len(b.data))
	expected := divup(n, blockw)

	if size < expected {
		b.data = append(b.data, make([]uint32, expected-size)...)
	}
}

// divup performs the integer division (a / b) and rounds up the result.
func divup(a, b uint) uint {
	return (a + b - 1) / b
}

// Append appends a bit with a value of `v` to the bitarray.
func (b *BitArray) Append(v bool) {
	b.ensureCap(b.len + 1)

	if v {
		b.set(b.len)
		b.ones++
	}

	b.len++
	b.rs = nil
}

// set sets the `i`-th bit to 1.
// The caller must ensure, that the `i`-th bit exists, or else this function panics.
func (b *BitArray) set(i uint) {
	bitoff := i % blockw
	valindex := i / blockw

	mask := uint32(1) << (blockw - bitoff - 1)
	b.data[valindex] |= mask
}

// AppendN appends `n` bits with a value of `v` to the bitarray.
func (b *BitArray) AppendN(v bool, n int) {
	if n <= 0 {
		return
	}

	b.ensureCap(b.len + uint(n))

	if v {
		for i := uint(0); i < uint(n); i++ {
			b.set(b.len + i)
		}

		b.ones += uint(n)
	}

	b.len += uint(n)
	b.rs = nil
}

// Optimize optimizes this bitarray for rank and select queries,
// making it possible for rank to be performed in O(1) and select in O(log n) instead of O(n).
// This optimization is only done when the length exceeds the minimum limit for optimization.
func (b *BitArray) Optimize() {
	if b.len < minOptimLen {
		return
	}

	const factor = uint(defaultFactor)

	s := blockw * factor
	numSBlock := divup(b.len, s)

	rs := make([]uint32, numSBlock)

	for i := uint(1); i < numSBlock; i++ {
		start := (i - 1) * factor
		end := min(start+factor, uint(len(b.data)))

		rs[i] = rs[i-1] + popcnt(b.data[start:end])
	}

	b.factor = factor
	b.s = s
	b.rs = rs
}

// popcnt returns the number of 1-bits in `v`.
func popcnt(v []uint32) uint32 {
	c := uint32(0)
	for _, i := range v {
		c += popcnt32(i)
	}
	return c
}

// popcnt32 returns the number of 1-bits in `v`.
func popcnt32(v uint32) uint32 {
	return uint32(bits.OnesCount32(v))
}

// Rank performs the rank query on `b`, which is the number of 1-bits up to position `i`.
func (b *BitArray) Rank(i int) int {
	if i < 0 {
		return 0
	}
	return int(b.rank(uint(i)))
}

// rank is the subroutine for Rank.
func (b *BitArray) rank(i uint) uint32 {
	if i >= b.len {
		return uint32(b.ones)
	}

	i++

	res := uint32(0)
	aux := uint(0)
	if b.rs != nil {
		res = b.rs[i/b.s]
		aux = (i / b.s) * b.factor
	}

	bitLen := i - blockw*aux

	if bitLen != 0 {
		valLen := divup(bitLen, blockw)
		data := b.data[(blockw*aux)/blockw:]

		endbits := valLen*blockw - bitLen

		// Calculate number of 1-bits in the corresponding data
		if endbits != 0 {
			endIndex := valLen - 1
			res += popcnt(data[:endIndex])
			res += popcnt32(data[endIndex] >> endbits)
		} else {
			res += popcnt(data[:valLen])
		}
	}

	return res

}

// Select performs the select query on `b`, which is the position of the `i`-th 1-bit.
func (b *BitArray) Select(i int) int {
	if i <= 0 || i > int(b.ones) {
		return -1
	}

	var res uint
	if b.rs != nil {
		res = b.selectRg(uint(i))
	} else {
		res = b.selectBlocks(uint(i), 0)
	}

	return int(res)
}

// selectRg performs an optimized select query.
func (b *BitArray) selectRg(i uint) uint {
	lv := uint(0)
	rv := b.len / b.s
	mid := (lv + rv) / 2
	rankmid := uint(b.rs[mid])

	for lv <= rv {
		if rankmid < i {
			lv = mid + 1
		} else {
			rv = mid - 1
		}

		mid = (lv + rv) / 2
		rankmid = uint(b.rs[mid])
	}

	pos := mid * b.factor
	i -= rankmid

	return b.selectBlocks(i, pos)
}

// selectBlocks performs an unoptimized select query, starting at position `pos`.
func (b *BitArray) selectBlocks(i uint, pos uint) uint {
	numblocks := divup(b.len, blockw)

	var j uint32
	var ones uint
	for {
		j = b.data[pos]
		ones = uint(popcnt32(j))

		if ones >= i {
			break
		}

		i -= ones

		pos++
		if pos > numblocks {
			return b.len
		}
	}

	// Reverse the block if needed
	j = bits.Reverse32(j)

	return blockw*pos + selectBit(j, i-1)
}
