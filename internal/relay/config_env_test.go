package relay

import "testing"

const fileServerURL = "wss://file.example/ws/instance"

func TestApplyEnvOverridesTable(t *testing.T) {
	cases := []struct {
		name       string
		envEnabled string
		envURL     string
		in         Config
		want       Config
	}{
		{
			name:       "unset leaves config alone",
			envEnabled: "",
			envURL:     "",
			in:         Config{Enabled: true, ServerURL: fileServerURL},
			want:       Config{Enabled: true, ServerURL: fileServerURL},
		},
		{
			name:       "enabled true overrides file",
			envEnabled: "true",
			envURL:     "",
			in:         Config{Enabled: false, ServerURL: DefaultServerURL},
			want:       Config{Enabled: true, ServerURL: DefaultServerURL},
		},
		{
			name:       "enabled false overrides file-enabled relay",
			envEnabled: "false",
			envURL:     "",
			in:         Config{Enabled: true, ServerURL: DefaultServerURL},
			want:       Config{Enabled: false, ServerURL: DefaultServerURL},
		},
		{
			name:       "garbage bool does not override",
			envEnabled: "maybe",
			envURL:     "",
			in:         Config{Enabled: true, ServerURL: DefaultServerURL},
			want:       Config{Enabled: true, ServerURL: DefaultServerURL},
		},
		{
			name:       "valid server URL overrides file",
			envEnabled: "",
			envURL:     "wss://relay.test.example/ws/instance",
			in:         Config{Enabled: true, ServerURL: DefaultServerURL},
			want:       Config{Enabled: true, ServerURL: "wss://relay.test.example/ws/instance"},
		},
		{
			name:       "invalid server URL keeps file value",
			envEnabled: "",
			envURL:     "http://wrong-scheme.example/",
			in:         Config{Enabled: true, ServerURL: fileServerURL},
			want:       Config{Enabled: true, ServerURL: fileServerURL},
		},
		{
			name:       "both overrides apply together",
			envEnabled: "yes",
			envURL:     "wss://relay.test.example/ws/instance",
			in:         Config{Enabled: false, ServerURL: DefaultServerURL},
			want:       Config{Enabled: true, ServerURL: "wss://relay.test.example/ws/instance"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvRelayEnabled, tc.envEnabled)
			t.Setenv(EnvRelayServerURL, tc.envURL)

			cfg := tc.in
			ApplyEnvOverrides(&cfg)

			if cfg.Enabled != tc.want.Enabled {
				t.Errorf("Enabled = %v, want %v", cfg.Enabled, tc.want.Enabled)
			}
			if cfg.ServerURL != tc.want.ServerURL {
				t.Errorf("ServerURL = %q, want %q", cfg.ServerURL, tc.want.ServerURL)
			}
		})
	}
}

func TestApplyEnvOverridesNilConfigSafe(t *testing.T) {
	t.Setenv(EnvRelayEnabled, "true")
	t.Setenv(EnvRelayServerURL, "wss://relay.test.example/ws/instance")

	ApplyEnvOverrides(nil) // must not panic
}

func TestParseEnvBoolRecognizedValues(t *testing.T) {
	truthy := []string{"1", "true", "TRUE", "True", "yes", "y", "on", " on "}
	for _, v := range truthy {
		got, ok := parseEnvBool(v)
		if !ok || !got {
			t.Errorf("parseEnvBool(%q) = (%v, %v), want (true, true)", v, got, ok)
		}
	}
	falsy := []string{"0", "false", "FALSE", "no", "n", "off"}
	for _, v := range falsy {
		got, ok := parseEnvBool(v)
		if !ok || got {
			t.Errorf("parseEnvBool(%q) = (%v, %v), want (false, true)", v, got, ok)
		}
	}
	unrecognized := []string{"", "  ", "maybe", "2", "enable"}
	for _, v := range unrecognized {
		_, ok := parseEnvBool(v)
		if ok {
			t.Errorf("parseEnvBool(%q) reported ok=true, want false", v)
		}
	}
}
