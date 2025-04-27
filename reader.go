package pgdump

import (
	"io"
	"os/exec"
)

// dumpReader wraps pg_dump's stdout and waits for the process on Close.
type dumpReader struct {
	cmd  *exec.Cmd
	pipe io.ReadCloser
}

func (r *dumpReader) Read(p []byte) (int, error) {
	return r.pipe.Read(p)
}

func (r *dumpReader) Close() error {
	defer r.cmd.Wait()
	return r.pipe.Close()
}
