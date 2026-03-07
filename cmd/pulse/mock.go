package main

import "github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli"

func getMockEnvPath() string {
	return pulsecli.GetMockEnvPath(cliRuntime)
}
