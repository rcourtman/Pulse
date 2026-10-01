package installtests

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// These are reader-facing change sections, not internal qualification headings.
var releaseNoteChangeHeadings = map[string]bool{
	"what's improved": true, "what’s improved": true,
	"alerts and notifications": true, "disks and storage": true,
	"proxmox, pbs and backups": true, "truenas, vsphere and docker": true,
	"install, updates and agents": true, "updates and agents": true,
	"pulse pro, ai and hosted": true, "monitoring and service health": true,
	"security": true, "other improvements": true,
}

func stablePatchReleaseNotesIssues(notes, version, previous string) []string {
	var issues []string
	lines := strings.Split(strings.TrimSpace(notes), "\n")
	title := regexp.MustCompile(`^# Pulse v` + regexp.QuoteMeta(version) + `(?: Release Notes)?$`)
	if !title.MatchString(strings.TrimSpace(lines[0])) {
		issues = append(issues, "release title must name the exact current version")
	}
	changesSection, hasChanges := false, false
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			changesSection = releaseNoteChangeHeadings[strings.ToLower(strings.TrimPrefix(line, "## "))]
		} else if strings.HasPrefix(line, "#") {
			changesSection = false
		} else if changesSection && strings.HasPrefix(line, "- ") && len(strings.TrimSpace(line[2:])) > 0 {
			hasChanges = true
		}
	}
	if !hasChanges {
		issues = append(issues, "notes need non-empty user-facing change bullets")
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(notes), " "))
	for _, required := range []string{
		"not authenticode-signed", "unknown publisher warning",
		"rollback target is stable `v" + previous + "`",
	} {
		if !strings.Contains(normalized, required) {
			issues = append(issues, "missing operator safety disclosure: "+required)
		}
	}
	compatibleMobile := strings.Contains(normalized, "does not require a companion mobile release") ||
		strings.Contains(normalized, "pulse mobile works with this release unchanged") ||
		strings.Contains(normalized, "pulse mobile is being retired on 31 march 2027. it keeps working until then.")
	if !compatibleMobile {
		issues = append(issues, "missing companion mobile compatibility or approved retirement disclosure")
	}
	return issues
}

func TestStablePatchReleaseNotesAcceptAuthoredPublishedCopy(t *testing.T) {
	// Exact public source b50aeb6a8d9e38ab362b23f4ed15a010f675a90f.
	// A fixture is not permission to edit or re-render the published release.
	notes, err := os.ReadFile("testdata/release-notes-v6.4.5-authored.md")
	if err != nil {
		t.Fatal(err)
	}
	if issues := stablePatchReleaseNotesIssues(string(notes), "6.4.5", "6.4.1"); len(issues) > 0 {
		t.Fatal(issues)
	}
}

func TestStablePatchReleaseNotesKeepIdentityAndSafety(t *testing.T) {
	notes := "# Pulse v6.4.6 Release Notes\n\nPulse updates and History are clearer.\n\n" +
		"## What's improved\n\n- History stays with the current host.\n\n" +
		"## Before you upgrade\n\nWindows binaries are not Authenticode-signed. " +
		"Windows may show an Unknown Publisher warning. " +
		"This does not require a companion mobile release. " +
		"The rollback target is stable `v6.4.5`.\n"
	if issues := stablePatchReleaseNotesIssues(notes, "6.4.6", "6.4.5"); len(issues) > 0 {
		t.Fatal(issues)
	}
	for _, change := range []struct{ name, from, to string }{
		{"wrong title", "# Pulse v6.4.6", "# Pulse v6.4.5"},
		{"no changes", "- History stays with the current host.", ""},
		{"internal changes only", "## What's improved", "## Qualification"},
		{"signing omitted", "not Authenticode-signed", "signed"},
		{"publisher warning omitted", "Unknown Publisher warning", "warning"},
		{"mobile omitted", "does not require a companion mobile release", "requires a new mobile build"},
		{"wrong rollback", "rollback target is stable `v6.4.5`", "rollback target is stable `v6.4.1`"},
	} {
		t.Run(change.name, func(t *testing.T) {
			if issues := stablePatchReleaseNotesIssues(strings.ReplaceAll(notes, change.from, change.to), "6.4.6", "6.4.5"); len(issues) == 0 {
				t.Fatal("unsafe or mismatched notes accepted")
			}
		})
	}
	// Preserve the Markdown title/heading while exercising wrapped safety text.
	wrapped := strings.ReplaceAll(notes, "Windows binaries are", "Windows\nbinaries are")
	if issues := stablePatchReleaseNotesIssues(wrapped, "6.4.6", "6.4.5"); len(issues) > 0 {
		t.Fatal(issues)
	}
	for _, disclosure := range []string{
		"Pulse Mobile works with this release unchanged",
		"Pulse Mobile is being retired on 31 March 2027. It keeps working until then.",
	} {
		candidate := strings.ReplaceAll(notes, "This does not require a companion mobile release", disclosure)
		if issues := stablePatchReleaseNotesIssues(candidate, "6.4.6", "6.4.5"); len(issues) > 0 {
			t.Fatal(issues)
		}
	}
}
