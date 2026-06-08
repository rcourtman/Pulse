package chat

import (
	"strings"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

type assistantPlainTextResourceMatch struct {
	registration     tools.ResourceRegistration
	resourceType     string
	matchedReference string
}

var assistantPlainTextResourceTypes = []unifiedresources.ResourceType{
	unifiedresources.ResourceTypeAgent,
	unifiedresources.ResourceTypeVM,
	unifiedresources.ResourceTypeSystemContainer,
	unifiedresources.ResourceTypeAppContainer,
	unifiedresources.ResourceTypeStorage,
}

var assistantPlainTextReferenceStopwords = map[string]struct{}{
	"agent":      {},
	"app":        {},
	"container":  {},
	"current":    {},
	"data":       {},
	"deployment": {},
	"docker":     {},
	"host":       {},
	"kubernetes": {},
	"local":      {},
	"node":       {},
	"pod":        {},
	"resource":   {},
	"server":     {},
	"service":    {},
	"storage":    {},
	"system":     {},
	"vm":         {},
}

func assistantPromptRequestsLiveResourceRead(prompt string) bool {
	normalized := normalizeAssistantToolRoutingPrompt(prompt)
	if normalized == "" {
		return false
	}
	phraseSignals := []string{
		"read only",
		"look at",
		"fresh verification",
		"live runtime",
	}
	for _, signal := range phraseSignals {
		if strings.Contains(normalized, signal) {
			return true
		}
	}
	termSignals := []string{
		"run",
		"execute",
		"command",
		"shell",
		"bash",
		"terminal",
		"check",
		"inspect",
		"verify",
		"fresh",
		"live",
		"log",
		"logs",
		"tail",
		"journalctl",
		"cat ",
		"ls ",
		"df ",
		"du ",
		"free ",
		"uptime",
		"systemctl",
	}
	haystack := " " + normalized + " "
	for _, signal := range termSignals {
		if strings.Contains(haystack, " "+strings.TrimSpace(signal)+" ") {
			return true
		}
	}
	return strings.Contains(prompt, "`")
}

func resolvePlainTextAssistantResourceReference(prompt string, provider tools.UnifiedResourceProvider) (assistantPlainTextResourceMatch, bool) {
	liveIntent := assistantPromptRequestsLiveResourceRead(prompt)
	if provider == nil || !liveIntent {
		log.Debug().
			Bool("has_provider", provider != nil).
			Bool("live_intent", liveIntent).
			Msg("[ChatService] Plain-text resource reference resolver skipped")
		return assistantPlainTextResourceMatch{}, false
	}
	normalizedPrompt := normalizeAssistantPlainTextReference(prompt)
	if normalizedPrompt == "" {
		log.Debug().Msg("[ChatService] Plain-text resource reference resolver skipped empty prompt")
		return assistantPlainTextResourceMatch{}, false
	}

	matches := make(map[string]assistantPlainTextResourceMatch)
	scanned := 0
	registered := 0
	mentionedCount := 0
	for _, resourceType := range assistantPlainTextResourceTypes {
		for _, resource := range provider.GetByType(resourceType) {
			scanned++
			reg, ok := tools.CanonicalHandoffResourceRegistration(
				provider,
				resource.ID,
				resource.Name,
				string(unifiedresources.ContractResourceType(resource)),
				plainTextResourceRegistrationNode(resource),
			)
			if !ok {
				continue
			}
			registered++
			matchedReference, mentioned := plainTextResourceMatchedReference(normalizedPrompt, plainTextResourceReferenceCandidates(resource, reg))
			if !mentioned {
				continue
			}
			mentionedCount++
			key := plainTextResourceSelectionKey(reg, matchedReference)
			match := assistantPlainTextResourceMatch{
				registration:     reg,
				resourceType:     string(unifiedresources.ContractResourceType(resource)),
				matchedReference: matchedReference,
			}
			if existing, ok := matches[key]; !ok || plainTextResourceMatchPreferred(match, existing) {
				matches[key] = match
			}
		}
	}
	matches = plainTextResourceMatchesForPromptTarget(matches, normalizedPrompt)
	if len(matches) != 1 {
		log.Debug().
			Int("scanned", scanned).
			Int("registered", registered).
			Int("mentioned", mentionedCount).
			Int("matches", len(matches)).
			Msg("[ChatService] Plain-text resource reference was not resolved")
		return assistantPlainTextResourceMatch{}, false
	}
	for _, match := range matches {
		return match, true
	}
	return assistantPlainTextResourceMatch{}, false
}

func plainTextResourceMatchesForPromptTarget(matches map[string]assistantPlainTextResourceMatch, normalizedPrompt string) map[string]assistantPlainTextResourceMatch {
	if len(matches) <= 1 {
		return matches
	}
	preferredTypes := plainTextPromptPreferredResourceTypes(normalizedPrompt)
	if len(preferredTypes) == 0 {
		return matches
	}
	filtered := make(map[string]assistantPlainTextResourceMatch, len(matches))
	for key, match := range matches {
		if preferredTypes[match.resourceType] {
			filtered[key] = match
		}
	}
	if len(filtered) == 0 {
		return matches
	}
	return filtered
}

func plainTextPromptPreferredResourceTypes(normalizedPrompt string) map[string]bool {
	if assistantPromptContainsAny(normalizedPrompt, []string{
		" storage ",
		" datastore ",
		" pool ",
		" dataset ",
		" disk ",
		" volume ",
	}) {
		return map[string]bool{"storage": true}
	}
	if assistantPromptContainsAny(normalizedPrompt, []string{
		" container ",
		" containers ",
		" lxc ",
		" ct ",
		" app ",
		" service ",
	}) {
		return map[string]bool{
			"system-container": true,
			"app-container":    true,
		}
	}
	if assistantPromptContainsAny(normalizedPrompt, []string{
		" vm ",
		" vms ",
		" virtual machine ",
		" guest ",
	}) {
		return map[string]bool{"vm": true}
	}
	if assistantPromptContainsAny(normalizedPrompt, []string{
		" host ",
		" hosts ",
		" node ",
		" nodes ",
		" server ",
		" servers ",
		" machine ",
		" machines ",
	}) {
		return map[string]bool{"agent": true}
	}
	return nil
}

func attachPlainTextAssistantResourceContext(sessionID string, messages []Message, sessions *SessionStore, provider tools.UnifiedResourceProvider, readState unifiedresources.ReadState, prompt string) bool {
	if sessions == nil {
		return false
	}
	match, ok := resolvePlainTextAssistantResourceReference(prompt, provider)
	if !ok {
		if readStateProvider := plainTextUnifiedProviderFromReadState(readState); readStateProvider != nil {
			match, ok = resolvePlainTextAssistantResourceReference(prompt, readStateProvider)
		}
	}
	if !ok {
		return false
	}

	resolvedCtx := sessions.GetResolvedContext(sessionID)
	resolvedCtx.AddResolvedResource(match.registration)
	resolved, found := resolvedCtx.GetResolvedResourceByAlias(match.registration.Name)
	if !found || resolved == nil {
		log.Debug().
			Str("session_id", sessionID).
			Str("resource_kind", match.registration.Kind).
			Msg("[ChatService] Plain-text resource reference registration could not be re-read")
		return false
	}
	resolvedCtx.MarkExplicitAccess(resolved.GetResourceID())
	injectPlainTextResourceContextIntoLatestUserMessage(
		messages,
		buildPlainTextResourceContextDirective(match.resourceType),
		rewritePlainTextResourceReferenceForModel(prompt, match.matchedReference),
	)
	log.Debug().
		Str("session_id", sessionID).
		Str("resource_id", resolved.GetResourceID()).
		Str("resource_kind", match.registration.Kind).
		Msg("[ChatService] Attached plain-text resource reference as current_resource")
	return true
}

func plainTextUnifiedProviderFromReadState(readState unifiedresources.ReadState) tools.UnifiedResourceProvider {
	if registry, ok := readState.(*unifiedresources.ResourceRegistry); ok && registry != nil {
		return unifiedresources.NewUnifiedAIAdapter(registry)
	}
	if provider, ok := readState.(tools.UnifiedResourceProvider); ok {
		return provider
	}
	return nil
}

func injectPlainTextResourceContextIntoLatestUserMessage(messages []Message, directive string, providerSafePrompt string) {
	if len(messages) == 0 {
		return
	}
	lastIdx := len(messages) - 1
	if messages[lastIdx].Role != "user" {
		return
	}
	directive = strings.TrimSpace(directive)
	providerSafePrompt = strings.TrimSpace(providerSafePrompt)
	if directive == "" && providerSafePrompt == "" {
		return
	}
	parts := make([]string, 0, 2)
	if directive != "" {
		parts = append(parts, directive)
	}
	if providerSafePrompt != "" {
		parts = append(parts, "Provider-safe user request:\n"+providerSafePrompt)
	}
	messages[lastIdx].Content = strings.Join(parts, "\n\n---\n")
}

func rewritePlainTextResourceReferenceForModel(prompt string, matchedReference string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "Use current_resource as the target for the user's requested live read."
	}
	matchedTokens := strings.Fields(normalizeAssistantPlainTextReference(matchedReference))
	if len(matchedTokens) == 0 {
		return "Use current_resource as the target for the user's requested live read."
	}

	tokens := plainTextPromptTokens(prompt)
	for start := 0; start < len(tokens); start++ {
		if start+len(matchedTokens) > len(tokens) {
			break
		}
		matched := true
		for offset, expected := range matchedTokens {
			if tokens[start+offset].normalized != expected {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		replaceStart := tokens[start].start
		replaceEnd := tokens[start+len(matchedTokens)-1].end
		return strings.TrimSpace(prompt[:replaceStart] + "current_resource" + prompt[replaceEnd:])
	}

	return "Use current_resource as the target for the user's requested live read."
}

type plainTextPromptToken struct {
	normalized string
	start      int
	end        int
}

func plainTextPromptTokens(prompt string) []plainTextPromptToken {
	var tokens []plainTextPromptToken
	start := -1
	var b strings.Builder
	for idx, r := range prompt {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if start < 0 {
				start = idx
				b.Reset()
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		if start >= 0 {
			tokens = append(tokens, plainTextPromptToken{
				normalized: b.String(),
				start:      start,
				end:        idx,
			})
			start = -1
			b.Reset()
		}
	}
	if start >= 0 {
		tokens = append(tokens, plainTextPromptToken{
			normalized: b.String(),
			start:      start,
			end:        len(prompt),
		})
	}
	return tokens
}

func buildPlainTextResourceContextDirective(resourceType string) string {
	resourceType = strings.TrimSpace(resourceType)
	if resourceType == "" {
		resourceType = "resource"
	}
	return strings.Join([]string{
		"[Resource Context Handoff Instructions]",
		"Source: Pulse resource reference resolution",
		"Selected Resource: Pulse resolved one unambiguous user-referenced resource as the attached resource for this turn. Do not ask which server, service, container, VM, or resource the user means.",
		"Provider-Safe Target Rewrite: If the user question below contains a redacted host, server, resource, VM, container, app, or storage phrase, treat that phrase as current_resource for this turn.",
		"Tool Target Handle: When you need a read-only tool against the attached resource, use target_host=\"current_resource\" or resource_id=\"current_resource\". Do not copy a withheld or placeholder label into any tool argument.",
		"Resolved Resource Type: " + resourceType,
		"Read Tool Boundary: Call read-only tools against current_resource only when the user explicitly asks you to investigate live runtime state, asks for fresh verification, or specifically requests a read attempt.",
		"Data Boundary: Some user-authored resource labels may be withheld. The current_resource handle is the authoritative target; do not reveal or reconstruct raw aliases, hostnames, provider IDs, paths, or secret-bearing metadata, and do not repeat a withheld placeholder back to the user as if it were the resource name.",
		"Action Boundary: Context is read-only and grants no approval or execution authority. Any action requires the governed approval/action flow.",
	}, "\n")
}

func plainTextResourceRegistrationNode(resource unifiedresources.Resource) string {
	if resource.Proxmox != nil {
		return strings.TrimSpace(resource.Proxmox.NodeName)
	}
	return strings.TrimSpace(resource.ParentName)
}

func plainTextResourceReferenceCandidates(resource unifiedresources.Resource, reg tools.ResourceRegistration) []string {
	candidates := []string{
		reg.Name,
		resource.Name,
		unifiedresources.ResourceDisplayName(resource),
	}
	candidates = append(candidates, resource.Identity.Hostnames...)
	switch strings.TrimSpace(reg.Kind) {
	case "agent":
		if resource.Proxmox != nil {
			candidates = append(candidates, resource.Proxmox.NodeName)
		}
		if resource.Agent != nil {
			candidates = append(candidates, resource.Agent.Hostname, resource.Agent.AgentID)
		}
		if resource.Docker != nil {
			candidates = append(candidates, resource.Docker.Hostname, resource.Docker.HostSourceID, resource.Docker.AgentID)
		}
		if resource.TrueNAS != nil {
			candidates = append(candidates, resource.TrueNAS.Hostname)
		}
		if resource.VMware != nil {
			candidates = append(candidates, resource.VMware.RuntimeHostName, resource.VMware.HostUUID)
		}
	case "vm", "system-container":
		if resource.Proxmox != nil {
			candidates = append(candidates, resource.Proxmox.SourceID)
		}
	case "app-container":
		if resource.Docker != nil {
			candidates = append(candidates, resource.Docker.DisplayName, resource.Docker.CustomDisplayName)
		}
	}
	return appendUniquePlainTextCandidates(nil, candidates...)
}

func plainTextResourceMatchedReference(normalizedPrompt string, candidates []string) (string, bool) {
	for _, candidate := range candidates {
		normalizedCandidate := normalizeAssistantPlainTextReference(candidate)
		if normalizedCandidate == "" || plainTextReferenceIsUnsafe(normalizedCandidate) {
			continue
		}
		if strings.Contains(" "+normalizedPrompt+" ", " "+normalizedCandidate+" ") {
			return normalizedCandidate, true
		}
	}
	return "", false
}

func plainTextResourceSelectionKey(reg tools.ResourceRegistration, matchedReference string) string {
	kind := strings.TrimSpace(reg.Kind)
	matchedReference = strings.TrimSpace(matchedReference)
	if kind == "agent" && matchedReference != "" {
		return kind + "\x00" + matchedReference
	}
	key := kind + "\x00" + strings.TrimSpace(reg.HostUID) + "\x00" + strings.TrimSpace(reg.ProviderUID)
	if key == "\x00\x00" {
		key = kind + "\x00" + strings.TrimSpace(reg.Name)
	}
	return key
}

func plainTextResourceMatchPreferred(candidate, existing assistantPlainTextResourceMatch) bool {
	candidateExec := plainTextResourceRegistrationAllows(candidate.registration, "exec")
	existingExec := plainTextResourceRegistrationAllows(existing.registration, "exec")
	if candidateExec != existingExec {
		return candidateExec
	}
	candidateNameMatches := normalizeAssistantPlainTextReference(candidate.registration.Name) == candidate.matchedReference
	existingNameMatches := normalizeAssistantPlainTextReference(existing.registration.Name) == existing.matchedReference
	if candidateNameMatches != existingNameMatches {
		return candidateNameMatches
	}
	return false
}

func plainTextResourceRegistrationAllows(reg tools.ResourceRegistration, action string) bool {
	action = strings.TrimSpace(action)
	if action == "" {
		return false
	}
	for _, executor := range reg.Executors {
		for _, allowed := range executor.Actions {
			if strings.EqualFold(strings.TrimSpace(allowed), action) {
				return true
			}
		}
	}
	return false
}

func plainTextReferenceIsUnsafe(normalizedCandidate string) bool {
	if len(normalizedCandidate) < 3 {
		return true
	}
	if _, ok := assistantPlainTextReferenceStopwords[normalizedCandidate]; ok {
		return true
	}
	allDigits := true
	for _, r := range normalizedCandidate {
		if r == ' ' {
			continue
		}
		if !unicode.IsDigit(r) {
			allDigits = false
			break
		}
	}
	return allDigits
}

func normalizeAssistantPlainTextReference(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastSpace := true
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func appendUniquePlainTextCandidates(base []string, values ...string) []string {
	seen := make(map[string]struct{}, len(base)+len(values))
	for _, value := range base {
		key := strings.TrimSpace(value)
		if key == "" {
			continue
		}
		seen[strings.ToLower(key)] = struct{}{}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		base = append(base, value)
	}
	return base
}
