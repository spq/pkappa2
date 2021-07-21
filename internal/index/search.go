package index

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	"github.com/spq/pkappa2/internal/query"
	"github.com/spq/pkappa2/internal/seekbufio"
	"rsc.io/binaryregexp"
)

type (
	subQueryRange struct {
		indexMin, indexMax int
	}
	subQueryRanges struct {
		ranges []subQueryRange
	}

	subQuerySelection struct {
		remaining []map[string]subQueryRanges
	}
)

func (sqr subQueryRanges) split(other *subQueryRanges) (overlap subQueryRanges, rest subQueryRanges) {
	bi := 0
	for _, a := range sqr.ranges {
		for {
			if bi >= len(other.ranges) {
				rest.addRange(a.indexMin, a.indexMax)
				break
			}
			b := other.ranges[bi]
			if a.indexMax < b.indexMin {
				rest.addRange(a.indexMin, a.indexMax)
				break
			}
			if b.indexMax < a.indexMin {
				bi++
				continue
			}
			if a.indexMin < b.indexMin {
				rest.addRange(a.indexMin, b.indexMin-1)
				a.indexMin = b.indexMin
			}
			if a.indexMax <= b.indexMax {
				overlap.addRange(a.indexMin, a.indexMax)
				break
			}
			overlap.addRange(a.indexMin, b.indexMax)
			a.indexMin = b.indexMax + 1
			bi++
		}
	}
	return
}

func (sqr *subQueryRanges) empty() bool {
	return len(sqr.ranges) == 0
}

func (sqs *subQuerySelection) remove(subqueries []string, forbidden []*subQueryRanges) {
	oldRemaining := sqs.remaining
	sqs.remaining = []map[string]subQueryRanges(nil)
outer:
	for _, remaining := range oldRemaining {
		for sqi, sq := range subqueries {
			remove, keep := remaining[sq].split(forbidden[sqi])
			if remove.empty() {
				sqs.remaining = append(sqs.remaining, remaining)
				continue outer
			}
			if keep.empty() {
				continue
			}
			remaining[sq] = remove
			sqs.remaining = append(sqs.remaining, map[string]subQueryRanges{
				sq: keep,
			})
			new := &sqs.remaining[len(sqs.remaining)-1]
			for k, v := range remaining {
				if k != sq {
					(*new)[k] = subQueryRanges{
						ranges: append([]subQueryRange(nil), v.ranges...),
					}
				}
			}
		}
	}
}

func (sqs *subQuerySelection) empty() bool {
	return len(sqs.remaining) == 0
}

func (sqrs *subQueryRanges) add(id int) {
	sqrs.addRange(id, id)
}

func (sqrs *subQueryRanges) addRange(low, high int) {
	if l := len(sqrs.ranges); l != 0 {
		indexMax := &sqrs.ranges[l-1].indexMax
		if *indexMax == low-1 {
			*indexMax = high
			return
		}
		if low <= *indexMax {
			// unsorted insert, search for the right place
			for i := range sqrs.ranges {
				r := &sqrs.ranges[i]
				if low >= r.indexMin && high <= r.indexMax {
					return
				}
				if r.indexMax+1 < low {
					continue
				}
				if high+1 < r.indexMin {
					sqrs.ranges = append(sqrs.ranges[:i], append([]subQueryRange{{low, high}}, sqrs.ranges[i:]...)...)
					return
				}
				if low <= r.indexMin {
					r.indexMin = low
				}
				if high <= r.indexMax {
					return
				}
				r.indexMax = high
				for i+1 < len(sqrs.ranges) {
					r2 := &sqrs.ranges[i+1]
					if r.indexMax+1 < r2.indexMin {
						break
					}
					if r.indexMax < r2.indexMax {
						r.indexMax = r2.indexMax
					}
					sqrs.ranges = append(sqrs.ranges[:i+1], sqrs.ranges[i+2:]...)
				}
				return
			}
		}
	}
	sqrs.ranges = append(sqrs.ranges, subQueryRange{low, high})
}

