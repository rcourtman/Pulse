//go:build !release

package api

func shouldEnforceReleaseDemoFixtureRuntime() bool {
	return false
}
