package assets

import "embed"

//go:embed templates/*.html
var TemplateFS embed.FS
