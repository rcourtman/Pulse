package portal

import (
	"embed"
	"html/template"
	"io/fs"
)

//go:embed templates/portal.html templates/login.html assets/portal.css assets/portal_shell.js assets/portal_services.js assets/login.css assets/login.js
var portalTemplateFS embed.FS

var (
	portalPageTmpl       = template.Must(template.ParseFS(portalTemplateFS, "templates/portal.html"))
	loginPageTmpl        = template.Must(template.ParseFS(portalTemplateFS, "templates/login.html"))
	portalStyles         = mustEmbeddedCSS("assets/portal.css")
	portalShellScript    = mustEmbeddedJS("assets/portal_shell.js")
	portalServicesScript = mustEmbeddedJS("assets/portal_services.js")
	loginStyles          = mustEmbeddedCSS("assets/login.css")
	loginScript          = mustEmbeddedJS("assets/login.js")
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
