package notifications

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

const (
	emailSubjectComponentBytes = 48
	emailSubjectIdentityLimit  = 3
)

// Subject labels are hints, not replacements for the full identity in the body.
// Keep their byte budget independent of character width and do not let supplied
// names introduce header controls. Truncation never splits a UTF-8 character.
func emailSubjectLabel(value string) string {
	return strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, value)), " ")
}

func boundEmailSubjectLabel(value string) string {
	if len(value) <= emailSubjectComponentBytes {
		return value
	}
	end := emailSubjectComponentBytes - len("…")
	for !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end] + "…"
}

type emailSubjectIdentity struct {
	resource, node, display string
}

func subjectIdentity(alert *alerts.Alert) emailSubjectIdentity {
	resource := emailSubjectLabel(alert.ResourceName)
	if resource == "" {
		resource = emailSubjectLabel(alert.ResourceID)
	}
	if resource == "" {
		resource = "Unknown resource"
	}
	node, display := emailSubjectLabel(alert.Node), emailSubjectLabel(alert.NodeDisplayName)
	if display == "" || display == node {
		display = node
	}
	return emailSubjectIdentity{resource, node, display}
}

func (identity emailSubjectIdentity) label() string {
	node := boundEmailSubjectLabel(identity.node)
	display := boundEmailSubjectLabel(identity.display)
	host := display
	if host == "" {
		host = "unknown host"
	} else if node != "" && identity.display != identity.node {
		// Keep the raw host alongside a renamed host: display names need not be
		// unique, and an alias must not hide the three-host case in #2635.
		host += " [" + node + "]"
	}
	return boundEmailSubjectLabel(identity.resource) + " (" + host + ")"
}

func emailSubjectIdentities(alertList []*alerts.Alert) string {
	// Deduplicate and sort full labels BEFORE bounding. Different long labels
	// must not disappear just because their shortened hints look alike.
	seen := make(map[emailSubjectIdentity]struct{}, len(alertList))
	identities := make([]emailSubjectIdentity, 0, len(alertList))
	for _, alert := range alertList {
		if alert == nil {
			continue
		}
		identity := subjectIdentity(alert)
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		identities = append(identities, identity)
	}
	sort.Slice(identities, func(i, j int) bool {
		a, b := identities[i], identities[j]
		if a.resource != b.resource {
			return a.resource < b.resource
		}
		if a.node != b.node {
			return a.node < b.node
		}
		return a.display < b.display
	})
	labels := make([]string, 0, emailSubjectIdentityLimit+1)
	for i, identity := range identities {
		if i == emailSubjectIdentityLimit {
			labels = append(labels, fmt.Sprintf("+%d more", len(identities)-i))
			break
		}
		labels = append(labels, identity.label())
	}
	return strings.Join(labels, ", ")
}

// This is email-only. Other destinations keep their own resolution titles and
// all destinations keep the existing occurrence-qualified recovery admission.
func resolvedEmailSubject(alertList []*alerts.Alert) string {
	var primary *alerts.Alert
	count := 0
	for _, alert := range alertList {
		if alert != nil {
			if primary == nil {
				primary = alert
			}
			count++
		}
	}
	if count == 0 {
		return ""
	}
	prefix := "Pulse alert resolved"
	if count > 1 {
		prefix = fmt.Sprintf("Pulse alerts resolved (%d)", count)
	} else if primary.Resolution.Outcome() != "" {
		prefix = "Pulse alert moved"
	}
	return prefix + ": " + emailSubjectIdentities(alertList)
}
