package notifications

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"text/template"
)

func TestWebhookPayloadTemplateReusesOnlyExactStockSources(t *testing.T) {
	stock := make(map[string]bool)
	for _, provider := range GetWebhookTemplates() {
		for _, source := range []string{provider.PayloadTemplate, provider.ResolvedPayloadTemplate} {
			if source == "" {
				continue
			}
			stock[source] = true
			first, err := webhookPayloadTemplate(source)
			if err != nil {
				t.Fatal(err)
			}
			second, err := webhookPayloadTemplate(source)
			if err != nil || first != second {
				t.Fatalf("%s stock source was reparsed: %v", provider.Service, err)
			}
			// A saved edit is not the stock source, even if its output is equal.
			first, err = webhookPayloadTemplate(source + "{{/* saved edit */}}")
			if err != nil {
				t.Fatal(err)
			}
			second, err = webhookPayloadTemplate(source + "{{/* saved edit */}}")
			if err != nil || first == second {
				t.Fatalf("%s saved edit was retained: %v", provider.Service, err)
			}
		}
	}
	for i := range 100 {
		source := fmt.Sprintf(`{"synthetic_private_config_%d":"{{.Message | jsonString}}"}`, i)
		first, err := webhookPayloadTemplate(source)
		if err != nil {
			t.Fatal(err)
		}
		second, err := webhookPayloadTemplate(source)
		if err != nil || first == second {
			t.Fatalf("submitted source %d was retained: %v", i, err)
		}
		if _, err := webhookPayloadTemplate(fmt.Sprintf("{{ invalid_function_%d }}", i)); err == nil {
			t.Fatal("malformed submitted source unexpectedly parsed")
		}
	}
	parsers := builtInWebhookPayloadParsers()
	if len(parsers) != len(stock) {
		t.Fatalf("retained %d sources, want exactly %d stock sources", len(parsers), len(stock))
	}
	for source := range parsers {
		if !stock[source] {
			t.Fatal("retained a non-stock source")
		}
	}
}

func TestWebhookPayloadTemplateConcurrentFirstUse(t *testing.T) {
	// A private fresh index exercises cold OnceValues, irrespective of the order
	// in which other tests have already used the process-wide index.
	parsers := newBuiltInWebhookPayloadParsers()
	for _, provider := range GetWebhookTemplates() {
		for _, source := range []string{provider.PayloadTemplate, provider.ResolvedPayloadTemplate} {
			if source == "" {
				continue
			}
			t.Run(fmt.Sprintf("%s/%d", provider.Service, len(source)), func(t *testing.T) {
				const workers = 16
				start := make(chan struct{})
				results := make(chan *template.Template, workers)
				var wg sync.WaitGroup
				for i := range workers {
					data := webhookRenderingFixture(fmt.Sprintf("tenant-%d", i))
					want, err := freshWebhookPayload(source, data, provider.Service)
					if err != nil {
						t.Fatal(err)
					}
					wg.Go(func() {
						<-start
						parsed, err := parsers[source]()
						if err != nil {
							t.Error(err)
							return
						}
						results <- parsed
						for range 10 {
							var output bytes.Buffer
							if err := parsed.Execute(&output, data); err != nil {
								t.Error(err)
								return
							}
							if !bytes.Equal(output.Bytes(), want) {
								t.Error("concurrent stock execution changed bytes or crossed tenant data")
								return
							}
							// Also exercise the actual shared production path, not just
							// the cold parser index and template.Execute above.
							got, err := (&NotificationManager{}).generatePayloadFromTemplateWithService(source, data, provider.Service)
							if err != nil || !bytes.Equal(got, want) {
								t.Errorf("concurrent production render differs: %v", err)
								return
							}
						}
						// Returned payload buffers cannot alias the cached tree or
						// another caller's output.
						got, err := (&NotificationManager{}).generatePayloadFromTemplateWithService(source, data, provider.Service)
						if err != nil {
							t.Error(err)
							return
						}
						for j := range got {
							got[j] = 'x'
						}
						got, err = (&NotificationManager{}).generatePayloadFromTemplateWithService(source, data, provider.Service)
						if err != nil || !bytes.Equal(got, want) {
							t.Errorf("caller mutation changed later output: %v", err)
						}
					})
				}
				close(start)
				wg.Wait()
				close(results)
				var first *template.Template
				for parsed := range results {
					if first == nil {
						first = parsed
					} else if parsed != first {
						t.Fatal("concurrent first use parsed a stock source more than once")
					}
				}
			})
		}
	}
}
