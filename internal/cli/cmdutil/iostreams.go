package cmdutil

import (
	"io"
	"os"
)

// IOStreams bundles a command's input/output writers so they can be substituted
// in tests. Production wiring (SystemIOStreams) points at the process stdio;
// tests construct an IOStreams with bytes.Buffer-backed writers.
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

// SystemIOStreams returns an IOStreams bound to the process stdio.
func SystemIOStreams() *IOStreams {
	return &IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
}
