package query

type (
	SortingKey byte
	SortingDir bool

	Sorting struct {
		Key SortingKey
		Dir SortingDir
	}
)

const (
	SortingKeyID SortingKey = iota
	SortingKeyClientBytes
	SortingKeyServerBytes
	SortingKeyFirstPacketTime
	SortingKeyLastPacketTime
	SortingKeyClientHost
	SortingKeyServerHost
	SortingKeyClientPort
	SortingKeyServerPort

	SortingDirAscending  SortingDir = false
	SortingDirDescending SortingDir = true
)
