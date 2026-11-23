package pulsecli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/declarativeconfig"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const maxDeclarativeConfigBytes int64 = 4 << 20 // 4 MiB

var declarativeConfigStdinIsTerminal = readerIsTerminal

func readerIsTerminal(r io.Reader) bool {
	if f, ok := r.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

type declarativeConfigApplyResponse struct {
	Applied  bool                             `json:"applied"`
	PlanHash string                           `json:"planHash"`
	Summary  declarativeconfig.PlanSummary    `json:"summary"`
	Changes  []declarativeconfig.ConfigChange `json:"changes"`
}

func newDeclarativeConfigCmd(deps *RemoteDeps) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage declarative monitoring configuration",
		Long:  "Pull, diff, and apply declarative pulse.monitoring/v1 monitoring configuration against a Pulse server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	addDeclarativeConfigSubcommands(configCmd, deps)
	return configCmd
}

func addDeclarativeConfigSubcommands(configCmd *cobra.Command, deps *RemoteDeps) {
	if configCmd == nil {
		return
	}
	configCmd.AddCommand(newConfigPullCmd(deps))
	configCmd.AddCommand(newConfigDiffCmd(deps))
	configCmd.AddCommand(newConfigApplyCmd(deps))
}

func newConfigPullCmd(deps *RemoteDeps) *cobra.Command {
	var formatFlag string
	var outputFlag string

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull declarative monitoring configuration from Pulse",
		Long:  "Export current Pulse monitoring configuration as a redacted pulse.monitoring/v1 YAML or JSON document.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format := strings.ToLower(strings.TrimSpace(formatFlag))
			if format == "" {
				if flagBoolValue(cmd, "json") {
					format = "json"
				} else {
					format = "yaml"
				}
			}
			if format != "yaml" && format != "json" {
				return fmt.Errorf("invalid format %q: must be yaml or json", format)
			}

			resp, err := invokeDeclarativeConfigCapability(
				cmd,
				deps,
				agentcapabilities.GetDeclarativeConfigCapabilityName,
				map[string]any{"format": format},
			)
			if err != nil {
				return err
			}

			payload := formatPulledDeclarativeConfig(resp.Body, format)
			outputPath := strings.TrimSpace(outputFlag)
			if outputPath != "" {
				if err := writeConfigExportFile(outputPath, payload); err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Wrote declarative configuration to %s\n", outputPath)
				return nil
			}

			_, err = cmd.OutOrStdout().Write(payload)
			return err
		},
	}

	cmd.Flags().StringVar(&formatFlag, "format", "", "Output format: yaml or json (defaults to yaml, or json when --json is set)")
	cmd.Flags().StringVarP(&outputFlag, "output", "o", "", "Write configuration atomically to a file with 0600 permissions")
	return cmd
}

func newConfigDiffCmd(deps *RemoteDeps) *cobra.Command {
	var fileFlag string
	var pruneFlag bool

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Preview declarative monitoring configuration changes",
		Long:  "Validate a pulse.monitoring/v1 document, resolve local env: secret references, and preview server-side changes without mutating state.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			doc, err := loadDeclarativeConfigDocument(cmd, deps, fileFlag)
			if err != nil {
				return err
			}

			resp, err := invokeDeclarativeConfigCapability(
				cmd,
				deps,
				agentcapabilities.PlanDeclarativeConfigCapabilityName,
				map[string]any{
					"document": doc,
					"prune":    pruneFlag,
				},
			)
			if err != nil {
				return err
			}

			var plan declarativeconfig.PlanResult
			if err := json.Unmarshal(resp.Body, &plan); err != nil {
				return fmt.Errorf("decode declarative config plan response: %w", err)
			}

			if shouldOutputJSON(cmd, remoteIsTerminal(deps)) {
				return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
			}
			return renderDeclarativePlanHuman(cmd.OutOrStdout(), plan)
		},
	}

	cmd.Flags().StringVarP(&fileFlag, "file", "f", "", "Path to a YAML or JSON declarative config file, or - for stdin")
	cmd.Flags().BoolVar(&pruneFlag, "prune", false, "Plan removals for existing items omitted from included sections")
	return cmd
}

