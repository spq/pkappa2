package query

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net"
	"sort"
	"strings"
	"time"

	"rsc.io/binaryregexp"
)

type (
	NumberConditionSummandType uint8
	HostConditionSourceType    bool
)

const (
	NumberConditionSummandTypeID          NumberConditionSummandType = iota
	NumberConditionSummandTypeClientBytes NumberConditionSummandType = iota
	NumberConditionSummandTypeServerBytes NumberConditionSummandType = iota
	NumberConditionSummandTypeClientPort  NumberConditionSummandType = iota
	NumberConditionSummandTypeServerPort  NumberConditionSummandType = iota
)
const (
	HostConditionSourceTypeClient HostConditionSourceType = false
	HostConditionSourceTypeServer HostConditionSourceType = true
)
const (
	DataRequirementSequenceFlagsDirection               = 0b1
	DataRequirementSequenceFlagsDirectionClientToServer = 0b0
	DataRequirementSequenceFlagsDirectionServerToClient = 0b1

	FlagsHostConditionInverted     = 0b01
	FlagsHostConditionSource       = 0b10
	FlagsHostConditionSourceClient = 0b00
	FlagsHostConditionSourceServer = 0b10
)

type (
	HostConditionSource struct {
		SubQuery string
		Type     HostConditionSourceType
	}
	HostCondition struct {
		HostConditionSources []HostConditionSource
		Host                 net.IP
		Mask4                net.IP
		Mask6                net.IP
		Invert               bool
	}
	TagCondition struct {
		// this is fulfilled, when
		SubQuery string
		TagName  string
		Invert   bool
	}
	FlagCondition struct {
		// this is fulfilled, when (xored(i.Flags for i in SubQueries) ^ Value) & Mask != 0
		SubQueries []string
		Value      uint16
		Mask       uint16
	}
	TimeConditionSummand struct {
		SubQuery    string
		LTimeFactor int
		FTimeFactor int
	}
	NumberConditionSummand struct {
		SubQuery string
		Factor   int
		Type     NumberConditionSummandType
	}
	TimeCondition struct {
		// this is fulfilled, when Duration+sum(ftime*Summands.FTimeFactor)+sum(ltime*Summands.LTimeFactor) >= 0
		Summands            []TimeConditionSummand
		Duration            time.Duration
		ReferenceTimeFactor int
	}
	NumberCondition struct {
		// this is fulfilled, when Number+X >= 0
		Summands []NumberConditionSummand
		Number   int
	}
	DataConditionElementVariable struct {
		Position uint
		SubQuery string
		Name     string
	}
	DataConditionElement struct {
		SubQuery  string
		Regex     string
		Variables []DataConditionElementVariable
		Flags     uint8
	}
	DataCondition struct {
		Elements []DataConditionElement
		Inverted bool
	}
	ImpossibleCondition struct{}
	Condition           interface {
		fmt.Stringer
		impossible() bool
		equal(Condition) bool
		invert() ConditionsSet
	}
	Conditions    []Condition
	ConditionsSet []Conditions
)

var (
	impossibleCondition = ImpossibleCondition{}
)

func (qcs ConditionsSet) String() string {
	res := []string{}
	for _, c := range qcs {
		res = append(res, c.String())
	}
	return fmt.Sprintf("(%s)", strings.Join(res, ") | ("))
}

func (c *TagCondition) String() string {
	return fmt.Sprintf("%s%s%stag:%s", map[bool]string{false: "", true: "-"}[c.Invert], c.SubQuery, map[bool]string{false: ":", true: ""}[c.SubQuery == ""], c.TagName)
}

func (c *FlagCondition) String() string {
	type maskInfo struct {
		name       string
		valueNames map[uint16]string
	}
	info, ok := map[uint16]maskInfo{
		flagsStreamProtocol: {
			name: "protocol",
			valueNames: map[uint16]string{
				flagsStreamProtocolOther: "0(other)",
				flagsStreamProtocolTCP:   "1(tcp)",
				flagsStreamProtocolUDP:   "2(udp)",
				flagsStreamProtocolSCTP:  "3(sctp)",
			},
		},
	}[c.Mask]
	if !ok {
		info = maskInfo{
			name: fmt.Sprintf("flags&0x%x", c.Mask),
			valueNames: map[uint16]string{
				c.Value: fmt.Sprintf("0x%x", c.Value),
			},
		}
	}
	res := []string(nil)
	for _, sq := range c.SubQueries {
		colon := map[bool]string{false: ":", true: ""}[sq == ""]
		res = append(res, fmt.Sprintf("%s%s%s", sq, colon, info.name))
	}
	return fmt.Sprintf("%s != %s", strings.Join(res, " ^ "), info.valueNames[c.Value])
}

func (c *HostCondition) String() string {
	res := []string(nil)
	for _, hcs := range c.HostConditionSources {
		colon := map[bool]string{false: ":", true: ""}[hcs.SubQuery == ""]
		t := map[HostConditionSourceType]string{
			HostConditionSourceTypeClient: "chost",
			HostConditionSourceTypeServer: "shost",
		}[hcs.Type]
		res = append(res, fmt.Sprintf("%s%s%s", hcs.SubQuery, colon, t))
	}
	equals := map[bool]string{false: "==", true: "!="}[c.Invert]
	return fmt.Sprintf("%s %s %s/%s or %s", strings.Join(res, " ^ "), equals, c.Host.String(), c.Mask4.String(), c.Mask6.String())
}

func (c *TimeCondition) String() string {
	res := []string(nil)
	for _, s := range c.Summands {
		sq := s.SubQuery
		if sq != "" {
			sq += ":"
		}
		for i, f := range [2]int{s.FTimeFactor, s.LTimeFactor} {
			if f == 0 {
				continue
			}
			suffix := ""
			if f != 1 && f != -1 {
				n := f
				if n < 0 {
					n = -n
				}
				suffix = fmt.Sprintf("*%d", n)
			}
			prefix := ""
			if f < 0 {
				prefix = "-"
			} else if len(res) != 0 {
				prefix = "+"
			}
			res = append(res, fmt.Sprintf("%s%s%ctime%s", prefix, sq, "fl"[i], suffix))
		}
	}
	if c.ReferenceTimeFactor != 0 {
		tmp := c.ReferenceTimeFactor
		sign := "+"
		if tmp < 0 {
			tmp = -tmp
			sign = "-"
		}
		suffix := ""
		if tmp != 1 {
			suffix = fmt.Sprintf("*%d", tmp)
		}
		res = append(res, fmt.Sprintf("%snow%s", sign, suffix))
	}
	if c.Duration != 0 {
		tmp := c.Duration
		prefix := "+"
		if tmp < 0 {
			prefix = "-"
			tmp = -tmp
		}
		res = append(res, fmt.Sprintf("%s%s", prefix, tmp.String()))
	}
	return fmt.Sprintf("%s >= 0", strings.Join(res, ""))
}

func (c *NumberCondition) String() string {
	res := []string(nil)
	for _, s := range c.Summands {
		sq := s.SubQuery
		if sq != "" {
			sq += ":"
		}
		suffix := ""
		if s.Factor != 1 && s.Factor != -1 {
			n := s.Factor
			if n < 0 {
				n = -n
			}
			suffix = fmt.Sprintf("*%d", n)
		}
		prefix := ""
		if s.Factor < 0 {
			prefix = "-"
		} else if len(res) != 0 {
			prefix = "+"
		}
		name := map[NumberConditionSummandType]string{
			NumberConditionSummandTypeID:          "id",
			NumberConditionSummandTypeClientPort:  "cport",
			NumberConditionSummandTypeServerPort:  "sport",
			NumberConditionSummandTypeClientBytes: "cbytes",
			NumberConditionSummandTypeServerBytes: "sbytes",
		}[s.Type]
		res = append(res, fmt.Sprintf("%s%s%s%s", prefix, sq, name, suffix))
	}
	if c.Number != 0 {
		tmp := c.Number
		prefix := "+"
		if tmp < 0 {
			prefix = "-"
			tmp = -tmp
		}
		res = append(res, fmt.Sprintf("%s%d", prefix, tmp))
	}
	return fmt.Sprintf("%s >= 0", strings.Join(res, ""))
}

func (c *DataCondition) String() string {
	res := []string(nil)
	for i, e := range c.Elements {
		inv := map[bool]string{false: "", true: "-"}[c.Inverted && (i == len(c.Elements)-1)]
		who := map[uint8]string{
			DataRequirementSequenceFlagsDirectionClientToServer: "cdata",
			DataRequirementSequenceFlagsDirectionServerToClient: "sdata",
		}[e.Flags&DataRequirementSequenceFlagsDirection]
		sq := e.SubQuery
		if sq != "" {
			sq += ":"
		}
		res = append(res, fmt.Sprintf("%s%s%s:%q", inv, sq, who, e.Regex))
	}
	return strings.Join(res, " > ")
}

func (c *ImpossibleCondition) String() string {
	return "false"
}

