package hostagent

import (
	"context"
	"regexp"
	"strings"
	"time"
)

var unraidVersionPattern = regexp.MustCompile(`\b\d+(?:\.\d+)+(?:[-+._][A-Za-z0-9]+)*\b|\b\d+\b`)
var proxmoxPVEVersionPattern = regexp.MustCompile(`(?i)\bpve-manager/([^/\s]+)`)

const proxmoxPVEOSName = "Proxmox VE"
const proxmoxPVEVersionCommandTimeout = 10 * time.Second

func resolveHostOSIdentity(collector SystemCollector, osName, osVersion string) (string, string) {
	currentName := strings.TrimSpace(osName)
	currentVersion := strings.TrimSpace(osVersion)

	if collector == nil || collector.GOOS() != "linux" {
		return currentName, currentVersion
	}

	if name, version, ok := detectSynologyOSIdentity(collector); ok {
		return resolvedDetectedHostOSIdentity(name, version, currentVersion, true)
	}

	if name, version, ok := detectQNAPOSIdentity(collector); ok {
		return resolvedDetectedHostOSIdentity(name, version, currentVersion, true)
	}

	if name, version, ok := detectUnraidOSIdentity(collector); ok {
		return resolvedDetectedHostOSIdentity(name, version, currentVersion, true)
	}

	if name, version, ok := detectProxmoxVEOSIdentity(collector); ok {
		return resolvedDetectedHostOSIdentity(name, version, currentVersion, false)
	}

	return currentName, currentVersion
}

func resolvedDetectedHostOSIdentity(name, version, currentVersion string, allowVersionFallback bool) (string, string) {
	version = strings.TrimSpace(version)
	if version == "" && allowVersionFallback {
		version = currentVersion
	}
	return strings.TrimSpace(name), strings.TrimSpace(version)
}

func detectSynologyOSIdentity(collector SystemCollector) (string, string, bool) {
	hasSynologyDir := false
	if _, err := collector.Stat("/usr/syno"); err == nil {
		hasSynologyDir = true
	}

	for _, path := range []string{"/etc.defaults/VERSION", "/etc/VERSION"} {
		data, err := collector.ReadFile(path)
		if err != nil || len(data) == 0 {
			continue
		}

		values := parseAssignmentConfig(string(data))
		if len(values) == 0 {
			continue
		}
		if !hasSynologyDir && !looksLikeSynologyVersionFile(values) {
			continue
		}

		version := composeSynologyVersion(values)
		return "Synology DSM", version, true
	}

	return "", "", false
}

func composeSynologyVersion(values map[string]string) string {
	version := strings.TrimSpace(values["productversion"])
	if version == "" {
		major := strings.TrimSpace(values["majorversion"])
		minor := strings.TrimSpace(values["minorversion"])
		switch {
		case major != "" && minor != "":
			version = major + "." + minor
		case major != "":
			version = major
		case minor != "":
			version = minor
		}
	}

	build := strings.TrimSpace(values["buildnumber"])
	if build != "" && !strings.Contains(version, build) {
		if version != "" {
			version += "-" + build
		} else {
			version = build
		}
	}

	smallfix := strings.TrimSpace(values["smallfixnumber"])
	if smallfix != "" && smallfix != "0" {
		if version != "" {
			version += " Update " + smallfix
		} else {
			version = smallfix
		}
	}

	return strings.TrimSpace(version)
}

func looksLikeSynologyVersionFile(values map[string]string) bool {
	if values["majorversion"] != "" || values["minorversion"] != "" || values["buildnumber"] != "" || values["productversion"] != "" {
		return true
	}

	hints := strings.ToLower(strings.Join([]string{
		values["product"],
		values["unique"],
		values["buildphase"],
	}, " "))
	return strings.Contains(hints, "synology") || strings.Contains(hints, "dsm") || strings.Contains(hints, "diskstation")
}