func newConfigApplyCmd(deps *RemoteDeps) *cobra.Command {
	var fileFlag string
	var pruneFlag bool
	var planHashFlag string
	var yesFlag bool

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply declarative monitoring configuration atomically",
		Long:  "Apply a pulse.monitoring/v1 document to Pulse after validating drift and resolving local env: secret references.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			doc, err := loadDeclarativeConfigDocument(cmd, deps, fileFlag)
			if err != nil {
				return err
			}

			jsonMode := shouldOutputJSON(cmd, remoteIsTerminal(deps))
			planHash := strings.TrimSpace(planHashFlag)

			if planHash == "" {
				planResp, err := invokeDeclarativeConfigCapability(
					cmd,
					deps,
					agentcapabilities.PlanDeclarativeConfigCapabilityName,
					map[string]any{
						"document": doc,
						"prune":    pruneFlag,
					},
				)
				if err != nil {
					return err
				}

				var plan declarativeconfig.PlanResult
				if err := json.Unmarshal(planResp.Body, &plan); err != nil {
					return fmt.Errorf("decode declarative config plan response: %w", err)
				}

				if !plan.Summary.HasChanges {
					if jsonMode {
						return writePrettyJSON(cmd.OutOrStdout(), declarativeConfigApplyResponse{
							Applied:  false,
							PlanHash: plan.PlanHash,
							Summary:  plan.Summary,
							Changes:  plan.Changes,
						})
					}
					_, err := fmt.Fprintln(cmd.OutOrStdout(), "No changes to apply.")
					return err
				}

				planHash = strings.TrimSpace(plan.PlanHash)
				if !yesFlag {
					if strings.TrimSpace(fileFlag) == "-" || !declarativeConfigStdinIsTerminal(cmd.InOrStdin()) {
						if planHash != "" {
							return fmt.Errorf("confirmation required in non-interactive mode: pass --plan-hash %s from pulse config diff or pass --yes", planHash)
						}
						return fmt.Errorf("confirmation required in non-interactive mode: pass --plan-hash from pulse config diff or pass --yes")
					}

					promptOut := cmd.OutOrStdout()
					if jsonMode {
						promptOut = cmd.ErrOrStderr()
					}
					if err := renderDeclarativePlanHuman(promptOut, plan); err != nil {
						return err
					}
					if _, err := fmt.Fprint(promptOut, "Apply these changes? [y/N]: "); err != nil {
						return err
					}
					confirmed, err := readYesNoConfirmation(cmd.InOrStdin())
					if err != nil {
						return err
					}
					if !confirmed {
						return fmt.Errorf("apply cancelled")
					}
				}
			}

			applyArgs := map[string]any{
				"document": doc,
				"prune":    pruneFlag,
			}
			if planHash != "" {
				applyArgs["planHash"] = planHash
			} else if yesFlag {
				applyArgs["confirm"] = true
			}

			resp, err := invokeDeclarativeConfigCapability(
				cmd,
				deps,
				agentcapabilities.ApplyDeclarativeConfigCapabilityName,
				applyArgs,
			)
			if err != nil {
				return err
			}

			if jsonMode {
				return writePrettyJSONBytes(cmd.OutOrStdout(), resp.Body)
			}

			var result declarativeConfigApplyResponse
			if err := json.Unmarshal(resp.Body, &result); err != nil {
				return fmt.Errorf("decode declarative config apply response: %w", err)
			}
			return renderDeclarativeApplyHuman(cmd.OutOrStdout(), result)
		},
	}

	cmd.Flags().StringVarP(&fileFlag, "file", "f", "", "Path to a YAML or JSON declarative config file, or - for stdin")
	cmd.Flags().BoolVar(&pruneFlag, "prune", false, "Remove existing items omitted from included sections")
	cmd.Flags().StringVar(&planHashFlag, "plan-hash", "", "Reviewed plan hash from pulse config diff to guard against drift")
	cmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "Apply changes without an interactive confirmation prompt")
	return cmd
}

func loadDeclarativeConfigDocument(cmd *cobra.Command, deps *RemoteDeps, fileFlag string) (declarativeconfig.Document, error) {
	filePath := strings.TrimSpace(fileFlag)
	if filePath == "" {
		return declarativeconfig.Document{}, fmt.Errorf("config file is required (pass -f <file> or -f - for stdin)")
	}

	var raw []byte
	var err error
	if filePath == "-" {
		raw, err = readBoundedDeclarativeInput(cmd.InOrStdin(), maxDeclarativeConfigBytes)
		if err != nil {
			return declarativeconfig.Document{}, fmt.Errorf("read declarative config from stdin: %w", err)
		}
	} else {
		raw, err = ReadBoundedRegularFile(filePath, maxDeclarativeConfigBytes)
		if err != nil {
			return declarativeconfig.Document{}, fmt.Errorf("read declarative config file %q: %w", filePath, err)
		}
	}

	doc, err := declarativeconfig.UnmarshalDocument(raw)
	if err != nil {
		return declarativeconfig.Document{}, err
	}

	var getenv func(string) string
	if deps != nil {
		getenv = deps.Getenv
	}
	if err := declarativeconfig.ResolveEnvSecretRefs(&doc, func(key string) string {
		return cliGetenv(getenv, key)
	}); err != nil {
		return declarativeconfig.Document{}, err
	}

	return doc, nil
}

func readBoundedDeclarativeInput(r io.Reader, maxBytes int64) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("stdin reader is unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("input exceeds %d bytes", maxBytes)
	}
	return data, nil
}

