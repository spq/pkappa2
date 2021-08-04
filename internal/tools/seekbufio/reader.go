package seekbufio

import (
	"bufio"
	"io"
	"runtime/debug"
)

type (
	SeekableBufferReader struct {
		f   io.ReadSeeker
		b   *bufio.Reader
		pos int64
	}
)

func NewSeekableBufferReader(f io.ReadSeeker) *SeekableBufferReader {
	return &SeekableBufferReader{
		f:   f,
		b:   bufio.NewReader(f),
		pos: 0,
	}
}

func (r *SeekableBufferReader) Read(p []byte) (int, error) {
	n, err := r.b.Read(p)
	if err != nil {
		debug.PrintStack()
		return 0, err
	}
	r.pos += int64(n)
	return n, err
}

func (r *SeekableBufferReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		// handle absolute seek by transforming it to a relative seek
		offset -= r.pos
		fallthrough
	case io.SeekCurrent:
		if offset == 0 {
			return r.pos, nil
		}
		if offset > 0 && offset <= int64(r.b.Buffered()) {
			if _, err := r.b.Discard(int(offset)); err != nil {
				return 0, err
			}
			r.pos += offset
			return r.pos, nil
		}
		// fallback to using an absolute seek if we can't reuse the buffer
		whence = io.SeekStart
		offset += r.pos
	}

	// fallback by seeking on wrapped file and reset the buffer reader
	p, err := r.f.Seek(offset, whence)
	if err != nil {
		return 0, err
	}
	r.pos = p
	r.b.Reset(r.f)
	return r.pos, nil
}
