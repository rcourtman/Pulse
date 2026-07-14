// Command patrol-qualify runs independent-ground-truth Pulse Patrol
// qualification scenarios. Live fault injection is opt-in and restricted to
// exact-run-labelled disposable resources.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/qualification"
)

func main() {
	mode := flag.String("mode", "validate", "validate, list, live, replay, verify-replay, or compare")
	catalogDir := flag.String("catalog", "tests/qualification/patrol/scenarios", "scenario manifest directory")
	scenarioID := flag.String("scenario", "", "scenario id for live mode")
	baseURL := flag.String("url", "http://127.0.0.1:7655", "Pulse API base URL")
	username := flag.String("user", "admin", "Pulse API username")
	password := flag.String("password", "", "Pulse API password (prefer --password-env)")
	passwordEnv := flag.String("password-env", "PULSE_QUALIFY_PASSWORD", "environment variable containing the Pulse password")
	model := flag.String("model", "", "optional Patrol model override, restored after every run")
	expectedPulseVersion := flag.String("expected-pulse-version", "", "optional exact /api/version identity required from the tested Pulse runtime")
	dockerContext := flag.String("docker-context", "", "explicit Docker context for disposable resources")
	dockerSSHHost := flag.String("docker-ssh-host", "", "explicit SSH host whose Docker daemon holds disposable resources")
	allowSharedHost := flag.Bool("allow-shared-host", false, "allow exact-labelled fixtures on a manifest-approved shared Docker host")
	authorizeLive := flag.Bool("authorize-live-faults", false, "required acknowledgement for live fault injection")
	authorizeRemediation := flag.Bool("authorize-remediation", false, "separate acknowledgement required for action decisions or execution")
	repeats := flag.Int("repeats", 1, "number of independent live repetitions")
	repeatProfile := flag.String("repeat-profile", "", "use manifest repetition count: development, nightly, or qualification (overrides --repeats)")
	artifactRoot := flag.String("artifacts", "tmp/patrol-qualification", "artifact output root")
	replayPath := flag.String("replay-report", "", "captured report.json for deterministic scorer replay")
	replayBundlePath := flag.String("replay-bundle", "", "captured replay.json for ordered tool-transcript verification")
	reportsRoot := flag.String("reports", "tmp/patrol-qualification", "report tree for model comparison")
	qualificationTrack := flag.String("qualification-track", "", "optional launch gate for compare mode: watch, investigation, or remediation")
	publicationDir := flag.String("publication-dir", "", "optional directory for comparison.json, comparison.md, and checksums")
	flag.Parse()

	catalog, err := qualification.LoadCatalog(*catalogDir)
	if err != nil {
		fatal(err)
	}
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "validate":
		fmt.Printf("validated %d Patrol qualification manifests in %s\n", len(catalog.Manifests), *catalogDir)
	case "list":
		for _, manifest := range catalog.Manifests {
			fmt.Printf("%-44s %-14s %s\n", manifest.ID, manifest.Track, manifest.Title)
		}
	case "live":
		if !*authorizeLive {
			fatal(fmt.Errorf("live mode requires --authorize-live-faults"))
		}
		manifest, ok := catalog.ByID[*scenarioID]
		if !ok {
			fatal(fmt.Errorf("unknown scenario %q", *scenarioID))
		}
		repeatCount, err := liveRepeatCount(manifest, *repeats, *repeatProfile)
		if err != nil {
			fatal(err)
		}
		if manifest.Track == qualification.TrackRemediation && manifest.Remediation != nil &&
			manifest.Remediation.Decision != "observe" && !*authorizeRemediation {
			fatal(fmt.Errorf("scenario %q requires the separate --authorize-remediation gate", manifest.ID))
		}
		secret := *password
		if value := strings.TrimSpace(os.Getenv(*passwordEnv)); value != "" {
			secret = value
		}
		if secret == "" {
			fatal(fmt.Errorf("Pulse password is required through --password-env or --password"))
		}
		client, err := qualification.NewPulseClient(qualification.ClientConfig{BaseURL: *baseURL, Username: *username, Password: secret, Timeout: 15 * time.Minute})
		if err != nil {
			fatal(err)
		}
		target := qualification.DockerTarget{Context: strings.TrimSpace(*dockerContext), SSHHost: strings.TrimSpace(*dockerSSHHost), AllowSharedHost: *allowSharedHost}
		lab := qualification.NewDockerLab(nil, target)
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		gitSHA, dirty := qualification.GitEnvironment(ctx, nil, ".")
		failed := false
		for i := 0; i < repeatCount; i++ {
			runner, err := qualification.NewRunner(qualification.RunnerConfig{
				Manifest: manifest, Lab: lab, Client: client, ArtifactRoot: *artifactRoot,
				ModelOverride: *model, GitSHA: gitSHA, GitDirty: dirty,
				AuthorizeRemediation: *authorizeRemediation,
				ExpectedPulseVersion: *expectedPulseVersion,
			})
			if err != nil {
				fatal(err)
			}
			report, runErr := runner.Run(ctx)
			verdict := "PASS"
			if !report.Passed || runErr != nil {
				verdict = "FAIL"
				failed = true
			}
			fmt.Printf("[%s] %s model=%s recall=%.1f%% fp=%d artifacts=%s\n", verdict, report.RunID, report.Environment.Model, report.Score.Recall*100, report.Score.FalsePositives, filepath.Join(*artifactRoot, report.RunID))
			if runErr != nil {
				fmt.Fprintf(os.Stderr, "  error: %v\n", runErr)
			}
		}
		if failed {
			os.Exit(1)
		}
	case "replay":
		if strings.TrimSpace(*replayPath) == "" {
			fatal(fmt.Errorf("replay mode requires --replay-report"))
		}
		report, err := qualification.LoadReport(*replayPath)
		if err != nil {
			fatal(err)
		}
		replayed := qualification.ReplayScore(report)
		payload, _ := json.MarshalIndent(replayed.Score, "", "  ")
		fmt.Println(string(payload))
		if !replayed.Passed {
			os.Exit(1)
		}
	case "verify-replay":
		if strings.TrimSpace(*replayBundlePath) == "" {
			fatal(fmt.Errorf("verify-replay mode requires --replay-bundle"))
		}
		bundle, err := qualification.LoadReplayBundle(*replayBundlePath)
		if err != nil {
			fatal(err)
		}
		session, err := qualification.NewReplaySession(bundle)
		if err != nil {
			fatal(err)
		}
		for _, exchange := range bundle.Exchanges {
			if _, err := session.Call(exchange.ToolName, exchange.CanonicalInput); err != nil {
				fatal(err)
			}
		}
		if err := session.Complete(); err != nil {
			fatal(err)
		}
		fmt.Printf("verified ordered replay for run %s: %d tool exchanges, manifest %s\n", bundle.RunID, len(bundle.Exchanges), bundle.ManifestDigest)
	case "compare":
		paths, err := findReports(*reportsRoot)
		if err != nil {
			fatal(err)
		}
		comparison, err := qualification.CompareReports(paths)
		if err != nil {
			fatal(err)
		}
		if value := strings.TrimSpace(*qualificationTrack); value != "" {
			if err := qualification.ApplyQualificationGates(&comparison, catalog, qualification.Track(value)); err != nil {
				fatal(err)
			}
		}
		if output := strings.TrimSpace(*publicationDir); output != "" {
			if err := qualification.WriteComparisonReport(output, comparison); err != nil {
				fatal(err)
			}
			fmt.Fprintf(os.Stderr, "wrote Patrol model qualification publication to %s\n", output)
		}
		payload, _ := json.MarshalIndent(comparison, "", "  ")
		fmt.Println(string(payload))
		for _, verdict := range comparison.Qualification {
			if !verdict.Qualified {
				os.Exit(1)
			}
		}
	default:
		fatal(fmt.Errorf("unknown mode %q", *mode))
	}
}

func liveRepeatCount(manifest qualification.Manifest, explicit int, profile string) (int, error) {
	count := explicit
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "":
	case "development":
		count = manifest.Repeat.Development
	case "nightly":
		count = manifest.Repeat.Nightly
	case "qualification":
		count = manifest.Repeat.Qualification
	default:
		return 0, fmt.Errorf("unknown repeat profile %q", profile)
	}
	if count < 1 || count > 100 {
		return 0, fmt.Errorf("repeats must be between 1 and 100")
	}
	return count, nil
}

func findReports(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == "report.json" {
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	if err == nil && len(paths) == 0 {
		return nil, fmt.Errorf("no report.json files found under %s", root)
	}
	return paths, err
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "patrol-qualify:", err)
	os.Exit(2)
}
