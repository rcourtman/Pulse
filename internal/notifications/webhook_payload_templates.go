package notifications

import (
	"sync"
	"text/template"
)

// The key set is fixed by the shipped catalog, never by submitted templates,
// URLs, credentials or rendered alert data. Parse each stock source on first
// use; installations using only one service need not parse the others.
var builtInWebhookPayloadParsers = sync.OnceValue(newBuiltInWebhookPayloadParsers)

func newBuiltInWebhookPayloadParsers() map[string]func() (*template.Template, error) {
	parsers := make(map[string]func() (*template.Template, error))
	for _, provider := range GetWebhookTemplates() {
		for _, source := range []string{provider.PayloadTemplate, provider.ResolvedPayloadTemplate} {
			if source != "" {
				parsers[source] = sync.OnceValues(func() (*template.Template, error) {
					return template.New("webhook").Funcs(templateFuncMap()).Parse(source)
				})
			}
		}
	}
	return parsers
}

// Returned stock templates are immutable: callers only Execute with their own
// data and writer, which text/template permits concurrently. Exact source
// matching also lets saved copies of a stock template reuse its parse tree.
// Every other source remains request-local, including malformed templates.
func webhookPayloadTemplate(source string) (*template.Template, error) {
	if parse, ok := builtInWebhookPayloadParsers()[source]; ok {
		return parse()
	}
	return template.New("webhook").Funcs(templateFuncMap()).Parse(source)
}
