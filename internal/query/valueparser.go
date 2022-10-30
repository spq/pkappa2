package query

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer/stateful"
)

type (
	variableParser struct {
		Name, Sub string
	}
	timeParser struct {
		Time    time.Time
		HasDate bool
	}
	durationParser struct {
		Duration time.Duration
	}
	hostParser struct {
		Host net.IP
	}
	maskParser struct {
		V4Mask, V6Mask []byte
	}
	stringParser struct {
		Elements []struct {
			Content  string          `parser:"( @Characters"`
			Variable *variableParser `parser:"| @Variable )"`
		} `parser:"@@*"`
	}
	tokenListParser struct {
		List []struct {
			Token    string          `parser:"( @Token"`
			Variable *variableParser `parser:"| @Variable )"`
		} `parser:"@@ (GroupSeparator @@)*"`
	}
	numberRangeListParser struct {
		List []struct {
			Range []struct {
				Parts []struct {
					Operators string          `parser:"@Operator*"`
					Number    int             `parser:"( @Number"`
					Variable  *variableParser `parser:"| @Variable )"`
				} `parser:"@@*"`
			} `parser:"@@ (RangeSeparator @@)?"`
		} `parser:"@@ (GroupSeparator @@)*"`
	}
	timeRangeListParser struct {
		List []struct {
			Range []struct {
				Parts []struct {
					Operators string          `parser:"@Operator*"`
					Time      *timeParser     `parser:"( @Time"`
					Duration  *durationParser `parser:"| @Duration"`
					Variable  *variableParser `parser:"| @Variable )"`
				} `parser:"@@*"`
			} `parser:"@@ (RangeSeparator @@)?"`
		} `parser:"@@ (GroupSeparator @@)*"`
	}
	hostListParser struct {
		List []struct {
			Variable *variableParser `parser:"( @Variable"`
			Host     *hostParser     `parser:"| @( IP4 | IP6 ) )"`
			Masks    *maskParser     `parser:"@(Mask+)?"`
		} `parser:"@@ (GroupSeparator @@)*"`
	}
)

var (
	valueParserLexerRules = stateful.Rules{
		"Global": []stateful.Rule{
			stateful.Include("Variable"),
			{
				Name:    "whitespace",
				Pattern: `[ \t\n\r]+`,
			},
		},
		"Variable": {
			{
				Name:    "Variable",
				Pattern: `(?i)@(?:[a-z0-9]+:)?[a-z0-9]+@`,
			},
		},
		"String": {
			stateful.Include("Variable"),
			{
				Name:    "Characters",
				Pattern: `(?:[^@]|@@)+`,
			},
		},
		"TokenList": {
			stateful.Include("List"),
			{
				Name:    "Token",
				Pattern: `[a-z]+`,
			},
		},
		"List": []stateful.Rule{
			stateful.Include("Global"),
			{
				Name:    "GroupSeparator",
				Pattern: `,`,
			},
		},
		"RangeList": []stateful.Rule{
			stateful.Include("List"),
			{
				Name:    "RangeSeparator",
				Pattern: `:`,
			},
		},
		"Operator": {
			{
				Name:    "Operator",
				Pattern: `[+-]`,
			},
		},
		"NumberRangeList": []stateful.Rule{
			stateful.Include("RangeList"),
			stateful.Include("Operator"),
			{
				Name:    "Number",
				Pattern: `\d+`,
			},
		},
		"TimeRangeList": []stateful.Rule{
			stateful.Include("RangeList"),
			stateful.Include("Operator"),
			{
				Name:    "Duration",
				Pattern: `(?i)((?:\d+[.]\d+|[.]?\d+)(?:[muµn]s|[hms]))+`,
			}, {
				Name:    "Time",
				Pattern: `(?:\d{4}-\d\d-\d\d +)\d{4}(?:\d\d)?`,
			},
		},
		"HostList": []stateful.Rule{
			stateful.Include("List"),
			{
				Name:    "IP4",
				Pattern: `\d+[.]\d+[.]\d+[.]\d+`,
			}, {
				Name:    "IP6",
				Pattern: `(?i)[0-9a-f:]*:[0-9a-f:]+`,
			}, {
				Name:    "Mask",
				Pattern: `(?:/-?\d+)`,
			},
		},
	}
	valueStringParser = participle.MustBuild(
		&stringParser{},
		participle.Lexer(stateful.Must(valueParserLexerRules, stateful.InitialState("String"))),
	)
	valueTokenListParser = participle.MustBuild(
		&tokenListParser{},
		participle.Lexer(stateful.Must(valueParserLexerRules, stateful.InitialState("TokenList"))),
	)
	valueNumberRangeListParser = participle.MustBuild(
		&numberRangeListParser{},
		participle.Lexer(stateful.Must(valueParserLexerRules, stateful.InitialState("NumberRangeList"))),
	)
	valueTimeRangeListParser = participle.MustBuild(
		&timeRangeListParser{},
		participle.Lexer(stateful.Must(valueParserLexerRules, stateful.InitialState("TimeRangeList"))),
	)
	valueHostListParser = participle.MustBuild(
		&hostListParser{},
		participle.Lexer(stateful.Must(valueParserLexerRules, stateful.InitialState("HostList"))),
	)
)

