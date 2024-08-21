package bitmask

type (
	connectedBitmaskEntry struct {
		min, max uint
	}
	ConnectedBitmask struct {
		entries []connectedBitmaskEntry
	}
)

func MakeConnectedBitmask(min, max uint) ConnectedBitmask {
	return ConnectedBitmask{
		entries: []connectedBitmaskEntry{{
			min: min,
			max: max,
		}},
	}
}
func (bm ConnectedBitmask) IsSet(bit uint) bool {
	for _, e := range bm.entries {
		if bit < e.min {
			break
		}
		if bit <= e.max {
			return true
		}
	}
	return false
}

func (bm ConnectedBitmask) OnesCount() int {
	count := 0
	for _, e := range bm.entries {
		count += int(1 + e.max - e.min)
	}
	return count
}

func (bm ConnectedBitmask) Len() int {
	if len(bm.entries) != 0 {
		return int(bm.entries[len(bm.entries)-1].max + 1)
	}
	return 0
}

func (bm ConnectedBitmask) IsZero() bool {
	return len(bm.entries) == 0
}

func (bm *ConnectedBitmask) Set(bit uint) {
	for i := range bm.entries {
		e := &bm.entries[i]
		if bit < e.min {
			if bit == e.min-1 {
				e.min--
			} else {
				// add an entry before
				bm.entries = append(bm.entries[:i], append([]connectedBitmaskEntry{{
					min: bit,
					max: bit,
				}}, bm.entries[i:]...)...)
			}
			return
		}
		if bit <= e.max {
			// bit is already set
			return
		}
		if bit == e.max+1 {
			e.max++
			// maybe merge the two neighbours
			if i+1 >= len(bm.entries) {
				return
			}
			e2 := &bm.entries[i+1]
			if bit != e2.min-1 {
				return
			}
			e.max = e2.max
			bm.entries = append(bm.entries[:i+1], bm.entries[i+2:]...)
			return
		}
	}
	// append an entry
	bm.entries = append(bm.entries, connectedBitmaskEntry{
		min: bit,
		max: bit,
	})
}

func (bm *ConnectedBitmask) Unset(bit uint) {
	for i := range bm.entries {
		e := &bm.entries[i]
		if bit < e.min {
			return
		}
		if bit == e.min || bit == e.max {
			if e.min == e.max {
				// remove the entry
				bm.entries = append(bm.entries[:i], bm.entries[i+1:]...)
			} else if bit == e.min {
				e.min++
			} else {
				e.max--
			}
			return
		}
		if bit < e.max {
			// split the current entry
			bm.entries = append(bm.entries[:i+1], bm.entries[i:]...)
			e1, e2 := &bm.entries[i], &bm.entries[i+1]
			e1.max = bit - 1
			e2.min = bit + 1
			return
		}
	}
}

func (bm *ConnectedBitmask) Flip(bit uint) {
	//TODO: optimize when actually used
	if bm.IsSet(bit) {
		bm.Unset(bit)
	} else {
		bm.Set(bit)
	}
}

func (bm ConnectedBitmask) Equal(other ConnectedBitmask) bool {
	if len(bm.entries) != len(other.entries) {
		return false
	}
	for i, l := 0, len(bm.entries); i < l; i++ {
		if bm.entries[i] != other.entries[i] {
			return false
		}
	}
	return true
}

func (bm *ConnectedBitmask) Or(other ConnectedBitmask) {
	*bm = bm.OrCopy(other)
}

func (bm ConnectedBitmask) OrCopy(other ConnectedBitmask) ConnectedBitmask {
	new := []connectedBitmaskEntry(nil)
	aIdx, bIdx := 0, 0
	for aIdx < len(bm.entries) && bIdx < len(other.entries) {
		a, b := bm.entries[aIdx], other.entries[bIdx]
		n := connectedBitmaskEntry{}
		if a.max < b.min {
			n = a
			aIdx++
		} else if b.max < a.min {
			n = b
			bIdx++
		} else {
			n = a
			if b.min < n.min {
				n.min = b.min
			}
			if b.max > n.max {
				n.max = b.max
			}
			aIdx++
			bIdx++
		}
		for {
			if aIdx < len(bm.entries) {
				a = bm.entries[aIdx]
				if n.max+1 >= a.min {
					n.max = a.max
					aIdx++
					continue
				}
			}
			if bIdx < len(other.entries) {
				b = other.entries[bIdx]
				if n.max+1 >= b.min {
					n.max = b.max
					bIdx++
					continue
				}
			}
			break
		}
		new = append(new, n)
	}
	new = append(new, bm.entries[aIdx:]...)
	new = append(new, other.entries[bIdx:]...)
	return ConnectedBitmask{new}
}

func (bm *ConnectedBitmask) And(other ConnectedBitmask) {
	*bm = bm.AndCopy(other)
}

