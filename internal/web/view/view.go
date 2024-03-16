package view

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
)

const baseFilename = "base.html"

// View is a collection of templates used to render data. Every
// view has an unique name.
//
// A view combines the following templates to render a HTML page:
// - base.html (required)
// - {name}.html (optional)
// - partials/*.html (optional)
type View struct {
	name     string
	template *template.Template
}

// Parse parses the file system and returns a view for the given name.
func Parse(viewFS fs.FS, name string) (*View, error) {
	// Validate the filename, just to be sure.
	//
	// Generally these will be hardcoded, but if for some reason we end
	// up with user input as a filename, we don't want to allow them
	// access to the filesystem.
	if err := validateName(name); err != nil {
		return nil, err
	}

	// We attempt to load all necessary files for the view.

	// We always have a base template.
	files := []string{
		baseFilename,
	}

	// Then we append the named template if it's not the same as base.
	if name != baseFilename && name != "" {
		files = append(files, fmt.Sprintf("%s.html", name))
	}

	// And finally, we include all the partials we can find.
	partials, err := fs.Glob(viewFS, "partials/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to glob for partials: %w", err)
	}

	files = append(files, partials...)

	// We now create a new template and parse all the files as templates.
	t := template.New(baseFilename)
	templ, err := t.ParseFS(viewFS, files...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse view: %w", err)
	}

	return &View{
		name:     name,
		template: templ,
	}, nil
}

// Renders data using the view and writes the result to w.
func (v *View) Render(w io.Writer, data any) error {
	return v.template.Execute(w, data)
}

// validateName checks if all characters are alphanumeric, dashes or underscores.
func validateName(name string) error {
	for _, c := range name {
		if !validViewRune(c) {
			return fmt.Errorf("invalid character %v in view name: %s", c, name)
		}
	}
	return nil
}

func validViewRune(r rune) bool {
	if r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
		return true
	}

	return false
}
