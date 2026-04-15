package hostagent

import "strings"

func resolveHostOSIdentity(collector SystemCollector, osName, osVersion string) (string, string) {
	currentName := strings.TrimSpace(osName)
	currentVersion := strings.TrimSpace(osVersion)

	if collector == nil || collector.GOOS() != "linux" {
		return currentName, currentVersion
	}

	if name, version, ok := detectSynologyOSIdentity(collector); ok {
		if version == "" {
			version = currentVersion
		}
		return name, strings.TrimSpace(version)
	}

	if name, version, ok := detectQNAPOSIdentity(collector); ok {
		if version == "" {
			version = currentVersion
		}
		return name, strings.TrimSpace(version)
	}

	return currentName, currentVersion
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
