package view

import (
	"fmt"
	"io"
	"io/fs"
	"strings"
)

// MemRenderer renders views from memory.
type MemRenderer struct {
	views map[string]*View
}

// NewMemRenderer parses all the views in the given fs and stores the results in memory.
func NewMemRenderer(viewFS fs.FS) (*MemRenderer, error) {
	files, err := fs.Glob(viewFS, "*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to glob for views: %w", err)
	}

	views := make(map[string]*View, len(files))
	for _, file := range files {
		viewName := strings.TrimSuffix(file, ".html")
		view, err := Parse(viewFS, viewName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse view %q: %w", viewName, err)
		}

		views[viewName] = view
	}

	return &MemRenderer{
		views: views,
	}, nil
}

func (r *MemRenderer) Render(w io.Writer, name string, data any) error {
	v, ok := r.views[name]
	if !ok {
		return fmt.Errorf("view %q not found", name)
	}

	return v.Render(w, data)
}
