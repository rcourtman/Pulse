package tools

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
)

// diskUsageMaxLevels bounds how far disk_usage follows the largest folder.
const diskUsageMaxLevels = 12

// diskUsagePlainPath is a path du can take without quoting. Anything else is
// single-quoted, which the agent's command policy also reads.
var diskUsagePlainPath = regexp.MustCompile(`^[A-Za-z0-9/._+:@,=-]+$`)

type diskUsageEntry struct {
	path string
	kb   int64
}

type diskUsageLevel struct {
	path     string
	kb       int64
	children []diskUsageEntry // largest first
}

// executeReadDiskUsage answers "which folder holds the space" in one call.
//
// A disk alert is answered by the folder that filled it, not by the mount.
// Patrol drilled with du one level per call and, on 15 November 2026, spent
// its evidence budget and concluded at frigate/recordings/2026-11-15/02, one
// folder above the camera folder that held the space, while the prompt
// already told it to measure several levels per call. Here Pulse runs
// du -x -d 1 itself, level by level through the same read gates as exec,
// and follows the largest folder while it holds most of its parent.
func (e *PulseToolExecutor) executeReadDiskUsage(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	dir, _ := args["path"].(string)
	targetHost, _ := args["target_host"].(string)
	resourceRef, _ := args["resource_id"].(string)
	dockerContainer, legacyContainerArg := resolveContainerArg(args)

	dir = strings.TrimSpace(dir)
	if dir == "" {
		return NewErrorResult(fmt.Errorf("path is required for disk_usage: the mount or directory to size")), nil
	}
	if !strings.HasPrefix(dir, "/") {
		return NewErrorResult(fmt.Errorf("path must be absolute (start with /)")), nil
	}
	if strings.ContainsAny(dir, "*?[") {
		return NewErrorResult(fmt.Errorf("disk_usage takes one directory, not a pattern: name the directory and it sizes every folder below it")), nil
	}
	targetHost = e.readTargetHost(targetHost, resourceRef)
	if targetHost == "" {
		return NewErrorResult(fmt.Errorf("target_host is required")), nil
	}
	resolvedTargetHost, err := e.resolveCurrentResourceCommandTarget(targetHost)
	if err != nil {
		return NewErrorResult(err), nil
	}
	targetHost = resolvedTargetHost
	if legacyContainerArg {
		return NewErrorResult(fmt.Errorf("app_container is no longer supported; use app-container")), nil
	}

	top := path.Clean(dir)
	current := top
	var levels []diskUsageLevel
	var stop string
	unreadable := 0
	for {
		result, refused := e.runReadOnlyCommand(ctx, targetHost, dockerContainer, diskUsageCommand(current))
		if refused != nil {
			if len(levels) == 0 {
				return *refused, nil
			}
			stop = fmt.Sprintf("Pulse could not read inside %s, so this is as deep as it looked.", current)
			break
		}
		unreadable += strings.Count(result.Stderr, "\n")
		if result.Stderr != "" && !strings.HasSuffix(result.Stderr, "\n") {
			unreadable++
		}
		level, ok := parseDiskUsageLevel(result.Stdout, current)
		if !ok {
			if len(levels) == 0 {
				output := strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
				if redacted, n := safety.RedactSensitiveText(output); n > 0 {
					output = redacted
				}
				return NewTextResult(fmt.Sprintf("du gave no size for %s (exit code %d):\n%s", current, result.ExitCode, output)), nil
			}
			stop = fmt.Sprintf("du gave no size for %s, so this is as deep as Pulse looked.", current)
			break
		}
		levels = append(levels, level)
		if len(level.children) == 0 {
			stop = "It has no subfolders: its own files hold the space."
			break
		}
		largest := level.children[0]
		if largest.kb*2 <= level.kb {
			stop = "No single folder below it holds most of its space."
			break
		}
		if len(levels) >= diskUsageMaxLevels {
			stop = fmt.Sprintf("Stopped after %d levels; call disk_usage on %s to go deeper.", diskUsageMaxLevels, largest.path)
			break
		}
		current = largest.path
	}

	output := formatDiskUsage(targetHost, levels, stop, unreadable)
	if redacted, n := safety.RedactSensitiveText(output); n > 0 {
		output = redacted + fmt.Sprintf("\n\n[redacted %d sensitive value(s)]", n)
	}
	return NewTextResult(output), nil
}

