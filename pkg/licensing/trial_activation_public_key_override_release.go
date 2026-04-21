//go:build release

package licensing

func allowHostedTrialActivationPublicKeyEnvOverride() bool {
	return false
}