func (c *TagCondition) impossible() bool {
	return false
}

func (c *FlagCondition) impossible() bool {
	return false
}

func (c *HostCondition) impossible() bool {
	return false
}

func (c *TimeCondition) impossible() bool {
	return false
}

func (c *NumberCondition) impossible() bool {
	return false
}

func (c *DataCondition) impossible() bool {
	return false
}

func (c *ImpossibleCondition) impossible() bool {
	return true
}

func (c *TagCondition) equal(d Condition) bool {
	o, ok := d.(*TagCondition)
	return ok && c.Invert == o.Invert && c.SubQuery == o.SubQuery && c.TagName == o.TagName
}

func (c *FlagCondition) equal(d Condition) bool {
	o, ok := d.(*FlagCondition)
	if !(ok && c.Mask == o.Mask && c.Value == o.Value && len(c.SubQueries) == len(o.SubQueries)) {
		return false
	}
	for i := 0; i < len(c.SubQueries); i++ {
		if c.SubQueries[i] != o.SubQueries[i] {
			return false
		}
	}
	return true
}

func (c *HostCondition) equal(d Condition) bool {
	o, ok := d.(*HostCondition)
	//lint:ignore SA1021 intended
	//nolint:staticcheck
	if !(ok && bytes.Equal(c.Host, o.Host) && bytes.Equal(c.Mask4, o.Mask4) && bytes.Equal(c.Mask6, o.Mask6) && c.Invert == o.Invert && len(c.HostConditionSources) == len(o.HostConditionSources)) {
		return false
	}
	for i := 0; i < len(c.HostConditionSources); i++ {
		if c.HostConditionSources[i] != o.HostConditionSources[i] {
			return false
		}
	}
	return true
}

func (c *TimeCondition) equal(d Condition) bool {
	o, ok := d.(*TimeCondition)
	if !(ok && c.Duration == o.Duration && c.ReferenceTimeFactor == o.ReferenceTimeFactor && len(c.Summands) == len(o.Summands)) {
		return false
	}
	for i := 0; i < len(c.Summands); i++ {
		if c.Summands[i] != o.Summands[i] {
			return false
		}
	}
	return true
}

func (c *NumberCondition) equal(d Condition) bool {
	o, ok := d.(*NumberCondition)
	if !(ok && c.Number == o.Number && len(c.Summands) == len(o.Summands)) {
		return false
	}
	for i := 0; i < len(c.Summands); i++ {
		if c.Summands[i].Factor != o.Summands[i].Factor {
			return false
		}
		if c.Summands[i].Type != o.Summands[i].Type {
			return false
		}
		if c.Summands[i].SubQuery != o.Summands[i].SubQuery {
			return false
		}
	}
	return true
}

func (c *DataCondition) equal(d Condition) bool {
	o, ok := d.(*DataCondition)
	if !(ok && c.Inverted == o.Inverted && len(c.Elements) == len(o.Elements)) {
		return false
	}
	for i := 0; i < len(c.Elements); i++ {
		ce, oe := c.Elements[i], o.Elements[i]
		if !(ce.Flags == oe.Flags && ce.Regex == oe.Regex && ce.SubQuery == oe.SubQuery && len(ce.Variables) == len(oe.Variables)) {
			return false
		}
		for j := 0; j < len(ce.Variables); j++ {
			if ce.Variables[j] != oe.Variables[j] {
				return false
			}
		}
	}
	return true
}

func (c *ImpossibleCondition) equal(d Condition) bool {
	_, ok := d.(*ImpossibleCondition)
	return ok
}

func (c *TagCondition) invert() ConditionsSet {
	return ConditionsSet{
		Conditions{
			&TagCondition{
				SubQuery: c.SubQuery,
				TagName:  c.TagName,
				Invert:   !c.Invert,
			},
		},
	}
}

//tcp -> !udp & !sctp & !other
//!!udp -> udp -> !tcp & !sctp & !other
//!tcp -> !tcp & !sctp & !other
func (c *FlagCondition) invert() ConditionsSet {
	cond := Conditions(nil)
	for v := c.Value & c.Mask; ; {
		v--
		v &= c.Mask
		if v == c.Value&c.Mask {
			return ConditionsSet{cond}
		}
		cond = append(cond, &FlagCondition{
			SubQueries: c.SubQueries,
			Value:      v,
			Mask:       c.Mask,
		})
	}
}

func (c *HostCondition) invert() ConditionsSet {
	return ConditionsSet{Conditions{&HostCondition{
		HostConditionSources: c.HostConditionSources,
		Host:                 c.Host,
		Mask4:                c.Mask4,
		Mask6:                c.Mask6,
		Invert:               !c.Invert,
	}}}
}

func (c *TimeCondition) invert() ConditionsSet {
	// !(n >= 0) -> -n-1 >= 0
	cond := TimeCondition{
		Summands:            make([]TimeConditionSummand, 0, len(c.Summands)),
		Duration:            -c.Duration - 1,
		ReferenceTimeFactor: -c.ReferenceTimeFactor,
	}
	for _, s := range c.Summands {
		cond.Summands = append(cond.Summands, TimeConditionSummand{
			SubQuery:    s.SubQuery,
			FTimeFactor: -s.FTimeFactor,
			LTimeFactor: -s.LTimeFactor,
		})
	}
	return ConditionsSet{Conditions{&cond}}
}

func (c *NumberCondition) invert() ConditionsSet {
	// !(n >= 0) -> -n-1 >= 0
	cond := NumberCondition{
		Summands: make([]NumberConditionSummand, 0, len(c.Summands)),
		Number:   -c.Number - 1,
	}
	for _, s := range c.Summands {
		cond.Summands = append(cond.Summands, NumberConditionSummand{
			SubQuery: s.SubQuery,
			Factor:   -s.Factor,
			Type:     s.Type,
		})
	}
	return ConditionsSet{Conditions{&cond}}
}

func (c *DataCondition) invert() ConditionsSet {
	// !(a > b > c) = !a | a > !b | a > b > !c
	// !(a > b > !c) = !a | a > !b | a > b > c
	conds := ConditionsSet(nil)
	for l := 1; l <= len(c.Elements); l++ {
		inv := true
		if last := l == len(c.Elements); last {
			inv = !c.Inverted
		}
		conds = append(conds, Conditions{
			&DataCondition{
				Elements: c.Elements[:l],
				Inverted: inv,
			},
		})
	}
	return conds
}

func (c *ImpossibleCondition) invert() ConditionsSet {
	return ConditionsSet{}
}