//func (p *stringRoot) Parseable(lex *lexer.PeekingLexer) error {

func (p *variableParser) Capture(s []string) error {
	v := strings.Split(s[0][1:len(s[0])-1], ":")
	p.Name = v[len(v)-1]
	if len(v) == 2 {
		p.Sub = v[0]
	}
	return nil
}

func (p *timeParser) Capture(s []string) error {
	// format: [YYYY-MM-DD ]HHMM[SS]
	formats := []string{
		"1504",
		"150405",
		"2006-01-02 1504",
		"2006-01-02 150405",
	}
	var lastErr error
	for _, format := range formats {
		t, err := time.ParseInLocation(format, s[0], time.UTC)
		lastErr = err
		if err != nil {
			continue
		}
		p.HasDate = strings.ContainsRune(format, '-')
		p.Time = t
		return nil
	}
	return lastErr
}

func (p *durationParser) Capture(s []string) error {
	duration, err := time.ParseDuration(s[0])
	if err != nil {
		return err
	}
	p.Duration = duration
	return nil
}

func (p *hostParser) Capture(s []string) error {
	p.Host = net.ParseIP(s[0])
	if p.Host == nil {
		return fmt.Errorf("bad ip address %s", s[0])
	}
	if p.Host.To4() != nil {
		p.Host = p.Host.To4()
	}
	return nil
}

func (p *maskParser) Capture(s []string) error {
	p.V4Mask = make([]byte, 4)
	p.V6Mask = make([]byte, 16)
	for _, m := range s {
		n, err := strconv.ParseInt(m[1:], 10, 16)
		if err != nil {
			return err
		}
		switch {
		case n > 0:
			for i := int64(0); i < n; i++ {
				if i < 32 {
					p.V4Mask[i/8] ^= 1 << (7 - (i % 8))
					p.V6Mask[i/8] ^= 1 << (7 - (i % 8))
				} else if i < 128 {
					p.V6Mask[i/8] ^= 1 << (7 - (i % 8))
				} else {
					return fmt.Errorf("bad host mask: %q", m)
				}
			}
		case n < 0:
			if n < -128 {
				return fmt.Errorf("bad host mask: %q", m)
			}
			for i := n + 128; i < 128; i++ {
				p.V6Mask[i/8] ^= 1 << (7 - (i % 8))
			}
			if n >= -32 {
				for i := n + 32; i < 32; i++ {
					p.V4Mask[i/8] ^= 1 << (7 - (i % 8))
				}
			}
		}
	}
	return nil
}

func (p *variableParser) String() string {
	if p.Sub != "" {
		return fmt.Sprintf("@%s:%s@", p.Sub, p.Name)
	}
	return fmt.Sprintf("@%s@", p.Name)
}

func (p *numberRangeListParser) String() string {
	res := []string(nil)
	for _, l := range p.List {
		cur := []string(nil)
		for _, r := range l.Range {
			tmp := ""
			for _, p := range r.Parts {
				negative := strings.Count(p.Operators, "-")%2 == 1
				tmp += map[bool]string{true: "-", false: "+"}[negative]
				if p.Variable != nil {
					tmp += p.Variable.String()
				} else {
					tmp += fmt.Sprintf("%d", p.Number)
				}
			}
			cur = append(cur, tmp)
		}
		res = append(res, strings.Join(cur, ":"))
	}
	return strings.Join(res, ",")
}

func (p *timeRangeListParser) String() string {
	res := []string(nil)
	for _, l := range p.List {
		cur := []string(nil)
		for _, r := range l.Range {
			tmp := ""
			for _, p := range r.Parts {
				negative := strings.Count(p.Operators, "-")%2 == 1
				tmp += map[bool]string{true: "-", false: "+"}[negative]
				if p.Variable != nil {
					tmp += p.Variable.String()
				} else if p.Duration != nil {
					tmp += p.Duration.Duration.String()
				} else if p.Time != nil {
					tmp += p.Time.Time.String()
				}
			}
			cur = append(cur, tmp)
		}
		res = append(res, strings.Join(cur, ":"))
	}
	return strings.Join(res, ",")
}

func (p *hostListParser) String() string {
	res := []string(nil)
	for _, l := range p.List {
		tmp := ""
		if l.Variable != nil {
			tmp = l.Variable.String()
		} else {
			tmp = l.Host.Host.String()
		}
		if l.Masks != nil {
			tmp += fmt.Sprintf("/%s or %s", net.IP(l.Masks.V4Mask).String(), net.IP(l.Masks.V6Mask).String())
		}
		res = append(res, tmp)
	}
	return strings.Join(res, ",")
}
