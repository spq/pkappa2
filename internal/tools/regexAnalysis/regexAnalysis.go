package regexanalysis

import (
	"fmt"
	"math"
	"math/bits"

	"rsc.io/binaryregexp/syntax"
)

type (
	AcceptedLengths struct {
		MinLength uint
		MaxLength uint
	}
)

func NamedCaptures(regexString string) (map[string][]string, error) {
	r, err := syntax.Parse(regexString, syntax.Perl)
	if err != nil {
		return nil, err
	}
	extracts := map[string][]string{}
	stack := []*syntax.Regexp{r}
	for len(stack) != 0 {
		cur := stack[len(stack)-1]
		stack = append(stack[:len(stack)-1], cur.Sub...)
		if cur.Op != syntax.OpCapture || cur.Name == "" {
			continue
		}
		extracts[cur.Name] = append(extracts[cur.Name], cur.Sub[0].String())
	}
	return extracts, nil
}

func ConstantSuffix(regexString string) ([]byte, error) {
	r, err := syntax.Parse(regexString, syntax.Perl)
	if err != nil {
		return nil, err
	}
	p, err := syntax.Compile(r.Simplify())
	if err != nil {
		return nil, err
	}
	evaluate := (func(s *[]byte, pos uint32, seen []uint32) error)(nil)
	evaluate = func(s *[]byte, pos uint32, seen []uint32) error {
		for {
			i := p.Inst[pos]
			switch i.Op {
			case syntax.InstRune1, syntax.InstRune, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
				if len(i.Rune) == 1 && i.Rune[0] <= 0xFF && syntax.Flags(i.Arg)&syntax.FoldCase == 0 {
					*s = append(*s, byte(i.Rune[0]))
				} else {
					*s = nil
				}
				fallthrough
			case syntax.InstNop, syntax.InstEmptyWidth, syntax.InstCapture:
				pos = i.Out
				continue
			case syntax.InstAlt, syntax.InstAltMatch:
				for _, p := range seen {
					if p == pos {
						*s = nil
						return nil
					}
				}
				seen = append(seen, pos)
				s2 := append(make([]byte, 0, len(*s)), *s...)
				if err := evaluate(&s2, i.Out, seen); err != nil {
					return err
				}
				if err := evaluate(s, i.Arg, seen); err != nil {
					return err
				}
				for i := 0; ; i++ {
					if i < len(*s) && i < len(s2) {
						b, b2 := (*s)[len(*s)-i-1], s2[len(s2)-i-1]
						if b == b2 {
							continue
						}
					}
					*s = (*s)[len(*s)-i:]
					break
				}
				return nil
			case syntax.InstMatch:
				return nil
			case syntax.InstFail:
				*s = nil
				return nil
			}
			return fmt.Errorf("unsupported regex op %q", i.String())
		}
	}
	s := []byte(nil)
	return s, evaluate(&s, uint32(p.Start), nil)
}

func AcceptedLength(regexString string) (AcceptedLengths, error) {
	r, err := syntax.Parse(regexString, syntax.Perl)
	if err != nil {
		return AcceptedLengths{}, err
	}
	p, err := syntax.Compile(r.Simplify())
	if err != nil {
		return AcceptedLengths{}, err
	}
	cache := map[uint32]AcceptedLengths{}
	evaluate := (func(entry uint32, seen []uint32) (AcceptedLengths, error))(nil)
	evaluate = func(entry uint32, seen []uint32) (AcceptedLengths, error) {
		if r, ok := cache[entry]; ok {
			return r, nil
		}
		r := AcceptedLengths{}
		pos := entry
		for {
			i := p.Inst[pos]
			switch i.Op {
			case syntax.InstRune1, syntax.InstRune, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
				inc := func(v *uint) {
					if *v != math.MaxUint {
						(*v)++
					}
				}
				inc(&r.MinLength)
				inc(&r.MaxLength)
				fallthrough
			case syntax.InstNop, syntax.InstEmptyWidth, syntax.InstCapture:
				pos = i.Out
				continue
			case syntax.InstAlt, syntax.InstAltMatch:
				for _, s := range seen {
					if s == pos {
						cache[entry] = AcceptedLengths{math.MaxUint64, math.MaxUint64}
						return AcceptedLengths{math.MaxUint64, math.MaxUint64}, nil
					}
				}
				seen = append(seen, pos)
				r1, err := evaluate(i.Out, seen)
				if err != nil {
					return AcceptedLengths{}, err
				}
				r2, err := evaluate(i.Arg, seen)
				if err != nil {
					return AcceptedLengths{}, err
				}
				if r1.MinLength > r2.MinLength {
					r1.MinLength, r2.MinLength = r2.MinLength, r1.MinLength
				}
				if r1.MaxLength < r2.MaxLength {
					r1.MaxLength, r2.MaxLength = r2.MaxLength, r1.MaxLength
				}
				add := func(a, b uint) uint {
					c := ((a >> 1) + (b >> 1) + (a & b & 1)) >> (bits.UintSize - 1)
					if c != 0 {
						return math.MaxUint
					}
					return a + b
				}
				r.MinLength = add(r.MinLength, r1.MinLength)
				r.MaxLength = add(r.MaxLength, r1.MaxLength)
				fallthrough
			case syntax.InstMatch:
				cache[entry] = r
				return r, nil
			case syntax.InstFail:
				cache[entry] = AcceptedLengths{math.MaxUint64, math.MaxUint64}
				return AcceptedLengths{math.MaxUint64, math.MaxUint64}, nil
			}
			return AcceptedLengths{}, fmt.Errorf("unsupported regex op %q", i.String())
		}
	}
	return evaluate(uint32(p.Start), nil)
}