func (t *queryTerm) QueryConditions(pc *parserContext) (ConditionsSet, error) {
	conds := ConditionsSet(nil)
	switch t.Key {
	case "tag":
		for _, v := range strings.Split(t.Value, ",") {
			conds = append(conds, Conditions{
				&TagCondition{
					SubQuery: t.SubQuery,
					TagName:  strings.TrimSpace(v),
				},
			})
		}
	case "protocol":
		val := tokenListParser{}
		if err := valueTokenListParser.ParseString("", t.Value, &val); err != nil {
			return nil, err
		}
		for _, e := range val.List {
			if e.Variable != nil {
				if e.Variable.Name != "protocol" {
					return nil, fmt.Errorf("protocol filter can only contain protocol variables, not %q", e.Variable.Name)
				}
				if e.Variable.Sub != t.SubQuery {
					conds = append(conds, (&FlagCondition{
						SubQueries: []string{t.SubQuery, e.Variable.Sub},
						Mask:       flagsStreamProtocol,
					}).invert()...)
				}
				continue
			}
			f, ok := map[string]uint16{
				"tcp":   flagsStreamProtocolTCP,
				"udp":   flagsStreamProtocolUDP,
				"sctp":  flagsStreamProtocolSCTP,
				"other": flagsStreamProtocolOther,
			}[strings.ToLower(e.Token)]
			if !ok {
				return nil, fmt.Errorf("unknown protocol %q", e.Token)
			}
			conds = append(conds, (&FlagCondition{
				SubQueries: []string{t.SubQuery},
				Mask:       flagsStreamProtocol,
				Value:      f,
			}).invert()...)
		}
	case "chost", "shost", "host":
		val := hostListParser{}
		if err := valueHostListParser.ParseString("", t.Value, &val); err != nil {
			return nil, err
		}
		fTypes := map[string][]HostConditionSourceType{
			"chost": {HostConditionSourceTypeClient},
			"shost": {HostConditionSourceTypeServer},
			"host":  {HostConditionSourceTypeClient, HostConditionSourceTypeServer},
		}[t.Key]
		for _, fType := range fTypes {
			for _, e := range val.List {
				cond := &HostCondition{
					HostConditionSources: []HostConditionSource{{
						Type:     fType,
						SubQuery: t.SubQuery,
					}},
				}
				if e.Host != nil {
					cond.Host = e.Host.Host
				} else {
					vType, ok := map[string]HostConditionSourceType{
						"chost": HostConditionSourceTypeClient,
						"shost": HostConditionSourceTypeServer,
					}[e.Variable.Name]
					if !ok {
						return nil, fmt.Errorf("unsupported variable type in host filer: %q", e.Variable.Name)
					}
					cond.HostConditionSources = append(cond.HostConditionSources, HostConditionSource{
						SubQuery: e.Variable.Sub,
						Type:     vType,
					})
				}
				if e.Masks != nil {
					cond.Mask4 = e.Masks.V4Mask
					cond.Mask6 = e.Masks.V6Mask
				} else {
					cond.Mask4 = net.IP{
						255, 255, 255, 255,
					}
					cond.Mask6 = net.IP{
						255, 255, 255, 255, 255, 255, 255, 255,
						255, 255, 255, 255, 255, 255, 255, 255,
					}
				}
				conds = append(conds, Conditions{cond})
			}
		}
	case "id", "cport", "sport", "port", "cbytes", "sbytes", "bytes":
		val := numberRangeListParser{}
		if err := valueNumberRangeListParser.ParseString("", t.Value, &val); err != nil {
			return nil, err
		}
		for _, e := range val.List {
			ncs := [2]*NumberCondition{{}, {}}
			empty := [2]bool{false, false}
			for ir, r := range e.Range {
				nc := ncs[ir]
				empty[ir] = len(r.Parts) == 0
				for _, p := range r.Parts {
					factor := 1 - (2 * (strings.Count(p.Operators, "-") % 2))
					if p.Variable == nil {
						nc.Number += factor * p.Number
						continue
					}
					vType, ok := map[string]NumberConditionSummandType{
						"id":     NumberConditionSummandTypeID,
						"cport":  NumberConditionSummandTypeClientPort,
						"sport":  NumberConditionSummandTypeServerPort,
						"cbytes": NumberConditionSummandTypeClientBytes,
						"sbytes": NumberConditionSummandTypeServerBytes,
					}[p.Variable.Name]
					if !ok {
						return nil, errors.New("only id, [cs]port, [cs]bytes variables supported in filter of the same types")
					}
					for i, sc := 0, len(nc.Summands); i <= sc; i++ {
						if i == sc {
							nc.Summands = append(nc.Summands, NumberConditionSummand{
								SubQuery: p.Variable.Sub,
								Type:     vType,
							})
						}
						s := &nc.Summands[i]
						if s.SubQuery != p.Variable.Sub || s.Type != vType {
							continue
						}
						s.Factor += factor
						break
					}
				}
				if len(e.Range) == 1 {
					ncs[1].Number = ncs[0].Number
					ncs[1].Summands = make([]NumberConditionSummand, len(ncs[0].Summands))
					copy(ncs[1].Summands, ncs[0].Summands)
					empty[1] = empty[0]
				}
			}
			fTypes := map[string][]NumberConditionSummandType{
				"id":     {NumberConditionSummandTypeID},
				"cport":  {NumberConditionSummandTypeClientPort},
				"sport":  {NumberConditionSummandTypeServerPort},
				"port":   {NumberConditionSummandTypeClientPort, NumberConditionSummandTypeServerPort},
				"cbytes": {NumberConditionSummandTypeClientBytes},
				"sbytes": {NumberConditionSummandTypeServerBytes},
				"bytes":  {NumberConditionSummandTypeClientBytes, NumberConditionSummandTypeServerBytes},
			}[t.Key]
			ncsCopy := [2]*NumberCondition{
				ncs[0],
				ncs[1],
			}
			for _, fType := range fTypes {
				for nci := range ncs {
					nc := &NumberCondition{
						Summands: append([]NumberConditionSummand(nil), ncsCopy[nci].Summands...),
						Number:   ncsCopy[nci].Number,
					}
					ncs[nci] = nc
					for i, sc := 0, len(nc.Summands); i <= sc; i++ {
						if i == len(nc.Summands) {
							nc.Summands = append(nc.Summands, NumberConditionSummand{
								SubQuery: t.SubQuery,
								Type:     fType,
							})
						}
						s := &nc.Summands[i]
						if s.SubQuery == t.SubQuery && s.Type == fType {
							s.Factor--
							sc--
						}
						if s.Factor == 0 {
							*s = nc.Summands[len(nc.Summands)-1]
							nc.Summands = nc.Summands[:len(nc.Summands)-1]
							i--
							sc--
						}
					}
				}
				ncs[0].Number *= -1
				for i := range ncs[0].Summands {
					s := &ncs[0].Summands[i]
					s.Factor *= -1
				}
				cond := Conditions{}
				if !empty[0] {
					cond = append(cond, ncs[0])
				}
				if !empty[1] {
					cond = append(cond, ncs[1])
				}
				conds = append(conds, cond)
			}
		}
	case "ftime", "ltime", "time":
		val := timeRangeListParser{}
		if err := valueTimeRangeListParser.ParseString("", t.Value, &val); err != nil {
			return nil, err
		}
		for _, e := range val.List {
			tcs := [2]*TimeCondition{{}, {}}
			empty := [2]bool{false, false}
			for ir, r := range e.Range {
				tc := tcs[ir]
				empty[ir] = len(r.Parts) == 0
				for _, p := range r.Parts {
					factor := 1 - (2 * (strings.Count(p.Operators, "-") % 2))
					if p.Duration != nil {
						tc.Duration += time.Duration(factor) * p.Duration.Duration
					} else if p.Time != nil {
						t := p.Time.Time
						d := &t
						if !p.Time.HasDate {
							d = &pc.referenceTime
						}
						t = time.Date(
							d.Year(),
							d.Month(),
							d.Day(),
							t.Hour(),
							t.Minute(),
							t.Second(),
							t.Nanosecond(),
							pc.timezone,
						)
						tc.Duration += time.Duration(factor) * t.Sub(pc.referenceTime)
						tc.ReferenceTimeFactor -= factor
					} else if p.Variable != nil {
						for i, sc := 0, len(tc.Summands); i <= sc; i++ {
							if i == sc {
								tc.Summands = append(tc.Summands, TimeConditionSummand{
									SubQuery: p.Variable.Sub,
								})
							}
							s := &tc.Summands[i]
							if s.SubQuery != p.Variable.Sub {
								continue
							}
							switch p.Variable.Name {
							case "ftime":
								s.FTimeFactor += factor
							case "ltime":
								s.LTimeFactor += factor
							default:
								return nil, errors.New("only [fl]time variables supported in [fl]?time filters")
							}
							break
						}
					}
				}
				if len(e.Range) == 1 {
					tcs[1].Duration = tcs[0].Duration
					tcs[1].Summands = make([]TimeConditionSummand, len(tcs[0].Summands))
					copy(tcs[1].Summands, tcs[0].Summands)
					empty[1] = empty[0]
				}
			}
			for tci, tc := range tcs {
				for i, sc := 0, len(tc.Summands); i <= sc; i++ {
					if i == len(tc.Summands) {
						tc.Summands = append(tc.Summands, TimeConditionSummand{
							SubQuery: t.SubQuery,
						})
					}
					s := &tc.Summands[i]
					if s.SubQuery == t.SubQuery {
						switch t.Key {
						case "ftime":
							s.FTimeFactor--
						case "ltime":
							s.LTimeFactor--
						case "time":
							s.FTimeFactor -= tci
							s.LTimeFactor -= 1 - tci
						}
						sc--
					}
					if s.FTimeFactor == 0 && s.LTimeFactor == 0 {
						*s = tc.Summands[len(tc.Summands)-1]
						tc.Summands = tc.Summands[:len(tc.Summands)-1]
						i--
						sc--
					}
				}
			}
			tcs[0].Duration *= -1
			tcs[0].ReferenceTimeFactor *= -1
			for i := range tcs[0].Summands {
				s := &tcs[0].Summands[i]
				s.FTimeFactor *= -1
				s.LTimeFactor *= -1
			}
			cond := Conditions{}
			if !empty[0] {
				cond = append(cond, tcs[0])
			}
			if !empty[1] {
				cond = append(cond, tcs[1])
			}
			conds = append(conds, cond)
			fmt.Printf("%s\n", conds.String())
		}
	case "cdata", "sdata", "data":
		val := stringParser{}
		if err := valueStringParser.ParseString("", t.Value, &val); err != nil {
			return nil, err
		}
		content := ""
		testContent := ""
		variables := []DataConditionElementVariable(nil)
		for _, e := range val.Elements {
			if e.Variable == nil {
				content += e.Content
				testContent += e.Content
				continue
			}
			testContent += "(?:test)"
			variables = append(variables, DataConditionElementVariable{
				Position: uint(len(content)),
				SubQuery: e.Variable.Sub,
				Name:     e.Variable.Name,
			})
		}
		if _, err := binaryregexp.Compile(testContent); err != nil {
			return nil, err
		}
		flags := map[string][]uint8{
			"data": {
				DataRequirementSequenceFlagsDirectionClientToServer,
				DataRequirementSequenceFlagsDirectionServerToClient,
			},
			"cdata": {DataRequirementSequenceFlagsDirectionClientToServer},
			"sdata": {DataRequirementSequenceFlagsDirectionServerToClient},
		}[t.Key]
		for _, f := range flags {
			conds = append(conds, Conditions{
				&DataCondition{
					Elements: []DataConditionElement{
						{
							Regex:     content,
							Variables: variables,
							SubQuery:  t.SubQuery,
							Flags:     f,
						},
					},
				},
			})
		}
	}
	return conds, nil
}

