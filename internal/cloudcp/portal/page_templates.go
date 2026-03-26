package portal

import (
	"embed"
	"html/template"
	"io/fs"
)

//go:embed templates/portal.html assets/portal.css assets/portal_shell.js assets/portal_services.js
var portalTemplateFS embed.FS

var (
	portalPageTmpl       = template.Must(template.ParseFS(portalTemplateFS, "templates/portal.html"))
	portalStyles         = mustEmbeddedCSS("assets/portal.css")
	portalShellScript    = mustEmbeddedJS("assets/portal_shell.js")
	portalServicesScript = mustEmbeddedJS("assets/portal_services.js")
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
