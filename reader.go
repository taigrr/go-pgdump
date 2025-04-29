package pgdump

import (
	"fmt"
	"io"
	"os/exec"
)

// dumpReader wraps pg_dump's stdout and waits for the process on Close.
type dumpReader struct {
	cmd  *exec.Cmd
	pipe io.ReadCloser
}

func (r *dumpReader) Read(p []byte) (int, error) {
	if r.pipe == nil {
		return 0, io.EOF
	}
	return r.pipe.Read(p)
}

func (r *dumpReader) Close() error {
	if r == nil {
		return nil
	}
	var pipeErr, waitErr error
	if r.pipe != nil {
		pipeErr = r.pipe.Close()
	}
	if r.cmd != nil {
		waitErr = r.cmd.Wait()
	}
	if pipeErr != nil && waitErr != nil {
		return fmt.Errorf("pipe close error: %v; wait error: %v", pipeErr, waitErr)
	}
	if pipeErr != nil {
		return pipeErr
	}
	return waitErr
}
