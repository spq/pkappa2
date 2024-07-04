package tools

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

var (
	lastTime time.Time
	lastID   uint
	mtx      sync.Mutex
)

func MakeFilename(dir, extension string) string {
	t := time.Now().Truncate(time.Millisecond)
	mtx.Lock()
	if lastTime != t {
		lastTime = t
		lastID = 0
	} else {
		lastID++
	}
	curID := lastID
	mtx.Unlock()
	fn := filepath.Join(dir, fmt.Sprintf("%s.%d.%s", t.Format("2006-01-02_150405.000"), curID, extension))
	return fn
}
