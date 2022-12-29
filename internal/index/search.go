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
	"github.com/spq/pkappa2/internal/tools/bitmask"
	regexanalysis "github.com/spq/pkappa2/internal/tools/regexAnalysis"
	"github.com/spq/pkappa2/internal/tools/seekbufio"
	"rsc.io/binaryregexp"
)

type (
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
			remove := old.And(*forbidden[sqi])
			keep := old.Sub(*forbidden[sqi])
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

func (r *Reader) buildSearchObjects(subQuery string, queryPartIndex int, previousResults map[string]resultData, refTime time.Time, q *query.Conditions, superseedingIndexes []*Reader, limitIDs *bitmask.LongBitmask, tagDetails map[string]query.TagDetails, converters map[string]*Filter) (queryPart, error) {
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
	type (
		occ struct {
			condition, element int
		}
		regexVariant struct {
			regex          *binaryregexp.Regexp
			prefix, suffix []byte
			acceptedLength regexanalysis.AcceptedLengths
			childSubQuery  string
			children       []regexVariant
			isPrecondition bool
		}
		regex struct {
			occurence []occ
			root      regexVariant
		}
	)
	regexes := []regex(nil)
	regexConditions := []int(nil)
	regexDependencies := map[string]map[string]struct{}{}
conditions:
	for cIdx, c := range *q {
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
				*r = r.Or(lastSubQueryData[i].ranges)
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
				*r = r.Or(lastSubQueryData[i].ranges)
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
			shouldEvaluate, affectsSubquery := false, false
			filterName := ""
			for i, e := range cc.Elements {
				if i > 0 && e.FilterName != filterName {
					return queryPart{}, errors.New("all 'data then data' conditions must use the same filter")
				}
				filterName = e.FilterName
				if e.SubQuery == subQuery {
					for _, v := range e.Variables {
						if _, ok := previousResults[v.SubQuery]; v.SubQuery != subQuery && !ok {
							return queryPart{}, errors.New("SubQueries not yet fully supported")
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
				return queryPart{}, errors.New("SubQueries not yet fully supported")
			}
			if filterName != "" {
				if _, ok := converters[filterName]; !ok {
					return queryPart{}, errors.New("filter name not found")
				}
			}
			// loop over regexconditions and see if all of them have the same filterName as the curent one
			// TODO: allow this
			for _, rc := range regexConditions {
				dc := (*q)[rc].(*query.DataCondition)
				for _, e := range dc.Elements {
					if e.FilterName != filterName {
						return queryPart{}, errors.New("all data conditions must have the same filter name")
					}
				}
			}
			regexConditions = append(regexConditions, cIdx)
		regexElements:
			for eIdx, e := range cc.Elements {
			previousRegexes:
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
					for vIdx := range e.Variables {
						if e.Variables[vIdx] != oe.Variables[vIdx] {
							continue previousRegexes
						}
					}
					r.occurence = append(r.occurence, occ{
						condition: cIdx,
						element:   eIdx,
					})
					continue regexElements
				}
				regexes = append(regexes, regex{})
				r := &regexes[len(regexes)-1]
				r.occurence = append(r.occurence, occ{
					condition: cIdx,
					element:   eIdx,
				})
				for _, v := range e.Variables {
					if v.SubQuery == "" {
						continue
					}
					if _, ok := previousResults[v.SubQuery]; !ok {
						return queryPart{}, errors.New("SubQueries not yet fully supported")
					}
					dep := regexDependencies[v.SubQuery]
					if dep == nil {
						dep = make(map[string]struct{})
					}
					dep[v.Name] = struct{}{}
					regexDependencies[v.SubQuery] = dep
				}
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
	} else {
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

		impossibleSubQueries := map[string]*bitmask.ConnectedBitmask{}
		type (
			variableValues struct {
				quotedData []string
				results    bitmask.ConnectedBitmask
			}
			subQueryVariableData struct {
				variableIndex map[string]int
				variableData  []variableValues
			}
		)
		possibleSubQueries := map[string]subQueryVariableData{}
		for sq, vars := range regexDependencies {
			varNameIndex := make(map[string]int)
			for v := range vars {
				varNameIndex[v] = len(varNameIndex)
			}
			rd := previousResults[sq]
			badVarData := map[int]struct{}{}
			varData := []variableValues(nil)
			varDataMap := map[int]int{}
		vardata:
			for vdi := range rd.variableData {
				vd := &rd.variableData[vdi]
				if vd.uses == 0 {
					continue
				}
				quotedData := make([]string, len(varNameIndex))
				for v, vIdx := range varNameIndex {
					quoted := ""
					for _, d := range vd.data {
						if d.queryParts.IsSet(uint(queryPartIndex)) && d.name != v {
							continue
						}
						quoted += binaryregexp.QuoteMeta(d.value) + "|"
					}
					if quoted == "" {
						badVarData[vdi] = struct{}{}
						continue vardata
					}
					quotedData[vIdx] = quoted[:len(quoted)-1]
				}
			varDataElement:
				for i := range varData {
					vde := &varData[i]
					for j := range quotedData {
						if quotedData[j] != vde.quotedData[j] {
							continue varDataElement
						}
					}
					varDataMap[vdi] = i
					continue vardata
				}
				varDataMap[vdi] = len(varData)
				varData = append(varData, variableValues{
					quotedData: quotedData,
				})
			}
			possible := false
			impossible := &bitmask.ConnectedBitmask{}
			for sIdx, s := range rd.streams {
				if vdi, ok := rd.variableAssociation[s.StreamID]; ok {
					if _, ok := badVarData[vdi]; !ok {
						varData[varDataMap[vdi]].results.Set(uint(sIdx))
						possible = true
						continue
					}
				}
				// this stream can not succeed as it does not have the right variables
				impossible.Set(uint(sIdx))
			}
			if !possible {
				return queryPart{}, nil
			}
			if !impossible.IsZero() {
				impossibleSubQueries[sq] = impossible
			}
			possibleSubQueries[sq] = subQueryVariableData{
				variableIndex: varNameIndex,
				variableData:  varData,
			}
		}
		for rIdx := range regexes {
			r := &regexes[rIdx]
			o := &r.occurence[0]
			c := (*q)[o.condition].(*query.DataCondition)
			e := &c.Elements[o.element]
			if len(e.Variables) == 0 {
				var err error
				if r.root.regex, err = binaryregexp.Compile(e.Regex); err != nil {
					return queryPart{}, err
				}
				prefix, complete := r.root.regex.LiteralPrefix()
				r.root.prefix = []byte(prefix)
				if complete {
					r.root.acceptedLength = regexanalysis.AcceptedLengths{
						MinLength: uint(len(prefix)),
						MaxLength: uint(len(prefix)),
					}
					r.root.suffix = r.root.prefix
				} else {
					if r.root.acceptedLength, err = regexanalysis.AcceptedLength(e.Regex); err != nil {
						return queryPart{}, err
					}
					if r.root.suffix, err = regexanalysis.ConstantSuffix(e.Regex); err != nil {
						return queryPart{}, err
					}
				}
				continue
			}

			precomputeSubQueries := []string{""}
			usesLocalVariables := false
		variables:
			for _, v := range e.Variables {
				if v.SubQuery == "" {
					usesLocalVariables = true
					continue
				}
				for _, sq := range precomputeSubQueries[1:] {
					if sq == v.SubQuery {
						continue variables
					}
				}
				precomputeSubQueries = append(precomputeSubQueries, v.SubQuery)
			}
			variantCount := map[string]int{
				"": 1,
			}
			for _, sq := range precomputeSubQueries[1:] {
				variantCount[sq] = len(possibleSubQueries[sq].variableData)
			}
			if usesLocalVariables {
				precomputeSubQueries = precomputeSubQueries[:1]
			} else {
				sort.Slice(precomputeSubQueries[1:], func(i, j int) bool {
					a, b := precomputeSubQueries[i+1], precomputeSubQueries[j+1]
					return variantCount[a] < variantCount[b]
				})
				count := 1
				for l, sq := range precomputeSubQueries[1:] {
					if count >= 10_000 {
						precomputeSubQueries = precomputeSubQueries[:l+1]
						break
					}
					count *= variantCount[sq]
				}
			}
			for depth := range precomputeSubQueries {
				position := make([]int, depth+1)

			variants:
				for {
					isPrecondition := false
					regex := e.Regex
					for i := len(e.Variables) - 1; i >= 0; i-- {
						v := e.Variables[i]
						content := ""
						if v.SubQuery == "" {
							//TODO: maybe extract the regex for this variable
							content = ".*"
							isPrecondition = true
						} else {
							psq := possibleSubQueries[v.SubQuery]
							vdMin, vdMax := 0, variantCount[v.SubQuery]
							for pIdx, sq := range precomputeSubQueries[1 : depth+1] {
								if v.SubQuery == sq {
									pos := position[pIdx+1]
									vdMin, vdMax = pos, pos+1
									break
								}
							}
							vIdx := psq.variableIndex[v.Name]
							for vdIdx := vdMin; vdIdx < vdMax; vdIdx++ {
								content += psq.variableData[vdIdx].quotedData[vIdx] + "|"
							}
							content = content[:len(content)-1]
							if vdMax-vdMin != 1 {
								isPrecondition = true
							}
						}
						regex = regex[:v.Position] + "(?:" + content + ")" + regex[v.Position:]
					}
					root := &r.root
					for _, p := range position[1:] {
						root = &root.children[p]
					}
					if depth+1 < len(precomputeSubQueries) {
						root.childSubQuery = precomputeSubQueries[depth+1]
						root.children = make([]regexVariant, variantCount[root.childSubQuery])
					}

					var err error
					if root.regex, err = binaryregexp.Compile(regex); err != nil {
						return queryPart{}, err
					}
					prefix, complete := root.regex.LiteralPrefix()
					root.prefix = []byte(prefix)
					if complete {
						root.acceptedLength = regexanalysis.AcceptedLengths{
							MinLength: uint(len(prefix)),
							MaxLength: uint(len(prefix)),
						}
						root.suffix = root.prefix
					} else {
						if root.acceptedLength, err = regexanalysis.AcceptedLength(regex); err != nil {
							return queryPart{}, err
						}
						if root.suffix, err = regexanalysis.ConstantSuffix(regex); err != nil {
							return queryPart{}, err
						}
					}
					root.isPrecondition = isPrecondition

					for pIdx := range position[1:] {
						pIdx++
						p := &position[pIdx]
						*p++
						if *p < variantCount[precomputeSubQueries[pIdx]] {
							continue variants
						}
						*p = 0
					}
					break
				}
			}
		}
		if len(impossibleSubQueries) != 0 {
			filters = append(filters, func(sc *searchContext, s *stream) (bool, error) {
				for sq, imp := range impossibleSubQueries {
					sc.allowedSubQueries.remove([]string{sq}, []*bitmask.ConnectedBitmask{imp})
				}
				return !sc.allowedSubQueries.empty(), nil
			})
		}

		//add filter for scanning the data section
		sr := io.NewSectionReader(r.file, int64(r.header.Sections[sectionData].Begin), r.header.Sections[sectionData].size())
		br := seekbufio.NewSeekableBufferReader(sr)
		buffers := [2][]byte{}
		filters = append(filters, func(sc *searchContext, s *stream) (bool, error) {
			type (
				progressVariantFlag byte
				progressVariant     struct {
					streamOffset [2]int
					// how many regexes were sucessful
					nSuccessful int
					// the variables collected on the way
					variables map[string]string
					// the regex to use
					regex *binaryregexp.Regexp
					// the accepted length by the regex
					acceptedLength regexanalysis.AcceptedLengths
					// the prefix of the regex
					prefix []byte
					// the suffix of the regex
					suffix []byte
					// the variants chosen for this progress
					variant map[string]int
					// flags for this progress
					flags progressVariantFlag
				}
				progressGroup struct {
					variants []progressVariant
				}
			)
			const (
				progressVariantFlagState                    progressVariantFlag = 3
				progressVariantFlagStateUninitialzed        progressVariantFlag = 0
				progressVariantFlagStateExact               progressVariantFlag = 1
				progressVariantFlagStatePrecondition        progressVariantFlag = 2
				progressVariantFlagStatePreconditionMatched progressVariantFlag = 3
			)
			const (
				C2S = query.DataRequirementSequenceFlagsDirectionClientToServer / query.DataRequirementSequenceFlagsDirection
				S2C = query.DataRequirementSequenceFlagsDirectionServerToClient / query.DataRequirementSequenceFlagsDirection
			)
			progressGroups := make([]progressGroup, len(*q))
			for _, pIdx := range regexConditions {
				progressGroups[pIdx] = progressGroup{
					variants: make([]progressVariant, 1),
				}
			}

			o := regexes[0].occurence[0]
			filterName := (*q)[o.condition].(*query.DataCondition).Elements[o.element].FilterName
			streamLength := [2]int{}
			bufferLengths := [][2]int{{}}

			if filterName == "" {
				streamLength[C2S] = int(s.ClientBytes)
				streamLength[S2C] = int(s.ServerBytes)

				// read the data
				if _, err := br.Seek(int64(s.DataStart), io.SeekStart); err != nil {
					return false, err
				}
				for dir := range [2]int{C2S, S2C} {
					l := streamLength[dir]
					if cap(buffers[dir]) < l {
						buffers[dir] = make([]byte, l)
					} else {
						buffers[dir] = buffers[dir][:l]
					}
					if err := binary.Read(br, binary.LittleEndian, buffers[dir]); err != nil {
						return false, err
					}
				}
				// read the direction chunk sizes
				for dir := C2S; ; dir ^= C2S ^ S2C {
					last := bufferLengths[len(bufferLengths)-1]
					if last[C2S] == streamLength[C2S] && last[S2C] == streamLength[S2C] {
						break
					}
					sz := uint64(0)
					for {
						b := byte(0)
						if err := binary.Read(br, binary.LittleEndian, &b); err != nil {
							return false, err
						}
						sz <<= 7
						sz |= uint64(b & 0x7f)
						if b < 128 {
							break
						}
					}
					if sz == 0 {
						continue
					}
					new := [2]int{
						last[0],
						last[1],
					}
					new[dir] += int(sz)
					bufferLengths = append(bufferLengths, new)
				}
			} else {
				converter, ok := converters[filterName]
				if !ok {
					return false, fmt.Errorf("unknown filter %q", filterName)
				}
				if !converter.HasStream(s.StreamID) {
					return false, nil
				}

				data, dataSizes, clientBytes, serverBytes, err := converter.DataForSearch(s.StreamID)
				if err != nil {
					return false, fmt.Errorf("data for search %w", err)
				}
				streamLength[C2S] = int(clientBytes)
				streamLength[S2C] = int(serverBytes)
				buffers = data
				bufferLengths = dataSizes
			}
			// sdata.b64decode:hallo AND cdata:hi
			for {
				recheckRegexes := false
				for rIdx := range regexes {
					r := &regexes[rIdx]
					for _, o := range r.occurence {
						e := (*q)[o.condition].(*query.DataCondition).Elements[o.element]
						dir := (e.Flags & query.DataRequirementSequenceFlagsDirection) / query.DataRequirementSequenceFlagsDirection

						ps := &progressGroups[o.condition]
					outer2:
						for pIdx := 0; pIdx < len(ps.variants); pIdx++ {
							p := &ps.variants[pIdx]
							if o.element != p.nSuccessful {
								continue
							}
							if p.regex == nil {
								root := &r.root
								for {
									if root.childSubQuery == "" {
										break
									}
									v, ok := p.variant[root.childSubQuery]
									if !ok {
										break
									}
									root = &root.children[v]
								}
								explodeOneVariant := false
								switch p.flags & progressVariantFlagState {
								case progressVariantFlagStateUninitialzed:
									if root.regex != nil {
										p.regex = root.regex
										p.prefix = root.prefix
										p.suffix = root.suffix
										p.acceptedLength = root.acceptedLength
										if root.isPrecondition {
											p.flags = progressVariantFlagStatePrecondition
										} else {
											p.flags = progressVariantFlagStateExact
										}
									}
								case progressVariantFlagStateExact:
									panic("why am i here?")
								case progressVariantFlagStatePrecondition:
									panic("why am i here?")
								case progressVariantFlagStatePreconditionMatched:
									if root.childSubQuery == "" {
										explodeOneVariant = true
										break
									}
									for cIdx, c := range root.children[1:] {
										np := progressVariant{
											streamOffset:   p.streamOffset,
											nSuccessful:    p.nSuccessful,
											regex:          c.regex,
											acceptedLength: c.acceptedLength,
											prefix:         c.prefix,
											suffix:         c.suffix,
											variant: map[string]int{
												root.childSubQuery: cIdx,
											},
										}
										for sq, v := range p.variant {
											np.variant[sq] = v
										}
										if p.variables != nil {
											np.variables = make(map[string]string)
											for n, v := range p.variables {
												np.variables[n] = v
											}
										}
										if c.isPrecondition {
											np.flags = progressVariantFlagStatePrecondition
										} else {
											np.flags = progressVariantFlagStateExact
										}
										if cIdx == 0 {
											ps.variants[pIdx] = np
										} else {
											ps.variants = append(ps.variants, np)
										}
									}
									p = &ps.variants[pIdx]
								}

								if p.regex == nil {
									expr := e.Regex
									p.flags = progressVariantFlagStateExact
									for i := len(e.Variables) - 1; i >= 0; i-- {
										v := e.Variables[i]
										content := ""
										if v.SubQuery == "" {
											ok := false
											content, ok = p.variables[v.Name]
											if !ok {
												return false, fmt.Errorf("variable %q not defined", v.Name)
											}
											content = binaryregexp.QuoteMeta(content)
										} else {
											psq := possibleSubQueries[v.SubQuery]
											vIdx := psq.variableIndex[v.Name]
											variant, ok := p.variant[v.SubQuery]
											if ok || explodeOneVariant {
												if !ok {
													explodeOneVariant = false
													// we have not yet split this progress element
													// the precondition regex matched, split this progress element
													for j := 1; j < len(psq.variableData); j++ {
														np := progressVariant{
															streamOffset: p.streamOffset,
															nSuccessful:  p.nSuccessful,
															flags:        progressVariantFlagStateUninitialzed,
															variant:      map[string]int{v.SubQuery: j},
														}
														for k, v := range p.variant {
															np.variant[k] = v
														}
														if p.variables != nil {
															np.variables = make(map[string]string)
															for n, v := range p.variables {
																np.variables[n] = v
															}
														}
														ps.variants = append(ps.variants, np)
													}
													p = &ps.variants[pIdx]
													if p.variant == nil {
														p.variant = make(map[string]int)
													}
													p.variant[v.SubQuery] = 0
												}
												content = psq.variableData[variant].quotedData[vIdx]
											} else {
												p.flags = progressVariantFlagStatePrecondition
												for _, vd := range psq.variableData {
													content += vd.quotedData[vIdx] + "|"
												}
												content = content[:len(content)-1]
											}
										}
										expr = fmt.Sprintf("%s(?:%s)%s", expr[:v.Position], content, expr[v.Position:])
									}
									var err error
									if p.regex, err = binaryregexp.Compile(expr); err != nil {
										return false, err
									}
									prefix, complete := p.regex.LiteralPrefix()
									root.prefix = []byte(prefix)
									if complete {
										p.acceptedLength = regexanalysis.AcceptedLengths{
											MinLength: uint(len(prefix)),
											MaxLength: uint(len(prefix)),
										}
										root.suffix = root.prefix
									} else {
										if p.acceptedLength, err = regexanalysis.AcceptedLength(expr); err != nil {
											return false, err
										}
										if p.suffix, err = regexanalysis.ConstantSuffix(expr); err != nil {
											return false, err
										}
									}
								}
							}

							buffer := buffers[dir][p.streamOffset[dir]:]
							if uint(len(buffer)) < p.acceptedLength.MinLength {
								continue
							}

							if len(p.prefix) != 0 {
								//the regex has a prefix, find it
								pos := bytes.Index(buffer, p.prefix)
								if pos < 0 {
									// the prefix is not in the string, we can discard part of the buffer
									p.streamOffset[dir] = len(buffers[dir])
									continue
								}
								//skip the part that doesn't have the prefix
								p.streamOffset[dir] += pos
								buffer = buffer[pos:]
								if uint(len(buffer)) < p.acceptedLength.MinLength {
									continue
								}
							}
							if len(p.suffix) != 0 {
								//the regex has a suffix, find it
								pos := bytes.LastIndex(buffer, p.suffix)
								if pos < 0 {
									// the suffix is not in the string, we can discard part of the buffer
									p.streamOffset[dir] = len(buffers[dir])
									continue
								}
								//drop the part that doesn't have the suffix
								buffer = buffer[:pos+len(p.suffix)]
								if uint(len(buffer)) < p.acceptedLength.MinLength {
									continue
								}
							}

							var res []int
							if p.acceptedLength.MinLength == p.acceptedLength.MaxLength && len(p.prefix) == 0 && len(p.suffix) != 0 {
								beforeSuffixLen := int(p.acceptedLength.MinLength) - len(p.suffix)
								for {
									pos := bytes.Index(buffer[beforeSuffixLen:], p.suffix)
									if pos < 0 {
										p.streamOffset[dir] = len(buffers[dir])
										continue outer2
									}
									p.streamOffset[dir] += pos
									buffer = buffer[pos:]
									res = p.regex.FindSubmatchIndex(buffer[:p.acceptedLength.MinLength])
									if res != nil {
										break
									}
									p.streamOffset[dir]++
									buffer = buffer[1:]
								}
							} else {
								res = p.regex.FindSubmatchIndex(buffer)
							}

							if res == nil {
								p.streamOffset[dir] = len(buffers[dir])
								continue
							}
							if p.flags&progressVariantFlagState == progressVariantFlagStatePrecondition {
								recheckRegexes = true
								p.regex = nil
								p.flags += progressVariantFlagStatePreconditionMatched - progressVariantFlagStatePrecondition
								continue
							}
							p.nSuccessful++
							d := (*q)[o.condition].(*query.DataCondition)
							if p.nSuccessful != len(d.Elements) {
								// remember that we advanced a sequence that has a follow up and we have to re-check the regexes
								recheckRegexes = true
							} else if d.Inverted {
								return false, nil
							}
							variableNames := p.regex.SubexpNames()
							p.regex = nil
							p.flags = 0
							for i := 2; i < len(res); i += 2 {
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
								p.variables[varName] = string(buffer[res[i]:res[i+1]])
							}

							if res[1] != 0 {
								// update stream offsets: a follow up regex for the same direction
								// may consume the byte following the match, a regex for the other
								// direction may start reading from the next received packet,
								// so everything read before is out-of reach.
								p.streamOffset[dir] += res[1]
								for i := len(bufferLengths) - 1; ; i-- {
									if bufferLengths[i-1][dir] < p.streamOffset[dir] {
										p.streamOffset[(C2S^S2C)-dir] = bufferLengths[i][(C2S^S2C)-dir]
										break
									}
								}
							}
						}
					}
				}
				if !recheckRegexes {
					break
				}
			}

			// check if any of the regexe's failed and collect variable contents
			for _, cIdx := range regexConditions {
				d := (*q)[cIdx].(*query.DataCondition)
				pg := &progressGroups[cIdx]
				for pIdx := range pg.variants {
					p := &pg.variants[pIdx]
					nUnsuccessful := len(d.Elements) - p.nSuccessful
					if nUnsuccessful >= 2 || (nUnsuccessful != 0) != d.Inverted {
						if len(p.variant) == 0 {
							return false, nil
						}
						sqs := []string(nil)
						forbidden := []*bitmask.ConnectedBitmask(nil)
						for sq, v := range p.variant {
							sqs = append(sqs, sq)
							badSQR := &possibleSubQueries[sq].variableData[v].results
							forbidden = append(forbidden, badSQR)

						}
						sc.allowedSubQueries.remove(sqs, forbidden)
						if sc.allowedSubQueries.empty() {
							return false, nil
						}
						continue
					}
					if p.variables == nil {
						continue
					}
					if sc.outputVariables == nil {
						sc.outputVariables = make(map[string][]string)
					}
				outer:
					for n, v := range p.variables {
						values := sc.outputVariables[n]
						for _, on := range values {
							if n == on {
								continue outer
							}
						}
						sc.outputVariables[n] = append(values, v)
					}
				}
			}
			return true, nil
		})
	}
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
			at := a.r.ReferenceTime.Add(time.Nanosecond * time.Duration(a.stream.FirstPacketTimeNS))
			bt := b.r.ReferenceTime.Add(time.Nanosecond * time.Duration(b.stream.FirstPacketTimeNS))
			return at.Before(bt)
		},
		query.SortingKeyLastPacketTime: func(a, b *Stream) bool {
			if a.r == b.r {
				return a.stream.LastPacketTimeNS < b.stream.LastPacketTimeNS
			}
			at := a.r.ReferenceTime.Add(time.Nanosecond * time.Duration(a.stream.LastPacketTimeNS))
			bt := b.r.ReferenceTime.Add(time.Nanosecond * time.Duration(b.stream.LastPacketTimeNS))
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
	}
)

func SearchStreams(indexes []*Reader, filters map[string]*Filter, limitIDs *bitmask.LongBitmask, refTime time.Time, qs query.ConditionsSet, grouping *query.Grouping, sorting []query.Sorting, limit, skip uint, tagDetails map[string]query.TagDetails) ([]*Stream, bool, error) {
	if len(qs) == 0 {
		return nil, false, nil
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
				return nil, false, errors.New("SubQueries not yet fully supported")
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
							idx.readObject(section, 0, 0, res)
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
				queryPart, err := idx.buildSearchObjects(subQuery, qID, allResults, refTime, &qs[qID], indexes[idxIdx+1:], limitIDs, tagDetails, filters)
				if err != nil {
					return nil, false, err
				}
				if queryPart.possible && len(queryPart.lookups) == 0 && sortingLookup != nil {
					queryPart.lookups = append(queryPart.lookups, sortingLookup)
				}
				queryParts = append(queryParts, queryPart)
			}
			err := idx.searchStreams(&results, filters, allResults, queryParts, groupingData, sorter, resultLimit)
			if err != nil {
				return nil, false, err
			}
		}
		if len(results.streams) == 0 {
			return nil, false, nil
		}
		allResults[subQuery] = results
	}
	results := allResults[""]
	if uint(len(results.streams)) <= skip {
		return nil, false, nil
	}
	return results.streams[skip:], results.resultDropped != 0, nil
}

func (r *Reader) searchStreams(result *resultData, filters map[string]*Filter, subQueryResults map[string]resultData, queryParts []queryPart, grouper *grouper, sortingLess func(a, b *Stream) bool, limit uint) error {
	// check if all queries use lookups, if not don't evaluate them
	useLookups := true
	allImpossible := true
	for _, qp := range queryParts {
		if qp.possible {
			allImpossible = false
		}
		if len(qp.lookups) == 0 {
			useLookups = false
		}
	}
	if allImpossible {
		return nil
	}
	// a map of index to list of sub-queries that matched this id
	type streamIndex struct {
		si               uint32
		activeQueryParts bitmask.ShortBitmask
	}
	streamIndexes := []streamIndex(nil)
	if useLookups {
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
		if len(streamIndexes) == 0 {
			return nil
		}
	}

	// apply filters to lookup results or all streams, if no lookups could be used
	filterAndAddToResult := func(activeQueryParts bitmask.ShortBitmask, si uint32) error {
		s, err := r.streamByIndex(si)
		if err != nil {
			return err
		}
		ss, err := s.wrap(r, si)
		if err != nil {
			return err
		}

		// check if the sorting and limit would allow this stream
		if result.resultDropped != 0 && limit != 0 && uint(len(result.streams)) >= limit && (sortingLess == nil || !sortingLess(ss, result.streams[limit-1])) {
			return nil
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
					return nil
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
					return err
				}
				if !matching {
					continue queryPart
				}
			}
			matchingQueryParts.Set(uint(qpIdx))
			matchingSearchContexts = append(matchingSearchContexts, sc)
		}
		if matchingQueryParts.IsZero() {
			return nil
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
					return nil
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
				return nil
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
			return nil
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
			return nil
		}
		if freeSlot == len(result.variableData) {
			result.variableData = append(result.variableData, variableDataCollection{})
		}
		result.variableData[freeSlot] = variableDataCollection{
			uses: 1,
			data: vdv,
		}
		result.variableAssociation[s.StreamID] = freeSlot
		return nil
	}
	if len(streamIndexes) != 0 {
		for _, si := range streamIndexes {
			if err := filterAndAddToResult(si.activeQueryParts, si.si); err != nil {
				return err
			}
		}
	} else {
		activeQueryParts := bitmask.ShortBitmask{}
		for qpIdx, qp := range queryParts {
			if qp.possible {
				activeQueryParts.Set(uint(qpIdx))
			}
		}
		for si, sc := 0, r.StreamCount(); si < sc; si++ {
			if err := filterAndAddToResult(activeQueryParts, uint32(si)); err != nil {
				return err
			}
		}
	}
	return nil
}
