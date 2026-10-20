package command

import (
	"bytes"
	"fmt"
)

// Output bounds in-memory command output; complete streaming diagnostics go to the task log.
type Output struct {
	bytes.Buffer
	Limit int
}

func (o *Output) Write(p []byte) (int, error) {
	if o.Len()+len(p) > o.Limit {
		return 0, fmt.Errorf("command output exceeds %d bytes", o.Limit)
	}
	return o.Buffer.Write(p)
}
