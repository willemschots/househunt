package view_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/willemschots/househunt/internal/web/view"
)

func TestView_ParseAndRender(t *testing.T) {
	okTests := map[string]struct {
		files map[string]string
		name  string
		data  any
		want  string
	}{
		"base only": {
			files: map[string]string{
				"base.html": `<html>Hello {{ . }}</html>`,
			},
			name: "",
			data: "World!",
			want: `<html>Hello World!</html>`,
		},
		"base only w base name": {
			files: map[string]string{
				"base.html": `<html>Hello {{ . }}</html>`,
			},
			name: "base",
			data: "World!",
			want: `<html>Hello World!</html>`,
		},
		"base and home": {
			files: map[string]string{
				"base.html": `<html>{{template "content" . }}</html>`,
				"home.html": `{{define "content"}}<h1>Hello {{ . }}</h1>{{end}}`,
			},
			name: "home",
			data: "World!",
			want: `<html><h1>Hello World!</h1></html>`,
		},
		"base, home and greeting partial": {
			files: map[string]string{
				"base.html":              `<html>{{template "content" . }}</html>`,
				"home.html":              `{{define "content"}}<h1>{{template "greeting" . }}</h1>{{end}}`,
				"partials/greeting.html": `{{define "greeting"}}Hello {{ . }}{{end}}`,
			},
			name: "home",
			data: "World!",
			want: `<html><h1>Hello World!</h1></html>`,
		},
		"name with all allowed characters": {
			files: map[string]string{
				"base.html": `<html>{{template "content" . }}</html>`,
				"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.html": `{{define "content"}}<h1>Hello {{ . }}</h1>{{end}}`,
			},
			name: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_",
			data: "World!",
			want: `<html><h1>Hello World!</h1></html>`,
		},
		"check data is escaped": {
			files: map[string]string{
				"base.html": `<html>{{ . }}</html>`,
			},
			name: "",
			data: "<script>alert('xss')</script>",
			want: `<html>&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;</html>`,
		},
	}

	for name, tc := range okTests {
		t.Run(name, func(t *testing.T) {
			tempFS := tempFilesForTest(t, tc.files)

			v, err := view.Parse(tempFS, tc.name)
			if err != nil {
				t.Fatalf("unexpected error parsing view: %v", err)
			}

			buf := &bytes.Buffer{}
			err = v.Render(buf, tc.data)
			if err != nil {
				t.Fatalf("unexpected error rendering view: %v", err)
			}

			got := buf.String()
			if got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}

	parseFails := map[string]struct {
		files map[string]string
		name  string
	}{
		"no views": {
			files: map[string]string{},
			name:  "",
		},
		"no base": {
			files: map[string]string{
				"home.html": `<h1>Hello {{ . }}</h1>`,
			},
			name: "",
		},
		"no home": {
			files: map[string]string{
				"base.html":  `<html>{{template "content" . }}</html>`,
				"other.html": `<h1>Hello {{ . }}</h1>`,
			},
			name: "home",
		},
		"filename with disallowed rune": {
			files: map[string]string{
				"base.html": `<html>{{template "content" . }}</html>`,
				"#.html":    `<h1>Hello {{ . }}</h1>`,
			},
			name: "#",
		},
	}

	for name, tc := range parseFails {
		t.Run(name, func(t *testing.T) {
			tempFS := tempFilesForTest(t, tc.files)

			_, err := view.Parse(tempFS, tc.name)
			if err == nil {
				t.Fatalf("expected error, got <nil>")
			}
		})
	}
}

func tempFilesForTest(t *testing.T, files map[string]string) fs.FS {
	t.Helper()

	dir, err := os.MkdirTemp("", "househunt_view_test")
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
