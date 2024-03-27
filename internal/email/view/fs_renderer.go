package view

import (
	"io"
	"io/fs"

	"github.com/willemschots/househunt/internal/email"
)

type FSRenderer struct {
	fs fs.FS
}

func NewFSRenderer(fs fs.FS) *FSRenderer {
	return &FSRenderer{fs: fs}
}

func (r *FSRenderer) Render(w io.Writer, name string, element email.TemplateElement, data any) error {
	v, err := Parse(r.fs, name)
	if err != nil {
		return err
	}

	return v.Render(w, element, data)
}
