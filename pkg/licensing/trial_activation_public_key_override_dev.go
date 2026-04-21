//go:build !release

package licensing

import (
	"os"
	"strings"
)

func allowHostedTrialActivationPublicKeyEnvOverride() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_HOSTED_MODE")), "true")
}
