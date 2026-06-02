package main

import "testing"

func TestProviderMSPCommandExposesBackup(t *testing.T) {
	cmd := newProviderMSPCmd()
	for _, child := range cmd.Commands() {
		if child.Name() == "backup" {
			foundCreate := false
			foundVerify := false
			for _, backupChild := range child.Commands() {
				switch backupChild.Name() {
				case "create":
					foundCreate = true
				case "verify":
					foundVerify = true
				}
			}
			if !foundCreate {
				t.Fatal("provider-msp backup create command is not registered")
			}
			if !foundVerify {
				t.Fatal("provider-msp backup verify command is not registered")
			}
			return
		}
	}
	t.Fatal("provider-msp backup command is not registered")
}
