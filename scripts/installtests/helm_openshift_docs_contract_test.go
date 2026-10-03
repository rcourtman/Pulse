package installtests

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// A new server bootstraps in the browser; API_TOKENS in a server Secret does
// not provision v6 API tokens. Check the two-stage installation and private
// agent Secret instead of requiring the obsolete server-secret recipe.
func openShiftDocsIssues(docs string) []string {
	var issues []string
	_, profile, found := strings.Cut(docs, "### OpenShift profile (Helm)\n")
	if !found {
		return []string{"missing OpenShift profile"}
	}
	profile, _, _ = strings.Cut(profile, "\n### ")
	commands := regexp.MustCompile("(?s)```bash\n(.*?)```").FindAllStringSubmatch(profile, -1)
	if len(commands) != 2 {
		return []string{"need separate server bootstrap and collector commands"}
	}
	require := func(text, value string) {
		if !strings.Contains(text, value) {
			issues = append(issues, "missing "+value)
		}
	}
	for _, command := range commands {
		require(command[1], "helm upgrade --install pulse pulse/pulse")
		require(command[1], "--namespace pulse")
		require(command[1], "--set openShift.enabled=true")
	}
	require(commands[0][1], "--create-namespace")
	if strings.Contains(commands[0][1], "secretEnv") ||
		strings.Contains(commands[0][1], "openShift.kubernetesAgent.enabled=true") {
		issues = append(issues, "server must bootstrap before a scoped agent token exists")
	}
	require(profile, "complete its browser setup")
	require(profile, "Create `pulse-agent-env` using the private-file steps above")
	require(commands[1][1], "--set openShift.kubernetesAgent.enabled=true")
	require(commands[1][1], "--set openShift.kubernetesAgent.clusterID=")
	require(commands[1][1], "--set agent.secretEnv.name=pulse-agent-env")
	require(commands[1][1], "--set 'agent.secretEnv.keys[0]=PULSE_TOKEN'")
	require(docs, "create secret generic pulse-agent-env")
	require(docs, `--from-file=PULSE_TOKEN="$HOME/.config/pulse/kubernetes-agent-token"`)
	// Inspect shell blocks only: prose warns against these unsafe arguments.
	unsafe := regexp.MustCompile(`--from-literal=(?:PULSE_TOKEN|API_TOKENS?)=|--set(?:-string)?\s+(?:agent|server)\.secretEnv\.data\.`)
	for _, command := range regexp.MustCompile("(?s)```bash\n(.*?)```").FindAllStringSubmatch(docs, -1) {
		if unsafe.MatchString(command[1]) {
			issues = append(issues, "credential must not appear in kubectl arguments or Helm values")
		}
	}
	return issues
}

func TestOpenShiftDocsKeepBootstrapAndPrivateSecret(t *testing.T) {
	content, err := os.ReadFile(repoFile("docs", "KUBERNETES.md"))
	if err != nil {
		t.Fatal(err)
	}
	docs := string(content)
	if issues := openShiftDocsIssues(docs); len(issues) > 0 {
		t.Fatal(issues)
	}
	for _, change := range []struct{ name, from, to string }{
		{"profile", "### OpenShift profile (Helm)", "### Other profile"},
		{"bootstrap first", "--namespace pulse --create-namespace", "--namespace pulse --set openShift.kubernetesAgent.enabled=true"},
		{"browser setup", "complete its browser setup", "reuse an administrator token"},
		{"profile flag", "--set openShift.enabled=true", "--set openShift.enabled=false"},
		{"collector enabled", "--set openShift.kubernetesAgent.enabled=true", "--set openShift.kubernetesAgent.enabled=false"},
		{"cluster identity", "--set openShift.kubernetesAgent.clusterID=", "--set unrelated="},
		{"secret creation", "create secret generic pulse-agent-env", "create secret generic wrong-agent-env"},
		{"secret reference", "agent.secretEnv.name=pulse-agent-env", "agent.secretEnv.name=wrong-agent-env"},
		{"secret key", "agent.secretEnv.keys[0]=PULSE_TOKEN", "agent.secretEnv.keys[0]=WRONG_TOKEN"},
		{"private-file input", `--from-file=PULSE_TOKEN="$HOME/.config/pulse/kubernetes-agent-token"`, "--from-literal=PULSE_TOKEN=synthetic-not-a-secret"},
		{"Helm token value", "--set agent.secretEnv.name=pulse-agent-env", "--set-string agent.secretEnv.data.PULSE_TOKEN=synthetic-not-a-secret"},
	} {
		t.Run(change.name, func(t *testing.T) {
			mutated := strings.ReplaceAll(docs, change.from, change.to)
			if mutated == docs {
				t.Fatal("negative control did not change the guide")
			}
			if issues := openShiftDocsIssues(mutated); len(issues) == 0 {
				t.Fatal("unsafe or incomplete OpenShift setup accepted")
			}
		})
	}
}
