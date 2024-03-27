package view_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/email/view"
)

func Test_View_ParseAndRender(t *testing.T) {
	okTests := map[string]struct {
		files       map[string]string
		parseName   string
		renderData  any
		wantSubject string
		wantBody    string
	}{
		"ok, single template": {
			files: map[string]string{
				"test.tmpl": `{{ block "subject" . }}Hello world{{ end }} {{ block "body" . }}Message{{ end }}`,
			},
			parseName:   "test",
			renderData:  nil,
			wantSubject: "Hello world",
			wantBody:    "Message",
		},
		"ok, multiple templates": {
			files: map[string]string{
				"test-1.tmpl": `{{ block "subject" . }}Hello 1{{ end }} {{ block "body" . }}Message 1{{ end }}`,
				"test-2.tmpl": `{{ block "subject" . }}Hello 2{{ end }} {{ block "body" . }}Message 2{{ end }}`,
				"test-3.tmpl": `{{ block "subject" . }}Hello 3{{ end }} {{ block "body" . }}Message 3{{ end }}`,
			},
			parseName:   "test-2",
			renderData:  nil,
			wantSubject: "Hello 2",
			wantBody:    "Message 2",
		},
		"ok, with data": {
			files: map[string]string{
				"test.tmpl": `{{ block "subject" . }}Hello {{ .Name }}{{ end }} {{ block "body" . }}I'm saying {{.Message}}{{ end }}`,
			},
			parseName:   "test",
			renderData:  struct{ Name, Message string }{"world", "Hi!"},
			wantSubject: "Hello world",
			wantBody:    "I'm saying Hi!",
		},
	}

	for name, tc := range okTests {
		t.Run(name, func(t *testing.T) {
			fs := tempTestFS(t, tc.files)
			v, err := view.Parse(fs, tc.parseName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var buf bytes.Buffer
			err = v.Render(&buf, email.ElementSubject, tc.renderData)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotSubject := buf.String()
			if gotSubject != tc.wantSubject {
				t.Errorf("unexpected subject: got %q, want %q", gotSubject, tc.wantSubject)
			}

			buf.Reset()

			err = v.Render(&buf, email.ElementBody, tc.renderData)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotBody := buf.String()
			if gotBody != tc.wantBody {
				t.Errorf("unexpected body: got %q, want %q", gotBody, tc.wantBody)
			}
		})
	}

	parseFails := map[string]struct {
		files map[string]string
		name  string
	}{
		"no templates": {
			files: map[string]string{},
			name:  "test",
		},
		"no template for name": {
			files: map[string]string{
				"test.tmpl": `{{ block "subject" . }}Hello world{{ end }} {{ block "body" . }}Message{{ end }}`,
			},
			name: "other",
		},
		"empty template": {
			files: map[string]string{
				"test.tmpl": "",
			},
		},
		"missing subject block": {
			files: map[string]string{
				"test.tmpl": `{{ block "body" . }}Message{{ end }}`,
			},
			name: "test",
		},
		"missing body block": {
			files: map[string]string{
				"test.tmpl": `{{ block "subject" . }}Hello world{{ end }}`,
			},
			name: "test",
		},
		"syntax error": {
			files: map[string]string{
				"test.tmpl": `{{ block "subject" . }}Hello world{{ end }} {{ block "body" . }}Message{{ end }`,
			},
			name: "test",
		},
		"filename with disallowed rune": {
			files: map[string]string{
				"#.tmpl": `{{ block "subject" . }}Hello world{{ end }} {{ block "body" . }}Message{{ end }}`,
			},
			name: "#",
		},
	}

	for name, tc := range parseFails {
		t.Run(name, func(t *testing.T) {
			fs := tempTestFS(t, tc.files)
			_, err := view.Parse(fs, tc.name)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func tempTestFS(t *testing.T, files map[string]string) fs.FS {
	t.Helper()

	dir, err := os.MkdirTemp("", "househunt_email_view_test")
	if err != nil {
		t.Fatalf("failed to create temporary directory for views: %v", err)
	}

	t.Cleanup(func() {
		err := os.RemoveAll(dir)
		if err != nil {
			t.Fatalf("failed to remove temporary directory: %v", err)
		}
	})

	for name, content := range files {
		fn := filepath.Join(dir, name)
		err := os.MkdirAll(filepath.Dir(fn), 0755)
		if err != nil {
			t.Fatalf("failed to create path for temporary file: %v", err)
		}

		err = os.WriteFile(fn, []byte(content), 0644)
		if err != nil {
			t.Fatalf("failed to write temporary file: %v", err)
		}
	}

	return os.DirFS(dir)
}
