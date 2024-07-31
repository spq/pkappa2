package builder

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/ip4defrag"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/reassembly"
	"github.com/spq/pkappa2/internal/index"
	"github.com/spq/pkappa2/internal/index/streams"
	"github.com/spq/pkappa2/internal/index/udpreassembly"
	"github.com/spq/pkappa2/internal/tools"
	pcapmetadata "github.com/spq/pkappa2/internal/tools/pcapMetadata"
)

type (
	Builder struct {
		snapshots        []*snapshot
		knownPcaps       []*pcapmetadata.PcapInfo
		indexDir         string
		snapshotDir      string
		snapshotFilename string
	}
)

func New(pcapDir, indexDir, snapshotDir string, cachedKnownPcaps []*pcapmetadata.PcapInfo) (*Builder, error) {
	b := Builder{
		indexDir:    indexDir,
		snapshotDir: snapshotDir,
	}
	cachedKnownPcapsMap := map[string]*pcapmetadata.PcapInfo{}
	for _, p := range cachedKnownPcaps {
		cachedKnownPcapsMap[p.Filename] = p
	}
	// read all existing pcaps to build the info structs
	pcaps, err := os.ReadDir(pcapDir)
	if err != nil {
		return nil, err
	}
	for _, p := range pcaps {
		if p.IsDir() || (!strings.HasSuffix(p.Name(), ".pcap") && !strings.HasSuffix(p.Name(), ".pcapng")) {
			continue
		}
		pInfo, err := p.Info()
		if err != nil {
			log.Printf("error stat pcap %s: %v", p.Name(), err)
			continue
		}
		info := cachedKnownPcapsMap[p.Name()]
		if info == nil || info.Filesize != uint64(pInfo.Size()) {
			info, _, err = readPackets(pcapDir, p.Name(), nil)
			if err != nil {
				log.Printf("error reading pcap %s: %v", p.Name(), err)
				continue
			}
		}
		b.knownPcaps = append(b.knownPcaps, info)
	}
	// load the snapshot file with the most packets covered
	snapshotFiles, err := os.ReadDir(snapshotDir)
	if err != nil {
		return nil, err
	}
	chunkCounts := uint64(0)
	for _, f := range snapshotFiles {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".snap") {
			continue
		}
		snapshots, err := loadSnapshots(filepath.Join(snapshotDir, f.Name()))
		if err != nil {
			log.Printf("loadSnapshots(%q) failed: %v", f.Name(), err)
			continue
		}
		currentChunkCounts := uint64(0)
		for _, s := range snapshots {
			currentChunkCounts += s.chunkCount
		}
		if chunkCounts < currentChunkCounts {
			b.snapshots = snapshots
			b.snapshotFilename = f.Name()
			chunkCounts = currentChunkCounts
		}
	}
	return &b, nil
}