func invokeDeclarativeConfigCapability(cmd *cobra.Command, deps *RemoteDeps, capabilityName string, args map[string]any) (agentcapabilities.HTTPCallResponse, error) {
	target, err := resolveRemoteTarget(cmd, deps)
	if err != nil {
		return agentcapabilities.HTTPCallResponse{}, err
	}
	resp, err := agentcapabilities.CallRequestResponseCapabilityHTTPByName(
		cmd.Context(),
		remoteHTTPClient(deps),
		target.BaseURL,
		target.Token,
		agentcapabilities.CanonicalManifest().Capabilities,
		capabilityName,
		args,
	)
	if err != nil {
		return agentcapabilities.HTTPCallResponse{}, fmt.Errorf("%s failed: %w", capabilityName, err)
	}
	if err := formatDeclarativeConfigHTTPError(capabilityName, resp); err != nil {
		return agentcapabilities.HTTPCallResponse{}, err
	}
	return resp, nil
}

func formatDeclarativeConfigHTTPError(operation string, resp agentcapabilities.HTTPCallResponse) error {
	if resp.OK() {
		return nil
	}
	if resp.StatusCode == http.StatusPaymentRequired {
		return formatLicenseRequiredError(operation, fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode)), resp.Body)
	}
	if env, ok := resp.StableErrorEnvelope(); ok {
		if resp.StatusCode == http.StatusConflict && env.Error == agentcapabilities.AgentErrCodeDeclarativeConfigPlanDrift {
			expected := strings.TrimSpace(env.Details["expectedPlanHash"])
			if expected != "" {
				return fmt.Errorf("configuration changed since planning (%s). Re-run pulse config diff to review the updated plan (expected plan hash: %s)", env.Error, expected)
			}
			return fmt.Errorf("configuration changed since planning (%s). Re-run pulse config diff to review the updated plan", env.Error)
		}
		return fmt.Errorf("%s failed (%s): %s", operation, env.Error, env.Message)
	}
	return formatCapabilityHTTPError(operation, resp)
}

func formatPulledDeclarativeConfig(raw []byte, format string) []byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	if format == "json" {
		var indented bytes.Buffer
		if err := json.Indent(&indented, trimmed, "", "  "); err == nil {
			indented.WriteByte('\n')
			return indented.Bytes()
		}
	}
	if bytes.HasSuffix(raw, []byte("\n")) {
		return raw
	}
	out := make([]byte, len(raw)+1)
	copy(out, raw)
	out[len(raw)] = '\n'
	return out
}

func renderDeclarativeChangeTable(w io.Writer, changes []declarativeconfig.ConfigChange) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "ACTION\tDOMAIN\tKEY\tSUMMARY"); err != nil {
		return err
	}
	for _, change := range changes {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", change.Action, change.Domain, change.Key, change.Summary); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func renderDeclarativePlanHuman(w io.Writer, plan declarativeconfig.PlanResult) error {
	if !plan.Summary.HasChanges && len(plan.Changes) == 0 {
		if _, err := fmt.Fprintln(w, "No changes detected."); err != nil {
			return err
		}
		if strings.TrimSpace(plan.PlanHash) != "" {
			if _, err := fmt.Fprintf(w, "Plan hash: %s\n", strings.TrimSpace(plan.PlanHash)); err != nil {
				return err
			}
		}
		return nil
	}

	if len(plan.Changes) > 0 {
		if err := renderDeclarativeChangeTable(w, plan.Changes); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(
		w,
		"Summary: %d create, %d update, %d delete, %d unchanged\n",
		plan.Summary.Creates,
		plan.Summary.Updates,
		plan.Summary.Deletes,
		plan.Summary.Unchanged,
	); err != nil {
		return err
	}
	if strings.TrimSpace(plan.PlanHash) != "" {
		if _, err := fmt.Fprintf(w, "Plan hash: %s\n", strings.TrimSpace(plan.PlanHash)); err != nil {
			return err
		}
	}
	return nil
}

func renderDeclarativeApplyHuman(w io.Writer, result declarativeConfigApplyResponse) error {
	if !result.Applied && !result.Summary.HasChanges && len(result.Changes) == 0 {
		_, err := fmt.Fprintln(w, "No changes to apply.")
		return err
	}
	if len(result.Changes) > 0 {
		if err := renderDeclarativeChangeTable(w, result.Changes); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(
		w,
		"Applied declarative configuration: %d created, %d updated, %d deleted, %d unchanged\n",
		result.Summary.Creates,
		result.Summary.Updates,
		result.Summary.Deletes,
		result.Summary.Unchanged,
	); err != nil {
		return err
	}
	if strings.TrimSpace(result.PlanHash) != "" {
		if _, err := fmt.Fprintf(w, "Plan hash: %s\n", strings.TrimSpace(result.PlanHash)); err != nil {
			return err
		}
	}
	return nil
}

func readYesNoConfirmation(r io.Reader) (bool, error) {
	if r == nil {
		return false, nil
	}
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read confirmation response: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
