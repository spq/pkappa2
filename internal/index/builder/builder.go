package builder

import (
	"io/ioutil"
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
	pcaps, err := ioutil.ReadDir(pcapDir)
	if err != nil {
		return nil, err
	}
	for _, p := range pcaps {
		if p.IsDir() || !strings.HasSuffix(p.Name(), ".pcap") {
			continue
		}
		info := cachedKnownPcapsMap[p.Name()]
		if info == nil || info.Filesize != uint64(p.Size()) {
			info, _, err = readPackets(pcapDir, p.Name(), nil)
			if err != nil {
				return nil, err
			}
		}
		b.knownPcaps = append(b.knownPcaps, info)
	}
	// load the snapshot file with the most packets covered
	snapshotFiles, err := ioutil.ReadDir(snapshotDir)
	if err != nil {
		return nil, err
	}
	packetCounts := uint64(0)
	for _, f := range snapshotFiles {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".snap") {
			continue
		}
		snapshots, err := loadSnapshots(filepath.Join(snapshotDir, f.Name()))
		if err != nil {
			log.Printf("loadSnapshots(%q) failed: %v", f.Name(), err)
			continue
		}
		currentPacketCounts := uint64(0)
		for _, s := range snapshots {
			currentPacketCounts += s.packetCount
		}
		if packetCounts < currentPacketCounts {
			b.snapshots = snapshots
			b.snapshotFilename = f.Name()
			packetCounts = currentPacketCounts
		}
	}
	return &b, nil
}

