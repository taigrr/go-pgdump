package pgdump

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// dumpReader wraps pg_dump's stdout and waits for the process on Close.
type dumpReader struct {
	cmd    *exec.Cmd
	pipe   io.ReadCloser
	stderr *bytes.Buffer
}

func (r *dumpReader) Read(p []byte) (int, error) {
	if r.pipe == nil {
		return 0, io.EOF
	}
	return r.pipe.Read(p)
}

// Close releases the stdout pipe and waits for the underlying process to
// exit. If the process exits with a non-zero status and produced stderr
// output, the error message includes that output for diagnostics.
func (r *dumpReader) Close() error {
	if r == nil {
		return errors.New("dumpReader is nil")
	}
	var pipeErr, waitErr error
	if r.pipe != nil {
		pipeErr = r.pipe.Close()
	}
	if r.cmd != nil {
		waitErr = r.cmd.Wait()
		// Attach stderr output to a non-nil wait error so callers can
		// see why pg_dump failed without inspecting os.Stderr.
		if waitErr != nil && r.stderr != nil && r.stderr.Len() > 0 {
			waitErr = fmt.Errorf("%w: %s", waitErr, strings.TrimSpace(r.stderr.String()))
		}
	}
	if pipeErr != nil && waitErr != nil {
		return fmt.Errorf("pipe close error: %v; wait error: %v", pipeErr, waitErr)
	}
	if pipeErr != nil {
		return pipeErr
	}
	return waitErr
}
