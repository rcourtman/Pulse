package notifications

import "testing"

func TestEmailConfigKeepsSMTPServer(t *testing.T) {
	saved := EmailConfig{SMTPHost: "smtp.operator.example", SMTPPort: 587, StartTLS: true}
	cases := []struct {
		name      string
		saved     EmailConfig
		requested EmailConfig
		want      bool
	}{
		{"identical", saved, saved, true},
		{"host case and spacing", saved, EmailConfig{SMTPHost: " SMTP.Operator.Example ", SMTPPort: 587, StartTLS: true}, true},
		{"default port spelled out", EmailConfig{SMTPHost: "smtp.operator.example"}, EmailConfig{SMTPHost: "smtp.operator.example", SMTPPort: defaultSMTPPort}, true},
		{"STARTTLS to implicit TLS", saved, EmailConfig{SMTPHost: "smtp.operator.example", SMTPPort: 587, TLS: true}, true},
		{"port 465 is implicit TLS either way", EmailConfig{SMTPHost: "smtp.operator.example", SMTPPort: 465, TLS: true}, EmailConfig{SMTPHost: "smtp.operator.example", SMTPPort: 465}, true},
		{"plain stays plain", EmailConfig{SMTPHost: "smtp.operator.example", SMTPPort: 25}, EmailConfig{SMTPHost: "smtp.operator.example", SMTPPort: 25}, true},
		{"another host", saved, EmailConfig{SMTPHost: "smtp.other.example", SMTPPort: 587, StartTLS: true}, false},
		{"another port", saved, EmailConfig{SMTPHost: "smtp.operator.example", SMTPPort: 2525, StartTLS: true}, false},
		{"encryption turned off", saved, EmailConfig{SMTPHost: "smtp.operator.example", SMTPPort: 587}, false},
		{"no saved server", EmailConfig{}, EmailConfig{SMTPHost: "smtp.other.example", SMTPPort: 587}, false},
		{"no server either side", EmailConfig{}, EmailConfig{}, false},
	}
	for _, tc := range cases {
		if got := EmailConfigKeepsSMTPServer(tc.saved, tc.requested); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestAppriseConfigKeepsServer(t *testing.T) {
	saved := AppriseConfig{Mode: AppriseModeHTTP, ServerURL: "https://apprise.operator.example"}
	cases := []struct {
		name      string
		saved     AppriseConfig
		requested AppriseConfig
		want      bool
	}{
		{"identical", saved, saved, true},
		{"trailing slash, default port, other key", saved, AppriseConfig{ServerURL: " https://Apprise.operator.example:443/ ", ConfigKey: "other"}, true},
		{"certificate check turned on", AppriseConfig{ServerURL: saved.ServerURL, SkipTLSVerify: true}, saved, true},
		{"another host", saved, AppriseConfig{ServerURL: "https://apprise.other.example"}, false},
		{"another port", saved, AppriseConfig{ServerURL: "https://apprise.operator.example:8443"}, false},
		{"plain HTTP", saved, AppriseConfig{ServerURL: "http://apprise.operator.example"}, false},
		{"certificate check turned off", saved, AppriseConfig{ServerURL: saved.ServerURL, SkipTLSVerify: true}, false},
		{"no saved server", AppriseConfig{}, AppriseConfig{ServerURL: "https://apprise.other.example"}, false},
	}
	for _, tc := range cases {
		if got := AppriseConfigKeepsServer(tc.saved, tc.requested); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestWebhookURLKeepsOrigin(t *testing.T) {
	const saved = "https://hooks.operator.example/alerts"
	cases := []struct {
		name      string
		saved     string
		requested string
		want      bool
	}{
		{"identical", saved, saved, true},
		{"identical template", "https://{{.Node}}.operator.example/a", "https://{{.Node}}.operator.example/a", true},
		{"path, query and templated path", saved, "https://hooks.operator.example:443/{{.Level}}?x={{.ID}}", true},
		{"another host", saved, "https://hooks.other.example/alerts", false},
		{"subdomain", saved, "https://evil.hooks.operator.example/alerts", false},
		{"another port", saved, "https://hooks.operator.example:8443/alerts", false},
		{"plain HTTP", saved, "http://hooks.operator.example/alerts", false},
		{"saved host as userinfo", saved, "https://hooks.operator.example@hooks.other.example/alerts", false},
		{"template after saved host", saved, `https://hooks.operator.example{{"@hooks.other.example"}}/alerts`, false},
		{"templated host", saved, "https://{{.Node}}/alerts", false},
		{"changed template host", "https://{{.Node}}.operator.example/a", "https://{{.Node}}.other.example/a", false},
		{"no saved address", "", "https://hooks.other.example/alerts", false},
		{"no address either side", "", "", false},
	}
	for _, tc := range cases {
		if got := WebhookURLKeepsOrigin(tc.saved, tc.requested); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
