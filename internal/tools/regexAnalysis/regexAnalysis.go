package regexanalysis

import (
	"fmt"
	"math"

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
	evaluate := (func(a *AcceptedLengths, pos uint32, seen []uint32) error)(nil)
	evaluate = func(a *AcceptedLengths, pos uint32, seen []uint32) error {
		for {
			i := p.Inst[pos]
			switch i.Op {
			case syntax.InstRune1, syntax.InstRune, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
				a.MinLength++
				a.MaxLength++
				fallthrough
			case syntax.InstNop, syntax.InstEmptyWidth, syntax.InstCapture:
				pos = i.Out
				continue
			case syntax.InstAlt, syntax.InstAltMatch:
				for _, s := range seen {
					if s == pos {
						a.MinLength = math.MaxUint64
						a.MaxLength = math.MaxUint64
						return nil
					}
				}
				seen = append(seen, pos)
				a2 := *a
				if err := evaluate(&a2, i.Out, seen); err != nil {
					return err
				}
				if err := evaluate(a, i.Arg, seen); err != nil {
					return err
				}
				if a.MinLength > a2.MinLength {
					a.MinLength = a2.MinLength
				}
				if a.MaxLength < a2.MaxLength {
					a.MaxLength = a2.MaxLength
				}
				return nil
			case syntax.InstMatch:
				return nil
			case syntax.InstFail:
				a.MinLength = math.MaxUint64
				a.MaxLength = math.MaxUint64
				return nil
			}
			return fmt.Errorf("unsupported regex op %q", i.String())
		}
	}
	a := AcceptedLengths{}
	return a, evaluate(&a, uint32(p.Start), nil)
}
