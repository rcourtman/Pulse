package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

func main() {
	if err := regeneratePulseIntelligenceDocs(); err != nil {
		fmt.Fprintf(os.Stderr, "generate-pulse-intelligence-docs: %v\n", err)
		os.Exit(1)
	}
}

func regeneratePulseIntelligenceDocs() error {
	manifest := agentcapabilities.CanonicalManifest()
	if err := regeneratePulseMCPReadme(filepath.Join("cmd", "pulse-mcp", "README.md"), manifest); err != nil {
		return err
	}
	if err := regeneratePublicPulseIntelligenceOverview(filepath.Join("docs", "AI.md"), manifest); err != nil {
		return err
	}
	return nil
}

func regeneratePulseMCPReadme(path string, manifest agentcapabilities.Manifest) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	content := string(raw)
	replacements := []struct {
		start string
		end   string
		body  string
	}{
		{
			start: agentcapabilities.MCPReadmeSurfaceContractStartMarker,
			end:   agentcapabilities.MCPReadmeSurfaceContractEndMarker,
			body:  agentcapabilities.MCPSurfaceContractMarkdown(manifest.SurfaceContract),
		},
		{
			start: agentcapabilities.MCPReadmeScopeListStartMarker,
			end:   agentcapabilities.MCPReadmeScopeListEndMarker,
			body:  agentcapabilities.ManifestRequiredScopeMarkdownList(manifest),
		},
		{
			start: agentcapabilities.MCPReadmeClientConfigStartMarker,
			end:   agentcapabilities.MCPReadmeClientConfigEndMarker,
			body:  agentcapabilities.MCPClientConfigMarkdown(manifest.MCPAdapter),
		},
		{
			start: agentcapabilities.MCPReadmeToolInventoryStartMarker,
			end:   agentcapabilities.MCPReadmeToolInventoryEndMarker,
			body:  agentcapabilities.MCPToolCapabilityInventoryMarkdown(manifest),
		},
		{
			start: agentcapabilities.MCPReadmePromptInventoryStartMarker,
			end:   agentcapabilities.MCPReadmePromptInventoryEndMarker,
			body:  agentcapabilities.MCPPromptInventoryMarkdown(manifest),
		},
		{
			start: agentcapabilities.MCPReadmeErrorInventoryStartMarker,
			end:   agentcapabilities.MCPReadmeErrorInventoryEndMarker,
			body:  agentcapabilities.MCPErrorCodeInventoryMarkdown(manifest),
		},
	}

	var replaceErr error
	for _, replacement := range replacements {
		content, replaceErr = replaceMarkedBlock(content, replacement.start, replacement.end, replacement.body)
		if replaceErr != nil {
			return replaceErr
		}
	}
	if content == string(raw) {
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func regeneratePublicPulseIntelligenceOverview(path string, manifest agentcapabilities.Manifest) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	content, err := replaceMarkedBlock(
		string(raw),
		agentcapabilities.PulseIntelligenceOverviewStartMarker,
		agentcapabilities.PulseIntelligenceOverviewEndMarker,
		agentcapabilities.PulseIntelligenceOverviewMarkdown(manifest.SurfaceContract),
	)
	if err != nil {
		return err
	}
	if content == string(raw) {
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func replaceMarkedBlock(content, startMarker, endMarker, body string) (string, error) {
	start := strings.Index(content, startMarker)
	if start < 0 {
		return "", fmt.Errorf("missing marker %s", startMarker)
	}
	bodyStart := start + len(startMarker)
	end := strings.Index(content[bodyStart:], endMarker)
	if end < 0 {
		return "", fmt.Errorf("missing marker %s", endMarker)
	}
	bodyEnd := bodyStart + end
	return content[:bodyStart] + "\n" + strings.TrimSpace(body) + "\n" + content[bodyEnd:], nil
}
