package bitmask

import (
	"testing"
)

type (
	bitmask interface {
		Set(uint)
		Unset(uint)
		Flip(uint)
		Inject(uint, bool)
		// TODO: Extract(uint) bool
	}
)

func compareMasks(t *testing.T, c ConnectedBitmask, s ShortBitmask, l LongBitmask) {
	for i := uint(0); i < 100; i++ {
		if cv, sv, lv := c.IsSet(i), s.IsSet(i), l.IsSet(i); cv != sv || cv != lv {
			t.Fatalf("IsSet(%d): %v %v %v", i, cv, sv, lv)
		}
	}
	if c1, s1, l1 := c.OnesCount(), s.OnesCount(), l.OnesCount(); c1 != s1 || c1 != l1 {
		t.Errorf("OnesCount(): %d %d %d", c1, s1, l1)
	}
	if cl, sl, ll := c.Len(), s.Len(), l.Len(); cl != sl || cl != ll {
		t.Errorf("Len(): %d %d %d", cl, sl, ll)
	}
	if cz, sz, lz := c.IsZero(), s.IsZero(), l.IsZero(); cz != sz || cz != lz {
		t.Errorf("IsZero(): %v %v %v", cz, sz, lz)
	}
}

func TestBitmasks(t *testing.T) {
	testcasesBase := []struct {
		name string
		f    func(b bitmask)
	}{
		{
			name: "no-op",
			f: func(b bitmask) {
			},
		},
		{
			name: "set 0",
			f: func(b bitmask) {
				b.Set(0)
			},
		},
		{
			name: "set 10-20",
			f: func(b bitmask) {
				for i := uint(10); i <= 20; i++ {
					b.Set(i)
				}
			},
		},
		{
			name: "set 10-20 every 2",
			f: func(b bitmask) {
				for i := uint(10); i <= 20; i += 2 {
					b.Set(i)
				}
			},
		},
		{
			name: "set 10-20 backwards",
			f: func(b bitmask) {
				for i := uint(20); i >= 10; i-- {
					b.Set(i)
				}
			},
		},
		{
			name: "set 10-20 backwards every 2",
			f: func(b bitmask) {
				for i := uint(20); i >= 10; i -= 2 {
					b.Set(i)
				}
			},
		},
		{
			name: "set 99",
			f: func(b bitmask) {
				b.Set(99)
			},
		},
		{
			name: "set 0 and 99",
			f: func(b bitmask) {
				b.Set(0)
				b.Set(99)
			},
		},
		{
			name: "set 0 to 63 unordered",
			f: func(b bitmask) {
				for i := uint(0); i < 64; i++ {
					b.Set(i ^ 18)
				}
			},
		},
	}
	testcasesMod := []struct {
		name string
		f    func(b bitmask)
	}{
		{
			name: "no-op",
			f: func(b bitmask) {
			},
		},
		{
			name: "unset 0",
			f: func(b bitmask) {
				b.Unset(0)
			},
		},
		{
			name: "unset 10-20",
			f: func(b bitmask) {
				for i := uint(10); i <= 20; i++ {
					b.Unset(i)
				}
			},
		},
		{
			name: "unset 10-20 every 2",
			f: func(b bitmask) {
				for i := uint(10); i <= 20; i += 2 {
					b.Unset(i)
				}
			},
		},
		{
			name: "unset 10-20 backwards",
			f: func(b bitmask) {
				for i := uint(20); i >= 10; i-- {
					b.Unset(i)
				}
			},
		},
		{
			name: "unset 10-20 backwards every 2",
			f: func(b bitmask) {
				for i := uint(20); i >= 10; i -= 2 {
					b.Unset(i)
				}
			},
		},
		{
			name: "unset 99",
			f: func(b bitmask) {
				b.Unset(99)
			},
		},
		{
			name: "unset 0 and 99",
			f: func(b bitmask) {
				b.Unset(0)
				b.Unset(99)
			},
		},
		{
			name: "unset 0 to 63 unordered",
			f: func(b bitmask) {
				for i := uint(0); i < 64; i++ {
					b.Unset(i ^ 18)
				}
			},
		},
		{
			name: "flip 0",
			f: func(b bitmask) {
				b.Flip(0)
			},
		},
		{
			name: "flip 10-20",
			f: func(b bitmask) {
				for i := uint(10); i <= 20; i++ {
					b.Flip(i)
				}
			},
		},
		{
			name: "flip 10-20 every 2",
			f: func(b bitmask) {
				for i := uint(10); i <= 20; i += 2 {
					b.Flip(i)
				}
			},
		},
		{
			name: "flip 10-20 backwards",
			f: func(b bitmask) {
				for i := uint(20); i >= 10; i-- {
					b.Flip(i)
				}
			},
		},
		{
			name: "flip 10-20 backwards every 2",
			f: func(b bitmask) {
				for i := uint(20); i >= 10; i -= 2 {
					b.Flip(i)
				}
			},
		},
		{
			name: "flip 99",
			f: func(b bitmask) {
				b.Flip(99)
			},
		},
		{
			name: "flip 0 and 99",
			f: func(b bitmask) {
				b.Flip(0)
				b.Flip(99)
			},
		},
		{
			name: "flip 0 to 63 unordered",
			f: func(b bitmask) {
				for i := uint(0); i < 64; i++ {
					b.Flip(i ^ 18)
				}
			},
		},
		{
			name: "inject false 0",
			f: func(b bitmask) {
				b.Inject(0, false)
			},
		},
		{
			name: "inject false 10-20",
			f: func(b bitmask) {
				for i := uint(10); i <= 20; i++ {
					b.Inject(i, false)
				}
			},
		},
		{
			name: "inject false 10-20 every 2",
			f: func(b bitmask) {
				for i := uint(10); i <= 20; i += 2 {
					b.Inject(i, false)
				}
			},
		},
		{
			name: "inject false 10-20 backwards",
			f: func(b bitmask) {
				for i := uint(20); i >= 10; i-- {
					b.Inject(i, false)
				}
			},
		},
		{
			name: "inject false 10-20 backwards every 2",
			f: func(b bitmask) {
				for i := uint(20); i >= 10; i -= 2 {
					b.Inject(i, false)
				}
			},
		},
		{
			name: "inject false 99",
			f: func(b bitmask) {
				b.Inject(99, false)
			},
		},
		{
			name: "inject false 0 and 99",
			f: func(b bitmask) {
				b.Inject(0, false)
				b.Inject(99, false)
			},
		},
		{
			name: "inject false 0 to 63 unordered",
			f: func(b bitmask) {
				for i := uint(0); i < 64; i++ {
					b.Inject(i^18, false)
				}
			},
		},
		{
			name: "inject true 0",
			f: func(b bitmask) {
				b.Inject(0, true)
			},
		},
		{
			name: "inject true 10-20",
			f: func(b bitmask) {
				for i := uint(10); i <= 20; i++ {
					b.Inject(i, true)
				}
			},
		},
		{
			name: "inject true 10-20 every 2",
			f: func(b bitmask) {
				for i := uint(10); i <= 20; i += 2 {
					b.Inject(i, true)
				}
			},
		},
		{
			name: "inject true 10-20 backwards",
			f: func(b bitmask) {
				for i := uint(20); i >= 10; i-- {
					b.Inject(i, true)
				}
			},
		},
		{
			name: "inject true 10-20 backwards every 2",
			f: func(b bitmask) {
				for i := uint(20); i >= 10; i -= 2 {
					b.Inject(i, true)
				}
			},
		},
		{
			name: "inject true 99",
			f: func(b bitmask) {
				b.Inject(99, true)
			},
		},
		{
			name: "inject true 0 and 99",
			f: func(b bitmask) {
				b.Inject(0, true)
				b.Inject(99, true)
			},
		},
		{
			name: "inject true 0 to 63 unordered",
			f: func(b bitmask) {
				for i := uint(0); i < 64; i++ {
					b.Inject(i^18, true)
				}
			},
		},
	}
	testcasesCombine := []struct {
		name string
		f    func(c *ConnectedBitmask, s *ShortBitmask, l *LongBitmask, c2 *ConnectedBitmask, s2 *ShortBitmask, l2 *LongBitmask) (*ConnectedBitmask, *ShortBitmask, *LongBitmask)
	}{
		{
			name: "And",
			f: func(c *ConnectedBitmask, s *ShortBitmask, l *LongBitmask, c2 *ConnectedBitmask, s2 *ShortBitmask, l2 *LongBitmask) (*ConnectedBitmask, *ShortBitmask, *LongBitmask) {
				c.And(*c2)
				s.And(*s2)
				l.And(*l2)
				return c, s, l
			},
		},
		{
			name: "Or",
			f: func(c *ConnectedBitmask, s *ShortBitmask, l *LongBitmask, c2 *ConnectedBitmask, s2 *ShortBitmask, l2 *LongBitmask) (*ConnectedBitmask, *ShortBitmask, *LongBitmask) {
				c.Or(*c2)
				s.Or(*s2)
				l.Or(*l2)
				return c, s, l
			},
		},
		{
			name: "Xor",
			f: func(c *ConnectedBitmask, s *ShortBitmask, l *LongBitmask, c2 *ConnectedBitmask, s2 *ShortBitmask, l2 *LongBitmask) (*ConnectedBitmask, *ShortBitmask, *LongBitmask) {
				c.Xor(*c2)
				s.Xor(*s2)
				l.Xor(*l2)
				return c, s, l
			},
		},
		{
			name: "Sub",
			f: func(c *ConnectedBitmask, s *ShortBitmask, l *LongBitmask, c2 *ConnectedBitmask, s2 *ShortBitmask, l2 *LongBitmask) (*ConnectedBitmask, *ShortBitmask, *LongBitmask) {
				c.Sub(*c2)
				s.Sub(*s2)
				l.Sub(*l2)
				return c, s, l
			},
		},
		{
			name: "AndCopy",
			f: func(c *ConnectedBitmask, s *ShortBitmask, l *LongBitmask, c2 *ConnectedBitmask, s2 *ShortBitmask, l2 *LongBitmask) (*ConnectedBitmask, *ShortBitmask, *LongBitmask) {
				c.AndCopy(*c2)
				s.AndCopy(*s2)
				l.AndCopy(*l2)
				return c, s, l
			},
		},
		{
			name: "OrCopy",
			f: func(c *ConnectedBitmask, s *ShortBitmask, l *LongBitmask, c2 *ConnectedBitmask, s2 *ShortBitmask, l2 *LongBitmask) (*ConnectedBitmask, *ShortBitmask, *LongBitmask) {
				c.OrCopy(*c2)
				s.OrCopy(*s2)
				l.OrCopy(*l2)
				return c, s, l
			},
		},
		{
			name: "XorCopy",
			f: func(c *ConnectedBitmask, s *ShortBitmask, l *LongBitmask, c2 *ConnectedBitmask, s2 *ShortBitmask, l2 *LongBitmask) (*ConnectedBitmask, *ShortBitmask, *LongBitmask) {
				c.XorCopy(*c2)
				s.XorCopy(*s2)
				l.XorCopy(*l2)
				return c, s, l
			},
		},
		{
			name: "SubCopy",
			f: func(c *ConnectedBitmask, s *ShortBitmask, l *LongBitmask, c2 *ConnectedBitmask, s2 *ShortBitmask, l2 *LongBitmask) (*ConnectedBitmask, *ShortBitmask, *LongBitmask) {
				c.SubCopy(*c2)
				s.SubCopy(*s2)
				l.SubCopy(*l2)
				return c, s, l
			},
		},
	}
	cList := []*ConnectedBitmask{}
	sList := []*ShortBitmask{}
	lList := []*LongBitmask{}
	for _, tcBase := range testcasesBase {
		for _, tcMod := range testcasesMod {
			c := &ConnectedBitmask{}
			s := &ShortBitmask{}
			l := &LongBitmask{}
			tcBase.f(c)
			tcMod.f(c)
			tcBase.f(s)
			tcMod.f(s)
			tcBase.f(l)
			tcMod.f(l)
			isNew := true
			for i := range cList {
				c2 := *cList[i]
				s2 := *sList[i]
				l2 := *lList[i]
				ce, se, le := c.Equal(c2), s.Equal(s2), l.Equal(l2)
				if ce != se || ce != le {
					t.Fatalf("Equal(): %v %v %v", ce, se, le)
				}
				if ce {
					isNew = false
				}
			}
			if isNew {
				cList = append(cList, c)
				sList = append(sList, s)
				lList = append(lList, l)
			}
			t.Run(tcBase.name, func(t *testing.T) {
				t.Run(tcMod.name, func(t *testing.T) {
					compareMasks(t, *c, *s, *l)
					cc := c.Copy()
					sc := s.Copy()
					lc := l.Copy()
					compareMasks(t, cc, sc, lc)
					sc.Shrink()
					lc.Shrink()
					compareMasks(t, cc, sc, lc)
				})
			})
		}
	}
	for _, tcCombine := range testcasesCombine {
		t.Run(tcCombine.name, func(t *testing.T) {
			for i := range cList {
				for j := range cList {
					c, c2 := cList[i].Copy(), cList[j].Copy()
					s, s2 := sList[i].Copy(), sList[j].Copy()
					l, l2 := lList[i].Copy(), lList[j].Copy()
					cc, sc, lc := tcCombine.f(&c2, &s2, &l2, &c, &s, &l)
					compareMasks(t, c2, s2, l2)
					compareMasks(t, c, s, l)
					compareMasks(t, *cc, *sc, *lc)
				}
			}
		})
	}
}