func (b *Builder) FromPcap(pcapDir string, pcapFilenames []string, existingIndexes []*index.Reader) (int, []*index.Reader, error) {
	log.Printf("Building indexes from pcaps %q\n", pcapFilenames)
	// load, find ts of oldest new package
	newPcapInfos := []*pcapmetadata.PcapInfo(nil)
	newPackets := []Packet(nil)
	oldestTs := time.Time{}
	nProcessedPcaps := 0
	for _, pcapFilename := range pcapFilenames {
		knownPcapInfo := (*pcapmetadata.PcapInfo)(nil)
		for _, p := range b.knownPcaps {
			if p.Filename == pcapFilename {
				knownPcapInfo = p
				break
			}
		}
		pcapInfo, pcapPackets, err := readPackets(pcapDir, pcapFilename, knownPcapInfo)
		if err != nil {
			log.Printf("readPackets(%q) failed: %v", pcapFilename, err)
			if nProcessedPcaps == 0 {
				// report that we failed to process a single pcap,
				// the caller can then decide what to do...
				return 1, nil, err
			}
			// process the other pcaps that we already loaded and
			// let the next run deal with the problematic pcap...
			break
		}
		log.Printf("Loaded %d packets from pcap file %q\n", len(pcapPackets), pcapFilename)
		nProcessedPcaps++
		if len(pcapPackets) == 0 {
			continue
		}
		newPcapInfos = append(newPcapInfos, pcapInfo)
		newPackets = append(newPackets, pcapPackets...)
		if oldestTs.IsZero() || oldestTs.After(pcapInfo.PacketTimestampMin) {
			oldestTs = pcapInfo.PacketTimestampMin
		}
		if len(newPackets) >= 10_000_000 {
			break
		}
	}
	if len(newPackets) == 0 {
		return nProcessedPcaps, nil, nil
	}

	// find last snapshot with ts < oldest new package
	bestSnapshot := &snapshot{}
	for _, ss := range b.snapshots {
		// ignore snapshots older than the best
		if bestSnapshot.timestamp.After(ss.timestamp) {
			continue
		}
		// ignore snapshots younger than our packets
		if oldestTs.Before(ss.timestamp) {
			continue
		}
		bestSnapshot = ss
	}
	if bestSnapshot.timestamp.IsZero() {
		log.Printf("Using no snapshot\n")
	} else {
		log.Printf("Using snapshot missing %s\n", oldestTs.Sub(bestSnapshot.timestamp).String())
	}

	// select all pcaps that need to be loaded
	allNeededPcaps := []*pcapmetadata.PcapInfo(nil)
outer:
	for _, pcap := range b.knownPcaps {
		for _, newPcap := range newPcapInfos {
			if pcap == newPcap {
				continue outer
			}
		}
		packetIndexes := bestSnapshot.referencedPackets[pcap.Filename]
		if !bestSnapshot.timestamp.After(pcap.PacketTimestampMax) || len(packetIndexes) != 0 {
			allNeededPcaps = append(allNeededPcaps, pcap)
		}
	}
	sort.Slice(allNeededPcaps, func(i, j int) bool {
		a, b := allNeededPcaps[i], allNeededPcaps[j]
		return a.PacketTimestampMin.Before(b.PacketTimestampMin)
	})

	// create empty reassemblers
	ip4defragmenter := ip4defrag.NewIPv4Defragmenter()

	streamFactory := &streams.StreamFactory{}
	tcpAssembler := [0x100]*reassembly.Assembler{}
	for i := range tcpAssembler {
		pool := reassembly.NewStreamPool(streamFactory)
		tcpAssembler[i] = reassembly.NewAssembler(pool)
	}
	udpAssembler := udpreassembly.NewAssembler(streamFactory)

	nPacketsAfterSnapshot := uint64(0)
	previousPacketTimestamp := time.Time{}
	newSnapshots := []*snapshot{}
	for _, s := range b.snapshots {
		if !bestSnapshot.timestamp.Before(s.timestamp) {
			// s.ts <= b.ts
			newSnapshots = append(newSnapshots, s)
		}
	}

	// sort all loaded packets by timestamp or packet index or pcap filename
	comparePackets := func(a, b *Packet) bool {
		if !a.Timestamp().Equal(b.Timestamp()) {
			return a.Timestamp().Before(b.Timestamp())
		}
		apmd := pcapmetadata.FromPacketMetadata(a.CaptureInfo())
		bpmd := pcapmetadata.FromPacketMetadata(b.CaptureInfo())
		if apmd.PcapInfo != bpmd.PcapInfo {
			return apmd.PcapInfo.Filename < bpmd.PcapInfo.Filename
		}
		return apmd.Index < bpmd.Index
	}
	sortPackets := func(ps []Packet) {
		sort.Slice(ps, func(i, j int) bool {
			return comparePackets(&ps[i], &ps[j])
		})
	}
	sortPackets(newPackets)
	oldPackets := []Packet(nil)
	newPacketIndex := 0
	for pcapIndex := -1; pcapIndex < len(allNeededPcaps); pcapIndex++ {
		if pcapIndex >= 0 {
			pcap := allNeededPcaps[pcapIndex]
			packets := []Packet(nil)
			var err error
			_, packets, err = readPackets(pcapDir, pcap.Filename, pcap)
			if err != nil {
				// we couldn't load an old pcap that contains packets that we
				// have to re-evaluate, if we just continue here, we lose data.
				return 0, nil, err
			}
			if bestSnapshot.timestamp.After(pcap.PacketTimestampMin) {
				packetIndexes := bestSnapshot.referencedPackets[pcap.Filename]
				neededPackets := []Packet(nil)
				for _, i := range packetIndexes {
					neededPackets = append(neededPackets, packets[i])
				}
				if !bestSnapshot.timestamp.After(pcap.PacketTimestampMax) {
					for _, p := range packets {
						if !bestSnapshot.timestamp.After(p.Timestamp()) {
							neededPackets = append(neededPackets, p)
						}
					}
				}
				packets = neededPackets
			}
			log.Printf("Loaded %d packets from pcap file %q\n", len(packets), pcap.Filename)
			if len(oldPackets) == 0 {
				oldPackets = packets
			} else {
				oldPackets = append(oldPackets, packets...)
			}
			packets = nil
			sortPackets(oldPackets)
		}

		// if we have a next pcap, stop processing packets when the next pcap needs to be loaded
		loadNextTimestamp := time.Time{}
		if pcapIndex+1 < len(allNeededPcaps) {
			nextPcap := allNeededPcaps[pcapIndex+1]
			loadNextTimestamp = nextPcap.PacketTimestampMin
		}

		// process all packets that we should process before loading the next pcap

		for oldPacketIndex := 0; ; {
			useOld := oldPacketIndex < len(oldPackets)
			if useNew := newPacketIndex < len(newPackets); !(useOld || useNew) {
				oldPackets = nil
				break
			} else if useOld && useNew {
				useOld = comparePackets(&oldPackets[oldPacketIndex], &newPackets[newPacketIndex])
			}
			packet := (*Packet)(nil)
			if useOld {
				packet = &oldPackets[oldPacketIndex]
				oldPacketIndex++
			} else {
				packet = &newPackets[newPacketIndex]
				newPacketIndex++
			}
			ts := packet.Timestamp()
			if !(loadNextTimestamp.IsZero() || loadNextTimestamp.After(ts)) {
				// drop all processed packets from the slice if we reached a packet that requires the next pcap to be loaded
				if useOld {
					oldPackets = oldPackets[oldPacketIndex-1:]
				} else {
					oldPackets = oldPackets[oldPacketIndex:]
					newPacketIndex--
				}
				break
			}
			// create new snapshots for packets after snapshot referenced ones
			tsTimeouted := ts.Add(streams.InactivityTimeout)
			if nPacketsAfterSnapshot >= 100_000 && !ts.Equal(previousPacketTimestamp) {
				udpAssembler.FlushCloseOlderThan(tsTimeouted)
				for _, a := range tcpAssembler {
					a.FlushCloseOlderThan(tsTimeouted)
				}
				// create new snapshot
				referencedPackets := map[string][]uint64{}
				// TODO: dump packets from ip4defragmenter
				timeoutedStreams := 0
				worstStreams := [2]struct {
					duration time.Duration
					packets  int
				}{}
				for _, s := range streamFactory.Streams {
					if s.Flags&streams.StreamFlagsComplete != 0 {
						continue
					}
					firstPacketTs := s.Packets[0].Timestamp
					lastPacketTs := s.Packets[len(s.Packets)-1].Timestamp
					if lastPacketTs.Before(tsTimeouted) {
						timeoutedStreams++
						continue
					}
					streamDuration := lastPacketTs.Sub(firstPacketTs)
					if worstStreams[0].duration < streamDuration {
						worstStreams[0].duration = streamDuration
						worstStreams[0].packets = len(s.Packets)
					}
					if worstStreams[1].packets < len(s.Packets) {
						worstStreams[1].duration = streamDuration
						worstStreams[1].packets = len(s.Packets)
					}

					for _, p := range s.Packets {
						pmds := pcapmetadata.AllFromPacketMetadata(&p)
						for _, pmd := range pmds {
							referencedPackets[pmd.PcapInfo.Filename] = append(referencedPackets[pmd.PcapInfo.Filename], pmd.Index)
						}
					}
				}
				if timeoutedStreams != 0 {
					log.Printf("There were %d timeouted streams o_O\n", timeoutedStreams)
				}
				log.Printf("Worst streams: duration: %s (%d packets) packets: %d (%s duration)\n",
					worstStreams[0].duration.String(),
					worstStreams[0].packets,
					worstStreams[1].packets,
					worstStreams[1].duration.String(),
				)
				newSnapshots = compactSnapshots(append(newSnapshots, &snapshot{
					timestamp:         ts,
					chunkCount:        1,
					referencedPackets: referencedPackets,
				}))
				nPacketsAfterSnapshot = 0
			}
			if nPacketsAfterSnapshot != 0 || !bestSnapshot.timestamp.After(ts) {
				previousPacketTimestamp = ts
				nPacketsAfterSnapshot++
			}

			// process packet with ip, tcp & udp reassemblers
			func() {
				parsed := packet.Parsed()
				network := parsed.NetworkLayer()
				if network == nil {
					return
				}
				switch network.LayerType() {
				case layers.LayerTypeIPv4:
					ip4defragmenter.DiscardOlderThan(tsTimeouted)
					defragmented, err := ip4defragmenter.DefragIPv4WithTimestamp(network.(*layers.IPv4), ts)
					if err != nil {
						pmd := pcapmetadata.FromPacketMetadata(packet.CaptureInfo())
						log.Printf("Bad packet %s:%d: %v", pmd.PcapInfo.Filename, pmd.Index, err)
						return
					}
					if defragmented == nil {
						return
					}
					if defragmented != network {
						b := gopacket.NewSerializeBuffer()
						ipPayload, _ := b.PrependBytes(len(defragmented.Payload))
						copy(ipPayload, defragmented.Payload)
						pmd := pcapmetadata.FromPacketMetadata(packet.CaptureInfo())
						if err := defragmented.SerializeTo(b, gopacket.SerializeOptions{
							FixLengths:       true,
							ComputeChecksums: true,
						}); err != nil {
							log.Printf("Bad packet %s:%d: %v", pmd.PcapInfo.Filename, pmd.Index, err.Error())
							return
						}
						newPacket := gopacket.NewPacket(b.Bytes(), layers.LayerTypeIPv4, gopacket.Default)
						if err := newPacket.ErrorLayer(); err != nil {
							log.Printf("Bad packet %s:%d: %v", pmd.PcapInfo.Filename, pmd.Index, err.Error())
							return
						}
						md := newPacket.Metadata()
						md.CaptureLength = len(newPacket.Data())
						md.Length = len(newPacket.Data())
						md.Timestamp = ts
						// TODO: add metadata from previous packets
						pcapmetadata.AddPcapMetadata(&md.CaptureInfo, pmd.PcapInfo, pmd.Index)
						packet = &Packet{
							ci: md.CaptureInfo,
							p:  newPacket,
						}
					}
				case layers.LayerTypeIPv6:
					// TODO: implement ipv6 reassembly (if needed, unsure)
				default:
					return
				}
				transport := parsed.TransportLayer()
				if transport == nil {
					return
				}
				switch transport.LayerType() {
				case layers.LayerTypeTCP:
					tcp := transport.(*layers.TCP)
					k := tcp.SrcPort ^ tcp.DstPort
					k = 0xff & (k ^ (k >> 8))
					a := tcpAssembler[k]
					a.FlushCloseOlderThan(tsTimeouted)
					asc := streams.AssemblerContext{
						CaptureInfo: *packet.CaptureInfo(),
					}
					a.AssembleWithContext(parsed.NetworkLayer().NetworkFlow(), tcp, &asc)
				case layers.LayerTypeUDP:
					udp := transport.(*layers.UDP)
					asc := streams.AssemblerContext{
						CaptureInfo: *packet.CaptureInfo(),
					}
					udpAssembler.FlushCloseOlderThan(tsTimeouted)
					udpAssembler.AssembleWithContext(parsed.NetworkLayer().NetworkFlow(), udp, &asc)
				default:
					// TODO: implement sctp support
				}
			}()

			//clear all data associated with the packet
			*packet = Packet{}
		}
	}

	// scan for next unused stream id
	nextStreamID := uint64(0)
	for _, idx := range existingIndexes {
		maxStreamID := idx.MaxStreamID()
		if nextStreamID <= maxStreamID {
			nextStreamID = maxStreamID + 1
		}
	}

	indexBuilders := []*index.Writer{}
	if err := func() error {
		// dump collected streams to new indexes
		for _, s := range streamFactory.Streams {
			id := nextStreamID
			touchedByNewPcaps := false
		outer:
			for pi := range s.Packets {
				pmd := pcapmetadata.FromPacketMetadata(&s.Packets[pi])
				for _, p := range newPcapInfos {
					if pmd.PcapInfo == p {
						touchedByNewPcaps = true
						if id != nextStreamID {
							break outer
						}
						continue outer
					}
				}
				if id != nextStreamID {
					continue
				}
				for _, idx := range existingIndexes {
					stream, err := idx.StreamByFirstPacketSource(pmd.PcapInfo.Filename, pmd.Index)
					if err != nil {
						return err
					}
					if stream != nil {
						id = stream.ID()
						break
					}
				}
				if touchedByNewPcaps {
					break
				}
			}
			if !touchedByNewPcaps {
				continue
			}
			if id == nextStreamID {
				nextStreamID++
			}

			for i := 0; ; i++ {
				if i == len(indexBuilders) {
					ib, err := index.NewWriter(tools.MakeFilename(b.indexDir, "idx"))
					if err != nil {
						return err
					}
					indexBuilders = append(indexBuilders, ib)
				}
				ib := indexBuilders[i]
				ok, err := ib.AddStream(s, id)
				if err != nil {
					return err
				}
				if ok {
					break
				}
			}
		}
		return nil
	}(); err != nil {
		for _, ib := range indexBuilders {
			ib.Close()
			os.Remove(ib.Filename())
		}
		return 0, nil, err
	}

	indexes := []*index.Reader{}
	for ibIdx, ib := range indexBuilders {
		i, err := ib.Finalize()
		if err != nil {
			for _, i := range indexes {
				i.Close()
				os.Remove(i.Filename())
			}
			for _, ib := range indexBuilders[ibIdx:] {
				ib.Close()
				os.Remove(ib.Filename())
			}
			return 0, nil, err
		}
		indexes = append(indexes, i)
	}

	// save new snapshots
	newSnapshotFilename := tools.MakeFilename(b.snapshotDir, "snap")
	err := saveSnapshots(newSnapshotFilename, newSnapshots)
	if err != nil {
		log.Printf("saveSnapshots(%q) failed: %v", newSnapshotFilename, err)
	} else {
		if b.snapshotFilename != "" {
			os.Remove(filepath.Join(b.snapshotDir, b.snapshotFilename))
		}
		b.snapshotFilename = filepath.Base(newSnapshotFilename)
	}

	b.knownPcaps = append(b.knownPcaps, newPcapInfos...)
	b.snapshots = newSnapshots

	outputFiles := []string{}
	for _, i := range indexes {
		outputFiles = append(outputFiles, i.Filename())
	}
	log.Printf("Built indexes %q from pcaps %q\n", outputFiles, pcapFilenames)
	return nProcessedPcaps, indexes, nil
}

func (b *Builder) KnownPcaps() []*pcapmetadata.PcapInfo {
	return b.knownPcaps
}
