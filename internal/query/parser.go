package query

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer/stateful"
)

const (
	flagsStreamProtocol      = 0b011
	flagsStreamProtocolOther = 0b000
	flagsStreamProtocolTCP   = 0b001
	flagsStreamProtocolUDP   = 0b010
	flagsStreamProtocolSCTP  = 0b011
)

type (
	parserContext struct {
		referenceTime time.Time
		timezone      *time.Location
		sortTerm      *sortTerm
		limitTerm     *limitTerm
		groupTerm     *groupTerm
	}
	queryRoot struct {
		Term *queryOrCondition `parser:"@@?"`
	}
	queryOrCondition struct {
		Or []*queryAndCondition `parser:"@@ ( OperatorOr @@ )*"`
	}
	queryAndCondition struct {
		And []*queryThenCondition `parser:"@@ ( OperatorAnd? @@ )*"`
	}
	queryThenCondition struct {
		Then []*queryCondition `parser:"@@ ( OperatorThen @@ )*"`
	}
	queryCondition struct {
		Negated   *queryCondition   `parser:"  Negation @@"`
		Grouped   *queryOrCondition `parser:"| '(' @@ ')'"`
		Term      *queryTerm        `parser:"| @( SubQuery? Key ConverterName? ( UnquotedValue | QuotedValue ) )"`
		SortTerm  *sortTerm         `parser:"| ( SortKey @( UnquotedValue | QuotedValue ) )"`
		LimitTerm *limitTerm        `parser:"| ( LimitKey @( UnquotedValue | QuotedValue ) )"`
		GroupTerm *groupTerm        `parser:"| ( GroupKey @( UnquotedValue | QuotedValue ) )"`
	}
	queryTerm struct {
		SubQuery      string
		Key           string
		ConverterName string
		Value         string
	}
	sortTerm  []Sorting
	limitTerm uint
	groupTerm string

	Grouping struct {
		Constant  string
		Variables []DataConditionElementVariable
	}

	Query struct {
		Debug         []string
		Conditions    ConditionsSet
		Sorting       []Sorting
		Limit         *uint
		Grouping      *Grouping
		ReferenceTime time.Time
	}
)

var (
	parser = participle.MustBuild(
		&queryRoot{},
		participle.Lexer(stateful.MustSimple([]stateful.Rule{
			{
				Name:    "whitespace",
				Pattern: `[ \t\n\r]+`,
			}, {
				Name:    "Negation",
				Pattern: `[!-]`,
			}, {
				Name:    "SubQuery",
				Pattern: `(?i)@([a-z0-9]+):`,
			}, {
				Name:    "Key",
				Pattern: `(?i)(id|tag|service|mark|protocol|generated|[fl]?time|[cs]?(data|port|host|bytes))`,
			}, {
				Name:    "ConverterName",
				Pattern: `\.([^:]+)`,
			}, {
				Name:    "SortKey",
				Pattern: `(?i)sort`,
			}, {
				Name:    "LimitKey",
				Pattern: `(?i)limit`,
			}, {
				Name:    "GroupKey",
				Pattern: `(?i)group`,
			}, {
				Name:    "OperatorOr",
				Pattern: `(?i)or`,
			}, {
				Name:    "OperatorAnd",
				Pattern: `(?i)and`,
			}, {
				Name:    "OperatorThen",
				Pattern: `(?i)then`,
			}, {
				Name:    "BracketOpen",
				Pattern: `[(]`,
			}, {
				Name:    "BracketClose",
				Pattern: `[)]`,
			}, {
				Name:    "QuotedValue",
				Pattern: `[:=]"(?:[^"]*|"")*"`,
			}, {
				Name:    "UnquotedValue",
				Pattern: `[:=](?:(?:[^"\\ \t\n\r]|\\.)(?:[^\\ \t\n\r]|\\.)*)?(?:[^)\\ \t\n\r]|\\.)`,
			},
		})),
		participle.CaseInsensitive("Key"),
		participle.CaseInsensitive("SortKey"),
		participle.CaseInsensitive("OperatorOr"),
		participle.CaseInsensitive("OperatorAnd"),
		participle.CaseInsensitive("OperatorThen"),
	)
)

func parseValue(s string) string {
	s = s[1:]
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		s = s[1 : len(s)-1]
		s = strings.ReplaceAll(s, `""`, `"`)
	}
	return s
}

func (t *queryTerm) Capture(s []string) error {
	if len(s) >= 3 && strings.HasPrefix(s[0], "@") && strings.HasSuffix(s[0], ":") {
		t.SubQuery = s[0][1 : len(s[0])-1]
		s = s[1:]
	}
	t.Key = strings.ToLower(s[0])
	if len(s) >= 3 && strings.HasPrefix(s[1], ".") {
		t.ConverterName = s[1][1:]
		s = s[1:]
	}
	t.Value = parseValue(s[1])
	return nil
}

