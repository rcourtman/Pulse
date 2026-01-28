// Package knowledge provides persistent storage for AI-learned information about guests
package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rs/zerolog/log"
)

// Category constants for note categorization
const (
	CategoryCredential = "credential"
	CategoryService    = "service"
	CategoryPath       = "path"
	CategoryConfig     = "config"
	CategoryLearning   = "learning"
	CategoryHistory    = "history"
	CategoryInfra      = "infrastructure" // Auto-discovered infrastructure facts
)

// Note represents a single piece of learned information
type Note struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"` // "service", "path", "credential", "config", "learning", "history"
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GuestKnowledge represents all knowledge about a specific guest
type GuestKnowledge struct {
	GuestID   string    `json:"guest_id"`
	GuestName string    `json:"guest_name"`
	GuestType string    `json:"guest_type"` // "vm", "container", "node", "host"
	Notes     []Note    `json:"notes"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store manages persistent knowledge storage with encryption
type Store struct {
	dataDir                              string
	mu                                   sync.RWMutex
	cache                                map[string]*GuestKnowledge
	crypto                               *crypto.CryptoManager
	discoveryContextProvider             func() string
	discoveryContextProviderForResources func(resourceIDs []string) string
}

var newCryptoManagerAt = crypto.NewCryptoManagerAt
var beforeKnowledgeWriteLock func()

// NewStore creates a new knowledge store with encryption
func NewStore(dataDir string) (*Store, error) {
	knowledgeDir := filepath.Join(dataDir, "knowledge")
	if err := os.MkdirAll(knowledgeDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create knowledge directory: %w", err)
	}

	// Initialize crypto manager for encryption (uses same key as other Pulse secrets)
	cryptoMgr, err := newCryptoManagerAt(dataDir)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize crypto for knowledge store, data will be unencrypted")
	}

	return &Store{
		dataDir: knowledgeDir,
		cache:   make(map[string]*GuestKnowledge),
		crypto:  cryptoMgr,
	}, nil
}

// guestFilePath returns the file path for a guest's knowledge
func (s *Store) guestFilePath(guestID string) string {
	// Sanitize guest ID for filesystem
	safeID := filepath.Base(guestID) // Prevent path traversal
	// Use .enc extension for encrypted files
	if s.crypto != nil {
		return filepath.Join(s.dataDir, safeID+".enc")
	}
	return filepath.Join(s.dataDir, safeID+".json")
}

// GetKnowledge retrieves knowledge for a guest
func (s *Store) GetKnowledge(guestID string) (*GuestKnowledge, error) {
	s.mu.RLock()
	if cached, ok := s.cache[guestID]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	// Load from disk
	if beforeKnowledgeWriteLock != nil {
		beforeKnowledgeWriteLock()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if cached, ok := s.cache[guestID]; ok {
		return cached, nil
	}

	filePath := s.guestFilePath(guestID)
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		// Try legacy unencrypted file
		legacyPath := filepath.Join(s.dataDir, filepath.Base(guestID)+".json")
		data, err = os.ReadFile(legacyPath)
		if os.IsNotExist(err) {
			// No knowledge yet, return empty
			knowledge := &GuestKnowledge{
				GuestID: guestID,
				Notes:   []Note{},
			}
			s.cache[guestID] = knowledge
			return knowledge, nil
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read knowledge file: %w", err)
		}
		// Legacy file found - will be encrypted on next save
		log.Info().Str("guest_id", guestID).Msg("Found unencrypted knowledge file, will encrypt on next save")
	} else if err != nil {
		return nil, fmt.Errorf("failed to read knowledge file: %w", err)
	}

	// Decrypt if crypto is available and file is encrypted
	if s.crypto != nil && filepath.Ext(filePath) == ".enc" {
		decrypted, err := s.crypto.Decrypt(data)
		if err != nil {
			// Try as plain JSON (migration case)
			var knowledge GuestKnowledge
			if jsonErr := json.Unmarshal(data, &knowledge); jsonErr == nil {
				log.Info().Str("guest_id", guestID).Msg("Loaded unencrypted knowledge (will encrypt on next save)")
				s.cache[guestID] = &knowledge
				return &knowledge, nil
			}
			return nil, fmt.Errorf("failed to decrypt knowledge: %w", err)
		}
		data = decrypted
	}

	var knowledge GuestKnowledge
	if err := json.Unmarshal(data, &knowledge); err != nil {
		return nil, fmt.Errorf("failed to parse knowledge file: %w", err)
	}

	s.cache[guestID] = &knowledge
	return &knowledge, nil
}

// SaveNote adds or updates a note for a guest
func (s *Store) SaveNote(guestID, guestName, guestType, category, title, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create knowledge
	knowledge, ok := s.cache[guestID]
	if !ok {
		// Try to load from disk first
		knowledge = &GuestKnowledge{
			GuestID:   guestID,
			GuestName: guestName,
			GuestType: guestType,
			Notes:     []Note{},
		}

		// Check for existing file
		filePath := s.guestFilePath(guestID)
		if data, err := os.ReadFile(filePath); err == nil {
			// Decrypt if needed
			if s.crypto != nil && filepath.Ext(filePath) == ".enc" {
				if decrypted, err := s.crypto.Decrypt(data); err == nil {
					data = decrypted
				}
			}
			if err := json.Unmarshal(data, &knowledge); err != nil {
				log.Warn().Err(err).Str("guest_id", guestID).Msg("Failed to parse existing knowledge, starting fresh")
			}
		}
		s.cache[guestID] = knowledge
	}

	// Update guest info if provided
	if guestName != "" {
		knowledge.GuestName = guestName
	}
	if guestType != "" {
		knowledge.GuestType = guestType
	}

	now := time.Now()

	// Check if note with same title exists in category
	found := false
	for i, note := range knowledge.Notes {
		if note.Category == category && note.Title == title {
			// Update existing note
			knowledge.Notes[i].Content = content
			knowledge.Notes[i].UpdatedAt = now
			found = true
			break
		}
	}

	if !found {
		// Add new note
		note := Note{
			ID:        fmt.Sprintf("%s-%d", category, len(knowledge.Notes)+1),
			Category:  category,
			Title:     title,
			Content:   content,
			CreatedAt: now,
			UpdatedAt: now,
		}
		knowledge.Notes = append(knowledge.Notes, note)
	}

	knowledge.UpdatedAt = now

	// Save to disk (encrypted)
	return s.saveToFile(guestID, knowledge)
}

// DeleteNote removes a note
func (s *Store) DeleteNote(guestID, noteID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	knowledge, ok := s.cache[guestID]
	if !ok {
		return fmt.Errorf("guest not found: %s", guestID)
	}

	// Find and remove note
	for i, note := range knowledge.Notes {
		if note.ID == noteID {
			knowledge.Notes = append(knowledge.Notes[:i], knowledge.Notes[i+1:]...)
			knowledge.UpdatedAt = time.Now()
			return s.saveToFile(guestID, knowledge)
		}
	}

	return fmt.Errorf("note not found: %s", noteID)
}

// GetNotesByCategory returns notes filtered by category
func (s *Store) GetNotesByCategory(guestID, category string) ([]Note, error) {
	knowledge, err := s.GetKnowledge(guestID)
	if err != nil {
		return nil, err
	}

	var notes []Note
	for _, note := range knowledge.Notes {
		if category == "" || note.Category == category {
			notes = append(notes, note)
		}
	}
	return notes, nil
}

// FormatForContext formats knowledge for injection into AI context
func (s *Store) FormatForContext(guestID string) string {
	knowledge, err := s.GetKnowledge(guestID)
	if err != nil {
		log.Warn().Err(err).Str("guest_id", guestID).Msg("Failed to load guest knowledge")
		return ""
	}

	if len(knowledge.Notes) == 0 {
		return ""
	}

	// Group notes by category
	byCategory := make(map[string][]Note)
	for _, note := range knowledge.Notes {
		byCategory[note.Category] = append(byCategory[note.Category], note)
	}

	// Build formatted output with guidance on using this knowledge
	var result string
	result = fmt.Sprintf("\n## Previously Learned Information about %s\n", knowledge.GuestName)
	result += "**If relevant to the current task, use this saved information directly instead of rediscovering it.**\n"

	categoryOrder := []string{"credential", "service", "path", "config", "learning", "history", "infrastructure"}
	categoryNames := map[string]string{
		"credential":     "Credentials",
		"service":        "Services",
		"path":           "Important Paths",
		"config":         "Configuration",
		"learning":       "Learnings",
		"history":        "Session History",
		"infrastructure": "Discovered Infrastructure",
	}

	for _, cat := range categoryOrder {
		notes, ok := byCategory[cat]
		if !ok || len(notes) == 0 {
			continue
		}

		result += fmt.Sprintf("\n### %s\n", categoryNames[cat])
		for _, note := range notes {
			result += fmt.Sprintf("- **%s**: %s\n", note.Title, note.Content)
		}
	}

	return result
}

// saveToFile persists knowledge to disk with encryption
func (s *Store) saveToFile(guestID string, knowledge *GuestKnowledge) error {
	data, err := json.MarshalIndent(knowledge, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal knowledge: %w", err)
	}

	// Encrypt if crypto manager is available
	if s.crypto != nil {
		encrypted, err := s.crypto.Encrypt(data)
		if err != nil {
			return fmt.Errorf("failed to encrypt knowledge: %w", err)
		}
		data = encrypted
	}

	filePath := s.guestFilePath(guestID)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write knowledge file: %w", err)
	}

	// Remove legacy unencrypted file if it exists
	if s.crypto != nil {
		legacyPath := filepath.Join(s.dataDir, filepath.Base(guestID)+".json")
		if _, err := os.Stat(legacyPath); err == nil {
			os.Remove(legacyPath)
			log.Info().Str("guest_id", guestID).Msg("Removed legacy unencrypted knowledge file")
		}
	}

	log.Debug().
		Str("guest_id", guestID).
		Int("notes", len(knowledge.Notes)).
		Bool("encrypted", s.crypto != nil).
		Msg("Saved guest knowledge")

	return nil
}

// ListGuests returns all guests that have knowledge stored
func (s *Store) ListGuests() ([]string, error) {
	files, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read knowledge directory: %w", err)
	}

	var guests []string
	for _, file := range files {
		ext := filepath.Ext(file.Name())
		if ext == ".json" || ext == ".enc" {
			guestID := file.Name()[:len(file.Name())-len(ext)]
			guests = append(guests, guestID)
		}
	}
	return guests, nil
}

// FormatAllForContext returns a summary of all saved knowledge across all guests
// This is used when no specific target is selected to give the AI full context
// To prevent context bloat, it limits output to maxGuests and maxBytes
func (s *Store) FormatAllForContext() string {
	const maxGuests = 10  // Only include the 10 most recently updated guests
	const maxBytes = 8000 // Cap total output at ~8KB to leave room for other context

	guests, err := s.ListGuests()
	if err != nil || len(guests) == 0 {
		return ""
	}

	// Load all guests with notes and sort by most recently updated
	type guestWithTime struct {
		id        string
		knowledge *GuestKnowledge
	}
	var guestsWithNotes []guestWithTime

	for _, guestID := range guests {
		knowledge, err := s.GetKnowledge(guestID)
		if err != nil || len(knowledge.Notes) == 0 {
			continue
		}
		guestsWithNotes = append(guestsWithNotes, guestWithTime{id: guestID, knowledge: knowledge})
	}

	if len(guestsWithNotes) == 0 {
		return ""
	}

	// Sort by UpdatedAt descending (most recent first)
	for i := 0; i < len(guestsWithNotes)-1; i++ {
		for j := i + 1; j < len(guestsWithNotes); j++ {
			if guestsWithNotes[j].knowledge.UpdatedAt.After(guestsWithNotes[i].knowledge.UpdatedAt) {
				guestsWithNotes[i], guestsWithNotes[j] = guestsWithNotes[j], guestsWithNotes[i]
			}
		}
	}

	// Track how many guests and notes we're including vs total
	totalGuests := len(guestsWithNotes)
	totalNotes := 0
	for _, g := range guestsWithNotes {
		totalNotes += len(g.knowledge.Notes)
	}

	// Limit to maxGuests
	truncatedGuests := false
	if len(guestsWithNotes) > maxGuests {
		guestsWithNotes = guestsWithNotes[:maxGuests]
		truncatedGuests = true
	}

	var sections []string
	includedNotes := 0
	currentBytes := 0

	for _, g := range guestsWithNotes {
		knowledge := g.knowledge

		// Build a summary for this guest
		guestName := knowledge.GuestName
		if guestName == "" {
			guestName = g.id
		}

		// Group notes by category
		byCategory := make(map[string][]Note)
		for _, note := range knowledge.Notes {
			byCategory[note.Category] = append(byCategory[note.Category], note)
		}

		var guestSection string
		guestSection = fmt.Sprintf("\n### %s (%s)", guestName, knowledge.GuestType)

		categoryOrder := []string{"credential", "service", "path", "config", "learning", "infrastructure"}
		for _, cat := range categoryOrder {
			notes, ok := byCategory[cat]
			if !ok || len(notes) == 0 {
				continue
			}
			for _, note := range notes {
				// Mask credentials in the summary
				content := note.Content
				if cat == "credential" && len(content) > 6 {
					content = content[:2] + "****" + content[len(content)-2:]
				}
				noteLine := fmt.Sprintf("\n- **%s**: %s", note.Title, content)

				// Check if adding this note would exceed our byte limit
				if currentBytes+len(guestSection)+len(noteLine) > maxBytes {
					// Stop adding notes, we've hit the limit
					if includedNotes > 0 {
						log.Warn().
							Int("total_notes", totalNotes).
							Int("included_notes", includedNotes).
							Int("total_guests", totalGuests).
							Int("max_bytes", maxBytes).
							Msg("Knowledge context truncated to prevent bloat - consider cleaning up old notes")
					}
					goto finalize
				}
				guestSection += noteLine
				includedNotes++
			}
		}

		currentBytes += len(guestSection)
		sections = append(sections, guestSection)
	}

finalize:
	if len(sections) == 0 {
		return ""
	}

	// Build result with info about truncation if applicable
	var header string
	if truncatedGuests || includedNotes < totalNotes {
		header = fmt.Sprintf("\n\n## Saved Knowledge (%d/%d notes from %d/%d guests, most recent)\n",
			includedNotes, totalNotes, len(sections), totalGuests)
	} else {
		header = fmt.Sprintf("\n\n## Saved Knowledge (%d notes across %d guests)\n", totalNotes, len(sections))
	}
	result := header
	result += "This is information learned from previous sessions. Use it to avoid rediscovery.\n"
	result += strings.Join(sections, "\n")

	return result
}

// FormatForContextForResources returns a summary of saved knowledge scoped to specific resources.
// This avoids dumping global notes into targeted patrol runs.
func (s *Store) FormatForContextForResources(resourceIDs []string) string {
	if len(resourceIDs) == 0 {
		return ""
	}

	const maxGuests = 10
	const maxBytes = 8000

	guests, err := s.ListGuests()
	if err != nil || len(guests) == 0 {
		return ""
	}

	resourceTokens := buildResourceIDTokenSet(resourceIDs)
	if len(resourceTokens) == 0 {
		return ""
	}

	// Load only matching guests with notes and sort by most recently updated
	type guestWithTime struct {
		id        string
		knowledge *GuestKnowledge
	}
	var guestsWithNotes []guestWithTime

	for _, guestID := range guests {
		if !matchesResourceTokens(guestID, resourceTokens) {
			continue
		}
		knowledge, err := s.GetKnowledge(guestID)
		if err != nil || len(knowledge.Notes) == 0 {
			continue
		}
		guestsWithNotes = append(guestsWithNotes, guestWithTime{id: guestID, knowledge: knowledge})
	}

	if len(guestsWithNotes) == 0 {
		return ""
	}

	// Sort by UpdatedAt descending (most recent first)
	for i := 0; i < len(guestsWithNotes)-1; i++ {
		for j := i + 1; j < len(guestsWithNotes); j++ {
			if guestsWithNotes[j].knowledge.UpdatedAt.After(guestsWithNotes[i].knowledge.UpdatedAt) {
				guestsWithNotes[i], guestsWithNotes[j] = guestsWithNotes[j], guestsWithNotes[i]
			}
		}
	}

	totalGuests := len(guestsWithNotes)
	totalNotes := 0
	for _, g := range guestsWithNotes {
		totalNotes += len(g.knowledge.Notes)
	}

	// Limit to maxGuests
	truncatedGuests := false
	if len(guestsWithNotes) > maxGuests {
		guestsWithNotes = guestsWithNotes[:maxGuests]
		truncatedGuests = true
	}

	var sections []string
	includedNotes := 0
	currentBytes := 0

	for _, g := range guestsWithNotes {
		knowledge := g.knowledge

		guestName := knowledge.GuestName
		if guestName == "" {
			guestName = g.id
		}

		byCategory := make(map[string][]Note)
		for _, note := range knowledge.Notes {
			byCategory[note.Category] = append(byCategory[note.Category], note)
		}

		guestSection := fmt.Sprintf("\n### %s (%s)", guestName, knowledge.GuestType)

		categoryOrder := []string{"credential", "service", "path", "config", "learning", "infrastructure"}
		for _, cat := range categoryOrder {
			notes, ok := byCategory[cat]
			if !ok || len(notes) == 0 {
				continue
			}
			for _, note := range notes {
				content := note.Content
				if cat == "credential" && len(content) > 6 {
					content = content[:2] + "****" + content[len(content)-2:]
				}
				noteLine := fmt.Sprintf("\n- **%s**: %s", note.Title, content)

				if currentBytes+len(guestSection)+len(noteLine) > maxBytes {
					if includedNotes > 0 {
						log.Warn().
							Int("total_notes", totalNotes).
							Int("included_notes", includedNotes).
							Int("total_guests", totalGuests).
							Int("max_bytes", maxBytes).
							Msg("Knowledge context truncated to prevent bloat - consider cleaning up old notes")
					}
					goto finalize
				}
				guestSection += noteLine
				includedNotes++
			}
		}

		currentBytes += len(guestSection)
		sections = append(sections, guestSection)
	}

finalize:
	if len(sections) == 0 {
		return ""
	}

	var header string
	if truncatedGuests || includedNotes < totalNotes {
		header = fmt.Sprintf("\n\n## Saved Knowledge (%d/%d notes from %d/%d guests, most recent)\n",
			includedNotes, totalNotes, len(sections), totalGuests)
	} else {
		header = fmt.Sprintf("\n\n## Saved Knowledge (%d notes across %d guests)\n", totalNotes, len(sections))
	}
	result := header
	result += "This is information learned from previous sessions. Use it to avoid rediscovery.\n"
	result += strings.Join(sections, "\n")

	return result
}

// SetDiscoveryContextProvider sets the function that provides discovery context.
// This allows the knowledge store to include deep-scanned infrastructure info
// (service versions, CLI access, config paths, ports) in the context for investigations.
func (s *Store) SetDiscoveryContextProvider(provider func() string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discoveryContextProvider = provider
}

// SetDiscoveryContextProviderForResources sets the provider for scoped discovery context.
// This allows Patrol to request discovery context for specific resources.
func (s *Store) SetDiscoveryContextProviderForResources(provider func(resourceIDs []string) string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discoveryContextProviderForResources = provider
}

// GetInfrastructureContext returns all discovered infrastructure formatted for AI context.
// This is specifically used by Patrol and investigations to understand where services run
// and how to interact with them (e.g., knowing PBS runs in Docker so commands need docker exec).
//
// It combines two sources:
// 1. Discovery data (deep-scanned service details, versions, CLI access, ports)
// 2. Legacy knowledge notes (for backward compatibility)
func (s *Store) GetInfrastructureContext() string {
	s.mu.RLock()
	discoveryProvider := s.discoveryContextProvider
	s.mu.RUnlock()

	var sb strings.Builder

	// First, include discovery context (the rich, deep-scanned data)
	if discoveryProvider != nil {
		if discoveryContext := discoveryProvider(); discoveryContext != "" {
			sb.WriteString(discoveryContext)
		}
	}

	// Then, include legacy knowledge notes (for backward compatibility)
	legacyContext := s.getLegacyInfrastructureContext()
	if legacyContext != "" {
		// Only add if we don't already have discovery context
		// (discovery is more comprehensive and replaces legacy notes)
		if sb.Len() == 0 {
			sb.WriteString(legacyContext)
		}
	}

	return sb.String()
}

// GetInfrastructureContextForResources returns discovery context scoped to specific resources.
// If no scoped provider is configured, it returns an empty string to avoid over-broad context.
func (s *Store) GetInfrastructureContextForResources(resourceIDs []string) string {
	if len(resourceIDs) == 0 {
		return s.GetInfrastructureContext()
	}

	s.mu.RLock()
	provider := s.discoveryContextProviderForResources
	s.mu.RUnlock()

	if provider == nil {
		return ""
	}

	return provider(resourceIDs)
}

func buildResourceIDTokenSet(resourceIDs []string) map[string]struct{} {
	tokens := make(map[string]struct{})
	for _, id := range resourceIDs {
		addResourceIDTokens(tokens, id)
	}
	return tokens
}

func addResourceIDTokens(tokens map[string]struct{}, resourceID string) {
	trimmed := strings.TrimSpace(resourceID)
	if trimmed == "" {
		return
	}

	addToken(tokens, trimmed)

	if last := lastSegment(trimmed, '/'); last != "" {
		addToken(tokens, last)
	}
	if last := lastSegment(trimmed, ':'); last != "" {
		addToken(tokens, last)
	}

	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "vm-") {
		addToken(tokens, trimmed[3:])
	}
	if strings.HasPrefix(lower, "ct-") {
		addToken(tokens, trimmed[3:])
	}
	if strings.HasPrefix(lower, "lxc-") {
		addToken(tokens, trimmed[4:])
	}

	if strings.Contains(lower, "qemu/") || strings.Contains(lower, "lxc/") || strings.HasPrefix(lower, "vm-") || strings.HasPrefix(lower, "ct-") {
		if digits := trailingDigits(trimmed); digits != "" {
			addToken(tokens, digits)
		}
	}

	if strings.Contains(trimmed, ":") {
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 2 {
			rest := parts[1]
			if slash := strings.Index(rest, "/"); slash >= 0 {
				host := strings.TrimSpace(rest[:slash])
				container := strings.TrimSpace(rest[slash+1:])
				addToken(tokens, host)
				addToken(tokens, container)
			}
		}
	}
}

func matchesResourceTokens(guestID string, tokens map[string]struct{}) bool {
	if guestID == "" || len(tokens) == 0 {
		return false
	}
	guestTokens := buildResourceIDTokenSet([]string{guestID})
	for token := range guestTokens {
		if _, ok := tokens[token]; ok {
			return true
		}
	}
	return false
}

func addToken(tokens map[string]struct{}, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	tokens[strings.ToLower(trimmed)] = struct{}{}
}

func lastSegment(value string, sep byte) string {
	if value == "" {
		return ""
	}
	idx := strings.LastIndexByte(value, sep)
	if idx == -1 || idx+1 >= len(value) {
		return ""
	}
	return value[idx+1:]
}

func trailingDigits(value string) string {
	if value == "" {
		return ""
	}
	i := len(value)
	for i > 0 {
		c := value[i-1]
		if c < '0' || c > '9' {
			break
		}
		i--
	}
	if i == len(value) {
		return ""
	}
	return value[i:]
}

// getLegacyInfrastructureContext returns infrastructure context from legacy knowledge notes.
func (s *Store) getLegacyInfrastructureContext() string {
	guests, err := s.ListGuests()
	if err != nil || len(guests) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## Discovered Infrastructure\n")
	sb.WriteString("The following services have been auto-discovered on your infrastructure.\n")
	sb.WriteString("Use this information to propose correct commands (e.g., use 'docker exec' for containerized services).\n\n")

	hasNotes := false
	for _, guestID := range guests {
		knowledge, err := s.GetKnowledge(guestID)
		if err != nil {
			continue
		}

		// Filter for infrastructure notes only
		var infraNotes []Note
		for _, note := range knowledge.Notes {
			if note.Category == CategoryInfra {
				infraNotes = append(infraNotes, note)
			}
		}

		if len(infraNotes) == 0 {
			continue
		}

		hasNotes = true
		guestName := knowledge.GuestName
		if guestName == "" {
			guestName = guestID
		}

		sb.WriteString(fmt.Sprintf("### %s\n", guestName))
		for _, note := range infraNotes {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", note.Title, note.Content))
		}
		sb.WriteString("\n")
	}

	if !hasNotes {
		return ""
	}

	return sb.String()
}