func (r *Reader) buildSearchObjects(subQuery string, previousResults map[string][]*Stream, refTime time.Time, q *query.Conditions, superseedingIndexes []*Reader, tags map[string][]uint64) (bool, []func(*subQuerySelection, *stream) (bool, error), []func() ([]uint32, error), error) {
	filters := []func(*subQuerySelection, *stream) (bool, error)(nil)
	lookups := []func() ([]uint32, error)(nil)

	// filter out streams superseeded by newer indexes
	if len(superseedingIndexes) != 0 {
		filters = append(filters, func(_ *subQuerySelection, s *stream) (bool, error) {
			for _, r2 := range superseedingIndexes {
				if _, ok := r2.containedStreamIds[s.StreamID]; ok {
					return false, nil
				}
			}
			return true, nil
		})
	}

	minIDFilter, maxIDFilter := uint64(0), uint64(math.MaxUint64)
	hostConditionBitmaps := [][]uint64(nil)
	type occ struct {
		condition, element int
	}
	type regex struct {
		regex     *binaryregexp.Regexp
		occurence []occ
	}
	regexes := []regex(nil)
conditions:
	for cIdx, c := range *q {
		c := c
		switch cc := c.(type) {
		case *query.TagCondition:
			if cc.SubQuery != subQuery {
				continue
			}
			bv := tags[cc.TagName]
			inv := cc.Invert
			filters = append(filters, func(_ *subQuerySelection, s *stream) (bool, error) {
				idx := int(s.StreamID / 64)
				bit := s.StreamID % 64
				res := idx < len(bv) && ((bv[idx]>>bit)&1) != 0
				return res != inv, nil
			})
		case *query.FlagCondition:
			shouldEvaluate := false
			for _, sq := range cc.SubQueries {
				if sq == subQuery {
					shouldEvaluate = true
				} else if _, ok := previousResults[sq]; !ok {
					shouldEvaluate = false
					break
				}
			}
			if !shouldEvaluate {
				continue
			}
			if len(cc.SubQueries) == 1 {
				filters = append(filters, func(_ *subQuerySelection, s *stream) (bool, error) {
					return s.Flags&cc.Mask != cc.Value, nil
				})
				continue
			}
			flagValues := map[uint16][]*subQueryRanges{
				cc.Value & cc.Mask: nil,
			}
			subqueries := []string(nil)
			for _, sq := range cc.SubQueries {
				if sq == subQuery {
					continue
				}
				subqueries = append(subqueries, sq)
				curFlagValues := map[uint16]*subQueryRanges{}
				for pos, res := range previousResults[sq] {
					f := res.Flags & cc.Mask
					cfv := curFlagValues[f]
					if cfv == nil {
						cfv = &subQueryRanges{}
						curFlagValues[f] = cfv
					}
					cfv.add(pos)
				}
				tmp := flagValues
				flagValues = make(map[uint16][]*subQueryRanges)
				for f1, d1 := range tmp {
					for f2, d2 := range curFlagValues {
						d := append(append([]*subQueryRanges(nil), d1...), d2)
						flagValues[f1^f2] = d
					}
				}
			}
			filters = append(filters, func(sqs *subQuerySelection, s *stream) (bool, error) {
				forbidden, possible := flagValues[s.Flags&cc.Mask]
				if !possible {
					// no combination of sub queries produces the forbidden result
					return true, nil
				}
				if len(flagValues) == 1 {
					// the only combination of sub queries produces the forbidden result
					return false, nil
				}
				sqs.remove(subqueries, forbidden)
				return !sqs.empty(), nil
			})
		case *query.HostCondition:
			hcsc, hcss := false, false
			usedType := map[query.HostConditionSourceType]*bool{
				query.HostConditionSourceTypeClient: &hcsc,
				query.HostConditionSourceTypeServer: &hcss,
			}
			subQueryAffected := false
			for _, hcs := range cc.HostConditionSources {
				if hcs.SubQuery == subQuery {
					u := usedType[hcs.Type]
					*u = !*u
				} else if _, ok := previousResults[hcs.SubQuery]; ok {
					subQueryAffected = true
				} else {
					hcsc = false
					hcss = false
					break
				}
			}
			if !(hcsc || hcss) {
				continue
			}
			if subQueryAffected {
				if len(cc.Host) != 0 || len(cc.HostConditionSources) != 2 || (hcsc && hcss) {
					return false, nil, nil, errors.New("complex host condition not supported")
				}
				otherSubQuery := ""
				myHcss := hcss
				hcsc, hcss = false, false
				for _, hcs := range cc.HostConditionSources {
					if hcs.SubQuery != subQuery {
						otherSubQuery = hcs.SubQuery
						*usedType[hcs.Type] = true
						break
					}
				}
				otherHcss := hcss
				relevantResults := previousResults[otherSubQuery]
				if cc.Mask4.IsUnspecified() && cc.Mask6.IsUnspecified() {
					// only check if the ip version is the same, can be done on a hg level
					otherHosts := [2]subQueryRanges{}
					for rIdx, r := range relevantResults {
						otherSize := r.r.hostGroups[r.HostGroup].hostSize
						otherHosts[otherSize/16].add(rIdx)
					}
					if !cc.Invert {
						otherHosts[0], otherHosts[1] = otherHosts[1], otherHosts[0]
					}
					forbiddenSubQueryResultsPerHostGroup := make([]*subQueryRanges, len(r.hostGroups))
					for hgi, hg := range r.hostGroups {
						forbiddenSubQueryResultsPerHostGroup[hgi] = &otherHosts[hg.hostSize/16]
					}
					filters = append(filters, func(sqs *subQuerySelection, s *stream) (bool, error) {
						f := forbiddenSubQueryResultsPerHostGroup[s.HostGroup]
						sqs.remove([]string{otherSubQuery}, []*subQueryRanges{f})
						return !sqs.empty(), nil
					})
					continue
				}
				filters = append(filters, func(sqs *subQuerySelection, s *stream) (bool, error) {
					myHG := &r.hostGroups[s.HostGroup]
					myHid := s.ClientHost
					if myHcss {
						myHid = s.ServerHost
					}
					myH := myHG.get(myHid)
					mask := cc.Mask4
					if myHG.hostSize == 16 {
						mask = cc.Mask6
					}

					forbidden := subQueryRanges{}
				outer:
					for resIdx, res := range relevantResults {
						otherHG := &res.r.hostGroups[res.HostGroup]
						if myHG.hostSize != otherHG.hostSize {
							if !cc.Invert {
								forbidden.add(resIdx)
							}
							continue
						}
						otherHid := res.ClientHost
						if otherHcss {
							otherHid = res.ServerHost
						}
						otherH := otherHG.get(otherHid)
						for i := range myH {
							if (myH[i]^otherH[i])&mask[i] == 0 {
								continue
							}
							if !cc.Invert {
								forbidden.add(resIdx)
								continue outer
							}
						}
						if cc.Invert {
							forbidden.add(resIdx)
						}
					}
					sqs.remove([]string{otherSubQuery}, []*subQueryRanges{&forbidden})
					return !sqs.empty(), nil
				})
				continue
			}
			for hgi, hg := range r.hostGroups {
				if len(hostConditionBitmaps) <= hgi {
					hostConditionBitmaps = append(hostConditionBitmaps, make([]uint64, (hg.hostCount*hg.hostCount+63)/64))
				}
				if len(cc.Host) != hg.hostSize && len(cc.Host) != 0 {
					if !cc.Invert {
						hostConditionBitmaps[hgi] = nil
					}
					continue
				}
				bitmap := hostConditionBitmaps[hgi]
				m := cc.Mask4
				if hg.hostSize == 16 {
					m = cc.Mask6
				}
				i := 0
				for server := 0; server < hg.hostCount; server++ {
					for client := 0; client < hg.hostCount; client++ {
						h := make([]byte, hg.hostSize)
						if len(cc.Host) != 0 {
							copy(h, cc.Host)
						}
						if hcsc {
							ch := hg.get(uint16(client))
							for i := range h {
								h[i] ^= ch[i]
							}
						}
						if hcss {
							sh := hg.get(uint16(server))
							for i := range h {
								h[i] ^= sh[i]
							}
						}
						f := false
						for i := range h {
							f = h[i]&m[i] != 0
							if f {
								break
							}
						}
						f = f != cc.Invert
						if f {
							bitmap[i/64] |= 1 << (i % 64)
						}
						i++
					}
				}
				succeed := false
				for i != 0 {
					i--
					succeed = (bitmap[i/64]>>(i%64))&1 == 0
					if succeed {
						break
					}
				}
				if !succeed {
					bitmap = nil
				}
				hostConditionBitmaps[hgi] = bitmap
			}
		case *query.NumberCondition:
			if len(cc.Summands) == 1 && cc.Summands[0].SubQuery == subQuery && cc.Summands[0].Type == query.NumberConditionSummandTypeID {
				switch cc.Summands[0].Factor {
				case +1:
					// id >= -N
					if minIDFilter < uint64(-cc.Number) {
						minIDFilter = uint64(-cc.Number)
					}
				case -1:
					// id <= N
					if maxIDFilter > uint64(cc.Number) {
						maxIDFilter = uint64(cc.Number)
					}
				}
			}
			type factor struct {
				id, clientBytes, serverBytes, clientPort, serverPort int
			}
			factors := map[string]factor{}
			for _, sum := range cc.Summands {
				if _, ok := previousResults[sum.SubQuery]; sum.SubQuery != subQuery && !ok {
					continue conditions
				}
				f := factors[sum.SubQuery]
				switch sum.Type {
				case query.NumberConditionSummandTypeID:
					f.id += sum.Factor
				case query.NumberConditionSummandTypeClientBytes:
					f.clientBytes += sum.Factor
				case query.NumberConditionSummandTypeServerBytes:
					f.serverBytes += sum.Factor
				case query.NumberConditionSummandTypeClientPort:
					f.clientPort += sum.Factor
				case query.NumberConditionSummandTypeServerPort:
					f.serverPort += sum.Factor
				}
				if f.clientBytes == 0 && f.clientPort == 0 && f.id == 0 && f.serverBytes == 0 && f.serverPort == 0 {
					delete(factors, sum.SubQuery)
				} else {
					factors[sum.SubQuery] = f
				}
			}
			if _, ok := factors[subQuery]; !ok {
				continue
			}
			myFactors := factors[subQuery]
			delete(factors, subQuery)
			if len(factors) == 0 {
				filters = append(filters, func(_ *subQuerySelection, s *stream) (bool, error) {
					n := cc.Number
					n += myFactors.id * int(s.StreamID)
					n += myFactors.clientBytes * int(s.ClientBytes)
					n += myFactors.serverBytes * int(s.ServerBytes)
					n += myFactors.clientPort * int(s.ClientPort)
					n += myFactors.serverPort * int(s.ServerPort)
					return n >= 0, nil
				})
				continue
			}
			type (
				subQueryResult struct {
					number int
					ranges subQueryRanges
				}
			)
			subQueryData := [][]subQueryResult(nil)
			subQueries := []string(nil)
			minSum, maxSum := 0, 0
			for sq, f := range factors {
				numbers := map[int]int{}
				results := []subQueryResult(nil)
				for resId, res := range previousResults[sq] {
					n := 0
					n += f.id * int(res.StreamID)
					n += f.clientBytes * int(res.ClientBytes)
					n += f.serverBytes * int(res.ServerBytes)
					n += f.clientPort * int(res.ClientPort)
					n += f.serverPort * int(res.ServerPort)
					if pos, ok := numbers[n]; ok {
						results[pos].ranges.add(resId)
						continue
					}
					numbers[n] = len(results)
					results = append(results, subQueryResult{
						number: n,
						ranges: subQueryRanges{
							ranges: []subQueryRange{{resId, resId}},
						},
					})
				}
				subQueries = append(subQueries, sq)
				sort.Slice(results, func(i, j int) bool {
					return results[i].number < results[j].number
				})
				subQueryData = append(subQueryData, results)
				minSum += results[0].number
				maxSum += results[len(results)-1].number
			}
			filters = append(filters, func(sqs *subQuerySelection, s *stream) (bool, error) {
				n := cc.Number
				n += myFactors.id * int(s.StreamID)
				n += myFactors.clientBytes * int(s.ClientBytes)
				n += myFactors.serverBytes * int(s.ServerBytes)
				n += myFactors.clientPort * int(s.ClientPort)
				n += myFactors.serverPort * int(s.ServerPort)
				if n+minSum >= 0 {
					return true, nil
				}
				if n+maxSum < 0 {
					return false, nil
				}
				// minSum < -n <= maxSum
				pos := make([]int, len(subQueries))
			outer:
				for {
					// calculate the sum
					sqN := n
					for i, j := range pos {
						sqN += subQueryData[i][j].number
					}
					// remove from sqs if not a valid sum
					if sqN < 0 {
						forbidden := []*subQueryRanges(nil)
						for i, j := range pos {
							forbidden = append(forbidden, &subQueryData[i][j].ranges)
						}
						sqs.remove(subQueries, forbidden)
					}
					// go to next combination
					for i := range pos {
						p := &pos[i]
						(*p)++
						if *p < len(subQueryData[i]) {
							continue outer
						}
						*p = 0
					}
					break
				}
				return !sqs.empty(), nil
			})
		case *query.TimeCondition:
			type factor struct {
				ftime, ltime int
			}
			factors := map[string]factor{}
			for _, s := range cc.Summands {
				if _, ok := previousResults[s.SubQuery]; s.SubQuery != subQuery && !ok {
					continue conditions
				}
				factors[s.SubQuery] = factor{
					ftime: s.FTimeFactor,
					ltime: s.LTimeFactor,
				}
			}
			if _, ok := factors[subQuery]; !ok {
				continue
			}
			myFactors := factors[subQuery]
			delete(factors, subQuery)
			startD := cc.Duration + time.Duration(myFactors.ftime+myFactors.ltime)*r.referenceTime.Sub(refTime)
			if len(factors) == 0 {
				filter := func(_ *subQuerySelection, s *stream) (bool, error) {
					d := startD
					d += time.Duration(myFactors.ftime) * time.Duration(s.FirstPacketTimeNS)
					d += time.Duration(myFactors.ltime) * time.Duration(s.LastPacketTimeNS)
					return d >= 0, nil
				}
				if myFactors.ftime == 0 || myFactors.ltime == 0 {
					matchesOnEarlyPacket, _ := filter(nil, &stream{
						FirstPacketTimeNS: r.firstPacketTimeNS.min,
						LastPacketTimeNS:  r.lastPacketTimeNS.min,
					})
					matchesOnLatePacket, _ := filter(nil, &stream{
						FirstPacketTimeNS: r.firstPacketTimeNS.max,
						LastPacketTimeNS:  r.lastPacketTimeNS.max,
					})
					if matchesOnEarlyPacket != matchesOnLatePacket {
						filters = append(filters, filter)
					} else if !matchesOnEarlyPacket {
						return false, nil, nil, nil
					}
				} else {
					filters = append(filters, filter)
				}
				continue
			}
			type (
				subQueryResult struct {
					duration time.Duration
					ranges   subQueryRanges
				}
			)
			subQueryData := [][]subQueryResult(nil)
			subQueries := []string(nil)
			minSum, maxSum := time.Duration(0), time.Duration(0)
			for sq, f := range factors {
				durations := map[time.Duration]int{}
				results := []subQueryResult(nil)
				for resId, res := range previousResults[sq] {
					d := time.Duration(f.ftime+f.ltime) * res.r.referenceTime.Sub(refTime)
					d += time.Duration(f.ftime) * time.Duration(res.FirstPacketTimeNS)
					d += time.Duration(f.ltime) * time.Duration(res.LastPacketTimeNS)
					if pos, ok := durations[d]; ok {
						results[pos].ranges.add(resId)
						continue
					}
					durations[d] = len(results)
					results = append(results, subQueryResult{
						duration: d,
						ranges: subQueryRanges{
							ranges: []subQueryRange{{resId, resId}},
						},
					})
				}
				subQueries = append(subQueries, sq)
				sort.Slice(results, func(i, j int) bool {
					return results[i].duration < results[j].duration
				})
				subQueryData = append(subQueryData, results)
				minSum += results[0].duration
				maxSum += results[len(results)-1].duration
			}
			filters = append(filters, func(sqs *subQuerySelection, s *stream) (bool, error) {
				d := startD
				d += time.Duration(myFactors.ftime) * time.Duration(s.FirstPacketTimeNS)
				d += time.Duration(myFactors.ltime) * time.Duration(s.LastPacketTimeNS)
				if d+minSum >= 0 {
					return true, nil
				}
				if d+maxSum < 0 {
					return false, nil
				}
				// minSum < -n <= maxSum
				pos := make([]int, len(subQueries))
			outer:
				for {
					// calculate the sum
					sqD := d
					for i, j := range pos {
						sqD += subQueryData[i][j].duration
					}
					// remove from sqs if not a valid sum
					if sqD < 0 {
						forbidden := []*subQueryRanges(nil)
						for i, j := range pos {
							forbidden = append(forbidden, &subQueryData[i][j].ranges)
						}
						sqs.remove(subQueries, forbidden)
					}
					// go to next combination
					for i := range pos {
						p := &pos[i]
						(*p)++
						if *p < len(subQueryData[i]) {
							continue outer
						}
						*p = 0
					}
					break
				}
				return !sqs.empty(), nil
			})
		case *query.DataCondition:
			shouldEvaluate, affectsSubquery := false, false
			for _, e := range cc.Elements {
				if e.SubQuery == subQuery {
					for _, v := range e.Variables {
						if _, ok := previousResults[v.SubQuery]; v.SubQuery != subQuery && !ok {
							return false, nil, nil, errors.New("SubQueries not yet fully supported")
						}
					}
					shouldEvaluate = true
				} else if _, ok := previousResults[e.SubQuery]; !ok {
					continue conditions
				} else {
					affectsSubquery = true
				}
			}
			if !shouldEvaluate {
				continue
			}
			if affectsSubquery {
				return false, nil, nil, errors.New("SubQueries not yet fully supported")
			}
			for eIdx, e := range cc.Elements {
				found := (*regex)(nil)
			regexes:
				for rIdx := range regexes {
					r := &regexes[rIdx]
					o := r.occurence[0]
					oe := (*q)[o.condition].(*query.DataCondition).Elements[o.element]
					if e.Regex != oe.Regex {
						continue
					}
					if len(e.Variables) != len(oe.Variables) {
						continue
					}
					for i := range e.Variables {
						if e.Variables[i] != oe.Variables[i] {
							continue regexes
						}
					}
					found = r
					break
				}
				if found == nil {
					var compiled *binaryregexp.Regexp
					if len(e.Variables) == 0 {
						var err error
						compiled, err = binaryregexp.Compile(e.Regex)
						if err != nil {
							return false, nil, nil, err
						}
					}
					regexes = append(regexes, regex{
						regex: compiled,
					})
					found = &regexes[len(regexes)-1]
				}
				found.occurence = append(found.occurence, occ{
					condition: cIdx,
					element:   eIdx,
				})
			}
		}
	}
	if minIDFilter == maxIDFilter {
		idx, ok := r.containedStreamIds[minIDFilter]
		if !ok {
			return false, nil, nil, nil
		}
		lookups = append(lookups, func() ([]uint32, error) {
			return []uint32{idx}, nil
		})
	}
	if hostConditionBitmaps != nil {
		someFail, someSucceed := false, false
	outer:
		for _, bm := range hostConditionBitmaps {
			if bm == nil {
				someFail = true
				continue
			}
			someSucceed = true
			if someFail {
				break
			}
			for _, n := range bm {
				if n != 0 {
					someFail = true
					break outer
				}
			}
		}
		if !someSucceed {
			return false, nil, nil, nil
		}
		if someFail {
			filters = append(filters, func(_ *subQuerySelection, s *stream) (bool, error) {
				hg := hostConditionBitmaps[s.HostGroup]
				if len(hg) == 0 {
					return hg != nil, nil
				}
				count := r.hostGroups[s.HostGroup].hostCount
				bit := int(s.ClientHost) + int(s.ServerHost)*count
				fail := (hg[bit/64]>>(bit%64))&1 != 0
				return !fail, nil
			})
		}
	}
	if regexes != nil {
		//sort the regexes
		for rIdx := range regexes {
			r := &regexes[rIdx]
			sort.Slice(r.occurence, func(i, j int) bool {
				if r.occurence[i].element != r.occurence[j].element {
					return r.occurence[i].element < r.occurence[j].element
				}
				return r.occurence[i].condition < r.occurence[j].condition
			})
		}
		sort.Slice(regexes, func(i, j int) bool {
			if regexes[i].occurence[0].element != regexes[j].occurence[0].element {
				return regexes[i].occurence[0].element < regexes[j].occurence[0].element
			}
			return regexes[i].occurence[0].condition < regexes[j].occurence[0].condition
		})

		//add filter for scanning the data section
		sr := io.NewSectionReader(r.file, int64(r.header.Sections[sectionData].Begin), r.header.Sections[sectionData].size())
		br := seekbufio.NewSeekableBufferReader(sr)
		filters = append(filters, func(sqs *subQuerySelection, s *stream) (bool, error) {
			if _, err := br.Seek(int64(s.DataStart), io.SeekStart); err != nil {
				return false, err
			}
			type progressData struct {
				streamOffset [2]int
				// how many regexes were sucessful
				nSuccessful int
				// the variables collected on the way
				variables map[string]string
			}
			progress := make([]progressData, len(*q))
			buffers := [2][]byte{}
			bufferOffsets := [2]int{}
			h := dataHeader{}
			for {
				if err := binary.Read(br, binary.LittleEndian, &h); err != nil {
					return false, err
				}
				if h.Flags&flagsDataHasNext == 0 && h.Length == 0 {
					// fake data block for data-less streams
					break
				}
				direction := uint8((h.Flags & flagsDataDirection) / flagsDataDirection)
				type regexOcc struct {
					regex, occ int
				}

				allFinished := h.Flags&flagsDataHasNext == 0
				for alreadyRead := false; ; {
					interested := []regexOcc{}
					wrongDirection, needFullData := false, false
					for rIdx := range regexes {
						r := &regexes[rIdx]
						for oIdx, o := range r.occurence {
							p := &progress[o.condition]
							if o.element != p.nSuccessful {
								continue
							}
							e := (*q)[o.condition].(*query.DataCondition).Elements[o.element]
							f := e.Flags
							wantDirection := (f & query.DataRequirementSequenceFlagsDirection) / query.DataRequirementSequenceFlagsDirection
							// xor with the c2s value for both flag sources to make sure the same number has the same meaning
							wantDirection ^= query.DataRequirementSequenceFlagsDirectionClientToServer / query.DataRequirementSequenceFlagsDirection
							wantDirection ^= flagsDataDirectionClientToServer / flagsDataDirection
							if direction != wantDirection {
								wrongDirection = true
								continue
							}
							if r.regex == nil {
								expr := e.Regex
								for i := len(e.Variables) - 1; i >= 0; i-- {
									v := e.Variables[i]
									if v.SubQuery != subQuery {
										//TODO: maybe wrap this whole code in a function that is repeatedly called for all values from the previous results
										return false, errors.New("SubQueries not yet fully supported")
									}
									content, ok := p.variables[v.Name]
									if !ok {
										return false, fmt.Errorf("variable %q not defined", v.Name)
									}
									expr = fmt.Sprintf("%s(?:%s)%s", expr[:v.Position], binaryregexp.QuoteMeta(content), expr[v.Position:])
								}
								compiled, err := binaryregexp.Compile(expr)
								if err != nil {
									return false, err
								}
								r.regex = compiled
							}
							if p, _ := r.regex.LiteralPrefix(); p == "" {
								// regexes with no literal prefix should only be checked when the full data is read, they often perform bad otherwise
								gotAvailable := bufferOffsets[direction] + len(buffers[direction]) + int(h.Length)
								wantAvaliable := uint64(0)
								switch direction {
								case flagsDataDirectionClientToServer / flagsDataDirection:
									wantAvaliable = s.ClientBytes
								case flagsDataDirectionServerToClient / flagsDataDirection:
									wantAvaliable = s.ServerBytes
								}
								if uint64(gotAvailable) != wantAvaliable {
									needFullData = true
									continue
								}
							}
							// we got new data that this regex is interested in
							interested = append(interested, regexOcc{
								regex: rIdx,
								occ:   oIdx,
							})
						}
					}
					if !wrongDirection {
						// all active regex'es are interested in the current direction.
						// we can release the buffer of the other direction.
						bufferOffsets[1-direction] += len(buffers[1-direction])
						buffers[1-direction] = nil
					}
					if len(interested) == 0 && !needFullData {
						if !wrongDirection {
							// no-one is interested in more data
							allFinished = true
						} else if !alreadyRead {
							// no-one was interested in this block of data
							if _, err := br.Seek(int64(h.Length), io.SeekCurrent); err != nil {
								return false, err
							}
							bufferOffsets[direction] += len(buffers[1-direction]) + int(h.Length)
							buffers[direction] = nil
						}
						break
					}

					// at least one regex is interested in the data, read it
					if !alreadyRead {
						alreadyRead = true
						// read the new data and append it to the buffer
						newbuf := make([]byte, len(buffers[direction])+int(h.Length))
						copy(newbuf, buffers[direction])
						if err := binary.Read(br, binary.LittleEndian, newbuf[len(buffers[direction]):]); err != nil {
							return false, err
						}
						buffers[direction] = newbuf
					}

					if len(interested) == 0 {
						break
					}

					recheckRegexes := false
					minStreamPos := bufferOffsets[direction]
					if !needFullData {
						minStreamPos += len(buffers[direction])
					}
					for _, i := range interested {
						r := &regexes[i.regex]
						o := &r.occurence[i.occ]
						p := &progress[o.condition]
						d := (*q)[o.condition].(*query.DataCondition)
						prefix, _ := r.regex.LiteralPrefix()

						if prefix != "" {
							//the regex has a prefix, find it
							buffer := buffers[direction][p.streamOffset[direction]-bufferOffsets[direction]:]
							pos := bytes.Index(buffer, []byte(prefix))
							if pos < 0 {
								// the prefix is not in the string, we can discard part of the buffer
								keepLength := len(prefix) - 1
								if keepLength > len(buffer) {
									keepLength = len(buffer)
								}
								for ; keepLength != 0 && !bytes.HasSuffix(buffer, []byte(prefix[len(prefix)-keepLength:])); keepLength-- {
								}

								p.streamOffset[direction] = bufferOffsets[direction] + len(buffers[direction]) - keepLength
								if minStreamPos > p.streamOffset[direction] {
									minStreamPos = p.streamOffset[direction]
								}
								continue
							}
							//skip the part that doesn't have the prefix
							p.streamOffset[direction] += pos
						}
						if prefix != "" || h.Flags&flagsDataHasNext == 0 {
							data := buffers[direction][p.streamOffset[direction]-bufferOffsets[direction]:]
							res := r.regex.FindSubmatchIndex(data)
							if res != nil {
								//fmt.Printf("matched regex %q on stream %d for dir %d at offset %d-%d (int: %d in %+v)\n", r.regex, s.StreamID, direction, p.streamOffset[direction], bufferOffsets[direction], interestedIndex, interested)
								p.nSuccessful++
								if p.nSuccessful != len(d.Elements) {
									// remember that we advanced a sequence that has a follow up and we have to re-check the regexes
									recheckRegexes = true
								} else if d.Inverted {
									return false, nil
								}
								variableNames := r.regex.SubexpNames()
								for i := 2; i < len(res); i += 2 {
									varContent := string(data[res[i]:res[i+1]])
									varName := variableNames[i/2]
									if varName == "" {
										continue
									}
									if _, ok := p.variables[varName]; ok {
										return false, fmt.Errorf("variable %q already seen", varName)
									}
									if p.variables == nil {
										p.variables = make(map[string]string)
									}
									p.variables[varName] = varContent
								}

								// update stream offsets: a follow up regex for the same direction
								// may consume the byte following the match, a regex for the other
								// direction may start reading from the next received packet,
								// so everything read so far is out-of reach.
								p.streamOffset[direction] += res[1]
								//TODO: when we captured the whole stream, this makes following matches for the other direction always fail
								p.streamOffset[1-direction] = bufferOffsets[1-direction] + len(buffers[1-direction])
							}
						}
						if minStreamPos > p.streamOffset[direction] {
							minStreamPos = p.streamOffset[direction]
						}
					}
					buffers[direction] = buffers[direction][minStreamPos-bufferOffsets[direction]:]
					bufferOffsets[direction] = minStreamPos
					if !recheckRegexes {
						break
					}
				}
				if allFinished {
					break
				}
			}

			// check if any of the regexe's failed
			for pIdx, p := range progress {
				d, ok := (*q)[pIdx].(*query.DataCondition)
				if !ok {
					continue
				}
				nUnsuccessful := len(d.Elements) - p.nSuccessful
				if nUnsuccessful >= 2 || (nUnsuccessful != 0) != d.Inverted {
					return false, nil
				}
			}
			return true, nil
		})
	}
	return true, filters, lookups, nil
}