func (t *sortTerm) Capture(s []string) error {
	v := parseValue(s[0])
	for _, v := range strings.Split(v, ",") {
		v = strings.TrimSpace(v)
		dir := SortingDirAscending
		if strings.HasPrefix(v, "-") {
			dir = SortingDirDescending
			v = strings.TrimSpace(strings.TrimPrefix(v, "-"))
		}
		key, ok := map[string]SortingKey{
			"id":     SortingKeyID,
			"ftime":  SortingKeyFirstPacketTime,
			"ltime":  SortingKeyLastPacketTime,
			"cbytes": SortingKeyClientBytes,
			"sbytes": SortingKeyServerBytes,
			"chost":  SortingKeyClientHost,
			"shost":  SortingKeyServerHost,
			"cport":  SortingKeyClientPort,
			"sport":  SortingKeyServerPort,
		}[v]
		if !ok {
			return fmt.Errorf("invalid sort key %q", v)
		}
		*t = append(*t, Sorting{
			Dir: dir,
			Key: key,
		})
	}
	return nil
}

func (t *limitTerm) Capture(s []string) error {
	v := strings.TrimSpace(parseValue(s[0]))
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return err
	}
	*t = limitTerm(n)
	return nil
}

func (t *queryTerm) String() string {
	if t.ConverterName != "" {
		return fmt.Sprintf("%s.%s:%q", t.Key, t.ConverterName, t.Value)
	}
	return fmt.Sprintf("%s:%q", t.Key, t.Value)
}

func (c *queryCondition) String() string {
	switch {
	case c.Negated != nil:
		return fmt.Sprintf("-%s", c.Negated.String())
	case c.Grouped != nil:
		return fmt.Sprintf("(%s)", c.Grouped.String())
	case c.Term != nil:
		return c.Term.String()
	default:
		return "?"
	}
}

func (c *queryThenCondition) String() string {
	if len(c.Then) == 1 {
		return c.Then[0].String()
	}
	a := []string{}
	for _, i := range c.Then {
		a = append(a, i.String())
	}
	return fmt.Sprintf("sequence(%s)", strings.Join(a, ","))
}

func (c *queryAndCondition) String() string {
	if len(c.And) == 1 {
		return c.And[0].String()
	}
	a := []string{}
	for _, i := range c.And {
		a = append(a, i.String())
	}
	return fmt.Sprintf("and(%s)", strings.Join(a, ","))
}

func (c *queryOrCondition) String() string {
	if len(c.Or) == 1 {
		return c.Or[0].String()
	}
	a := []string{}
	for _, i := range c.Or {
		a = append(a, i.String())
	}
	return fmt.Sprintf("or(%s)", strings.Join(a, ","))
}

func (r *queryRoot) String() string {
	if r.Term == nil {
		return ""
	}
	return r.Term.String()
}

func Parse(q string) (*Query, error) {
	root := &queryRoot{}
	if err := parser.ParseString("", q, root); err != nil {
		return nil, err
	}
	pc := parserContext{
		referenceTime: time.Now(),
		timezone:      time.Local,
	}
	cond, err := root.QueryConditions(&pc)
	if err != nil {
		return nil, err
	}
	if cond != nil {
		cond = cond.clean()
		if cond.impossible() {
			cond = nil
		} else if len(cond) == 0 {
			cond = ConditionsSet{
				Conditions{},
			}
		}
	} else {
		cond = ConditionsSet{
			Conditions{},
		}
	}
	sorting := []Sorting(nil)
	if pc.sortTerm != nil {
		sorting = *pc.sortTerm
	}
	limit := (*uint)(pc.limitTerm)
	grouping := (*Grouping)(nil)
	if pc.groupTerm != nil {
		val := stringParser{}
		if err := valueStringParser.ParseString("", string(*pc.groupTerm), &val); err != nil {
			return nil, err
		}
		grouping = &Grouping{}
		for _, e := range val.Elements {
			if e.Variable == nil {
				grouping.Constant += e.Content
				continue
			}
			grouping.Variables = append(grouping.Variables, DataConditionElementVariable{
				Position: uint(len(grouping.Constant)),
				SubQuery: e.Variable.Sub,
				Name:     e.Variable.Name,
			})
		}
	}
	return &Query{
		Debug:         []string{root.String(), cond.String()},
		Conditions:    cond,
		Sorting:       sorting,
		Limit:         limit,
		ReferenceTime: pc.referenceTime,
		Grouping:      grouping,
	}, nil
}
