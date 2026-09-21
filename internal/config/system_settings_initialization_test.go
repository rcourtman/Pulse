package config

import (
	"errors"
	"os"
	"testing"
)

type initializationReadFailure struct{ FileSystem }

func (f initializationReadFailure) ReadFile(string) ([]byte, error) { return nil, os.ErrPermission }

func TestInitializeSystemSettingsPreservesExistingBytes(t *testing.T) {
	for name, content := range map[string]string{"custom": `{"fullWidthMode":true,"connectionTimeout":23,"futureOption":"keep"}`, "malformed": `{"broken":`, "empty": ""} {
		t.Run(name, func(t *testing.T) {
			p := NewConfigPersistence(t.TempDir())
			if err := os.WriteFile(p.systemFile, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if err := p.InitializeSystemSettings(*DefaultSystemSettings()); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(p.systemFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != content {
				t.Fatalf("existing settings changed: %q", got)
			}
		})
	}
}
func TestInitializeSystemSettingsMissing(t *testing.T) {
	p := NewConfigPersistence(t.TempDir())
	if err := p.InitializeSystemSettings(*DefaultSystemSettings()); err != nil {
		t.Fatal(err)
	}
	got, err := p.LoadSystemSettings()
	if err != nil || got == nil {
		t.Fatalf("load initialized settings: %v", err)
	}
	got.FullWidthMode = true
	if err := p.SaveSystemSettings(*got); err != nil {
		t.Fatal(err)
	}
	if err := p.InitializeSystemSettings(*DefaultSystemSettings()); err != nil {
		t.Fatal(err)
	}
	got, err = p.LoadSystemSettings()
	if err != nil || !got.FullWidthMode {
		t.Fatalf("ordinary save lost after initialization: %v", err)
	}
}
func TestInitializeSystemSettingsReadError(t *testing.T) {
	p := NewConfigPersistence(t.TempDir())
	p.fs = initializationReadFailure{p.fs}
	if err := p.InitializeSystemSettings(*DefaultSystemSettings()); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected read error, got %v", err)
	}
	if _, err := os.Stat(p.systemFile); !os.IsNotExist(err) {
		t.Fatalf("unexpected settings write: %v", err)
	}
}