func SearchStreams(indexes []*Reader, refTime time.Time, qs query.ConditionsSet, sorting []query.Sorting, limit, skip uint, tags map[string][]uint64) ([]*Stream, bool, error) {
	if len(qs) == 0 {
		return nil, false, nil
	}
	sorters := []func(a, b *Stream) bool{}
	sorterFunctions := map[query.SortingDir]map[query.SortingKey]func(a, b *Stream) bool{
		query.SortingDirAscending: map[query.SortingKey]func(a *Stream, b *Stream) bool{
			query.SortingKeyID: func(a, b *Stream) bool {
				return a.stream.StreamID < b.stream.StreamID
			},
			query.SortingKeyClientBytes: func(a, b *Stream) bool {
				return a.stream.ClientBytes < b.stream.ClientBytes
			},
			query.SortingKeyServerBytes: func(a, b *Stream) bool {
				return a.stream.ServerBytes < b.stream.ServerBytes
			},
			query.SortingKeyFirstPacketTime: func(a, b *Stream) bool {
				if a.r == b.r {
					return a.stream.FirstPacketTimeNS < b.stream.FirstPacketTimeNS
				}
				at := a.r.referenceTime.Add(time.Nanosecond * time.Duration(a.stream.FirstPacketTimeNS))
				bt := b.r.referenceTime.Add(time.Nanosecond * time.Duration(b.stream.FirstPacketTimeNS))
				return at.Before(bt)
			},
			query.SortingKeyLastPacketTime: func(a, b *Stream) bool {
				if a.r == b.r {
					return a.stream.LastPacketTimeNS < b.stream.LastPacketTimeNS
				}
				at := a.r.referenceTime.Add(time.Nanosecond * time.Duration(a.stream.LastPacketTimeNS))
				bt := b.r.referenceTime.Add(time.Nanosecond * time.Duration(b.stream.LastPacketTimeNS))
				return at.Before(bt)
			},
			query.SortingKeyClientHost: func(a, b *Stream) bool {
				if a.stream.ClientHost == b.stream.ClientHost && a.r == b.r && a.stream.HostGroup == b.stream.HostGroup {
					return false
				}
				ah := a.r.hostGroups[a.stream.HostGroup].get(a.stream.ClientHost)
				bh := b.r.hostGroups[b.stream.HostGroup].get(b.stream.ClientHost)
				cmp := bytes.Compare(ah, bh)
				return cmp < 0
			},
			query.SortingKeyServerHost: func(a, b *Stream) bool {
				if a.stream.ServerHost == b.stream.ServerHost && a.r == b.r && a.stream.HostGroup == b.stream.HostGroup {
					return false
				}
				ah := a.r.hostGroups[a.stream.HostGroup].get(a.stream.ServerHost)
				bh := b.r.hostGroups[b.stream.HostGroup].get(b.stream.ServerHost)
				cmp := bytes.Compare(ah, bh)
				return cmp < 0
			},
			query.SortingKeyClientPort: func(a, b *Stream) bool {
				return a.stream.ClientPort < b.stream.ClientPort
			},
			query.SortingKeyServerPort: func(a, b *Stream) bool {
				return a.stream.ServerPort < b.stream.ServerPort
			},
		},
		query.SortingDirDescending: map[query.SortingKey]func(a *Stream, b *Stream) bool{
			query.SortingKeyID: func(a, b *Stream) bool {
				return a.stream.StreamID > b.stream.StreamID
			},
			query.SortingKeyClientBytes: func(a, b *Stream) bool {
				return a.stream.ClientBytes > b.stream.ClientBytes
			},
			query.SortingKeyServerBytes: func(a, b *Stream) bool {
				return a.stream.ServerBytes > b.stream.ServerBytes
			},
			query.SortingKeyFirstPacketTime: func(a, b *Stream) bool {
				if a.r == b.r {
					return a.stream.FirstPacketTimeNS > b.stream.FirstPacketTimeNS
				}
				at := a.r.referenceTime.Add(time.Nanosecond * time.Duration(a.stream.FirstPacketTimeNS))
				bt := b.r.referenceTime.Add(time.Nanosecond * time.Duration(b.stream.FirstPacketTimeNS))
				return at.After(bt)
			},
			query.SortingKeyLastPacketTime: func(a, b *Stream) bool {
				if a.r == b.r {
					return a.stream.LastPacketTimeNS > b.stream.LastPacketTimeNS
				}
				at := a.r.referenceTime.Add(time.Nanosecond * time.Duration(a.stream.LastPacketTimeNS))
				bt := b.r.referenceTime.Add(time.Nanosecond * time.Duration(b.stream.LastPacketTimeNS))
				return at.After(bt)
			},
			query.SortingKeyClientHost: func(a, b *Stream) bool {
				if a.stream.ClientHost == b.stream.ClientHost && a.r == b.r && a.stream.HostGroup == b.stream.HostGroup {
					return false
				}
				ah := a.r.hostGroups[a.stream.HostGroup].get(a.stream.ClientHost)
				bh := b.r.hostGroups[b.stream.HostGroup].get(b.stream.ClientHost)
				cmp := bytes.Compare(ah, bh)
				return cmp > 0
			},
			query.SortingKeyServerHost: func(a, b *Stream) bool {
				if a.stream.ServerHost == b.stream.ServerHost && a.r == b.r && a.stream.HostGroup == b.stream.HostGroup {
					return false
				}
				ah := a.r.hostGroups[a.stream.HostGroup].get(a.stream.ServerHost)
				bh := b.r.hostGroups[b.stream.HostGroup].get(b.stream.ServerHost)
				cmp := bytes.Compare(ah, bh)
				return cmp > 0
			},
			query.SortingKeyClientPort: func(a, b *Stream) bool {
				return a.stream.ClientPort > b.stream.ClientPort
			},
			query.SortingKeyServerPort: func(a, b *Stream) bool {
				return a.stream.ServerPort > b.stream.ServerPort
			},
		},
	}
	var sortingLess func(a, b *Stream) bool
	switch len(sorting) {
	case 0:
		// default search order is -ftime
		sortingLess = sorterFunctions[query.SortingDirDescending][query.SortingKeyFirstPacketTime]
	case 1:
		sortingLess = sorterFunctions[sorting[0].Dir][sorting[0].Key]
	default:
		for _, s := range sorting {
			sorters = append(sorters, sorterFunctions[s.Dir][s.Key])
		}
		sortingLess = func(a, b *Stream) bool {
			for _, sorter := range sorters {
				if sorter(a, b) {
					// a < b
					return true
				}
				if sorter(b, a) {
					// a > b
					return false
				}
				// a == b -> check next sorter
			}
			return false
		}
	}

	allResults := map[string][]*Stream{}
	moreResults := false
	for _, subQuery := range qs.SubQueries() {
		results := []*Stream{}
		sorter := sortingLess
		resultLimit := limit + skip
		if subQuery != "" {
			sorter = nil
			resultLimit = 0
		}
		for i := len(indexes); i > 0; {
			i--
			idx := indexes[i]

			//get all filters and lookups for each sub-query
			filtersList := [][]func(*subQuerySelection, *stream) (bool, error){}
			lookupsList := [][]func() ([]uint32, error){}
			for qID := range qs {
				//build search structures
				possible, filters, lookups, err := idx.buildSearchObjects(subQuery, allResults, refTime, &qs[qID], indexes[i+1:], tags)
				if err != nil {
					return nil, false, err
				}
				if !possible {
					continue
				}
				filtersList = append(filtersList, filters)
				lookupsList = append(lookupsList, lookups)
			}
			if len(filtersList) != 0 {
				streams, dropped, err := idx.searchStreams(results, allResults, filtersList, lookupsList, sorter, resultLimit)
				if err != nil {
					return nil, false, err
				}
				if subQuery == "" && dropped {
					moreResults = true
				}
				results = streams
			}
		}
		allResults[subQuery] = results
	}
	results := allResults[""]
	if uint(len(results)) <= skip {
		return nil, false, nil
	}
	return results[skip:], moreResults, nil
}