func (cs Conditions) invert() ConditionsSet {
	// !(a & b & c) == !a | !b | !c
	res := ConditionsSet(nil)
	for _, c := range cs {
		res = res.or(c.invert())
	}
	return res
}

func (c ConditionsSet) invert() ConditionsSet {
	// !(a | b | c) == (!a & !b & !c)
	conds := ConditionsSet{}
	for _, cc := range c {
		conds = conds.and(cc.invert())
	}
	return conds
}

func (a Conditions) then(b Conditions) Conditions {
	res := Conditions(nil)
	adcs, bdcs := []Condition(nil), []Condition(nil)
	for _, cc := range a {
		if _, ok := cc.(*DataCondition); ok {
			adcs = append(adcs, cc)
		} else {
			res = append(res, cc)
		}
	}
	for _, cc := range b {
		if _, ok := cc.(*DataCondition); ok {
			bdcs = append(bdcs, cc)
		} else {
			res = append(res, cc)
		}
	}
	if len(adcs) == 0 || len(bdcs) == 0 {
		res = append(res, adcs...)
		res = append(res, bdcs...)
		return res
	}
	for _, acc := range adcs {
		adc := acc.(*DataCondition)
		l := len(adc.Elements)
		if adc.Inverted {
			res = append(res, acc)
			l--
		}
		for _, bcc := range bdcs {
			bdc := bcc.(*DataCondition)
			res = append(res, &DataCondition{
				Inverted: bdc.Inverted,
				Elements: append(append([]DataConditionElement(nil), adc.Elements[:l]...), bdc.Elements...),
			})
		}
	}
	return res
}

func (a ConditionsSet) then(b ConditionsSet) ConditionsSet {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	res := ConditionsSet(nil)
	for _, c1 := range a {
		for _, c2 := range b {
			res = res.or(ConditionsSet{c1.then(c2)})
		}
	}
	return res
}

func (a Conditions) and(b Conditions) Conditions {
	return append(append(Conditions(nil), a...), b...).clean()
}

func (a ConditionsSet) and(b ConditionsSet) ConditionsSet {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	res := ConditionsSet{}
	for _, c1 := range a {
		for _, c2 := range b {
			res = res.or(ConditionsSet{c1.and(c2)})
		}
	}
	return res
}

//lint:ignore U1000 intended
func (a Conditions) or(b Conditions) ConditionsSet {
	return ConditionsSet{a, b}
}

func (a ConditionsSet) or(b ConditionsSet) ConditionsSet {
	return append(append(ConditionsSet(nil), a...), b...)
}

func (c Conditions) impossible() bool {
	c = c.clean()
	return len(c) == 1 && impossibleCondition.equal(c[0])
}

func (c ConditionsSet) impossible() bool {
	c = c.clean()
	return len(c) == 1 && c[0].impossible()
}

func (a Conditions) equal(b Conditions) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].equal(b[i]) {
			return false
		}
	}
	return true
}

func (c ConditionsSet) clean() ConditionsSet {
	new := ConditionsSet(nil)
outer:
	for _, cc := range c {
		if cc.impossible() {
			continue
		}
		cc = cc.clean()
		for i, cc2 := range new {
			if cc2.equal(cc) {
				continue outer
			}
			anded := cc.and(cc2).clean()
			if anded.equal(cc) {
				continue outer
			}
			if anded.equal(cc2) {
				new[i] = cc
				continue outer
			}
		}
		new = append(new, cc)
	}
	if len(new) == 0 && len(c) != 0 {
		return ConditionsSet{Conditions{&impossibleCondition}}
	}
	return new
}

func cleanTagConditions(lcs *[]TagCondition) bool {
	if len(*lcs) == 0 {
		return true
	}
	type key struct{ s, t string }
	m := map[key]bool{}
	for _, lc := range *lcs {
		k := key{s: lc.SubQuery, t: lc.TagName}
		if inv, ok := m[k]; ok && inv != lc.Invert {
			return false
		}
		m[k] = lc.Invert
	}
	*lcs = nil
	for k, inv := range m {
		*lcs = append(*lcs, TagCondition{
			SubQuery: k.s,
			TagName:  k.t,
			Invert:   inv,
		})
	}
	sort.Slice(*lcs, func(i, j int) bool {
		lca, lcb := (*lcs)[i], (*lcs)[j]
		if lca.SubQuery != lcb.SubQuery {
			return lca.SubQuery < lcb.SubQuery
		}
		return lca.TagName < lcb.TagName
	})
	return true
}

func cleanFlagConditions(fcs *[]FlagCondition) bool {
	if len(*fcs) == 0 {
		return true
	}
	type forbiddenFlagValues struct {
		SubQueries []string
		forbidden  []uint64
	}
	infos := []forbiddenFlagValues(nil)
next_fc:
	for _, fc := range *fcs {
		sort.Strings(fc.SubQueries)
		for i := 1; i < len(fc.SubQueries); i++ {
			if fc.SubQueries[i-1] == fc.SubQueries[i] {
				fc.SubQueries = append(fc.SubQueries[:i-1], fc.SubQueries[i+1:]...)
				i -= 2
			}
		}
		if len(fc.SubQueries) == 0 {
			if fc.Value&fc.Mask == 0 {
				return false
			}
			continue
		}
		forbidden := make([]uint64, 0x10000/64)
		for v := uint16(0); ; v++ {
			if v&fc.Mask == fc.Value {
				forbidden[v/64] |= 1 << (v % 64)
			}
			if v == math.MaxUint16 {
				break
			}
		}
	next_info:
		for _, info := range infos {
			if len(info.SubQueries) != len(fc.SubQueries) {
				continue
			}
			for i := range fc.SubQueries {
				if fc.SubQueries[i] != info.SubQueries[i] {
					continue next_info
				}
			}
			for i := range forbidden {
				info.forbidden[i] |= forbidden[i]
			}
			continue next_fc
		}
		infos = append(infos, forbiddenFlagValues{
			SubQueries: fc.SubQueries,
			forbidden:  forbidden,
		})
	}
	*fcs = nil
	for _, info := range infos {
		mask := uint16(0)
		for bit := 0; bit < 16; bit++ {
			m := uint16(1 << bit)
			for v := ^m; ; v = (v - 1) & ^m {
				f1 := 1 & (info.forbidden[v/64] >> (v % 64))
				f2 := 1 & (info.forbidden[(v^m)/64] >> ((v ^ m) % 64))
				if f1 != f2 {
					mask |= m
					break
				}
				if v == 0 {
					break
				}
			}
		}
		if mask == 0 {
			if info.forbidden[0]&1 == 0 {
				continue
			}
			return false
		}
		for v := mask; ; v = (v - 1) & mask {
			f := 1 & (info.forbidden[v/64] >> (v % 64))
			if f != 0 {
				*fcs = append(*fcs, FlagCondition{
					SubQueries: append([]string(nil), info.SubQueries...),
					Mask:       mask,
					Value:      v,
				})
			}
			if v == 0 {
				break
			}
		}
	}
	//TODO: split masks if it creates less conditions
	//TODO: try to remove dependencies between multiple subqueries if possible
	sort.Slice(*fcs, func(i, j int) bool {
		a, b := (*fcs)[i], (*fcs)[j]
		if len(a.SubQueries) != len(b.SubQueries) {
			return len(a.SubQueries) < len(b.SubQueries)
		}
		for i := range a.SubQueries {
			if a.SubQueries[i] != b.SubQueries[i] {
				return a.SubQueries[i] < b.SubQueries[i]
			}
		}
		if a.Mask != b.Mask {
			return a.Mask < b.Mask
		}
		return a.Value < b.Value
	})
	return true
}

