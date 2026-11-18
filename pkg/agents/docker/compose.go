package dockeragent

import (
	"regexp"
	"strings"

	"github.com/distribution/reference"
)

// ComposeImageChange contains review facts only. Agent-local registration
// owns paths, remote names, credentials and all deployment arguments.
type ComposeImageChange struct {
	Project     string `json:"project"`
	Service     string `json:"service"`
	Mode        string `json:"mode"`
	SourceHash  string `json:"sourceHash"`
	BaseCommit  string `json:"baseCommit,omitempty"`
	BaseBranch  string `json:"baseBranch,omitempty"`
	BeforeImage string `json:"beforeImage"`
	AfterImage  string `json:"afterImage"`
}

var composeNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
var composeHashPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var composeCommitPattern = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)

// ComposeHasImageTag distinguishes a tracked tag from a digest-only pin.
func ComposeHasImageTag(base string) bool {
	ref, err := reference.ParseNormalizedNamed(base)
	if err != nil {
		return false
	}
	_, tagged := ref.(reference.NamedTagged)
	return tagged
}

func (c *ComposeImageChange) Valid() bool {
	if c == nil || !composeNamePattern.MatchString(c.Project) || !composeNamePattern.MatchString(c.Service) || !composeHashPattern.MatchString(c.SourceHash) || len(c.BeforeImage) > 512 || len(c.AfterImage) > 600 || c.BeforeImage == c.AfterImage {
		return false
	}
	base, _, wasPinned := strings.Cut(c.BeforeImage, "@")
	if !wasPinned && !ComposeHasImageTag(base) {
		base += ":latest"
	}
	after, digest, pinned := strings.Cut(c.AfterImage, "@")
	if _, err := reference.ParseNormalizedNamed(c.BeforeImage); err != nil || !pinned || !composeHashPattern.MatchString(digest) || base != after {
		return false
	}
	switch c.Mode {
	case "local":
		return c.BaseBranch == "" && c.BaseCommit == ""
	case "git":
		if !composeCommitPattern.MatchString(c.BaseCommit) || len(c.BaseBranch) > 200 || strings.Contains(c.BaseBranch, "..") {
			return false
		}
		for _, segment := range strings.Split(c.BaseBranch, "/") {
			if !composeNamePattern.MatchString(segment) || strings.HasSuffix(segment, ".lock") || strings.HasSuffix(segment, ".") {
				return false
			}
		}
		return true
	}
	return false
}
func (c *ComposeImageChange) Clone() *ComposeImageChange {
	if c == nil {
		return nil
	}
	out := *c
	return &out
}
