package dockeragent

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/distribution/reference"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"gopkg.in/yaml.v3"
)

const maxComposeSourceBytes = 1 << 20

var composeProjectName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,127}$`)
var composeServiceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)
var composeImageDigest = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

// Registration is supplied by the operator of the agent. Labels identify a
// running service, but cannot grant authority over a file, repository or ref.
type composeSource struct {
	Project    string `json:"project"`
	Mode       string `json:"mode"`
	File       string `json:"file"`
	Repository string `json:"repository,omitempty"`
	Remote     string `json:"remote,omitempty"`
	Branch     string `json:"branch,omitempty"`
}

func readComposeSourceFile(path string) ([]byte, error) {
	initial, err := os.Lstat(path)
	if err != nil || !initial.Mode().IsRegular() {
		return nil, errors.New("expected a regular source file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || !stat.Mode().IsRegular() || !os.SameFile(initial, stat) || stat.Size() > maxComposeSourceBytes {
		return nil, errors.New("expected a regular Compose source within 1 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxComposeSourceBytes+1))
	if len(data) > maxComposeSourceBytes {
		return nil, errors.New("Compose source exceeds 1 MiB")
	}
	return data, err
}

func loadComposeSources(path string) (map[string]composeSource, error) {
	if path == "" {
		return nil, nil
	}
	if !filepath.IsAbs(path) {
		return nil, errors.New("source registry needs an absolute path")
	}
	data, err := readComposeSourceFile(path)
	if err != nil {
		return nil, err
	}
	var registry struct {
		Projects []composeSource `json:"projects"`
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&registry); err != nil {
		return nil, errors.New("invalid Compose source registry")
	}
	if d.Decode(new(any)) != io.EOF || len(registry.Projects) > 128 {
		return nil, errors.New("invalid Compose source registry")
	}
	result := make(map[string]composeSource, len(registry.Projects))
	for _, s := range registry.Projects {
		if !composeProjectName.MatchString(s.Project) || s.File == "" {
			return nil, errors.New("invalid Compose source project or file")
		}
		if _, duplicates := result[s.Project]; duplicates {
			return nil, errors.New("duplicate Compose project in source registry")
		}
		switch s.Mode {
		case "local":
			if !filepath.IsAbs(s.File) || s.Repository != "" || s.Remote != "" || s.Branch != "" {
				return nil, errors.New("local Compose source requires an absolute file path")
			}
		case "git":
			if !filepath.IsAbs(s.Repository) || !filepath.IsLocal(s.File) || filepath.Clean(s.File) != s.File || strings.HasPrefix(s.File, ".git/") || !composeServiceName.MatchString(s.Remote) || !validComposeBaseBranch(s.Branch) {
				return nil, errors.New("invalid Git Compose source")
			}
		default:
			return nil, errors.New("unknown Compose deployment owner")
		}
		result[s.Project] = s
	}
	return result, nil
}

// Limit Git ref arguments to unambiguous branch names, including nested names.
func validComposeBaseBranch(name string) bool {
	if len(name) > 200 || strings.Contains(name, "..") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if !composeServiceName.MatchString(part) || strings.HasSuffix(part, ".lock") || strings.HasSuffix(part, ".") {
			return false
		}
	}
	return true
}

// Unrelated fields and comments must survive editing byte for byte.
// Resolve one literal single-line image only. Compose itself subsequently
// validates the proposed file and its rendered service before a local deploy.
func composeImageEdit(data []byte, service, digest string) (before, after string, updated []byte, err error) {
	if len(data) > maxComposeSourceBytes || !composeServiceName.MatchString(service) || !composeImageDigest.MatchString(digest) {
		return "", "", nil, errors.New("invalid Compose image change")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if decoder.Decode(&doc) != nil || len(doc.Content) != 1 || decoder.Decode(new(yaml.Node)) != io.EOF {
		return "", "", nil, errors.New("expected one Compose document")
	}
	root := doc.Content[0]
	if root.Kind == yaml.MappingNode {
		for i := 0; i < len(root.Content); i += 2 {
			if root.Content[i].Value == "include" {
				return "", "", nil, errors.New("multi-file Compose projects require their deployment owner")
			}
		}
	}
	services, err := composeMappingField(root, "services")
	if err != nil {
		return "", "", nil, err
	}
	definition, err := composeMappingField(services, service)
	if err != nil {
		return "", "", nil, err
	}
	image, err := composeMappingField(definition, "image")
	if err != nil || image.Kind != yaml.ScalarNode || image.Tag != "!!str" || image.Anchor != "" {
		return "", "", nil, errors.New("service image must be a literal image reference")
	}
	for i := 0; i < len(definition.Content); i += 2 {
		switch definition.Content[i].Value {
		case "build", "extends":
			return "", "", nil, errors.New("service uses a build or extended definition")
		}
	}
	before = image.Value
	_, err = reference.ParseNormalizedNamed(before)
	if err != nil || strings.ContainsAny(before, "\r\n$") {
		return "", "", nil, errors.New("service image must be a literal registry reference")
	}
	// Retain the operator's familiar repository and tag spelling while pinning
	// the reviewed manifest. Other platform and environment settings stay put.
	base, _, wasPinned := strings.Cut(before, "@")
	if !wasPinned && !agentsdocker.ComposeHasImageTag(base) {
		base += ":latest"
	}
	after = base + "@" + digest
	if after == before {
		return "", "", nil, errors.New("service image is already pinned to this digest")
	}
	// Locate the scalar using YAML coordinates. Never replace another matching
	// image string elsewhere in the document or silently reformat the source.
	lines := bytes.SplitAfter(data, []byte{'\n'})
	if image.Line < 1 || image.Line > len(lines) || image.Column < 1 {
		return "", "", nil, errors.New("image source location unavailable")
	}
	line := lines[image.Line-1]
	runes := []rune(string(line))
	if image.Column > len(runes) {
		return "", "", nil, errors.New("image source location unavailable")
	}
	offset := len(string(runes[:image.Column-1]))
	quote := ""
	switch image.Style {
	case 0:
	case yaml.SingleQuotedStyle:
		quote = "'"
	case yaml.DoubleQuotedStyle:
		quote = "\""
	default:
		return "", "", nil, errors.New("unsupported image scalar style")
	}
	literal := quote + before + quote
	if !bytes.HasPrefix(line[offset:], []byte(literal)) {
		return "", "", nil, errors.New("image text is escaped or spans multiple lines")
	}
	replacement := make([]byte, 0, len(line)+len(after))
	replacement = append(replacement, line[:offset]...)
	replacement = append(replacement, []byte(quote+after+quote)...)
	replacement = append(replacement, line[offset+len(literal):]...)
	lines[image.Line-1] = replacement
	return before, after, bytes.Join(lines, nil), nil
}
func composeMappingField(mapping *yaml.Node, field string) (*yaml.Node, error) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("missing or indirect %s mapping", field)
	}
	seen := make(map[string]bool, len(mapping.Content)/2)
	var value *yaml.Node
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		if key.Kind != yaml.ScalarNode || key.Value == "<<" || seen[key.Value] {
			return nil, errors.New("ambiguous or merged Compose mapping")
		}
		seen[key.Value] = true
		if key.Value == field {
			value = mapping.Content[i+1]
		}
	}
	if value == nil {
		return nil, fmt.Errorf("missing %s in Compose source", field)
	}
	return value, nil
}

func composeSourceHash(data []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(data)) }