func cleanHostConditions(hcs *[]HostCondition) bool {
	hcsLess := func(a, b *HostConditionSource) bool {
		if a.SubQuery != b.SubQuery {
			return a.SubQuery < b.SubQuery
		}
		if a.Type != b.Type {
			return a.Type == HostConditionSourceTypeClient
		}
		return false
	}
	for i := 0; i < len(*hcs); i++ {
		hcsi := &(*hcs)[i]
		sort.Slice(hcsi.HostConditionSources, func(i, j int) bool {
			return hcsLess(&hcsi.HostConditionSources[i], &hcsi.HostConditionSources[j])
		})
		for j := 1; j < len(hcsi.HostConditionSources); j++ {
			a, b := hcsi.HostConditionSources[j-1], hcsi.HostConditionSources[j]
			if a.SubQuery != b.SubQuery {
				continue
			}
			if a.Type != b.Type {
				continue
			}
			hcsi.HostConditionSources = append(hcsi.HostConditionSources[:j-1], hcsi.HostConditionSources[j+1:]...)
		}
		zeroHost := true
		switch len(hcsi.Host) {
		case 4:
			for i := range hcsi.Host {
				hcsi.Host[i] &= hcsi.Mask4[i]
				zeroHost = zeroHost && hcsi.Host[i] == 0
			}
		case 16:
			for i := range hcsi.Host {
				hcsi.Host[i] &= hcsi.Mask6[i]
				zeroHost = zeroHost && hcsi.Host[i] == 0
			}
		}
		if len(hcsi.HostConditionSources) != 0 {
			continue
		}
		if zeroHost == hcsi.Invert {
			return false
		}
		*hcs = append((*hcs)[:i], (*hcs)[i+1:]...)
		i--
	}
	sort.Slice(*hcs, func(i, j int) bool {
		a, b := (*hcs)[i], (*hcs)[j]
		if len(a.HostConditionSources) != len(b.HostConditionSources) {
			return len(a.HostConditionSources) < len(b.HostConditionSources)
		}
		for i := range a.HostConditionSources {
			alb := hcsLess(&a.HostConditionSources[i], &b.HostConditionSources[i])
			bla := hcsLess(&b.HostConditionSources[i], &a.HostConditionSources[i])
			if alb || bla {
				return alb
			}
		}
		if cmp := bytes.Compare(a.Host, b.Host); cmp != 0 {
			return cmp < 0
		}
		if cmp := bytes.Compare(a.Mask4, b.Mask4); cmp != 0 {
			return cmp < 0
		}
		if cmp := bytes.Compare(a.Mask6, b.Mask6); cmp != 0 {
			return cmp < 0
		}
		return b.Invert && !a.Invert
	})
outer:
	for i := 1; i < len(*hcs); i++ {
		a, b := (*hcs)[i-1], (*hcs)[i]
		if len(a.HostConditionSources) != len(b.HostConditionSources) {
			continue
		}
		for j := 0; j < len(a.HostConditionSources); j++ {
			if hcsLess(&a.HostConditionSources[j], &b.HostConditionSources[j]) {
				continue outer
			}
			if hcsLess(&b.HostConditionSources[j], &a.HostConditionSources[j]) {
				continue outer
			}
		}
		//lint:ignore SA1021 intended
		//nolint:staticcheck
		if !bytes.Equal(a.Host, b.Host) {
			continue
		}
		//lint:ignore SA1021 intended
		//nolint:staticcheck
		if !bytes.Equal(a.Mask4, b.Mask4) {
			continue
		}
		//lint:ignore SA1021 intended
		//nolint:staticcheck
		if !bytes.Equal(a.Mask6, b.Mask6) {
			continue
		}
		if a.Invert != b.Invert {
			return false
		}
		copy((*hcs)[i-1:], (*hcs)[i:])
		*hcs = (*hcs)[:len(*hcs)-1]
		i--
	}
	//TODO: implement more impossibility checks, try to remove dependencies
	return true
}

func cleanNumberConditions(ncs *[]NumberCondition) bool {
	for i := 0; i < len(*ncs); i++ {
		nc := &(*ncs)[i]
		new := NumberCondition{
			Summands: make([]NumberConditionSummand, len(nc.Summands)),
			Number:   nc.Number,
		}
		copy(new.Summands, nc.Summands)
		*nc = new
		sort.Slice(nc.Summands, func(i, j int) bool {
			a, b := &nc.Summands[i], &nc.Summands[j]
			if a.SubQuery != b.SubQuery {
				return a.SubQuery < b.SubQuery
			}
			return a.Type < b.Type
		})
		for j := 1; j < len(nc.Summands); {
			a, b := &nc.Summands[j-1], &nc.Summands[j]
			if a.SubQuery == b.SubQuery && a.Type == b.Type {
				a.Factor += b.Factor
				nc.Summands = append(nc.Summands[:j], nc.Summands[j+1:]...)
			} else if a.Factor == 0 {
				nc.Summands = append(nc.Summands[:j-1], nc.Summands[j:]...)
			} else {
				j++
			}
		}
		if len(nc.Summands) == 0 {
			if nc.Number < 0 {
				return false
			}
			*ncs = append((*ncs)[:i], (*ncs)[i+1:]...)
			i--
			continue
		}
		commonFactor := nc.Summands[0].Factor
		if commonFactor < 0 {
			commonFactor = -commonFactor
		}
		for j := 1; commonFactor != 1 && j < len(nc.Summands); {
			f := nc.Summands[j].Factor
			if f < 0 {
				f = -f
			}
			if f%commonFactor == 0 {
				continue
			}
			if commonFactor%f == 0 {
				commonFactor = f
				continue
			}
			oldCommonFactor := commonFactor
			for commonFactor--; commonFactor > 1; commonFactor-- {
				if oldCommonFactor%commonFactor == 0 && f%commonFactor == 0 {
					break
				}
			}
		}
		if commonFactor == 1 {
			continue
		}
		f := nc.Number
		if f < 0 {
			f = -f
		}
		if f%commonFactor != 0 {
			oldCommonFactor := commonFactor
			for commonFactor--; commonFactor > 1; commonFactor-- {
				if oldCommonFactor%commonFactor == 0 && f%commonFactor == 0 {
					break
				}
			}
		}
		nc.Number /= commonFactor
		for j := range nc.Summands {
			nc.Summands[j].Factor /= commonFactor
		}
	}
	sort.Slice(*ncs, func(i, j int) bool {
		a, b := &(*ncs)[i], &(*ncs)[j]
		if len(a.Summands) != len(b.Summands) {
			return len(a.Summands) < len(b.Summands)
		}
		for i := range a.Summands {
			as, bs := a.Summands[i], b.Summands[i]
			if as.SubQuery != bs.SubQuery {
				return as.SubQuery < bs.SubQuery
			}
			if as.Type != bs.Type {
				return as.Type < bs.Type
			}
			if as.Factor != bs.Factor {
				return as.Factor < bs.Factor
			}
		}
		return a.Number < b.Number
	})
outer:
	for i := 1; i < len(*ncs); i++ {
		a, b := &(*ncs)[i-1], &(*ncs)[i]
		if len(a.Summands) != len(b.Summands) {
			continue
		}
		for i := range a.Summands {
			as, bs := a.Summands[i], b.Summands[i]
			if as.SubQuery != bs.SubQuery {
				continue outer
			}
			if as.Type != bs.Type {
				continue outer
			}
			if as.Factor != bs.Factor {
				continue outer
			}
		}
		*ncs = append((*ncs)[:i], (*ncs)[i+1:]...)
		i--
	}
	for i := 0; i < len(*ncs); i++ {
		nc := (*ncs)[i]
		allPositive := nc.Number >= 0
		allNegative := nc.Number < 0
		for _, s := range nc.Summands {
			if !(allNegative || allPositive) {
				break
			}
			if s.Factor > 0 {
				allNegative = false
			}
			if s.Factor < 0 {
				allPositive = false
			}
		}
		if allPositive {
			*ncs = append((*ncs)[:i], (*ncs)[i+1:]...)
			i--
			continue
		}
		if allNegative {
			return false
		}
	}
	return true
}

