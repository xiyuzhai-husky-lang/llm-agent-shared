package execution

import (
	"bytes"
	"fmt"
)

// boundedBuffer caps subprocess output before it can consume unbounded memory.
type boundedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *boundedBuffer) Write(data []byte) (int, error) {
	if b.buffer.Len()+len(data) > b.limit {
		return 0, fmt.Errorf("process output exceeds %d bytes", b.limit)
	}
	return b.buffer.Write(data)
}
