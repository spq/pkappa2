package index

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"time"

	"github.com/spq/pkappa2/internal/query"
	"github.com/spq/pkappa2/internal/tools/bitmask"
)

type (
	ConverterAccess interface {
		Data(stream *Stream, moreDetails bool) (data []Data, clientBytes, serverBytes uint64, wasCached bool, err error)
		DataForSearch(streamID uint64) ([2][]byte, [][2]int, uint64, uint64, bool, error)
	}
	subQuerySelection struct {
		remaining []map[string]bitmask.ConnectedBitmask
	}
	searchContext struct {
		allowedSubQueries subQuerySelection
		outputVariables   map[string][]string
	}
	variableDataValue struct {
		name, value string
		queryParts  bitmask.ShortBitmask
	}
	variableDataCollection struct {
		uses int
		data []variableDataValue
	}
	DataRegexes struct {
		Client []string
		Server []string
	}
	resultData struct {
		streams             []*Stream
		matchingQueryPart   []bitmask.ConnectedBitmask
		groups              map[string]int
		variableAssociation map[uint64]int
		variableData        []variableDataCollection
		resultDropped       uint
	}
	queryPart struct {
		filters  []func(*searchContext, *stream) (bool, error)
		lookups  []func() ([]uint32, error)
		possible bool
	}
	grouper struct {
		key  func(s *Stream) []byte
		vars []string
	}
)

func (sqs *subQuerySelection) remove(subqueries []string, forbidden []*bitmask.ConnectedBitmask) {
	oldRemaining := sqs.remaining
	sqs.remaining = nil
outer:
	for _, remaining := range oldRemaining {
		for sqi, sq := range subqueries {
			old := remaining[sq]
			remove := old.AndCopy(*forbidden[sqi])
			keep := old.SubCopy(*forbidden[sqi])
			if remove.IsZero() {
				sqs.remaining = append(sqs.remaining, remaining)
				continue outer
			}
			if keep.IsZero() {
				continue
			}
			remaining[sq] = remove
			sqs.remaining = append(sqs.remaining, map[string]bitmask.ConnectedBitmask{
				sq: keep,
			})
			new := &sqs.remaining[len(sqs.remaining)-1]
			for k, v := range remaining {
				if k != sq {
					(*new)[k] = v.Copy()
				}
			}
		}
	}
}

func (sqs *subQuerySelection) empty() bool {
	return len(sqs.remaining) == 0
}

var (
	alwaysSuccess = ([]func(sc *searchContext, s *stream) (bool, error))(nil)
	alwaysFail    = []func(sc *searchContext, s *stream) (bool, error){
		func(sc *searchContext, s *stream) (bool, error) {
			return false, nil
		},
	}
)

func isAlwaysFail(f []func(*searchContext, *stream) (bool, error)) bool {
	if len(f) != 1 {
		return false
	}
	return &f[0] == &alwaysFail[0]
}

