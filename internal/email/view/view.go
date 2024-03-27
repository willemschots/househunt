package view

import (
	"fmt"
	"io"
	"io/fs"
	"text/template"

	"github.com/willemschots/househunt/internal/email"
)

// View is a template used to render email messages.
type View struct {
	tmpl *template.Template
}

// Parse parses the file system and returns a view for the given name.
// fs is expected to contain *.tmpl files in the root directory.
func Parse(fs fs.FS, name string) (*View, error) {
	// Validate the view name, just to be sure.
	//
	// Generally these will be hardcoded, but if for some reason we end
	// up with user input as a view name, we want to error. These view names
	// are used to construct filenames and we don't want to inadvertently
	// allow directory traversal.
	if err := validateName(name); err != nil {
		return nil, err
	}

	filename := fmt.Sprintf("%s.tmpl", name)
	tmpl, err := template.New(name).ParseFS(fs, filename)
	if err != nil {
		return nil, err
	}

	subjectTmpl := tmpl.Lookup(string(email.ElementSubject))
	if subjectTmpl == nil {
		return nil, fmt.Errorf("missing %s template", email.ElementSubject)
	}

	bodyTempl := tmpl.Lookup(string(email.ElementBody))
	if bodyTempl == nil {
		return nil, fmt.Errorf("missing %s template", email.ElementBody)
	}

	return &View{
		tmpl: tmpl,
	}, nil
}

func (v *View) Render(w io.Writer, element email.TemplateElement, data any) error {
	if err := v.tmpl.ExecuteTemplate(w, string(element), data); err != nil {
		return err
	}

	return nil
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
