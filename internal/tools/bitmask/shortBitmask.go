package bitmask

import "math/bits"

type (
	ShortBitmask struct {
		mask uint64
		next *ShortBitmask
	}
)

func MakeShortBitmask(mask uint64) ShortBitmask {
	return ShortBitmask{
		mask: mask,
	}
}

func (bm ShortBitmask) Copy() ShortBitmask {
	var next *ShortBitmask
	if bm.next != nil {
		tmp := bm.next.Copy()
		next = &tmp
	}
	return ShortBitmask{
		mask: bm.mask,
		next: next,
	}
}

func (bm ShortBitmask) IsSet(bit uint) bool {
	for {
		if bit < 64 {
			return (bm.mask>>bit)&1 != 0
		}
		if bm.next == nil {
			return false
		}
		bm = *bm.next
		bit -= 64
	}
}

func (bm ShortBitmask) OnesCount() int {
	count := 0
	for {
		count += bits.OnesCount64(bm.mask)

		if bm.next == nil {
			return count
		}
		bm = *bm.next
	}
}

func (bm ShortBitmask) Len() int {
	if bm.next != nil {
		if l := bm.next.Len(); l != 0 {
			return l + 64
		}
	}
	return bits.Len64(bm.mask)
}

func (bm ShortBitmask) IsZero() bool {
	for {
		if bm.mask != 0 {
			return false
		}
		if bm.next == nil {
			return true
		}
		bm = *bm.next
	}
}

func (bm *ShortBitmask) Set(bit uint) {
	for {
		if bit < 64 {
			bm.mask |= 1 << bit
			break
		}
		if bm.next == nil {
			bm.next = &ShortBitmask{}
		}
		bm = bm.next
		bit -= 64
	}
}

func (bm *ShortBitmask) Unset(bit uint) {
	for {
		if bit < 64 {
			bm.mask &= ^(1 << bit)
			break
		}
		if bm.next == nil {
			bm.next = &ShortBitmask{}
		}
		bm = bm.next
		bit -= 64
	}
}

func (bm *ShortBitmask) Flip(bit uint) {
	for {
		if bit < 64 {
			bm.mask ^= 1 << bit
			break
		}
		if bm.next == nil {
			bm.next = &ShortBitmask{}
		}
		bm = bm.next
		bit -= 64
	}
}

func (bm ShortBitmask) Equal(other ShortBitmask) bool {
	for {
		if bm.mask != other.mask {
			return false
		}
		if bm.next == nil && other.next == nil {
			return true
		}
		if bm.next == nil {
			return other.next.IsZero()
		}
		if other.next == nil {
			return bm.next.IsZero()
		}
		bm = *bm.next
		other = *other.next
	}
}

func (bm *ShortBitmask) OrCopy(other ShortBitmask) ShortBitmask {
	res := bm.Copy()
	res.Or(other)
	return res
}

func (bm *ShortBitmask) Or(other ShortBitmask) {
	for {
		bm.mask |= other.mask

		if other.next == nil {
			break
		}
		if bm.next == nil {
			bm.next = &ShortBitmask{}
		}
		bm = bm.next
		other = *other.next
	}
}

func (bm ShortBitmask) AndCopy(other ShortBitmask) ShortBitmask {
	res := bm.Copy()
	res.And(other)
	return res
}

func (bm *ShortBitmask) And(other ShortBitmask) {
	for {
		bm.mask &= other.mask

		if other.next == nil {
			break
		}
		if bm.next == nil {
			bm.next = &ShortBitmask{}
		}
		bm = bm.next
		other = *other.next
	}

func (bm *ShortBitmask) XorCopy(other ShortBitmask) ShortBitmask {
	res := bm.Copy()
	res.Xor(other)
	return res
}

func (bm *ShortBitmask) Xor(other ShortBitmask) {
	for {
		bm.mask ^= other.mask

		if other.next == nil {
			break
		}
		if bm.next == nil {
			bm.next = &ShortBitmask{}
		}
		bm = bm.next
		other = *other.next
	}
}

func (bm *ShortBitmask) SubCopy(other ShortBitmask) ShortBitmask {
	res := bm.Copy()
	res.Sub(other)
	return res
}

func (bm *ShortBitmask) Sub(other ShortBitmask) {
	for {
		bm.mask &= ^other.mask

		if other.next == nil {
			break
		}
		if bm.next == nil {
			bm.next = &ShortBitmask{}
		}
		bm = bm.next
		other = *other.next
	}
}

func (bm *ShortBitmask) Shrink() {
	lastNonZero := bm
	for {
		bm = bm.next
		if bm == nil {
			lastNonZero.next = nil
			return
		}
		if bm.mask != 0 {
			lastNonZero = bm
		}
	}
}