func diskUsageCommand(dir string) string {
	arg := dir
	if !diskUsagePlainPath.MatchString(dir) {
		arg = shellEscape(dir)
	}
	return "du -x -k -d 1 " + arg
}

// parseDiskUsageLevel reads `du -k -d 1 dir` output: the size of dir and of
// each folder directly below it. Lines that are not "<KiB>\t<path>" under dir
// are ignored, so a stray name with a newline cannot add a level.
func parseDiskUsageLevel(stdout, dir string) (diskUsageLevel, bool) {
	level := diskUsageLevel{path: dir, kb: -1}
	for _, line := range strings.Split(stdout, "\n") {
		size, name, found := strings.Cut(line, "\t")
		if !found {
			continue
		}
		kb, err := strconv.ParseInt(strings.TrimSpace(size), 10, 64)
		if err != nil || kb < 0 {
			continue
		}
		name = path.Clean(name)
		switch {
		case name == dir:
			level.kb = kb
		case path.Dir(name) == dir:
			level.children = append(level.children, diskUsageEntry{path: name, kb: kb})
		}
	}
	if level.kb < 0 {
		return diskUsageLevel{}, false
	}
	sort.SliceStable(level.children, func(i, j int) bool {
		return level.children[i].kb > level.children[j].kb
	})
	return level, true
}

func formatDiskUsage(host string, levels []diskUsageLevel, stop string, unreadable int) string {
	var b strings.Builder
	top := levels[0]
	fmt.Fprintf(&b, "Folder sizes on %s from %s (this filesystem only), following the largest folder down:\n", host, top.path)
	fmt.Fprintf(&b, "%s %s\n", top.path, formatKiB(top.kb))
	for i := 1; i < len(levels); i++ {
		parent := levels[i-1]
		fmt.Fprintf(&b, "%s %s, %s of %s", levels[i].path, formatKiB(levels[i].kb), percentOf(levels[i].kb, parent.kb), path.Base(parent.path))
		if others := otherEntries(parent.children, levels[i].path, 3); others != "" {
			fmt.Fprintf(&b, " (beside it: %s)", others)
		}
		b.WriteString("\n")
	}

	last := levels[len(levels)-1]
	fmt.Fprintf(&b, "\nLargest folder: %s holds %s", last.path, formatKiB(last.kb))
	if len(levels) > 1 {
		fmt.Fprintf(&b, ", %s of %s", percentOf(last.kb, top.kb), top.path)
	}
	b.WriteString(". ")
	b.WriteString(stop)
	if len(last.children) > 0 {
		var sum int64
		for _, child := range last.children {
			sum += child.kb
		}
		fmt.Fprintf(&b, " Largest below it: %s", otherEntries(last.children, "", 5))
		if own := last.kb - sum; own > 0 {
			fmt.Fprintf(&b, "; files directly in it %s", formatKiB(own))
		}
		b.WriteString(".")
	}
	if unreadable > 0 {
		fmt.Fprintf(&b, "\ndu could not read %d path(s) (for example permission denied); their space is not counted.", unreadable)
	}
	return b.String()
}

func otherEntries(entries []diskUsageEntry, skip string, limit int) string {
	parts := make([]string, 0, limit)
	for _, entry := range entries {
		if entry.path == skip {
			continue
		}
		if len(parts) == limit {
			break
		}
		parts = append(parts, fmt.Sprintf("%s %s", path.Base(entry.path), formatKiB(entry.kb)))
	}
	return strings.Join(parts, ", ")
}

func percentOf(part, whole int64) string {
	if whole <= 0 {
		return "?%"
	}
	return fmt.Sprintf("%d%%", (part*100+whole/2)/whole)
}

// formatKiB prints a du -k size the way du -h would.
func formatKiB(kb int64) string {
	value := float64(kb)
	units := []string{"K", "M", "G", "T", "P"}
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if value < 10 && unit > 0 {
		return strconv.FormatFloat(value, 'f', 1, 64) + units[unit]
	}
	return strconv.FormatFloat(value, 'f', 0, 64) + units[unit]
}
