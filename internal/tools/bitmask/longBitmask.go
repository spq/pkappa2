package bitmask

import "math/bits"

type (
	LongBitmask struct {
		mask []uint64
	}
)

func WrapAsLongBitmask(mask []uint64) LongBitmask {
	return LongBitmask{mask: mask}
}

func (bm LongBitmask) Mask() []uint64 {
	return bm.mask
}

func (bm LongBitmask) Copy() LongBitmask {
	return LongBitmask{
		mask: append([]uint64(nil), bm.mask...),
	}
}

func (bm LongBitmask) IsSet(bit uint) bool {
	idx, lbit := bit/64, bit%64
	if idx < uint(len(bm.mask)) {
		return (bm.mask[idx]>>lbit)&1 != 0
	}
	return false
}

func (bm LongBitmask) OnesCount() int {
	count := 0
	for _, m := range bm.mask {
		count += bits.OnesCount64(m)
	}
	return count
}

func (bm LongBitmask) Len() int {
	for idx := len(bm.mask) - 1; idx >= 0; idx-- {
		m := bm.mask[idx]
		if m != 0 {
			return idx*64 + bits.Len64(m)
		}
	}
	return 0
}

func (bm LongBitmask) IsZero() bool {
	for _, m := range bm.mask {
		if m != 0 {
			return false
		}
	}
	return true
}

func (bm LongBitmask) TrailingZerosFrom(bit uint) int {
	startIdx, lbit := bit/64, bit%64
	if startIdx >= uint(len(bm.mask)) {
		return -1
	}
	for idx, m := range bm.mask[startIdx:] {
		if idx == 0 {
			m >>= lbit
			m <<= lbit
		}
		if m == 0 {
			continue
		}
		return (idx)*64 + bits.TrailingZeros64(m) - int(lbit)
	}
	return -1
}

func (bm *LongBitmask) Set(bit uint) {
	idx, lbit := bit/64, bit%64
	if idx >= uint(len(bm.mask)) {
		bm.mask = append(bm.mask, make([]uint64, idx+1-uint(len(bm.mask)))...)
	}
	bm.mask[idx] |= 1 << lbit
}

func (bm *LongBitmask) Unset(bit uint) {
	idx, lbit := bit/64, bit%64
	if idx >= uint(len(bm.mask)) {
		return
	}
	bm.mask[idx] &= ^(1 << lbit)
}

func (bm *LongBitmask) Flip(bit uint) {
	idx, lbit := bit/64, bit%64
	if idx >= uint(len(bm.mask)) {
		bm.mask = append(bm.mask, make([]uint64, idx+1-uint(len(bm.mask)))...)
	}
	bm.mask[idx] ^= 1 << lbit
}

func (bm LongBitmask) Equal(other LongBitmask) bool {
	longer, shorter := bm.mask, other.mask
	if len(longer) < len(shorter) {
		longer, shorter = shorter, longer
	}
	for idx := range shorter {
		if shorter[idx] != longer[idx] {
			return false
		}
	}
	for _, m := range longer[len(shorter):] {
		if m != 0 {
			return false
		}
	}
	return true
}

func (bm LongBitmask) OrCopy(other LongBitmask) LongBitmask {
	res := bm.Copy()
	res.Or(other)
	return res
}

func (bm *LongBitmask) Or(other LongBitmask) {
	min := len(other.mask)
	if min > len(bm.mask) {
		min = len(bm.mask)
	}
	for idx := 0; idx < min; idx++ {
		bm.mask[idx] |= other.mask[idx]
	}
	if len(bm.mask) < len(other.mask) {
		bm.mask = append(bm.mask, other.mask[len(bm.mask):]...)
	}
}

func (bm LongBitmask) AndCopy(other LongBitmask) LongBitmask {
	res := bm.Copy()
	res.And(other)
	return res
}

func (bm *LongBitmask) And(other LongBitmask) {
	if len(bm.mask) > len(other.mask) {
		bm.mask = bm.mask[:len(other.mask)]
	}
	for idx := range bm.mask {
		bm.mask[idx] &= other.mask[idx]
	}
}

func (bm LongBitmask) XorCopy(other LongBitmask) LongBitmask {
	res := bm.Copy()
	res.Xor(other)
	return res
}

func (bm *LongBitmask) Xor(other LongBitmask) {
	min := len(other.mask)
	if min > len(bm.mask) {
		min = len(bm.mask)
	}
	for idx := 0; idx < min; idx++ {
		bm.mask[idx] ^= other.mask[idx]
	}
	if len(bm.mask) < len(other.mask) {
		bm.mask = append(bm.mask, other.mask[len(bm.mask):]...)
	}
}

func (bm LongBitmask) SubCopy(other LongBitmask) LongBitmask {
	res := bm.Copy()
	res.Sub(other)
	return res
}

func (bm *LongBitmask) Sub(other LongBitmask) {
	min := len(other.mask)
	if min > len(bm.mask) {
		min = len(bm.mask)
	}
	for idx := 0; idx < min; idx++ {
		bm.mask[idx] &= ^other.mask[idx]
	}
}

func (bm *LongBitmask) Shrink() {
	for len(bm.mask) != 0 {
		if bm.mask[len(bm.mask)-1] != 0 {
			return
		}
		bm.mask = bm.mask[:len(bm.mask)-1]
	}
}

func (bm LongBitmask) Next(bit *uint) bool {
	res := bm.TrailingZerosFrom(*bit)
	if res != -1 {
		*bit += uint(res)
		return true
	}
	return false
}

func (bm *LongBitmask) Inject(bit uint, value bool) {
	idx := bit / 64
	if idx >= uint(len(bm.mask)) {
		if value {
			bm.Set(bit)
		}
		return
	}
	bit = bit & 63
	m := &bm.mask[idx]
	carry := *m >= 1<<63
	*m = *m&((1<<bit)-1) | (*m&^((1<<bit)-1))<<1
	if value {
		*m |= 1 << bit
	}
	bm.Inject(idx*64+64, carry)
}
