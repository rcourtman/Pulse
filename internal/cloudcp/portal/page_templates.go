package portal

import (
	"embed"
	"html/template"
)

//go:embed templates/portal.html templates/login.html
var portalTemplateFS embed.FS

var (
	portalPageTmpl = template.Must(template.ParseFS(portalTemplateFS, "templates/portal.html"))
	loginPageTmpl  = template.Must(template.ParseFS(portalTemplateFS, "templates/login.html"))
)