func detectQNAPOSIdentity(collector SystemCollector) (string, string, bool) {
	for _, path := range []string{"/etc/config/uLinux.conf", "/etc/default_config/uLinux.conf"} {
		data, err := collector.ReadFile(path)
		if err != nil || len(data) == 0 {
			continue
		}

		values := parseAssignmentConfig(string(data))
		if len(values) == 0 {
			continue
		}

		name := "QNAP QTS"
		hintFields := strings.ToLower(strings.Join([]string{
			values["display_name"],
			values["platform"],
			values["system_name"],
			values["version"],
		}, " "))
		if strings.Contains(hintFields, "quts") {
			name = "QNAP QuTS"
		}
		return name, strings.TrimSpace(values["version"]), true
	}

	if _, err := collector.Stat("/etc/config/qpkg.conf"); err == nil {
		return "QNAP QTS", "", true
	}
	if _, err := collector.Stat("/sbin/getcfg"); err == nil {
		return "QNAP QTS", "", true
	}

	return "", "", false
}

func detectUnraidOSIdentity(collector SystemCollector) (string, string, bool) {
	data, err := collector.ReadFile(hostAgentUnraidVersionPath)
	if err != nil {
		if _, statErr := collector.Stat(hostAgentUnraidVersionPath); statErr == nil {
			return "Unraid", "", true
		}
		return detectUnraidOSReleaseIdentity(collector)
	}

	version := cleanUnraidVersion(string(data))
	return "Unraid", version, true
}

func detectProxmoxVEOSIdentity(collector SystemCollector) (string, string, bool) {
	hasPVE := false
	if _, err := collector.Stat("/etc/pve"); err == nil {
		hasPVE = true
	}

	pveVersionPath, err := collector.LookPath("pveversion")
	if err == nil {
		hasPVE = true
	}

	if !hasPVE {
		if _, err := collector.LookPath("pvesh"); err != nil {
			return "", "", false
		}
	}

	version := ""
	if packageVersion := detectProxmoxVEPackageVersion(collector); packageVersion != "" {
		version = packageVersion
	} else if pveVersionPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), proxmoxPVEVersionCommandTimeout)
		if output, err := collector.CommandCombinedOutput(ctx, pveVersionPath); err == nil {
			version = cleanProxmoxPVEVersion(output)
		}
		cancel()
	}

	return proxmoxPVEOSName, version, true
}

func detectProxmoxVEPackageVersion(collector SystemCollector) string {
	dpkgQueryPath, err := collector.LookPath("dpkg-query")
	if err != nil || dpkgQueryPath == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := collector.CommandCombinedOutput(ctx, dpkgQueryPath, "-W", "-f=${Version}", "pve-manager")
	if err != nil {
		return ""
	}
	return cleanProxmoxPVEPackageVersion(output)
}

func cleanProxmoxPVEPackageVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.ContainsAny(raw, "\r\n\t ") {
		return ""
	}
	return raw
}

func cleanProxmoxPVEVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if match := proxmoxPVEVersionPattern.FindStringSubmatch(raw); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func detectUnraidOSReleaseIdentity(collector SystemCollector) (string, string, bool) {
	data, err := collector.ReadFile("/etc/os-release")
	if err != nil || len(data) == 0 {
		return "", "", false
	}

	values := parseAssignmentConfig(string(data))
	hints := strings.ToLower(strings.Join([]string{
		values["id"],
		values["name"],
		values["pretty_name"],
	}, " "))
	if !strings.Contains(hints, "unraid") {
		return "", "", false
	}

	version := strings.TrimSpace(values["version_id"])
	if version == "" {
		version = cleanUnraidVersion(values["version"])
	}
	return "Unraid", version, true
}

func cleanUnraidVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if match := unraidVersionPattern.FindString(line); match != "" {
			return match
		}
	}

	return ""
}

func parseAssignmentConfig(content string) map[string]string {
	parsed := make(map[string]string)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "[") {
			continue
		}

		idx := strings.IndexRune(line, '=')
		if idx <= 0 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		value := strings.TrimSpace(line[idx+1:])
		value = strings.Trim(value, `"'`)
		if key == "" || value == "" {
			continue
		}

		parsed[key] = value
	}

	return parsed
}
