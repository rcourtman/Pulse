package tokens

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type TokenType string

const (
	TokenTypeRegistration TokenType = "registration"
)

type Token struct {
	Token        string    `json:"token"`
	Type         TokenType `json:"type"`
	Created      time.Time `json:"created"`
	Expires      time.Time `json:"expires"`
	MaxUses      int       `json:"maxUses"`
	UsedCount    int       `json:"usedCount"`
	AllowedTypes []string  `json:"allowedTypes"`
	CreatedBy    string    `json:"createdBy"`
	Description  string    `json:"description,omitempty"`
}

type TokenManager struct {
	mu         sync.RWMutex
	tokens     map[string]*Token
	configPath string
	stopCh     chan struct{}
	stopped    bool
}

func NewTokenManager(configPath string) *TokenManager {
	tm := &TokenManager{
		tokens:     make(map[string]*Token),
		configPath: configPath,
		stopCh:     make(chan struct{}),
		stopped:    false,
	}
	
	if err := tm.load(); err != nil {
		log.Warn().Err(err).Msg("Failed to load tokens, starting with empty store")
	}
	
	go tm.cleanupLoop()
	
	return tm
}

func (tm *TokenManager) GenerateToken(validityMinutes int, maxUses int, allowedTypes []string, createdBy string, description string) (*Token, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	tokenBytes := make([]byte, 8)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random token: %w", err)
	}
	
	tokenStr := fmt.Sprintf("PULSE-REG-%s", hex.EncodeToString(tokenBytes))
	
	token := &Token{
		Token:        tokenStr,
		Type:         TokenTypeRegistration,
		Created:      time.Now(),
		Expires:      time.Now().Add(time.Duration(validityMinutes) * time.Minute),
		MaxUses:      maxUses,
		UsedCount:    0,
		AllowedTypes: allowedTypes,
		CreatedBy:    createdBy,
		Description:  description,
	}
	
	tm.tokens[tokenStr] = token
	
	if err := tm.save(); err != nil {
		delete(tm.tokens, tokenStr)
		return nil, fmt.Errorf("failed to save token: %w", err)
	}
	
	log.Info().
		Str("token", tokenStr).
		Int("validity_minutes", validityMinutes).
		Int("max_uses", maxUses).
		Str("created_by", createdBy).
		Msg("Generated registration token")
	
	return token, nil
}

func (tm *TokenManager) ValidateToken(tokenStr string, nodeType string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	token, exists := tm.tokens[tokenStr]
	if !exists {
		return fmt.Errorf("invalid token")
	}
	
	if time.Now().After(token.Expires) {
		delete(tm.tokens, tokenStr)
		tm.save()
		return fmt.Errorf("token expired")
	}
	
	if token.MaxUses > 0 && token.UsedCount >= token.MaxUses {
		return fmt.Errorf("token usage limit exceeded")
	}
	
	if len(token.AllowedTypes) > 0 {
		allowed := false
		for _, t := range token.AllowedTypes {
			if t == nodeType {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("node type %s not allowed for this token", nodeType)
		}
	}
	
	token.UsedCount++
	
	if token.MaxUses > 0 && token.UsedCount >= token.MaxUses {
		log.Info().Str("token", tokenStr).Msg("Token reached max uses, removing")
		delete(tm.tokens, tokenStr)
	}
	
	if err := tm.save(); err != nil {
		token.UsedCount--
		return fmt.Errorf("failed to update token usage: %w", err)
	}
	
	log.Info().
		Str("token", tokenStr).
		Int("used_count", token.UsedCount).
		Int("max_uses", token.MaxUses).
		Msg("Token validated and used")
	
	return nil
}

func (tm *TokenManager) RevokeToken(tokenStr string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if _, exists := tm.tokens[tokenStr]; !exists {
		return fmt.Errorf("token not found")
	}
	
	delete(tm.tokens, tokenStr)
	
	if err := tm.save(); err != nil {
		return fmt.Errorf("failed to save after revoke: %w", err)
	}
	
	log.Info().Str("token", tokenStr).Msg("Token revoked")
	return nil
}

func (tm *TokenManager) ListTokens() []*Token {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	tokens := make([]*Token, 0, len(tm.tokens))
	for _, token := range tm.tokens {
		tokens = append(tokens, token)
	}
	
	return tokens
}

func (tm *TokenManager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			tm.cleanup()
		case <-tm.stopCh:
			log.Debug().Msg("Token manager cleanup loop stopped")
			return
		}
	}
}

// Stop stops the token manager and its cleanup goroutine
func (tm *TokenManager) Stop() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if !tm.stopped {
		close(tm.stopCh)
		tm.stopped = true
	}
}

func (tm *TokenManager) cleanup() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	now := time.Now()
	expired := []string{}
	
	for tokenStr, token := range tm.tokens {
		if now.After(token.Expires) {
			expired = append(expired, tokenStr)
		}
	}
	
	if len(expired) > 0 {
		for _, tokenStr := range expired {
			delete(tm.tokens, tokenStr)
			log.Debug().Str("token", tokenStr).Msg("Cleaned up expired token")
		}
		tm.save()
	}
}

func (tm *TokenManager) save() error {
	tokenFile := filepath.Join(tm.configPath, "tokens.json")
	
	data, err := json.MarshalIndent(tm.tokens, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(tokenFile, data, 0600)
}

func (tm *TokenManager) load() error {
	tokenFile := filepath.Join(tm.configPath, "tokens.json")
	
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	
	return json.Unmarshal(data, &tm.tokens)
}