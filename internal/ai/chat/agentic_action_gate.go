package chat

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

// maxAdvertisedActionGateBlocks bounds the advertised-action gate. When the
// user asked for a lifecycle action, the session has resolved resources that
// advertise it, pulse_control was offered, and the model still ends the run
// with prose instead of a governed call, the gate refuses that final answer
// once and steers the model to submit pulse_control for each target. It fails
// open after this many refusals so a model that has a genuine reason not to
// act (which it must then state from tool evidence) cannot livelock.
const maxAdvertisedActionGateBlocks = 1

// lifecycleRequestPatterns maps operator phrasing to the canonical lifecycle
// verb pulse_control accepts. Order matters: "restart" must resolve before the
// bare "start" pattern is considered, and the reboot/restart pair is folded
// to one verb because the action lifecycle treats them as synonyms.
var lifecycleRequestPatterns = []struct {
	action  string
	pattern *regexp.Regexp
}{
	{action: "reboot", pattern: regexp.MustCompile(`\b(reboot|restart|power[- ]?cycle|bounce)\b`)},
	{action: "shutdown", pattern: regexp.MustCompile(`\b(shut ?down|power[- ]?off|halt)\b`)},
	{action: "stop", pattern: regexp.MustCompile(`\bstop\b`)},
	{action: "start", pattern: regexp.MustCompile(`\b(start|boot|power[- ]?on|bring up|spin up)\b`)},
}

// interrogativeLead matches messages that ask about an action rather than
// request one ("why did X reboot?", "is it safe to stop Y?"). Those must keep
// their investigative answer; the gate only applies to action requests.
var interrogativeLead = regexp.MustCompile(`^\s*(why|what|when|where|who|how|did|was|were|is|are|has|have|should|do|does|which|whether)\b`)

// requestedLifecycleAction reports the canonical lifecycle verb an operator
// message asks Pulse to perform, if any.
func requestedLifecycleAction(userText string) (string, bool) {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" || interrogativeLead.MatchString(text) {
		return "", false
	}
	for _, candidate := range lifecycleRequestPatterns {
		if candidate.pattern.MatchString(text) {
			return candidate.action, true
		}
	}
	return "", false
}

// latestUserRequest returns the most recent operator message in the run's
// input transcript, ignoring tool-result carriers that share the user role.
func latestUserRequest(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != "user" || msg.ToolResult != nil {
			continue
		}
		if content := strings.TrimSpace(msg.Content); content != "" {
			return content
		}
	}
	return ""
}

func providerToolOffered(offered []providers.Tool, name string) bool {
	for _, tool := range offered {
		if strings.TrimSpace(tool.Name) == name {
			return true
		}
	}
	return false
}

// buildAdvertisedActionGatePrompt is the user-role correction injected when
// the gate refuses a prose-only ending. It names the exact calls to make and
// forbids the invented-prerequisite failure mode seen in the field.
func buildAdvertisedActionGatePrompt(action string, targets []tools.AdvertisedActionTarget) string {
	lines := make([]string, 0, len(targets))
	for _, target := range targets {
		lines = append(lines, fmt.Sprintf("- pulse_control {\"type\":\"resource\",\"resource_id\":%q,\"action\":%q} (%s %s)", target.CanonicalID, target.Capability, target.Kind, target.Name))
	}
	noun := "resource advertises"
	if len(targets) != 1 {
		noun = "resources advertise"
	}
	return fmt.Sprintf(`BLOCKED: the user asked you to %s, and %d resolved %s that capability right now. Pulse offers pulse_control for exactly this, so a final answer that narrates next steps, manual commands, or prerequisites is not acceptable. Submit the governed action for each target now, one call per target, using the canonical resource id:
%s
Pulse owns planning, approval, execution, and verification from there; the user approves in Pulse, not by running commands. If a tool result in this turn reported a real boundary for a target, quote that exact result for that target instead. Do not invent prerequisites such as discovery, session or context binding, or guest-agent availability.`, action, len(targets), noun, strings.Join(lines, "\n"))
}