func (bm ConnectedBitmask) AndCopy(other ConnectedBitmask) ConnectedBitmask {
	new := []connectedBitmaskEntry(nil)
	for aIdx, bIdx := 0, 0; aIdx < len(bm.entries) && bIdx < len(other.entries); {
		a, b := bm.entries[aIdx], other.entries[bIdx]
		if a.max < b.min {
			aIdx++
			continue
		}
		if b.max < a.min {
			bIdx++
			continue
		}
		n := a
		if n.min < b.min {
			n.min = b.min
		}
		if n.max > b.max {
			n.max = b.max
		}
		new = append(new, n)
		if n.max == a.max {
			aIdx++
		}
		if n.max == b.max {
			bIdx++
		}
	}
	return ConnectedBitmask{new}
}

func (bm *ConnectedBitmask) Xor(other ConnectedBitmask) {
	*bm = bm.XorCopy(other)
}

func (bm ConnectedBitmask) XorCopy(other ConnectedBitmask) ConnectedBitmask {
	new := []connectedBitmaskEntry(nil)
	aIdx, bIdx := 0, 0
	for aIdx < len(bm.entries) && bIdx < len(other.entries) {
		a, b := bm.entries[aIdx], other.entries[bIdx]
		for {
			if a.max < b.min {
				new = append(new, a)
				aIdx++
				break
			}
			if b.max < a.min {
				new = append(new, b)
				bIdx++
				break
			}
			if a.min != b.min {
				n := connectedBitmaskEntry{}
				if a.min < b.min {
					n.min = a.min
					n.max = b.min - 1
				} else {
					n.min = b.min
					n.max = a.min - 1
				}
				new = append(new, n)
			}
			if a.max == b.max {
				aIdx++
				bIdx++
				break
			}
			if b.max < a.max {
				a.min = b.max + 1
				bIdx++
				if bIdx >= len(other.entries) {
					new = append(new, a)
					break
				}
				b = other.entries[bIdx]
			} else {
				b.min = a.max + 1
				aIdx++
				if aIdx >= len(bm.entries) {
					new = append(new, b)
					break
				}
				a = bm.entries[aIdx]
			}
		}
	}
	new = append(new, bm.entries[aIdx:]...)
	new = append(new, other.entries[bIdx:]...)
	return ConnectedBitmask{new}
}

func (bm *ConnectedBitmask) Sub(other ConnectedBitmask) {
	*bm = bm.SubCopy(other)
}

func (bm ConnectedBitmask) SubCopy(other ConnectedBitmask) ConnectedBitmask {
	new := []connectedBitmaskEntry(nil)
outer:
	for aIdx, bIdx := 0, 0; aIdx < len(bm.entries); aIdx++ {
		a := bm.entries[aIdx]
		for bIdx < len(other.entries) {
			b := other.entries[bIdx]
			if b.max < a.min {
				bIdx++
				continue
			}
			if a.max < b.min {
				break
			}
			if a.min < b.min {
				new = append(new, connectedBitmaskEntry{
					min: a.min,
					max: b.min - 1,
				})
			}
			if a.max <= b.max {
				continue outer
			}
			a.min = b.max + 1
			bIdx++
		}
		new = append(new, a)
	}
	return ConnectedBitmask{new}
}

func (bm ConnectedBitmask) Copy() ConnectedBitmask {
	res := ConnectedBitmask{}
	res.entries = make([]connectedBitmaskEntry, len(bm.entries))
	copy(res.entries, bm.entries)
	return res
}

func (bm *ConnectedBitmask) Inject(bit uint, value bool) {
	for i := len(bm.entries) - 1; i >= 0; i-- {
		e := &bm.entries[i]
		if e.max < bit {
			break
		}
		e.max++
		if e.min < bit {
			break
		}
		e.min++
	}
	if value {
		bm.Set(bit)
	} else {
		bm.Unset(bit)
	}
}

func (bm *ConnectedBitmask) Extract(bit uint) {
	for i := len(bm.entries) - 1; i >= 0; i-- {
		e := &bm.entries[i]
		if e.max < bit {
			break
		}
		e.max--
		if e.min < bit {
			break
		}
		if e.min == bit {
			if e.max < e.min {
				// remove the entry, it was {bit, bit} before
				bm.entries = append(bm.entries[:i], bm.entries[i+1:]...)
			}
			break
		}
		e.min--
		if bit == e.min {
			// we might be extracting a 1 bit wide gap between two entries, if so, merge them
			if i == 0 {
				break
			}
			e2 := &bm.entries[i-1]
			if e2.max+1 != e.min {
				break
			}
			e2.max = e.max
			bm.entries = append(bm.entries[:i], bm.entries[i+1:]...)
			break
		}
	}
}