func (r *Reader) searchStreams(currentResults []*Stream, subQueryResults map[string][]*Stream, allFilters [][]func(*subQuerySelection, *stream) (bool, error), allLookups [][]func() ([]uint32, error), sortingLess func(a, b *Stream) bool, limit uint) ([]*Stream, bool, error) {
	// check if all queries use lookups, if not don't evaluate them
	useLookups := true
	for _, ls := range allLookups {
		if len(ls) == 0 {
			useLookups = false
			break
		}
	}
	// a map of index to list of sub-queries that matched this id
	streamIndexes := map[uint32][]int{}
	if useLookups {
		for qID, ls := range allLookups {
			streamIndexesOfQuery := []uint32(nil)
			for _, l := range ls {
				newStreamIndexes, err := l()
				if err != nil {
					return nil, false, err
				}
				if len(newStreamIndexes) == 0 {
					streamIndexesOfQuery = nil
					break
				}
				if len(streamIndexesOfQuery) == 0 {
					streamIndexesOfQuery = newStreamIndexes
					continue
				}
				newStreamIndexesMap := make(map[uint32]struct{}, len(newStreamIndexes))
				for _, si := range newStreamIndexes {
					newStreamIndexesMap[si] = struct{}{}
				}
				// filter out old stream indexes with the new lookup
				removed := 0
				for i := 0; i < len(streamIndexesOfQuery); i++ {
					si := streamIndexesOfQuery[i]
					if _, ok := newStreamIndexesMap[si]; !ok {
						removed++
					} else if removed != 0 {
						streamIndexesOfQuery[i-removed] = si
					}
				}
				streamIndexesOfQuery = streamIndexesOfQuery[:len(streamIndexesOfQuery)-removed]
				if len(streamIndexesOfQuery) == 0 {
					break
				}
			}
			for _, s := range streamIndexesOfQuery {
				streamIndexes[s] = append(streamIndexes[s], qID)
			}
		}
		if len(streamIndexes) == 0 {
			return nil, false, nil
		}
	}

	// apply filters to lookup results or all streams, if no lookups could be used
	resultsDropped := false
	filterAndAddToResult := func(filterIndexes []int, si uint32) error {
		s, err := r.streamByIndex(si)
		if err != nil {
			return err
		}
		ss, err := s.wrap(r, si)
		if err != nil {
			return err
		}

		// check if the sorting and limit would allow this stream
		if limit != 0 && sortingLess != nil && uint(len(currentResults)) >= limit && !sortingLess(ss, currentResults[limit-1]) {
			resultsDropped = true
			return nil
		}

		matchingByAny := false
		for _, fi := range filterIndexes {
			sqs := subQuerySelection{}
			tmp := map[string]subQueryRanges{}
			for k, v := range subQueryResults {
				tmp[k] = subQueryRanges{[]subQueryRange{{0, len(v) - 1}}}
			}
			sqs.remaining = append(sqs.remaining, tmp)
			matchingByThis := true
			for _, f := range allFilters[fi] {
				matching, err := f(&sqs, s)
				if err != nil {
					return err
				}
				matchingByThis = matching
				if !matchingByThis {
					break
				}
			}
			matchingByAny = matchingByThis
			if matchingByAny {
				break
			}
		}
		if !matchingByAny {
			return nil
		}
		if limit != 0 && uint(len(currentResults)) >= limit {
			// drop the last result
			currentResults[len(currentResults)-1] = nil
			resultsDropped = true
		} else {
			currentResults = append(currentResults, nil)
		}
		// insert the result at the right place
		pos := len(currentResults) - 1
		if sortingLess != nil {
			pos = sort.Search(len(currentResults)-1, func(i int) bool {
				return sortingLess(ss, currentResults[i])
			})
			for i := len(currentResults) - 1; i > pos; i-- {
				currentResults[i] = currentResults[i-1]
			}
		}
		currentResults[pos] = ss
		return nil
	}
	if len(streamIndexes) != 0 {
		sortedStreamIndexes := make([]uint32, 0, len(streamIndexes))
		for s := range streamIndexes {
			sortedStreamIndexes = append(sortedStreamIndexes, s)
		}
		sort.Slice(sortedStreamIndexes, func(i, j int) bool {
			return sortedStreamIndexes[i] < sortedStreamIndexes[j]
		})
		for _, si := range sortedStreamIndexes {
			if err := filterAndAddToResult(streamIndexes[si], si); err != nil {
				return nil, false, err
			}
		}
	} else {
		filtersToApply := make([]int, 0, len(allFilters))
		for i := 0; i < len(allFilters); i++ {
			filtersToApply = append(filtersToApply, i)
		}
		for si, sc := 0, r.StreamCount(); si < sc; si++ {
			if err := filterAndAddToResult(filtersToApply, uint32(si)); err != nil {
				return nil, false, err
			}
		}
	}
	return currentResults, resultsDropped, nil
}
