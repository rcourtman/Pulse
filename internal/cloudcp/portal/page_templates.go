package portal

import (
	"embed"
	"html/template"
	"io/fs"
)

//go:embed templates/portal.html dist/portal_app.css dist/portal_app.js
var portalTemplateFS embed.FS

var (
	portalPageTmpl    = template.Must(template.ParseFS(portalTemplateFS, "templates/portal.html"))
	portalStyles      = mustEmbeddedCSS("dist/portal_app.css")
	portalShellScript = mustEmbeddedJS("dist/portal_app.js")
)

func mustEmbeddedCSS(path string) template.CSS {
	content, err := fs.ReadFile(portalTemplateFS, path)
	if err != nil {
		panic(err)
	}
	return template.CSS(string(content))
}

func mustEmbeddedJS(path string) template.JS {
	content, err := fs.ReadFile(portalTemplateFS, path)
	if err != nil {
		panic(err)
	}
	return template.JS(string(content))
}
