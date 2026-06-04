package assets

import "embed"

//go:embed migrations
var MigracijeFS embed.FS

//go:embed web/static/css
var StaticFS embed.FS

//go:embed web/templates
var TemplatesFS embed.FS