func cleanTimeConditions(tcs *[]TimeCondition) bool {
	for i := 0; i < len(*tcs); i++ {
		tc := &(*tcs)[i]
		sort.Slice(tc.Summands, func(i, j int) bool {
			a, b := &tc.Summands[i], &tc.Summands[j]
			if a.SubQuery != b.SubQuery {
				return a.SubQuery < b.SubQuery
			}
			if a.FTimeFactor != b.FTimeFactor {
				return a.FTimeFactor < b.FTimeFactor
			}
			return a.LTimeFactor < b.LTimeFactor
		})
		for j := 1; j < len(tc.Summands); {
			a, b := &tc.Summands[j-1], &tc.Summands[j]
			if a.SubQuery == b.SubQuery {
				a.FTimeFactor += b.FTimeFactor
				a.LTimeFactor += b.LTimeFactor
				tc.Summands = append(tc.Summands[:j], tc.Summands[j+1:]...)
			} else if a.FTimeFactor == 0 && a.LTimeFactor == 0 {
				tc.Summands = append(tc.Summands[:j-1], tc.Summands[j:]...)
			} else {
				j++
			}
		}
		if len(tc.Summands) >= 1 && tc.Summands[len(tc.Summands)-1].FTimeFactor == 0 && tc.Summands[len(tc.Summands)-1].LTimeFactor == 0 {
			tc.Summands = tc.Summands[:len(tc.Summands)-1]
		}
		switch len(tc.Summands) {
		case 0:
			if tc.Duration < 0 {
				return false
			}
			*tcs = append((*tcs)[:i], (*tcs)[i+1:]...)
			i--
			continue
		case 1:
			s := tc.Summands[0]
			if s.FTimeFactor+s.LTimeFactor != 0 {
				break
			}
			if s.FTimeFactor > 0 {
				if tc.Duration < 0 {
					return false
				}
			} else {
				if tc.Duration >= 0 {
					*tcs = append((*tcs)[:i], (*tcs)[i+1:]...)
					i--
				}
			}
		}
	}
	sort.Slice(*tcs, func(i, j int) bool {
		a, b := &(*tcs)[i], &(*tcs)[j]
		if len(a.Summands) != len(b.Summands) {
			return len(a.Summands) < len(b.Summands)
		}
		for i := range a.Summands {
			as, bs := a.Summands[i], b.Summands[i]
			if as.SubQuery != bs.SubQuery {
				return as.SubQuery < bs.SubQuery
			}
			if as.FTimeFactor != bs.FTimeFactor {
				return as.FTimeFactor < bs.FTimeFactor
			}
			if as.LTimeFactor != bs.LTimeFactor {
				return as.LTimeFactor < bs.LTimeFactor
			}
		}
		if a.ReferenceTimeFactor != b.ReferenceTimeFactor {
			return a.ReferenceTimeFactor < b.ReferenceTimeFactor
		}
		return a.Duration < b.Duration
	})
outer:
	for i := 1; i < len(*tcs); i++ {
		a, b := &(*tcs)[i-1], &(*tcs)[i]
		if len(a.Summands) != len(b.Summands) {
			continue
		}
		for i := range a.Summands {
			as, bs := a.Summands[i], b.Summands[i]
			if as.SubQuery != bs.SubQuery {
				continue outer
			}
			if as.FTimeFactor != bs.FTimeFactor {
				continue outer
			}
			if as.LTimeFactor != bs.LTimeFactor {
				continue outer
			}
		}
		if a.ReferenceTimeFactor != b.ReferenceTimeFactor {
			continue
		}
		*tcs = append((*tcs)[:i], (*tcs)[i+1:]...)
		i--
	}
	return true
}

func cleanDataConditions(dcs *[]DataCondition) bool {
	sort.Slice(*dcs, func(i, j int) bool {
		a, b := (*dcs)[i], (*dcs)[j]
		for i := 0; i < len(a.Elements) && i < len(b.Elements); i++ {
			ae, be := a.Elements[i], b.Elements[i]
			if ae.SubQuery != be.SubQuery {
				return ae.SubQuery < be.SubQuery
			}
			if ae.Flags != be.Flags {
				return ae.Flags < be.Flags
			}
			if ae.Regex != be.Regex {
				return ae.Regex < be.Regex
			}
			for j := 0; j < len(ae.Variables) && j < len(be.Variables); j++ {
				aev, bev := ae.Variables[j], be.Variables[j]
				if aev.Position != bev.Position {
					return aev.Position < bev.Position
				}
				if aev.SubQuery != bev.SubQuery {
					return aev.SubQuery < bev.SubQuery
				}
				if aev.Name != bev.Name {
					return aev.Name < bev.Name
				}
			}
			if len(ae.Variables) != len(be.Variables) {
				return len(ae.Variables) < len(be.Variables)
			}
		}
		if len(a.Elements) != len(b.Elements) {
			return len(a.Elements) < len(b.Elements)
		}
		return false
	})
outer:
	for i := 1; i < len(*dcs); i++ {
		a, b := &(*dcs)[i-1], &(*dcs)[i]
		for i := 0; i < len(a.Elements) && i < len(b.Elements); i++ {
			ae, be := a.Elements[i], b.Elements[i]
			if ae.SubQuery != be.SubQuery {
				continue outer
			}
			if ae.Flags != be.Flags {
				continue outer
			}
			if ae.Regex != be.Regex {
				continue outer
			}
			for j := 0; j < len(ae.Variables) && j < len(be.Variables); j++ {
				aev, bev := ae.Variables[j], be.Variables[j]
				if aev.Position != bev.Position {
					continue outer
				}
				if aev.SubQuery != bev.SubQuery {
					continue outer
				}
				if aev.Name != bev.Name {
					continue outer
				}
			}
			if len(ae.Variables) != len(be.Variables) {
				continue outer
			}
		}
		if len(a.Elements) == len(b.Elements) && a.Inverted != b.Inverted {
			return false
		}
		*dcs = append((*dcs)[:i-1], (*dcs)[i:]...)
		i--
	}
	return true
}

