package installtests

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var externalImageDigest = regexp.MustCompile(`@sha256:[0-9a-f]{64}$`)

func TestEveryDockerfilePinsExternalBases(t *testing.T) {
	err := filepath.WalkDir(repoFile("."), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "node_modules" || entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasPrefix(entry.Name(), "Dockerfile") {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		stages := make(map[string]bool)
		scanner := bufio.NewScanner(file)
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			fields := strings.Fields(scanner.Text())
			if len(fields) < 2 || !strings.EqualFold(fields[0], "FROM") {
				continue
			}
			imageIndex := 1
			if strings.HasPrefix(fields[imageIndex], "--platform=") {
				imageIndex++
			}
			if imageIndex >= len(fields) {
				t.Fatalf("%s:%d has malformed FROM instruction", path, lineNumber)
			}
			image := fields[imageIndex]
			if !stages[image] && !externalImageDigest.MatchString(image) {
				t.Fatalf("%s:%d external base image %q is not pinned to a full sha256 digest", path, lineNumber, image)
			}
			for index := imageIndex + 1; index+1 < len(fields); index++ {
				if strings.EqualFold(fields[index], "AS") {
					stages[fields[index+1]] = true
					break
				}
			}
		}
		return scanner.Err()
	})
	if err != nil {
		t.Fatalf("audit Dockerfiles: %v", err)
	}
}

func TestNodeToolchainParity(t *testing.T) {
	workflowPaths, err := filepath.Glob(repoFile(".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}
	versionPattern := regexp.MustCompile(`node-version:\s*['"]?([^'"\s#]+)`)
	setupCount := 0
	versionCount := 0
	for _, path := range workflowPaths {
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(contentBytes)
		setupCount += strings.Count(content, "actions/setup-node@")
		for _, match := range versionPattern.FindAllStringSubmatch(content, -1) {
			versionCount++
			if match[1] != nodeToolchainLine {
				t.Fatalf("%s selects Node.js %s; all workflows must use governed line %s", path, match[1], nodeToolchainLine)
			}
		}
	}
	if setupCount == 0 || versionCount != setupCount {
		t.Fatalf("every setup-node step must select the governed line: setup=%d versions=%d", setupCount, versionCount)
	}

	workerBytes, err := os.ReadFile(repoFile("scripts", "release-preflight-worker.sh"))
	if err != nil {
		t.Fatalf("read release preflight worker: %v", err)
	}
	if !strings.Contains(string(workerBytes), `!= "`+nodeToolchainLine+`"`) {
		t.Fatalf("release preflight worker must enforce Node.js %s parity", nodeToolchainLine)
	}

	devcontainerBytes, err := os.ReadFile(repoFile(".devcontainer", "devcontainer.json"))
	if err != nil {
		t.Fatalf("read devcontainer config: %v", err)
	}
	devcontainer := string(devcontainerBytes)
	if !strings.Contains(devcontainer, `"version": "`+nodeToolchainLine+`"`) ||
		!regexp.MustCompile(`ghcr\.io/devcontainers/features/node@sha256:[0-9a-f]{64}`).MatchString(devcontainer) {
		t.Fatalf("devcontainer must use Node.js %s from a digest-pinned feature", nodeToolchainLine)
	}
}
