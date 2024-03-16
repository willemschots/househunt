package view

import (
	"fmt"
	"io"
	"io/fs"
)

// FSRenderer renders views from a file system.
type FSRenderer struct {
	fs fs.FS
}

// NewFSRenderer returns a new FSRenderer.
func NewFSRenderer(fs fs.FS) *FSRenderer {
	return &FSRenderer{fs: fs}
}

func (r *FSRenderer) Render(w io.Writer, name string, data any) error {
	v, err := Parse(r.fs, name)
	if err != nil {
		return fmt.Errorf("failed to parse view: %w", err)
	}
	return v.Render(w, data)
}