func (c Conditions) clean() Conditions {
	lcs := []TagCondition(nil)
	fcs := []FlagCondition(nil)
	hcs := []HostCondition(nil)
	ncs := []NumberCondition(nil)
	tcs := []TimeCondition(nil)
	dcs := []DataCondition(nil)
	for _, cc := range c {
		switch ccc := cc.(type) {
		case *TagCondition:
			lcs = append(lcs, *ccc)
		case *FlagCondition:
			fcs = append(fcs, *ccc)
		case *HostCondition:
			hcs = append(hcs, *ccc)
		case *NumberCondition:
			ncs = append(ncs, *ccc)
		case *TimeCondition:
			tcs = append(tcs, *ccc)
		case *DataCondition:
			dcs = append(dcs, *ccc)
		case *ImpossibleCondition:
			return Conditions{
				&impossibleCondition,
			}
		}
	}
	possible := true
	possible = possible && cleanTagConditions(&lcs)
	possible = possible && cleanFlagConditions(&fcs)
	possible = possible && cleanHostConditions(&hcs)
	possible = possible && cleanNumberConditions(&ncs)
	possible = possible && cleanTimeConditions(&tcs)
	possible = possible && cleanDataConditions(&dcs)
	if !possible {
		return Conditions{&impossibleCondition}
	}
	res := Conditions(nil)
	for i := range lcs {
		res = append(res, &lcs[i])
	}
	for i := range fcs {
		res = append(res, &fcs[i])
	}
	for i := range hcs {
		res = append(res, &hcs[i])
	}
	for i := range ncs {
		res = append(res, &ncs[i])
	}
	for i := range tcs {
		res = append(res, &tcs[i])
	}
	for i := range dcs {
		res = append(res, &dcs[i])
	}
	return res

	/*if len(c.Ids) >= 2 {
		ids := map[uint64]struct{}{}
		for _, i := range c.Ids {
			ids[i] = struct{}{}
		}
		if len(ids) >= 2 && !c.IdsInverted {
			return Conditions{
				Ids: []uint64{0, 1},
			}
		}
		c.Ids = []uint64{}
		for i := range ids {
			c.Ids = append(c.Ids, i)
		}
		sort.Slice(c.Ids, func(i, j int) bool {
			return c.Ids[i] < c.Ids[j]
		})
	}
	if len(c.TagsRequired)+len(c.TagsForbidden) >= 2 {
		tags := map[string]bool{}
		newTagsRequired := []string{}
		newTagsForbidden := []string{}
		for _, t := range c.TagsRequired {
			if _, ok := tags[t]; !ok {
				tags[t] = true
				newTagsRequired = append(newTagsRequired, t)
			}
		}
		for _, t := range c.TagsForbidden {
			if req, ok := tags[t]; !ok {
				tags[t] = false
				newTagsForbidden = append(newTagsForbidden, t)
			} else if req {
				return Conditions{
					Ids: []uint64{0, 1},
				}
			}
		}
		sort.Strings(newTagsRequired)
		sort.Strings(newTagsForbidden)
		c.TagsRequired = newTagsRequired
		c.TagsForbidden = newTagsForbidden
	}
	foundImpossibleConditions := false
	cleanupPortRange := func(fpr []PortRange) []PortRange {
		if len(fpr) == 0 {
			return nil
		}
		forbidden := [0x10000 / 64]uint64{}
		for _, pr := range fpr {
			if pr.Min > pr.Max {
				continue
			}
			for p := pr.Min; ; p++ {
				forbidden[p/64] |= 1 << (p % 64)
				if p == pr.Max {
					break
				}
			}
		}
		new := []PortRange(nil)
		for p := uint16(0); ; p++ {
			if forbidden := 0 != 1&(forbidden[p/64]>>(p%64)); forbidden {
				if len(new) != 0 && new[len(new)-1].Max == p-1 {
					new[len(new)-1].Max++
				} else {
					new = append(new, PortRange{
						Min: p,
						Max: p,
					})
				}
			}
			if p == math.MaxUint16 {
				break
			}
		}
		if len(new) == 1 && new[0].Min == 0 && new[0].Max == math.MaxUint16 {
			foundImpossibleConditions = true
		}
		return new
	}
	cleanupBytesRange := func(fbrs []NumberCondition) []NumberCondition {
		if len(fbrs) == 0 {
			return nil
		}
		new := []NumberCondition{}
		lowerBound := uint64(0)
		for {
			allowed := true
			upperBound := uint64(math.MaxUint64)
			for _, fbr := range fbrs {
				if lowerBound >= fbr.Min && lowerBound <= fbr.Max {
					allowed = false
					break
				}
			}
			if allowed {
				for _, fbr := range fbrs {
					if lowerBound < fbr.Min && upperBound > fbr.Min-1 {
						upperBound = fbr.Min - 1
					}
				}
			} else {
				for _, fbr := range fbrs {
					if lowerBound >= fbr.Min && lowerBound <= fbr.Max && upperBound > fbr.Max {
						upperBound = fbr.Max
					}
				}
				if len(new) == 0 || new[len(new)-1].Max+1 != lowerBound {
					new = append(new, NumberCondition{
						Min: lowerBound,
					})
				}
				new[len(new)-1].Max = upperBound
			}
			if upperBound == math.MaxUint64 {
				break
			}
			lowerBound = upperBound + 1
		}
		if len(new) == 1 && new[0].Min == 0 && new[0].Max == math.MaxUint64 {
			foundImpossibleConditions = true
		}
		return new
	}
	c.ForbiddenClientPorts = cleanupPortRange(c.ForbiddenClientPorts)
	c.ForbiddenServerPorts = cleanupPortRange(c.ForbiddenServerPorts)
	c.ForbiddenClientBytes = cleanupBytesRange(c.ForbiddenClientBytes)
	c.ForbiddenServerBytes = cleanupBytesRange(c.ForbiddenServerBytes)
	if foundImpossibleConditions {
		return Conditions{
			Ids: []uint64{0, 1},
		}
	}
	cleanupHost := func(hms []HostCondition) []HostCondition {
		cmp := func(i, j int) int {
			a, b := &hms[i], &hms[j]
			if len(a.Host) != len(b.Host) {
				if len(a.Host) < len(b.Host) {
					return -1
				}
				return 1
			}
			if a.Inverted != b.Inverted {
				if b.Inverted {
					return -1
				}
				return 1

			}
			if a.Masklen != b.Masklen {
				if a.Masklen < b.Masklen {
					return -1
				}
				return 1
			}
			return bytes.Compare(a.Host, b.Host)
		}
		sort.Slice(hms, func(i, j int) bool {
			return cmp(i, j) < 0
		})
		deleted := 0
		for i := 1; i < len(hms); i++ {
			if cmp(i, i-1-deleted) == 0 {
				deleted++
				continue
			}
			if deleted != 0 {
				hms[i-deleted] = hms[i]
			}
		}
		return hms[:len(hms)-deleted]
	}
	c.ClientHost = cleanupHost(c.ClientHost)
	c.ServerHost = cleanupHost(c.ServerHost)
	if len(c.ForbiddenFlagValues) >= 2 {
		forbidden := map[struct{ mask, value uint16 }]struct{}{}
		for _, ffv := range c.ForbiddenFlagValues {
			forbidden[struct{ mask, value uint16 }{ffv.Mask, ffv.Value & ffv.Mask}] = struct{}{}
		}
		c.ForbiddenFlagValues = nil
		for k := range forbidden {
			c.ForbiddenFlagValues = append(c.ForbiddenFlagValues, FlagCondition{
				Mask:  k.mask,
				Value: k.value,
			})
		}
		sort.Slice(c.ForbiddenFlagValues, func(i, j int) bool {
			a, b := c.ForbiddenFlagValues[i], c.ForbiddenFlagValues[j]
			if a.Mask == b.Mask {
				return a.Mask < b.Mask
			}
			return a.Value < b.Value
		})
	}
	if len(c.Time) >= 2 {
		times := map[uint8]time.Time{}
		for _, t := range c.Time {
			// p.ts > t1 & p.ts > t2 & t1 > t2
			if old, ok := times[t.Flags]; ok {
				overwrite := false
				switch t.Flags & (TimeRequirementFlagsCompare | TimeRequirementFlagsInvert) {
				case TimeRequirementFlagsCompareGE, TimeRequirementFlagsCompareLE | TimeRequirementFlagsInvert: // > >=
					overwrite = old.Before(t.Time)
				case TimeRequirementFlagsCompareLE, TimeRequirementFlagsCompareGE | TimeRequirementFlagsInvert: // < <=
					overwrite = old.After(t.Time)
				}
				if !overwrite {
					continue
				}
			}
			times[t.Flags] = t.Time
		}

		const (
			ge = TimeRequirementFlagsCompareGE
			le = TimeRequirementFlagsCompareLE
			gt = TimeRequirementFlagsCompareLE | TimeRequirementFlagsInvert
			lt = TimeRequirementFlagsCompareGE | TimeRequirementFlagsInvert

			firstHigher = 0
			equal       = 1
			lastHigher  = 2

			impossible = 0
			dropFirst  = 1
			dropLast   = 2
		)
		foundImpossibleConditions := false
		check := func(firstFlags, lastFlags, cmp, action uint8) {
			for _, relativeFlag := range []byte{0, TimeRequirementFlagsRelative} {
				fullFirstFlags := firstFlags | relativeFlag | TimeRequirementFlagsAppliesToFirst
				first, firstOk := times[fullFirstFlags]
				fullLastlags := lastFlags | relativeFlag | TimeRequirementFlagsAppliesToLast
				last, lastOk := times[fullLastlags]
				if !(firstOk && lastOk) {
					return
				}
				switch cmp {
				case equal:
					if first != last {
						return
					}
				case firstHigher:
					if !(first.After(last)) {
						return
					}
				case lastHigher:
					if !(last.After(first)) {
						return
					}
				}
				switch action {
				case impossible:
					foundImpossibleConditions = true
				case dropFirst:
					delete(times, fullFirstFlags)
				case dropLast:
					delete(times, fullLastlags)
				}
			}
		}
		//first >= 10 and last >= 10 - dropLast
		check(ge, ge, equal, dropLast)
		//first >= 15 and last >=  5 - dropLast
		check(ge, ge, firstHigher, dropLast)
		//first >= 15 and last >   5 - dropLast
		check(ge, gt, firstHigher, dropLast)
		//first >= 15 and last <=  5 - impossible
		check(ge, le, firstHigher, impossible)
		//first >= 10 and last <  10 - impossible
		check(ge, lt, equal, impossible)
		//first >= 15 and last <   5 - impossible
		check(ge, lt, firstHigher, impossible)
		//first >  10 and last >= 10 - dropLast
		check(gt, ge, equal, dropLast)
		//first >  15 and last >=  5 - dropLast
		check(gt, ge, firstHigher, dropLast)
		//first >  10 and last >  10 - dropLast
		check(gt, gt, equal, dropLast)
		//first >  15 and last >   5 - dropLast
		check(gt, gt, firstHigher, dropLast)
		//first >  10 and last <= 10 - impossible
		check(gt, le, equal, impossible)
		//first >  15 and last <=  5 - impossible
		check(gt, le, firstHigher, impossible)
		//first >  10 and last <  10 - impossible
		check(gt, lt, equal, impossible)
		//first >  15 and last <   5 - impossible
		check(gt, lt, firstHigher, impossible)
		//first <= 10 and last <= 10 - dropFirst
		check(le, le, equal, dropFirst)
		//first <= 15 and last <=  5 - dropFirst
		check(le, le, firstHigher, dropFirst)
		//first <= 10 and last <  10 - dropFirst
		check(le, lt, equal, dropFirst)
		//first <= 15 and last <   5 - dropFirst
		check(le, lt, firstHigher, dropFirst)
		//first <  15 and last <=  5 - dropFirst
		check(lt, le, firstHigher, dropFirst)
		//first <  10 and last <  10 - dropFirst
		check(lt, lt, equal, dropFirst)
		//first <  15 and last <   5 - dropFirst
		check(lt, lt, firstHigher, dropFirst)
		if foundImpossibleConditions {
			return Conditions{
				Ids: []uint64{0, 1},
			}
		}
		c.Time = nil
		for f, t := range times {
			if other, ok := times[f^(TimeRequirementFlagsCompare|TimeRequirementFlagsInvert)]; ok {
				// another entry exists that compares the same time in the same direction (< <= or > >=)
				if other.Equal(t) && f&TimeRequirementFlagsInvert == 0 {
					continue
				}
				switch f & (TimeRequirementFlagsCompare | TimeRequirementFlagsInvert) {
				case TimeRequirementFlagsCompareGE, TimeRequirementFlagsCompareLE | TimeRequirementFlagsInvert: // > >=
					if other.After(t) {
						continue
					}
				case TimeRequirementFlagsCompareLE, TimeRequirementFlagsCompareGE | TimeRequirementFlagsInvert: // < <=
					if other.Before(t) {
						continue
					}
				}
			}
			c.Time = append(c.Time, TimeCondition{
				Time:  t,
				Flags: f,
			})
		}
		sort.Slice(c.Time, func(i, j int) bool {
			return c.Time[i].Flags < c.Time[j].Flags
		})
	}
	if len(c.DataConditions) >= 2 {
		new := []DataCondition{}
		for _, cc := range c.DataConditions {
			found := false
			superseedes := -1
			for ncIndex, nc := range new {
				cmp := func(a, b []DataConditionElement) (int, int, int) {
					ls, ll := len(a), len(b)
					if ls > ll {
						ls, ll = ll, ls
					}
					for i := 0; i < ls; i++ {
						if a[i] != b[i] {
							return i, ls - i, ll - ls
						}
					}
					return ls, 0, ll - ls
				}
				cmpRev := func(a, b []DataConditionElement) int {
					l := len(a)
					if lb := len(b); l > lb {
						l = lb
					}
					for i := 0; i < l; i++ {
						if a[len(a)-i-1] != b[len(b)-i-1] {
							return i
						}
					}
					return l
				}
				nEqual, nNotEqual, nLonger := cmp(cc.Sequence, nc.Elements)
				if nLonger == 0 && nNotEqual == 0 {
					// both elements are equal
					if cc.Inverted != nc.Inverted {
						return Conditions{
							Ids: []uint64{0, 1},
						}
					}
					found = true
					break
				}
				if nLonger == 0 {
					continue
				}
				//(content "a" & content "a" > "b")  # len(nc) < len(cc)
				//(content "a" > "b" & content "a")  # len(nc) > len(cc)
				if nNotEqual == 0 && !cc.Inverted && !nc.Inverted {
					if len(nc.Elements) < len(cc.Sequence) {
						superseedes = ncIndex
					}
					found = true
					break
				}
				//(content !"a" & content "a" > !"b")
				if nNotEqual == 0 && (cc.Inverted && len(cc.Sequence) == nEqual+nNotEqual) != (nc.Inverted && len(nc.Elements) == nEqual+nNotEqual) {
					return Conditions{
						Ids: []uint64{0, 1},
					}
				}
				//(content "a" > !"x" & content "a" > "b" > "c" > !"x")  # len(nc) < len(cc)
				//(content "a" > "b" > "c" > !"x" & content "a" > !"x")  # len(nc) > len(cc)
				if nNotEqual >= 1 && cc.Inverted && nc.Inverted && cmpRev(nc.Elements, cc.Sequence) >= 1 {
					if len(nc.Elements) > len(cc.Sequence) {
						superseedes = ncIndex
					}
					found = true
					break
				}
			}
			if !found {
				new = append(new, cc)
			} else if superseedes >= 0 {
				new[superseedes] = cc
			}
		}
		c.DataConditions = new
	}*/
}

