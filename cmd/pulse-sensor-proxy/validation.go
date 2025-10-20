package main

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	// nodeNameRegex validates node names (alphanumeric, dots, underscores, hyphens, 1-64 chars)
	// Must not start with hyphen to prevent SSH option injection
	nodeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

	// ipv4Regex validates IPv4 addresses
	ipv4Regex = regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`)

	// ipv6Regex validates IPv6 addresses (simplified)
	ipv6Regex = regexp.MustCompile(`^[0-9a-fA-F:]+$`)
)

var (
	allowedCommands = map[string]struct{}{
		"sensors":  {},
		"ipmitool": {},
	}
)

// sanitizeCorrelationID validates and sanitizes a correlation ID
// Returns a valid UUID, generating a new one if input is missing or invalid
func sanitizeCorrelationID(id string) string {
	if id == "" {
		return uuid.NewString()
	}
	if _, err := uuid.Parse(id); err != nil {
		return uuid.NewString()
	}
	return id
}

// validateNodeName checks if a node name is in valid format
func validateNodeName(name string) error {
	if name == "" {
		return fmt.Errorf("invalid node name")
	}

	if ipv4Regex.MatchString(name) {
		return nil
	}

	candidate := name
	if strings.HasPrefix(candidate, "[") && strings.HasSuffix(candidate, "]") {
		candidate = candidate[1 : len(candidate)-1]
	}

	if ip := net.ParseIP(candidate); ip != nil {
		return nil
	}

	if nodeNameRegex.MatchString(name) {
		return nil
	}

	return fmt.Errorf("invalid node name")
}

func validateCommand(name string, args []string) error {
	if err := validateCommandName(name); err != nil {
		return err
	}

	for _, arg := range args {
		if err := validateCommandArg(arg); err != nil {
			return err
		}
	}

	if name == "ipmitool" {
		if err := validateIPMIToolArgs(args); err != nil {
			return err
		}
	}

	return nil
}

func validateCommandName(name string) error {
	if name == "" {
		return errors.New("command required")
	}

	if strings.Contains(name, "/") {
		return errors.New("absolute command paths not allowed")
	}

	if _, ok := allowedCommands[name]; !ok {
		return fmt.Errorf("command %q not permitted", name)
	}

	if !isASCII(name) {
		return errors.New("command must be ASCII")
	}

	return nil
}

func validateCommandArg(arg string) error {
	if len(arg) == 0 {
		return nil
	}

	if len(arg) > 1024 {
		return errors.New("argument too long")
	}

	if !utf8.ValidString(arg) {
		return errors.New("argument contains invalid UTF-8")
	}

	if hasNullByte(arg) {
		return errors.New("argument contains null byte")
	}

	if !isASCII(arg) {
		return errors.New("argument must be ASCII")
	}

	if hasShellMeta(arg) {
		return errors.New("argument contains forbidden shell characters")
	}

	if strings.Contains(arg, "=") && !strings.HasPrefix(arg, "-") {
		return errors.New("environment-style arguments not permitted")
	}

	return nil
}

func validateIPMIToolArgs(args []string) error {
	lowered := make([]string, len(args))
	for i, arg := range args {
		lowered[i] = strings.ToLower(arg)
	}

	for i := 0; i < len(lowered); i++ {
		token := lowered[i]
		switch token {
		case "shell", "raw", "exec", "lanplus", "lanplusciphers":
			return errors.New("dangerous ipmitool arguments not permitted")
		case "chassis":
			if i+1 < len(lowered) {
				switch lowered[i+1] {
				case "power", "bootparam", "status", "policy":
					return errors.New("chassis operations not permitted")
				}
			}
		case "power", "reset", "off", "cycle", "bmc", "mc":
			return errors.New("power control commands not permitted")
		}
	}

	return nil
}

func hasShellMeta(s string) bool {
	forbidden := []string{";", "|", "&", "$", "`", "\\", ">", "<", "(", ")", "[", "]", "{", "}", "!", "~"}
	for _, ch := range forbidden {
		if strings.Contains(s, ch) {
			return true
		}
	}

	if strings.Contains(s, "..") {
		return true
	}

	if strings.ContainsAny(s, "\n\r\t") {
		return true
	}

	if strings.HasPrefix(s, "-") && strings.Contains(s, "=") {
		if strings.Contains(s, "/") {
			return true
		}
	}

	return false
}

func hasNullByte(s string) bool {
	return strings.IndexByte(s, 0) >= 0
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}