func (r *Reader) buildSearchObjects(subQuery string, queryPartIndex int, previousResults map[string]resultData, refTime time.Time, q *query.Conditions, superseedingIndexes []*Reader, limitIDs *bitmask.LongBitmask, tagDetails map[string]query.TagDetails, converters map[string]ConverterAccess) (queryPart, error) {
	filters := []func(*searchContext, *stream) (bool, error)(nil)
	lookups := []func() ([]uint32, error)(nil)

	// filter to caller requested ids
	if limitIDs != nil {
		filters = append(filters, func(_ *searchContext, s *stream) (bool, error) {
			return limitIDs.IsSet(uint(s.StreamID)), nil
		})
	}

	// filter out streams superseeded by newer indexes
	if len(superseedingIndexes) != 0 {
		filters = append(filters, func(_ *searchContext, s *stream) (bool, error) {
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
	dcc := dataConditionsContainer{}
conditions:
	for _, c := range *q {
		c := c
		switch cc := c.(type) {
		case *query.TagCondition:
			if cc.SubQuery != subQuery {
				continue
			}
			td, ok := tagDetails[cc.TagName]
			if !ok {
				return queryPart{}, fmt.Errorf("tag %q does not exist", cc.TagName)
			}
			var f func(uint64) bool
			switch cc.Accept {
			case 0:
				// accept never
				f = func(id uint64) bool {
					return false
				}
			case query.TagConditionAcceptUncertainMatching | query.TagConditionAcceptUncertainFailing | query.TagConditionAcceptMatching | query.TagConditionAcceptFailing:
				// accept always
			case query.TagConditionAcceptUncertainMatching | query.TagConditionAcceptUncertainFailing:
				// accept if uncertain
				f = func(id uint64) bool {
					return td.Uncertain.IsSet(uint(id))
				}
			case query.TagConditionAcceptMatching | query.TagConditionAcceptFailing:
				// accept if certain
				f = func(id uint64) bool {
					return !td.Uncertain.IsSet(uint(id))
				}
			case query.TagConditionAcceptMatching | query.TagConditionAcceptUncertainMatching:
				// accept if matching
				f = func(id uint64) bool {
					return td.Matches.IsSet(uint(id))
				}
			case query.TagConditionAcceptFailing | query.TagConditionAcceptUncertainFailing:
				// accept if failing
				f = func(id uint64) bool {
					return !td.Matches.IsSet(uint(id))
				}
			default:
				f = func(id uint64) bool {
					a := cc.Accept
					if td.Uncertain.IsSet(uint(id)) {
						a &= query.TagConditionAcceptUncertainMatching | query.TagConditionAcceptUncertainFailing
					} else {
						a &= query.TagConditionAcceptMatching | query.TagConditionAcceptFailing
					}
					if td.Matches.IsSet(uint(id)) {
						a &= query.TagConditionAcceptMatching | query.TagConditionAcceptUncertainMatching
					} else {
						a &= query.TagConditionAcceptFailing | query.TagConditionAcceptUncertainFailing
					}
					return a != 0
				}
			}
			if f != nil {
				filters = append(filters, func(_ *searchContext, s *stream) (bool, error) {
					return f(s.StreamID), nil
				})
				lookups = append(lookups, func() ([]uint32, error) {
					lookup := []uint32(nil)
					for id, index := range r.containedStreamIds {
						if f(id) {
							lookup = append(lookup, index)
						}
					}
					return lookup, nil
				})
			}
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
				filters = append(filters, func(_ *searchContext, s *stream) (bool, error) {
					return s.Flags&cc.Mask != cc.Value, nil
				})
				continue
			}
			flagValues := map[uint16][]*bitmask.ConnectedBitmask{
				cc.Value & cc.Mask: nil,
			}
			subqueries := []string(nil)
			for _, sq := range cc.SubQueries {
				if sq == subQuery {
					continue
				}
				subqueries = append(subqueries, sq)
				curFlagValues := map[uint16]*bitmask.ConnectedBitmask{}
				for pos, res := range previousResults[sq].streams {
					f := res.Flags & cc.Mask
					cfv := curFlagValues[f]
					if cfv == nil {
						cfv = &bitmask.ConnectedBitmask{}
						curFlagValues[f] = cfv
					}
					cfv.Set(uint(pos))
				}
				tmp := flagValues
				flagValues = make(map[uint16][]*bitmask.ConnectedBitmask)
				for f1, d1 := range tmp {
					for f2, d2 := range curFlagValues {
						d := append(append([]*bitmask.ConnectedBitmask(nil), d1...), d2)
						flagValues[f1^f2] = d
					}
				}
			}
			filters = append(filters, func(sc *searchContext, s *stream) (bool, error) {
				forbidden, possible := flagValues[s.Flags&cc.Mask]
				if !possible {
					// no combination of sub queries produces the forbidden result
					return true, nil
				}
				if len(flagValues) == 1 {
					// the only combination of sub queries produces the forbidden result
					return false, nil
				}
				sc.allowedSubQueries.remove(subqueries, forbidden)
				return !sc.allowedSubQueries.empty(), nil
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
					return queryPart{}, errors.New("complex host condition not supported")
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
					otherHosts := [2]bitmask.ConnectedBitmask{}
					for rIdx, r := range relevantResults.streams {
						otherSize := r.r.hostGroups[r.HostGroup].hostSize
						otherHosts[otherSize/16].Set(uint(rIdx))
					}
					if !cc.Invert {
						otherHosts[0], otherHosts[1] = otherHosts[1], otherHosts[0]
					}
					forbiddenSubQueryResultsPerHostGroup := make([]*bitmask.ConnectedBitmask, len(r.hostGroups))
					for hgi, hg := range r.hostGroups {
						forbiddenSubQueryResultsPerHostGroup[hgi] = &otherHosts[hg.hostSize/16]
					}
					filters = append(filters, func(sc *searchContext, s *stream) (bool, error) {
						f := forbiddenSubQueryResultsPerHostGroup[s.HostGroup]
						sc.allowedSubQueries.remove([]string{otherSubQuery}, []*bitmask.ConnectedBitmask{f})
						return !sc.allowedSubQueries.empty(), nil
					})
					continue
				}
				filters = append(filters, func(sc *searchContext, s *stream) (bool, error) {
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

					forbidden := bitmask.ConnectedBitmask{}
				outer:
					for resIdx, res := range relevantResults.streams {
						otherHG := &res.r.hostGroups[res.HostGroup]
						if myHG.hostSize != otherHG.hostSize {
							if !cc.Invert {
								forbidden.Set(uint(resIdx))
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
								forbidden.Set(uint(resIdx))
								continue outer
							}
						}
						if cc.Invert {
							forbidden.Set(uint(resIdx))
						}
					}
					sc.allowedSubQueries.remove([]string{otherSubQuery}, []*bitmask.ConnectedBitmask{&forbidden})
					return !sc.allowedSubQueries.empty(), nil
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
				// another host condition already excluded all results in this host group
				if bitmap == nil {
					continue
				}
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
				filters = append(filters, func(_ *searchContext, s *stream) (bool, error) {
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
					ranges bitmask.ConnectedBitmask
				}
			)
			subQueryData := [][]subQueryResult(nil)
			subQueries := []string(nil)
			minSum, maxSum := 0, 0
			for sq, f := range factors {
				numbers := map[int]int{}
				results := []subQueryResult(nil)
				for resId, res := range previousResults[sq].streams {
					n := 0
					n += f.id * int(res.StreamID)
					n += f.clientBytes * int(res.ClientBytes)
					n += f.serverBytes * int(res.ServerBytes)
					n += f.clientPort * int(res.ClientPort)
					n += f.serverPort * int(res.ServerPort)
					if pos, ok := numbers[n]; ok {
						results[pos].ranges.Set(uint(resId))
						continue
					}
					numbers[n] = len(results)
					results = append(results, subQueryResult{
						number: n,
						ranges: bitmask.MakeConnectedBitmask(uint(resId), uint(resId)),
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
			// combine the ranges of the last subQueryData
			// element n will contain the range of elements 0..n
			lastSubQueryData := subQueryData[len(subQueryData)-1]
			for i, l := 0, len(lastSubQueryData)-1; i < l; i++ {
				r := &lastSubQueryData[i+1].ranges
				*r = r.OrCopy(lastSubQueryData[i].ranges)
			}

			filters = append(filters, func(sc *searchContext, s *stream) (bool, error) {
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
				pos := make([]int, len(subQueries)-1)
				lastSubQueryData := subQueryData[len(subQueryData)-1]
			outer:
				for {
					// calculate the sum
					sqN := n
					for i, j := range pos {
						sqN += subQueryData[i][j].number
					}
					// remove from sqs if the lowest possible sum is still invalid
					if sqN+lastSubQueryData[0].number < 0 {
						forbidden := []*bitmask.ConnectedBitmask(nil)
						for i, j := range pos {
							forbidden = append(forbidden, &subQueryData[i][j].ranges)
						}
						if sqN+lastSubQueryData[len(lastSubQueryData)-1].number < 0 {
							// the highest possible value also doesn't result in a valid sum
							// we can remove the whole combination ignoring the last element
							sc.allowedSubQueries.remove(subQueries[:len(subQueries)-1], forbidden)
						} else {
							// the highest possible value results in a valid sum
							// find the position where the validity changes
							lastInvalid := sort.Search(len(lastSubQueryData)-2, func(i int) bool {
								return sqN+lastSubQueryData[i+1].number >= 0
							})
							forbidden = append(forbidden, &lastSubQueryData[lastInvalid].ranges)
							sc.allowedSubQueries.remove(subQueries, forbidden)
						}
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
				return !sc.allowedSubQueries.empty(), nil
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
			startD := cc.Duration + time.Duration(myFactors.ftime+myFactors.ltime)*r.ReferenceTime.Sub(refTime)
			if len(factors) == 0 {
				filter := func(_ *searchContext, s *stream) (bool, error) {
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
						return queryPart{}, nil
					}
				} else {
					filters = append(filters, filter)
				}
				continue
			}
			type (
				subQueryResult struct {
					duration time.Duration
					ranges   bitmask.ConnectedBitmask
				}
			)
			subQueryData := [][]subQueryResult(nil)
			subQueries := []string(nil)
			minSum, maxSum := time.Duration(0), time.Duration(0)
			for sq, f := range factors {
				durations := map[time.Duration]int{}
				results := []subQueryResult(nil)
				for resId, res := range previousResults[sq].streams {
					d := time.Duration(f.ftime+f.ltime) * res.r.ReferenceTime.Sub(refTime)
					d += time.Duration(f.ftime) * time.Duration(res.FirstPacketTimeNS)
					d += time.Duration(f.ltime) * time.Duration(res.LastPacketTimeNS)
					if pos, ok := durations[d]; ok {
						results[pos].ranges.Set(uint(resId))
						continue
					}
					durations[d] = len(results)
					results = append(results, subQueryResult{
						duration: d,
						ranges:   bitmask.MakeConnectedBitmask(uint(resId), uint(resId)),
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
			// combine the ranges of the last subQueryData
			// element n will contain the range of elements 0..n
			lastSubQueryData := subQueryData[len(subQueryData)-1]
			for i, l := 0, len(lastSubQueryData)-1; i < l; i++ {
				r := &lastSubQueryData[i+1].ranges
				*r = r.OrCopy(lastSubQueryData[i].ranges)
			}
			filters = append(filters, func(sc *searchContext, s *stream) (bool, error) {
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
				pos := make([]int, len(subQueries)-1)
				lastSubQueryData := subQueryData[len(subQueryData)-1]
			outer:
				for {
					// calculate the sum
					sqD := d
					for i, j := range pos {
						sqD += subQueryData[i][j].duration
					}
					// remove from sqs if the lowest possible sum is still invalid
					if sqD+lastSubQueryData[0].duration < 0 {
						forbidden := []*bitmask.ConnectedBitmask(nil)
						for i, j := range pos {
							forbidden = append(forbidden, &subQueryData[i][j].ranges)
						}
						if sqD+lastSubQueryData[len(lastSubQueryData)-1].duration < 0 {
							// the highest possible value also doesn't result in a valid sum
							// we can remove the whole combination ignoring the last element
							sc.allowedSubQueries.remove(subQueries[:len(subQueries)-1], forbidden)
						} else {
							// the highest possible value results in a valid sum
							// find the position where the validity changes
							lastInvalid := sort.Search(len(lastSubQueryData)-2, func(i int) bool {
								return sqD+lastSubQueryData[i+1].duration >= 0
							})
							forbidden = append(forbidden, &lastSubQueryData[lastInvalid].ranges)
							sc.allowedSubQueries.remove(subQueries, forbidden)
						}
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
				return !sc.allowedSubQueries.empty(), nil
			})
		case *query.DataCondition:
			if err := dcc.add(cc, subQuery, previousResults); err != nil {
				return queryPart{}, err
			}
		}
	}
	if minIDFilter == maxIDFilter {
		idx, ok := r.containedStreamIds[minIDFilter]
		if !ok {
			return queryPart{}, nil
		}
		lookups = append(lookups, func() ([]uint32, error) {
			return []uint32{idx}, nil
		})
	} else if minIDFilter != 0 || maxIDFilter != math.MaxUint64 {
		lookups = append(lookups, func() ([]uint32, error) {
			lookup := []uint32(nil)
			for id, index := range r.containedStreamIds {
				if id >= minIDFilter && id <= maxIDFilter {
					lookup = append(lookup, index)
				}
			}
			return lookup, nil
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
			return queryPart{}, nil
		}
		if someFail {
			filters = append(filters, func(_ *searchContext, s *stream) (bool, error) {
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
	dataFilters, err := dcc.finalize(r, queryPartIndex, previousResults, converters)
	if err != nil {
		return queryPart{}, err
	}
	if isAlwaysFail(dataFilters) {
		return queryPart{}, nil
	}
	filters = append(filters, dataFilters...)
	return queryPart{
		filters:  filters,
		lookups:  lookups,
		possible: true,
	}, nil
}

var (
	sorterLookupSections = map[query.SortingKey]section{
		query.SortingKeyID:              sectionStreamsByStreamID,
		query.SortingKeyFirstPacketTime: sectionStreamsByFirstPacketTime,
		query.SortingKeyLastPacketTime:  sectionStreamsByLastPacketTime,
	}
	sorterFunctions = map[query.SortingKey]func(a, b *Stream) bool{
		query.SortingKeyID: func(a, b *Stream) bool {
			return a.StreamID < b.StreamID
		},
		query.SortingKeyClientBytes: func(a, b *Stream) bool {
			return a.ClientBytes < b.ClientBytes
		},
		query.SortingKeyServerBytes: func(a, b *Stream) bool {
			return a.ServerBytes < b.ServerBytes
		},
		query.SortingKeyFirstPacketTime: func(a, b *Stream) bool {
			if a.r == b.r {
				return a.FirstPacketTimeNS < b.FirstPacketTimeNS
			}
			at := a.r.ReferenceTime.Add(time.Nanosecond * time.Duration(a.FirstPacketTimeNS))
			bt := b.r.ReferenceTime.Add(time.Nanosecond * time.Duration(b.FirstPacketTimeNS))
			return at.Before(bt)
		},
		query.SortingKeyLastPacketTime: func(a, b *Stream) bool {
			if a.r == b.r {
				return a.LastPacketTimeNS < b.LastPacketTimeNS
			}
			at := a.r.ReferenceTime.Add(time.Nanosecond * time.Duration(a.LastPacketTimeNS))
			bt := b.r.ReferenceTime.Add(time.Nanosecond * time.Duration(b.LastPacketTimeNS))
			return at.Before(bt)
		},
		query.SortingKeyClientHost: func(a, b *Stream) bool {
			if a.ClientHost == b.ClientHost && a.r == b.r && a.HostGroup == b.HostGroup {
				return false
			}
			ah := a.r.hostGroups[a.HostGroup].get(a.ClientHost)
			bh := b.r.hostGroups[b.HostGroup].get(b.ClientHost)
			cmp := bytes.Compare(ah, bh)
			return cmp < 0
		},
		query.SortingKeyServerHost: func(a, b *Stream) bool {
			if a.ServerHost == b.ServerHost && a.r == b.r && a.HostGroup == b.HostGroup {
				return false
			}
			ah := a.r.hostGroups[a.HostGroup].get(a.ServerHost)
			bh := b.r.hostGroups[b.HostGroup].get(b.ServerHost)
			cmp := bytes.Compare(ah, bh)
			return cmp < 0
		},
		query.SortingKeyClientPort: func(a, b *Stream) bool {
			return a.ClientPort < b.ClientPort
		},
		query.SortingKeyServerPort: func(a, b *Stream) bool {
			return a.ServerPort < b.ServerPort
		},
	}
)

func extractDataRegexes(qs query.ConditionsSet, tagDetails map[string]query.TagDetails) *DataRegexes {
	dataConditions := DataRegexes{}
	queue := []*query.ConditionsSet{&qs}
	for len(queue) > 0 {
		cs := *queue[0]
		queue = queue[1:]
		for _, ccs := range cs.InlineTagFilters(tagDetails) {
			for _, cc := range ccs {
				switch ccc := cc.(type) {
				case *query.DataCondition:
					for _, e := range ccc.Elements {
						if e.Flags&query.DataRequirementSequenceFlagsDirection == query.DataRequirementSequenceFlagsDirectionClientToServer {
							if !slices.Contains(dataConditions.Client, e.Regex) {
								dataConditions.Client = append(dataConditions.Client, e.Regex)
							}
						} else {
							if !slices.Contains(dataConditions.Server, e.Regex) {
								dataConditions.Server = append(dataConditions.Server, e.Regex)
							}
						}
					}
				case *query.TagCondition:
					ti := tagDetails[ccc.TagName]
					queue = append(queue, &ti.Conditions)
				}
			}
		}
	}
	return &dataConditions
}

func SearchStreams(ctx context.Context, indexes []*Reader, limitIDs *bitmask.LongBitmask, refTime time.Time, qs query.ConditionsSet, grouping *query.Grouping, sorting []query.Sorting, limit, skip uint, tagDetails map[string]query.TagDetails, converters map[string]ConverterAccess, extractRegexes bool) ([]*Stream, bool, *DataRegexes, error) {
	if len(qs) == 0 {
		return nil, false, nil, nil
	}
	qs = qs.InlineTagFilters(tagDetails)

	var sortingLess func(a, b *Stream) bool
	switch len(sorting) {
	case 0:
		// default search order is -ftime
		sorting = []query.Sorting{{
			Key: query.SortingKeyFirstPacketTime,
			Dir: query.SortingDirDescending,
		}}
		fallthrough
	case 1:
		sortingLess = sorterFunctions[sorting[0].Key]
		if sorting[0].Dir == query.SortingDirDescending {
			asc := sortingLess
			sortingLess = func(a, b *Stream) bool {
				return asc(b, a)
			}
		}
	default:
		sorters := []func(a, b *Stream) bool{}
		for _, s := range sorting {
			af := sorterFunctions[s.Key]
			df := func(a, b *Stream) bool {
				return af(b, a)
			}
			switch s.Dir {
			case query.SortingDirAscending:
				sorters = append(sorters, af)
			case query.SortingDirDescending:
				sorters = append(sorters, df)
			}
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

	groupingData := (*grouper)(nil)
	if grouping != nil {
		groupingKeyMap := map[string]func(s *Stream) []byte{
			"id": func(s *Stream) []byte {
				b := [8]byte{}
				binary.LittleEndian.PutUint64(b[:], s.StreamID)
				return b[:]
			},
			"cport": func(s *Stream) []byte {
				b := [2]byte{}
				binary.LittleEndian.PutUint16(b[:], s.ClientPort)
				return b[:]
			},
			"sport": func(s *Stream) []byte {
				b := [2]byte{}
				binary.LittleEndian.PutUint16(b[:], s.ServerPort)
				return b[:]
			},
			"bytes": func(s *Stream) []byte {
				b := [8]byte{}
				binary.LittleEndian.PutUint64(b[:], s.ClientBytes+s.ServerBytes)
				return b[:]
			},
			"cbytes": func(s *Stream) []byte {
				b := [8]byte{}
				binary.LittleEndian.PutUint64(b[:], s.ClientBytes)
				return b[:]
			},
			"sbytes": func(s *Stream) []byte {
				b := [8]byte{}
				binary.LittleEndian.PutUint64(b[:], s.ServerBytes)
				return b[:]
			},

			"ftime": func(s *Stream) []byte {
				b := [16]byte{}
				t := s.r.ReferenceTime.Add(time.Nanosecond * time.Duration(s.FirstPacketTimeNS))
				binary.LittleEndian.PutUint64(b[:8], uint64(t.Unix()))
				binary.LittleEndian.PutUint64(b[8:], uint64(t.UnixNano()))
				return b[:]
			},
			"ltime": func(s *Stream) []byte {
				b := [16]byte{}
				t := s.r.ReferenceTime.Add(time.Nanosecond * time.Duration(s.LastPacketTimeNS))
				binary.LittleEndian.PutUint64(b[:8], uint64(t.Unix()))
				binary.LittleEndian.PutUint64(b[8:], uint64(t.UnixNano()))
				return b[:]
			},
			"duration": func(s *Stream) []byte {
				b := [8]byte{}
				ft := s.r.ReferenceTime.Add(time.Nanosecond * time.Duration(s.FirstPacketTimeNS))
				lt := s.r.ReferenceTime.Add(time.Nanosecond * time.Duration(s.LastPacketTimeNS))
				binary.LittleEndian.PutUint64(b[:], uint64(lt.Sub(ft)))
				return b[:]
			},

			"chost": func(s *Stream) []byte {
				hg := s.r.hostGroups[s.HostGroup]
				return append([]byte{byte(hg.hostSize)}, hg.get(s.ClientHost)...)
			},
			"shost": func(s *Stream) []byte {
				hg := s.r.hostGroups[s.HostGroup]
				return append([]byte{byte(hg.hostSize)}, hg.get(s.ServerHost)...)
			},
		}
		keyFuncs := []func(s *Stream) []byte(nil)
		variables := []string(nil)
		for _, v := range grouping.Variables {
			if v.SubQuery != "" {
				return nil, false, nil, errors.New("SubQueries not yet fully supported")
			}
			g, ok := groupingKeyMap[v.Name]
			if ok {
				keyFuncs = append(keyFuncs, g)
			} else {
				variables = append(variables, v.Name)
			}
		}
		switch len(keyFuncs) {
		case 0:
			keyFuncs = append(keyFuncs, func(s *Stream) []byte {
				return nil
			})
			fallthrough
		case 1:
			groupingData = &grouper{
				key:  keyFuncs[0],
				vars: variables,
			}
		default:
			groupingData = &grouper{
				key: func(s *Stream) []byte {
					r := []byte(nil)
					for _, f := range keyFuncs {
						r = append(r, f(s)...)
					}
					return r
				},
				vars: variables,
			}
		}
	}

	allResults := map[string]resultData{}
	for _, subQuery := range qs.SubQueries() {
		results := resultData{
			matchingQueryPart: make([]bitmask.ConnectedBitmask, len(qs)),
		}
		sorter := sortingLess
		resultLimit := limit + skip
		limitIDs := limitIDs
		if subQuery != "" {
			sorter = nil
			resultLimit = 0
			limitIDs = nil
		}

		for idxIdx := len(indexes) - 1; idxIdx >= 0; idxIdx-- {
			idx := indexes[idxIdx]

			sortingLookup := (func() ([]uint32, error))(nil)
			if resultLimit != 0 {
				if section, ok := sorterLookupSections[sorting[0].Key]; sorter != nil && ok {
					res := []uint32(nil)
					reverse := sorting[0].Dir == query.SortingDirDescending
					sortingLookup = func() ([]uint32, error) {
						if res == nil {
							res = make([]uint32, idx.StreamCount())
							if err := idx.readObjects(section, res); err != nil {
								return nil, err
							}
							if reverse {
								for i, j := 0, len(res)-1; i < j; {
									res[i], res[j] = res[j], res[i]
									i++
									j--
								}
							}
						}
						return res, nil
					}
				}
			}
			//get all filters and lookups for each sub-query
			queryParts := make([]queryPart, 0, len(qs))
			for qID := range qs {
				//build search structures
				queryPart, err := idx.buildSearchObjects(subQuery, qID, allResults, refTime, &qs[qID], indexes[idxIdx+1:], limitIDs, tagDetails, converters)
				if err != nil {
					return nil, false, nil, err
				}
				queryParts = append(queryParts, queryPart)
			}
			err := idx.searchStreams(ctx, &results, allResults, queryParts, groupingData, sorter, resultLimit, sortingLookup)
			if err != nil {
				return nil, false, nil, err
			}
		}
		if len(results.streams) == 0 {
			return nil, false, nil, nil
		}
		allResults[subQuery] = results
	}
	results := allResults[""]
	if uint(len(results.streams)) <= skip {
		return nil, false, nil, nil
	}
	var dataRegexes *DataRegexes
	if extractRegexes {
		dataRegexes = extractDataRegexes(qs, tagDetails)
	}
	return results.streams[skip:], results.resultDropped != 0, dataRegexes, nil
}

func (r *Reader) searchStreams(ctx context.Context, result *resultData, subQueryResults map[string]resultData, queryParts []queryPart, grouper *grouper, sortingLess func(a, b *Stream) bool, limit uint, sortingLookup func() ([]uint32, error)) error {
	// apply filters to lookup results or all streams, if no lookups could be used
	filterAndAddToResult := func(activeQueryParts bitmask.ShortBitmask, si uint32) (bool, error) {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		// check if the sorting and limit would allow any stream
		limitReached := result.resultDropped != 0 && limit != 0 && uint(len(result.streams)) >= limit
		if limitReached && sortingLess == nil {
			return true, nil
		}

		s, err := r.streamByIndex(si)
		if err != nil {
			return false, err
		}
		ss, err := s.wrap(r, si)
		if err != nil {
			return false, err
		}

		// check if the sorting and limit would allow this stream
		if limitReached && !sortingLess(ss, result.streams[limit-1]) {
			return true, nil
		}

		// check if the sorting within the groupKey allow this stream
		groupKey := []byte(nil)
		groupPos := -1
		if grouper != nil && len(grouper.vars) == 0 {
			groupKey = grouper.key(ss)
			pos, ok := result.groups[string(groupKey)]
			if ok {
				groupPos = pos
				if sortingLess == nil || !sortingLess(ss, result.streams[pos]) {
					return false, nil
				}
			}
		}

		matchingQueryParts := bitmask.ShortBitmask{}
		matchingSearchContexts := []*searchContext(nil)

	queryPart:
		for qpIdx, qpLen := 0, activeQueryParts.Len(); qpIdx < qpLen; qpIdx++ {
			if !activeQueryParts.IsSet(uint(qpIdx)) {
				continue
			}
			tmp := map[string]bitmask.ConnectedBitmask{}
			for k, v := range subQueryResults {
				if v.matchingQueryPart[qpIdx].IsZero() {
					continue queryPart
				}
				tmp[k] = v.matchingQueryPart[qpIdx].Copy()
			}
			sc := &searchContext{
				allowedSubQueries: subQuerySelection{
					remaining: []map[string]bitmask.ConnectedBitmask{tmp},
				},
			}
			for _, f := range queryParts[qpIdx].filters {
				matching, err := f(sc, s)
				if err != nil {
					return false, err
				}
				if !matching {
					continue queryPart
				}
			}
			matchingQueryParts.Set(uint(qpIdx))
			matchingSearchContexts = append(matchingSearchContexts, sc)
		}
		if matchingQueryParts.IsZero() {
			return false, nil
		}

		if grouper != nil && len(grouper.vars) != 0 {
			groupKey = grouper.key(ss)
			for _, vn := range grouper.vars {
				vvsm := map[string]struct{}{}
				vvsl := []string(nil)
				for _, sc := range matchingSearchContexts {
					for _, vv := range sc.outputVariables[vn] {
						if _, ok := vvsm[vv]; ok {
							continue
						}
						vvsm[vv] = struct{}{}
						vvsl = append(vvsl, vv)
					}
				}
				sort.Strings(vvsl)
				groupKey = append(groupKey, make([]byte, 8)...)
				binary.LittleEndian.PutUint64(groupKey[len(groupKey)-8:], uint64(len(vvsl)))
				for _, vv := range vvsl {
					groupKey = append(groupKey, make([]byte, 8)...)
					binary.LittleEndian.PutUint64(groupKey[len(groupKey)-8:], uint64(len(vv)))
					groupKey = append(groupKey, []byte(vv)...)
				}
			}
			pos, ok := result.groups[string(groupKey)]
			if ok {
				groupPos = pos
				if sortingLess == nil || !sortingLess(ss, result.streams[pos]) {
					result.resultDropped++
					return false, nil
				}
			}
		}

		replacePos := groupPos
		if groupPos == -1 {
			if limit == 0 || uint(len(result.streams)) < limit {
				// we have no limit or the limit is not yet reached
				replacePos = len(result.streams)
				result.streams = append(result.streams, nil)
			} else if sortingLess != nil && sortingLess(ss, result.streams[limit-1]) {
				// we have a limit but we are better than the last
				replacePos = len(result.streams) - 1
			} else {
				// we have a limit and are worse than the last
				result.resultDropped++
				return true, nil
			}
		}

		if r := &result.streams[replacePos]; *r != nil {
			if groupPos != -1 {
				// we should replace the group slot
				delete(result.groups, string(groupKey))
			} else if grouper != nil {
				// we should replace the last slot
				delete(result.groups, string(grouper.key(*r)))
			}
			if d, ok := result.variableAssociation[(*r).StreamID]; ok {
				result.variableData[d].uses--
				delete(result.variableAssociation, (*r).StreamID)
			}
			for i := range result.matchingQueryPart {
				result.matchingQueryPart[i].Extract(uint(replacePos))
			}
			*r = nil
			if groupPos == -1 {
				result.resultDropped++
			}
		}
		// replacePos now points to the position of a nil slot that we can use

		// insert the result at the right place
		insertPos := replacePos
		if sortingLess != nil {
			insertPos = sort.Search(len(result.streams)-1, func(i int) bool {
				if i >= replacePos {
					i++
				}
				return sortingLess(ss, result.streams[i])
			})
			if replacePos < insertPos {
				insertPos++
				for ; replacePos < insertPos; replacePos++ {
					result.streams[replacePos] = result.streams[replacePos+1]
				}
			} else if replacePos > insertPos {
				for ; replacePos > insertPos; replacePos-- {
					result.streams[replacePos] = result.streams[replacePos-1]
				}
			}
		}
		result.streams[insertPos] = ss

		if grouper != nil {
			if result.groups == nil {
				result.groups = make(map[string]int)
			}
			result.groups[string(groupKey)] = insertPos
		}

		vdv := []variableDataValue(nil)
		for scIdx, qpIdx, qpLen := -1, 0, matchingQueryParts.Len(); qpIdx < qpLen; qpIdx++ {
			matching := matchingQueryParts.IsSet(uint(qpIdx))
			result.matchingQueryPart[qpIdx].Inject(uint(insertPos), matching)
			if !matching {
				continue
			}
			scIdx++
			sc := matchingSearchContexts[scIdx]
			if sc.outputVariables == nil {
				continue
			}
			qp := bitmask.ShortBitmask{}
			qp.Set(uint(qpIdx))
			for k, vs := range sc.outputVariables {
			values:
				for _, v := range vs {
					for i := range vdv {
						vdvp := &vdv[i]
						if k != vdvp.name {
							continue
						}
						if v != vdvp.value {
							continue
						}
						vdvp.queryParts.Set(uint(qpIdx))
						continue values
					}
					vdv = append(vdv, variableDataValue{
						name:       k,
						value:      v,
						queryParts: qp,
					})
				}
			}
		}
		if len(vdv) == 0 {
			return false, nil
		}
		sort.Slice(vdv, func(i, j int) bool {
			a, b := &vdv[i], &vdv[j]
			if a.name != b.name {
				return a.name < b.name
			}
			return a.value < b.value
		})
		if result.variableAssociation == nil {
			result.variableAssociation = make(map[uint64]int)
		}
		freeSlot := len(result.variableData)
	varData:
		for i := range result.variableData {
			d := &result.variableData[i]
			if d.uses == 0 {
				freeSlot = i
			}
			if len(d.data) != len(vdv) {
				continue
			}
			for j := range vdv {
				if vdv[j].name != d.data[j].name {
					continue varData
				}
				if vdv[j].value != d.data[j].value {
					continue varData
				}
				if !vdv[j].queryParts.Equal(d.data[j].queryParts) {
					continue varData
				}
			}
			d.uses++
			result.variableAssociation[s.StreamID] = i
			return false, nil
		}
		if freeSlot == len(result.variableData) {
			result.variableData = append(result.variableData, variableDataCollection{})
		}
		result.variableData[freeSlot] = variableDataCollection{
			uses: 1,
			data: vdv,
		}
		result.variableAssociation[s.StreamID] = freeSlot
		return false, nil
	}

	// check if all queries use lookups, if not don't use lookups
	activeQueryParts := bitmask.ShortBitmask{}
	lookupMissing := false
	for qpIdx, qp := range queryParts {
		if !qp.possible {
			continue
		}
		activeQueryParts.Set(uint(qpIdx))
		if len(qp.lookups) == 0 {
			lookupMissing = true
		}
	}
	if activeQueryParts.OnesCount() == 0 {
		return nil
	}

	// if we don't have a limit, we should not use the sorting lookup as no early exit is possible
	if limit == 0 {
		sortingLookup = nil
	}

	if lookupMissing {
		// we miss a lookup for at least one query part, so we will be doing a full table scan

		// without sorting lookup, we will evaluate in file order without early exit
		if sortingLookup == nil {
			for si, sc := 0, r.StreamCount(); si < sc; si++ {
				if _, err := filterAndAddToResult(activeQueryParts, uint32(si)); err != nil {
					return err
				}
			}
			return nil
		}

		// with sorting lookup, we might be able to exit early if we reach the limit
		sortedStreamIndexes, err := sortingLookup()
		if err != nil {
			return err
		}
		for _, si := range sortedStreamIndexes {
			if limitReached, err := filterAndAddToResult(activeQueryParts, si); err != nil {
				return err
			} else if limitReached {
				break
			}
		}
	}

	// all query parts have lookups, build a map of stream indexes to active query parts
	type streamIndex struct {
		si               uint32
		activeQueryParts bitmask.ShortBitmask
	}
	streamIndexes := []streamIndex(nil)

	// build a list of stream indexes that match any query part
	streamIndexesPosition := map[uint32]int{}
	for qpIdx, qp := range queryParts {
		if !qp.possible {
			continue
		}
		streamIndexesOfQuery := []uint32(nil)
		for _, l := range qp.lookups {
			newStreamIndexes, err := l()
			if err != nil {
				return err
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
		for _, si := range streamIndexesOfQuery {
			pos, ok := streamIndexesPosition[si]
			if ok {
				sis := &streamIndexes[pos]
				sis.activeQueryParts.Set(uint(qpIdx))
			} else {
				streamIndexesPosition[si] = len(streamIndexes)
				streamIndexes = append(streamIndexes, streamIndex{
					si:               si,
					activeQueryParts: bitmask.ShortBitmask{},
				})
				streamIndexes[len(streamIndexes)-1].activeQueryParts.Set(uint(qpIdx))
			}
		}
	}

	// without sorting lookup, we can just evaluate the streams potentially matching
	// any of the query parts in file order, no early exit is possible
	if sortingLookup == nil {
		// sort the stream indexes to allow evaluating the streams in file order
		sort.Slice(streamIndexes, func(i, j int) bool {
			return streamIndexes[i].si < streamIndexes[j].si
		})
		for _, si := range streamIndexes {
			_, err := filterAndAddToResult(si.activeQueryParts, si.si)
			if err != nil {
				return err
			}
		}
		return nil
	}

	sortedStreamIndexes, err := sortingLookup()
	if err != nil {
		return err
	}

	// evaluate the steams using the sort order lookup and test
	// each index against the information from the lookups
	for _, si := range sortedStreamIndexes {
		pos, ok := streamIndexesPosition[si]
		if !ok {
			continue
		}
		aqp := streamIndexes[pos].activeQueryParts
		if limitReached, err := filterAndAddToResult(aqp, si); err != nil {
			return err
		} else if limitReached {
			break
		}
	}
	return nil
}