func (c *queryCondition) QueryConditions(pc *parserContext) (ConditionsSet, error) {
	switch {
	case c.Negated != nil:
		cond, err := c.Negated.QueryConditions(pc)
		if err != nil {
			return nil, err
		}
		if cond != nil {
			return cond.invert(), nil
		}
	case c.Grouped != nil:
		return c.Grouped.QueryConditions(pc)
	case c.Term != nil:
		return c.Term.QueryConditions(pc)
	case c.SortTerm != nil:
		if pc.sortTerm != nil {
			return nil, errors.New("only one sort `filter` is allowed")
		}
		pc.sortTerm = c.SortTerm
	case c.LimitTerm != nil:
		if pc.limitTerm != nil {
			return nil, errors.New("only one limit `filter` is allowed")
		}
		pc.limitTerm = c.LimitTerm
	default:
		return nil, fmt.Errorf("queryCondition is empty")
	}
	return nil, nil
}

func (c *queryThenCondition) QueryConditions(pc *parserContext) (ConditionsSet, error) {
	conds := ConditionsSet(nil)
	for _, a := range c.Then {
		cond, err := a.QueryConditions(pc)
		if err != nil {
			return nil, err
		}
		if cond != nil {
			conds = conds.then(cond)
		}
	}
	return conds, nil
}

func (c *queryAndCondition) QueryConditions(pc *parserContext) (ConditionsSet, error) {
	conds := ConditionsSet(nil)
	for _, a := range c.And {
		cond, err := a.QueryConditions(pc)
		if err != nil {
			return nil, err
		}
		if cond != nil {
			conds = conds.and(cond)
		}
	}
	return conds, nil
}

func (c *queryOrCondition) QueryConditions(pc *parserContext) (ConditionsSet, error) {
	conds := ConditionsSet(nil)
	for _, o := range c.Or {
		cond, err := o.QueryConditions(pc)
		if err != nil {
			return nil, err
		}
		if cond != nil {
			conds = conds.or(cond)
		}
	}
	return conds, nil
}

func (r *queryRoot) QueryConditions(pc *parserContext) (ConditionsSet, error) {
	if r.Term == nil {
		return nil, nil
	}
	return r.Term.QueryConditions(pc)
}

func (c *Conditions) String() string {
	res := []string{}
	for _, cc := range *c {
		res = append(res, cc.String())
	}
	return fmt.Sprintf("(%s)", strings.Join(res, ") & ("))
}

func (cs *ConditionsSet) ReferencedTags() []string {
	referencedTagsMap := map[string]struct{}{}
	referencedTags := []string(nil)
	for _, c := range *cs {
		for _, cc := range c {
			if tc, ok := cc.(*TagCondition); ok {
				if _, ok := referencedTagsMap[tc.TagName]; !ok {
					referencedTagsMap[tc.TagName] = struct{}{}
					referencedTags = append(referencedTags, tc.TagName)
				}

			}
		}
	}
	sort.Strings(referencedTags)
	return referencedTags
}

func (cs *ConditionsSet) SubQueries() []string {
	subQueryDependencies := func(cc Condition) []string {
		res := []string(nil)
		seen := map[string]struct{}{}
		add := func(s string) {
			if _, ok := seen[s]; !ok {
				seen[s] = struct{}{}
				res = append(res, s)
			}
		}
		switch ccc := cc.(type) {
		case *TagCondition:
			add(ccc.SubQuery)
		case *NumberCondition:
			for _, s := range ccc.Summands {
				add(s.SubQuery)
			}
		case *TimeCondition:
			for _, s := range ccc.Summands {
				add(s.SubQuery)
			}
		case *FlagCondition:
			for _, s := range ccc.SubQueries {
				add(s)
			}
		case *HostCondition:
			for _, s := range ccc.HostConditionSources {
				add(s.SubQuery)
			}
		case *DataCondition:
			for _, e := range ccc.Elements {
				add(e.SubQuery)
				for _, v := range e.Variables {
					add(v.SubQuery)
				}
			}
		case *ImpossibleCondition:
		}
		return res
	}
	var resolve func(string, map[string]struct{}) (uint, []string)
	resolve = func(wantedSubQuery string, forbidden map[string]struct{}) (uint, []string) {
		needed := map[string]struct{}{}
		filters := uint(0)
		for _, c := range *cs {
			for _, cc := range c {
				sqs := subQueryDependencies(cc)
				touchesWanted, touchesForbidden := false, false
				for _, sq := range sqs {
					if sq == wantedSubQuery {
						touchesWanted = true
					} else if _, ok := forbidden[sq]; ok {
						touchesForbidden = true
						break
					}
				}
				if touchesForbidden || !touchesWanted {
					continue
				}
				if len(sqs) == 1 {
					filters++
					continue
				}
				for _, sq := range sqs {
					if sq != wantedSubQuery {
						needed[sq] = struct{}{}
					}
				}
			}
		}
		if len(needed) == 0 {
			return filters, []string{wantedSubQuery}
		}
		bestOrder := []string(nil)
		bestFilters := uint(0)
		for sq := range needed {
			newForbidden := map[string]struct{}{}
			for f := range forbidden {
				newForbidden[f] = struct{}{}
			}
			newForbidden[wantedSubQuery] = struct{}{}
			curFilters, resolutionOrder := resolve(sq, newForbidden)
			if bestFilters > curFilters {
				continue
			}
			bestFilters = curFilters
			bestOrder = resolutionOrder
		}
		return bestFilters + filters, append(bestOrder, wantedSubQuery)
	}
	_, res := resolve("", nil)
	return res
}

func (cs *ConditionsSet) HasRelativeTimes() bool {
	for _, ccs := range *cs {
		for _, cc := range ccs {
			c, ok := cc.(*TimeCondition)
			if !ok {
				continue
			}
			nowFactors := c.ReferenceTimeFactor
			for _, s := range c.Summands {
				nowFactors += s.FTimeFactor
				nowFactors += s.LTimeFactor
			}
			if nowFactors < 0 {
				nowFactors = -nowFactors
			}
			if nowFactors%2 == 1 {
				return true
			}
		}
	}
	return false
}

func (cs *ConditionsSet) UpdateReferenceTime(oldReferenceTime, newReferenceTime time.Time) {
	delta := oldReferenceTime.Sub(newReferenceTime)
	if delta == 0 {
		return
	}
	for i := range *cs {
		ccs := &(*cs)[i]
		for j := range *ccs {
			c, ok := (*ccs)[j].(*TimeCondition)
			if ok && c.ReferenceTimeFactor != 0 {
				c.Duration += delta * time.Duration(c.ReferenceTimeFactor)
			}
		}
	}
}
