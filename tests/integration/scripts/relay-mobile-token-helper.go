package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	apiTokenMetadataPurpose          = "purpose"
	apiTokenPurposeRelayMobileAccess = "relay_mobile_access"
	apiTokenMetadataIssuedVia        = "issued_via"
	defaultIssuedVia                 = "hosted_mobile_onboarding_proof"
)

type helperResult struct {
	Action      string            `json:"action"`
	DataDir     string            `json:"dataDir"`
	OrgID       string            `json:"orgId"`
	PrunedCount int               `json:"prunedCount"`
	Record      helperTokenRecord `json:"record"`
	Token       string            `json:"token"`
}

type validationResult struct {
	Action  string             `json:"action"`
	DataDir string             `json:"dataDir"`
	Found   bool               `json:"found"`
	Record  *helperTokenRecord `json:"record,omitempty"`
}

type helperTokenRecord struct {
	ID       string            `json:"id"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Name     string            `json:"name"`
	OrgID    string            `json:"orgId"`
	Scopes   []string          `json:"scopes"`
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func usage(message string) {
	if strings.TrimSpace(message) != "" {
		fmt.Fprintf(os.Stderr, "error: %s\n\n", message)
	}
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  go run ./tests/integration/scripts/relay-mobile-token-helper.go create --data-dir <dir> --org-id <id> [options]")
	os.Exit(1)
}

func defaultRelayMobileTokenName(now time.Time) string {
	return fmt.Sprintf("Pulse Mobile relay access %s", now.UTC().Format(time.RFC3339))
}

func matchesExistingProofToken(record config.APITokenRecord, orgID, issuedVia string) bool {
	if strings.TrimSpace(record.OrgID) != strings.TrimSpace(orgID) {
		return false
	}
	if record.Metadata == nil {
		return false
	}
	return strings.TrimSpace(record.Metadata[apiTokenMetadataPurpose]) == apiTokenPurposeRelayMobileAccess &&
		strings.TrimSpace(record.Metadata[apiTokenMetadataIssuedVia]) == strings.TrimSpace(issuedVia)
}

func pruneExistingProofTokens(tokens []config.APITokenRecord, orgID, issuedVia string) ([]config.APITokenRecord, int) {
	filtered := make([]config.APITokenRecord, 0, len(tokens))
	pruned := 0
	for _, token := range tokens {
		if matchesExistingProofToken(token, orgID, issuedVia) {
			pruned++
			continue
		}
		filtered = append(filtered, token)
	}
	return filtered, pruned
}

func createRelayMobileToken(args []string) {
	flags := flag.NewFlagSet("create", flag.ExitOnError)
	dataDir := flags.String("data-dir", "", "Path to the tenant root data directory that owns api_tokens.json")
	issuedVia := flags.String("issued-via", defaultIssuedVia, "Metadata marker used when pruning prior proof tokens")
	name := flags.String("name", "", "Optional token display name")
	orgID := flags.String("org-id", "", "Org ID to bind the token to")
	if err := flags.Parse(args); err != nil {
		fatalf("%v", err)
	}

	scopedDataDir := strings.TrimSpace(*dataDir)
	scopedOrgID := strings.TrimSpace(*orgID)
	scopedIssuedVia := strings.TrimSpace(*issuedVia)

	if scopedDataDir == "" {
		usage("--data-dir is required")
	}
	if scopedOrgID == "" {
		usage("--org-id is required")
	}
	if scopedIssuedVia == "" {
		usage("--issued-via is required")
	}

	tokenName := strings.TrimSpace(*name)
	if tokenName == "" {
		tokenName = defaultRelayMobileTokenName(time.Now().UTC())
	}

	persistence := config.NewConfigPersistence(scopedDataDir)
	existingTokens, err := persistence.LoadAPITokens()
	if err != nil {
		fatalf("load api tokens: %v", err)
	}
	filteredTokens, prunedCount := pruneExistingProofTokens(existingTokens, scopedOrgID, scopedIssuedVia)

	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		fatalf("generate relay-mobile token: %v", err)
	}
	record, err := config.NewAPITokenRecord(rawToken, tokenName, []string{config.ScopeRelayMobileAccess})
	if err != nil {
		fatalf("construct relay-mobile token record: %v", err)
	}
	record.OrgID = scopedOrgID
	record.Metadata = map[string]string{
		apiTokenMetadataIssuedVia: scopedIssuedVia,
		apiTokenMetadataPurpose:   apiTokenPurposeRelayMobileAccess,
	}

	cfg := &config.Config{APITokens: filteredTokens}
	cfg.APITokens = append(cfg.APITokens, *record)
	cfg.SortAPITokens()

	if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
		fatalf("persist relay-mobile token: %v", err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(helperResult{
		Action:      "create",
		DataDir:     scopedDataDir,
		OrgID:       scopedOrgID,
		PrunedCount: prunedCount,
		Record: helperTokenRecord{
			ID:       record.ID,
			Metadata: record.Metadata,
			Name:     record.Name,
			OrgID:    record.OrgID,
			Scopes:   append([]string{}, record.Scopes...),
		},
		Token: rawToken,
	}); err != nil {
		fatalf("encode result: %v", err)
	}
}

func validateRelayMobileToken(args []string) {
	flags := flag.NewFlagSet("validate", flag.ExitOnError)
	dataDir := flags.String("data-dir", "", "Path to the tenant root data directory that owns api_tokens.json")
	token := flags.String("token", "", "Raw token value to validate")
	if err := flags.Parse(args); err != nil {
		fatalf("%v", err)
	}

	scopedDataDir := strings.TrimSpace(*dataDir)
	rawToken := strings.TrimSpace(*token)
	if scopedDataDir == "" {
		usage("--data-dir is required")
	}
	if rawToken == "" {
		usage("--token is required")
	}

	persistence := config.NewConfigPersistence(scopedDataDir)
	tokens, err := persistence.LoadAPITokens()
	if err != nil {
		fatalf("load api tokens: %v", err)
	}

	cfg := &config.Config{APITokens: tokens}
	record, ok := cfg.ValidateAPIToken(rawToken)
	result := validationResult{
		Action:  "validate",
		DataDir: scopedDataDir,
		Found:   ok && record != nil,
	}
	if ok && record != nil {
		result.Record = &helperTokenRecord{
			ID:       record.ID,
			Metadata: record.Metadata,
			Name:     record.Name,
			OrgID:    record.OrgID,
			Scopes:   append([]string{}, record.Scopes...),
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fatalf("encode result: %v", err)
	}
}

func main() {
	log.Logger = zerolog.Nop()

	if len(os.Args) < 2 {
		usage("missing action")
	}

	switch strings.ToLower(strings.TrimSpace(os.Args[1])) {
	case "create":
		createRelayMobileToken(os.Args[2:])
	case "validate":
		validateRelayMobileToken(os.Args[2:])
	case "--help", "-h", "help":
		usage("")
	default:
		usage(fmt.Sprintf("unsupported action %q", os.Args[1]))
	}
}
