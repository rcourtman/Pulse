package installtests

import (
	"encoding/xml"
	"os"
	"strings"
	"testing"
)

func TestUnraidCommunityAppTemplateContract(t *testing.T) {
	raw, err := os.ReadFile(repoFile("packaging", "unraid", "pulse.xml"))
	if err != nil {
		t.Fatalf("read packaging/unraid/pulse.xml: %v", err)
	}

	var tmpl struct {
		XMLName    xml.Name `xml:"Container"`
		Name       string   `xml:"Name"`
		Repository string   `xml:"Repository"`
		Privileged string   `xml:"Privileged"`
		WebUI      string   `xml:"WebUI"`
		Project    string   `xml:"Project"`
		Overview   string   `xml:"Overview"`
		Configs    []struct {
			Name     string `xml:"Name,attr"`
			Target   string `xml:"Target,attr"`
			Type     string `xml:"Type,attr"`
			Required string `xml:"Required,attr"`
			Value    string `xml:",chardata"`
		} `xml:"Config"`
	}
	if err := xml.Unmarshal(raw, &tmpl); err != nil {
		t.Fatalf("packaging/unraid/pulse.xml is not valid XML: %v", err)
	}

	if tmpl.Name != "pulse" {
		t.Fatalf("unexpected container name %q", tmpl.Name)
	}
	if !strings.HasPrefix(tmpl.Repository, "rcourtman/pulse:") {
		t.Fatalf("unexpected repository %q", tmpl.Repository)
	}
	if tmpl.Privileged != "false" {
		t.Fatalf("Unraid template must run unprivileged, got %q", tmpl.Privileged)
	}
	if tmpl.WebUI != "http://[IP]:[PORT:7655]" {
		t.Fatalf("unexpected WebUI %q", tmpl.WebUI)
	}
	if !strings.Contains(tmpl.Overview, "bootstrap-token") {
		t.Fatalf("overview should document bootstrap-token fallback")
	}

	hasPort := false
	hasDataPath := false
	hasDeployMethod := false
	for _, cfg := range tmpl.Configs {
		switch {
		case cfg.Type == "Port" && cfg.Target == "7655":
			hasPort = true
		case cfg.Type == "Path" && cfg.Target == "/data" && cfg.Required == "true":
			hasDataPath = true
		case cfg.Type == "Variable" && cfg.Target == "PULSE_DEPLOYMENT_METHOD" && cfg.Value == "container_other":
			hasDeployMethod = true
		}
	}
	if !hasPort || !hasDataPath || !hasDeployMethod {
		t.Fatalf("Unraid template missing required config entries: port=%v dataPath=%v deployMethod=%v", hasPort, hasDataPath, hasDeployMethod)
	}
}