func (b *Builder) FromPcap(pcapDir, pcapFilename string, existingIndexes []*index.Reader) ([]*index.Reader, error) {
	log.Printf("Building indexes from pcap %q\n", pcapFilename)
	// load, find ts of oldest new package
	newPcapInfo, allPackets, err := readPackets(pcapDir, pcapFilename, nil)
	if err != nil {
		return nil, err
	}
	if len(allPackets) == 0 {
		return nil, nil
	}
	oldestTs := newPcapInfo.PacketTimestampMin
	log.Printf("adding %d packets from pcap %q (0)\n", len(allPackets), pcapFilename)

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
	log.Printf("using snapshot missing %s\n", oldestTs.Sub(bestSnapshot.timestamp).String())

	// load all packets referenced in the snapshot or with ts >= snapshot ts from affected pcaps
	for _, pcap := range b.knownPcaps {
		packetIndexes := bestSnapshot.referencedPackets[pcap.Filename]
		if bestSnapshot.timestamp.After(pcap.PacketTimestampMax) && len(packetIndexes) == 0 {
			continue
		}
		_, packets, err := readPackets(pcapDir, pcap.Filename, pcap)
		if err != nil {
			return nil, err
		}
		if !bestSnapshot.timestamp.After(pcap.PacketTimestampMin) {
			allPackets = append(allPackets, packets...)
			log.Printf("adding %d packets from pcap %q (1)\n", len(packets), pcap.Filename)
			continue
		}
		log.Printf("adding %d packets from pcap %q (2)\n", len(packetIndexes), pcap.Filename)
		for _, i := range packetIndexes {
			allPackets = append(allPackets, packets[i])
		}
		if !bestSnapshot.timestamp.After(pcap.PacketTimestampMax) {
			before := len(allPackets)
			for _, p := range packets {
				if !bestSnapshot.timestamp.After(p.Metadata().Timestamp) {
					allPackets = append(allPackets, p)
				}
			}
			log.Printf("adding %d packets from pcap %q (3)\n", len(allPackets)-before, pcap.Filename)
		}
	}

	// sort all loaded packets by timestamp or packet index or pcap filename
	sort.Slice(allPackets, func(i, j int) bool {
		a, b := allPackets[i], allPackets[j]
		amd, bmd := a.Metadata(), b.Metadata()
		if !amd.Timestamp.Equal(bmd.Timestamp) {
			return amd.Timestamp.Before(bmd.Timestamp)
		}
		apmd := pcapmetadata.FromPacketMetadata(&amd.CaptureInfo)
		bpmd := pcapmetadata.FromPacketMetadata(&bmd.CaptureInfo)
		if apmd.PcapInfo != bpmd.PcapInfo {
			return apmd.PcapInfo.Filename < bpmd.PcapInfo.Filename
		}
		return apmd.Index < bpmd.Index
	})

	// create empty reassemblers
	ip4defragmenter := ip4defrag.NewIPv4Defragmenter()

	tcpStreamFactory := &streams.TCPStreamFactory{}
	tcpAssembler := [0x100]*reassembly.Assembler{}
	for i := range tcpAssembler {
		pool := reassembly.NewStreamPool(tcpStreamFactory)
		tcpAssembler[i] = reassembly.NewAssembler(pool)
	}
	// iterate over packets
	nPacketsAfterSnapshot := uint64(0)
	previousPacketTimestamp := time.Time{}
	newSnapshots := []*snapshot{}
	for _, s := range b.snapshots {
		if !bestSnapshot.timestamp.Before(s.timestamp) {
			// s.ts <= b.ts
			newSnapshots = append(newSnapshots, s)
		}
	}
	for _, packet := range allPackets {
		// create new snapshots for packets after snapshot referenced ones
		ts := packet.Metadata().Timestamp
		if nPacketsAfterSnapshot >= 1000 && !ts.Equal(previousPacketTimestamp) {
			// create new snapshot
			referencedPackets := map[string][]uint64{}
			// TODO: dump packets from ip4defragmenter
			for _, s := range tcpStreamFactory.Streams {
				if s.Complete {
					continue
				}
				for _, p := range s.Packets {
					pmds := pcapmetadata.AllFromPacketMetadata(p)
					for _, pmd := range pmds {
						referencedPackets[pmd.PcapInfo.Filename] = append(referencedPackets[pmd.PcapInfo.Filename], pmd.Index)
					}
				}
			}
			newSnapshots = compactSnapshots(append(newSnapshots, &snapshot{
				timestamp:         ts,
				packetCount:       nPacketsAfterSnapshot,
				referencedPackets: referencedPackets,
			}))
			nPacketsAfterSnapshot = 0
		}
		if nPacketsAfterSnapshot != 0 || !bestSnapshot.timestamp.After(ts) {
			previousPacketTimestamp = ts
			nPacketsAfterSnapshot++
		}
		tsTimeouted := ts.Add(streams.InactivityTimeout)

		// process packet with ip, tcp & udp reassemblers
		network := packet.NetworkLayer()
		if network == nil {
			continue
		}
		switch network.LayerType() {
		case layers.LayerTypeIPv4:
			ip4defragmenter.DiscardOlderThan(tsTimeouted)
			defragmented, err := ip4defragmenter.DefragIPv4WithTimestamp(network.(*layers.IPv4), ts)
			if err != nil {
				pmd := pcapmetadata.FromPacketMetadata(&packet.Metadata().CaptureInfo)
				log.Printf("Bad packet %s:%d: %v", pmd.PcapInfo.Filename, pmd.Index, err)
				continue
			}
			if defragmented == nil {
				continue
			}
			if defragmented != network {
				b := gopacket.NewSerializeBuffer()
				ipPayload, _ := b.PrependBytes(len(defragmented.Payload))
				copy(ipPayload, defragmented.Payload)
				pmd := pcapmetadata.FromPacketMetadata(&packet.Metadata().CaptureInfo)
				if err := defragmented.SerializeTo(b, gopacket.SerializeOptions{
					FixLengths:       true,
					ComputeChecksums: true,
				}); err != nil {
					log.Printf("Bad packet %s:%d: %v", pmd.PcapInfo.Filename, pmd.Index, err.Error())
					continue
				}
				newPacket := gopacket.NewPacket(b.Bytes(), layers.LayerTypeIPv4, gopacket.Default)
				if err := newPacket.ErrorLayer(); err != nil {
					log.Printf("Bad packet %s:%d: %v", pmd.PcapInfo.Filename, pmd.Index, err.Error())
					continue
				}
				md := newPacket.Metadata()
				md.CaptureLength = len(newPacket.Data())
				md.Length = len(newPacket.Data())
				md.Timestamp = ts
				// TODO: add metadata from previous packets
				pcapmetadata.AddPcapMetadata(&md.CaptureInfo, pmd.PcapInfo, pmd.Index)
				packet = newPacket
			}
		case layers.LayerTypeIPv6:
			// TODO: implement ipv6 reassembly (if needed, unsure)
		default:
			continue
		}
		transport := packet.TransportLayer()
		if transport == nil {
			continue
		}
		switch transport.LayerType() {
		case layers.LayerTypeTCP:
			tcp := transport.(*layers.TCP)
			k := tcp.SrcPort ^ tcp.DstPort
			k = 0xff & (k ^ (k >> 8))
			a := tcpAssembler[k]
			a.FlushCloseOlderThan(tsTimeouted)
			asc := streams.AssemblerContext{
				CaptureInfo: &packet.Metadata().CaptureInfo,
			}
			a.AssembleWithContext(packet.NetworkLayer().NetworkFlow(), tcp, &asc)
		case layers.LayerTypeUDP:
			// TODO: implement udp support
		default:
			// TODO: implement sctp support
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
		for _, s := range tcpStreamFactory.Streams {
			id := nextStreamID
			for pi := range s.Packets {
				pmd := pcapmetadata.FromPacketMetadata(s.Packets[pi])
				if pmd.PcapInfo == newPcapInfo {
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
				break
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
		return nil, err
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
			return nil, err
		}
		indexes = append(indexes, i)
	}

	// save new snapshots
	newSnapshotFilename := tools.MakeFilename(b.snapshotDir, "snap")
	err = saveSnapshots(newSnapshotFilename, newSnapshots)
	if err != nil {
		log.Printf("saveSnapshots(%q) failed: %v", newSnapshotFilename, err)
	} else {
		if b.snapshotFilename != "" {
			os.Remove(filepath.Join(b.snapshotDir, b.snapshotFilename))
		}
		b.snapshotFilename = filepath.Base(newSnapshotFilename)
	}

	b.knownPcaps = append(b.knownPcaps, newPcapInfo)
	b.snapshots = newSnapshots

	outputFiles := []string{}
	for _, i := range indexes {
		outputFiles = append(outputFiles, i.Filename())
	}
	log.Printf("build indexes %q from pcap %q\n", outputFiles, newPcapInfo.Filename)
	return indexes, nil
}

func (b *Builder) KnownPcaps() []*pcapmetadata.PcapInfo {
	return b.knownPcaps
}
