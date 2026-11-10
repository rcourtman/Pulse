package safety

import (
	"path"
	"path/filepath"
	"strings"
)

// IsSensitivePath returns (true, reason) when a file path is likely to contain secrets.
// This is used to block AI-initiated reads/writes of high-risk locations (SSH keys, shadow files, etc.).
func IsSensitivePath(p string) (bool, string) {
	if p == "" {
		return false, ""
	}

	clean := filepath.Clean(p)
	lower := strings.ToLower(clean)

	// System account databases.
	switch lower {
	case "/etc/shadow", "/etc/gshadow", "/etc/sudoers":
		return true, "system credential file"
	}

	// SSH keys and configs.
	if strings.Contains(lower, "/.ssh/") {
		return true, "ssh key/config directory"
	}
	for _, name := range []string{"id_rsa", "id_ed25519", "id_ecdsa", "id_dsa", "authorized_keys", "known_hosts"} {
		if strings.HasSuffix(lower, "/"+name) {
			return true, "ssh key material"
		}
	}

	// Secrets mounts.
	for _, prefix := range []string{"/run/secrets/", "/var/run/secrets/", "/etc/secrets/", "/secrets/"} {
		if strings.HasPrefix(lower, prefix) {
			return true, "secrets directory"
		}
	}

	// Process environment frequently contains secrets.
	if strings.HasPrefix(lower, "/proc/") && strings.HasSuffix(lower, "/environ") {
		return true, "process environment file"
	}

	// Private key / certificate formats.
	for _, ext := range []string{".pem", ".key", ".p12", ".pfx"} {
		if strings.HasSuffix(lower, ext) {
			return true, "private key or certificate file"
		}
	}

	// Pulse encrypted AI provider configuration store.
	if strings.HasSuffix(lower, "/ai.enc") || strings.Contains(lower, "ai.enc") {
		return true, "pulse encrypted AI provider config store"
	}

	// Common dotfiles that may include tokens.
	for _, base := range []string{".env", ".npmrc", ".pypirc", ".netrc", ".aws/credentials"} {
		if strings.HasSuffix(lower, "/"+base) {
			return true, "credentials dotfile"
		}
	}

	if !strings.Contains(lower, "/") {
		return false, ""
	}
	if reason := infrastructureCredentialReason(lower); reason != "" {
		return true, reason
	}
	return false, ""
}

// infrastructureCredentialReason covers the credential stores that monitored
// hosts carry for the software Pulse watches, and Pulse's own: the agent's
// token files, Pulse's encrypted stores, Proxmox VE and Backup Server private
// state, cluster keys and Kubernetes admin configs. Directory rules match
// anywhere in the path, so container root filesystems and /proc/<pid>/root
// views are covered too. lower is cleaned and lowercased.
func infrastructureCredentialReason(lower string) string {
	base := path.Base(lower)
	segments := pathSegments(lower)
	if len(segments) >= 3 && segments[0] == "proc" && wordIn(segments[2], "root cwd fd fdinfo map_files mem cmdline environ") {
		// Another process's filesystem view, open files, memory or arguments.
		return "process view"
	}
	switch {
	case strings.Contains(base, "shadow"):
		// /etc/shadow- backups, Proxmox shadow.cfg, PBS token.shadow and shadow.json.
		return "credential database"
	case base == "token" || strings.HasSuffix(base, ".token") || base == ".vault-token":
		// The Pulse agent's token files and other bearer-token files.
		return "token file"
	case strings.HasSuffix(lower, "/etc/pve/priv") || strings.Contains(lower, "/etc/pve/priv/"):
		return "proxmox private directory"
	case strings.HasSuffix(lower, "/etc/corosync/authkey"):
		return "cluster authentication key"
	case strings.HasPrefix(base, "ssh_host_") && strings.HasSuffix(base, "_key"):
		return "ssh key material"
	case base == "secrets" || strings.Contains(lower, "/secrets/"):
		// Secrets mounts anywhere, including inside container root filesystems.
		return "secrets directory"
	case strings.Contains(base, "encryption-key") || strings.Contains(base, "encryption_key"):
		return "encryption key file"
	case strings.Contains(lower, "/etc/kubernetes/") && strings.HasSuffix(base, ".conf"),
		strings.HasSuffix(lower, "/etc/rancher/k3s/k3s.yaml"),
		strings.HasSuffix(lower, "/etc/rancher/rke2/rke2.yaml"):
		return "kubernetes credentials"
	}
	for _, ext := range []string{".enc", ".keyring", ".kdbx", ".jks", ".keystore"} {
		if strings.HasSuffix(base, ext) {
			return "encrypted secret or key store"
		}
	}
	for _, suffix := range []string{"/.pgpass", "/.my.cnf", "/.git-credentials", "/.docker/config.json", "/.kube/config"} {
		if strings.HasSuffix(lower, suffix) {
			return "credentials dotfile"
		}
	}
	return ""
}

// sensitiveLocations are fixed credential locations. A glob can reach them
// without naming them, so glob words are also checked against this list
// segment by segment, at the filesystem root and, for locations of two or
// more segments, below any prefix (container root filesystems, /proc/<pid>/root).
// '*' segments match any one name.
var sensitiveLocations = []string{
	"/etc/shadow", "/etc/shadow-", "/etc/gshadow", "/etc/gshadow-", "/etc/sudoers",
	"/etc/pve/priv", "/etc/corosync/authkey",
	"/etc/proxmox-backup/authkey.key", "/etc/proxmox-backup/csrf.key",
	"/etc/proxmox-backup/shadow.json", "/etc/proxmox-backup/token.shadow",
	"/etc/proxmox-backup/tape-encryption-keys.json",
	"/etc/ssh/ssh_host_rsa_key", "/etc/ssh/ssh_host_ecdsa_key",
	"/etc/ssh/ssh_host_ed25519_key", "/etc/ssh/ssh_host_dsa_key",
	"/root/.ssh", "/home/*/.ssh",
	"/run/secrets", "/var/run/secrets", "/etc/secrets", "/secrets",
	"/var/lib/pulse-agent/token", "/var/lib/pulse-agent/runtime.token",
	"/run/pulse-agent/bootstrap.token",
	"/etc/pulse/.encryption.key", "/etc/pulse/.env", "/etc/pulse/*.enc",
	"/data/.encryption.key", "/data/.env", "/data/*.enc",
	"/etc/kubernetes/admin.conf", "/etc/kubernetes/super-admin.conf",
	"/etc/kubernetes/kubelet.conf", "/etc/kubernetes/controller-manager.conf",
	"/etc/kubernetes/scheduler.conf", "/etc/kubernetes/pki",
	"/etc/rancher/k3s/k3s.yaml", "/etc/rancher/rke2/rke2.yaml",
	"/var/lib/rancher/k3s/server/token", "/var/lib/rancher/k3s/server/tls",
	"/root/.my.cnf", "/root/.pgpass", "/root/.git-credentials", "/root/.netrc",
	"/root/.docker/config.json", "/root/.kube/config", "/root/.aws/credentials",
	"/root/.vault-token",
	"/home/*/.my.cnf", "/home/*/.pgpass", "/home/*/.git-credentials",
	"/home/*/.netrc", "/home/*/.docker/config.json", "/home/*/.kube/config",
	"/home/*/.aws/credentials",
	"/proc/*/environ", "/proc/*/cmdline", "/proc/*/root", "/proc/*/cwd",
	"/proc/*/fd", "/proc/*/fdinfo", "/proc/*/map_files", "/proc/*/mem",
}

// sensitiveNamePatterns are credential file names that IsSensitivePath
// recognises wherever they are. A glob whose last segment could expand to
// one of them is refused outside /var/log.
var sensitiveNamePatterns = []string{
	"*.pem", "*.key", "*.p12", "*.pfx", "*.enc", "*.keyring", "*.kdbx", "*.jks", "*.keystore",
	"token", "*.token", ".vault-token", "shadow", "shadow-", "gshadow", "gshadow-", "*.shadow",
	"shadow.json", "shadow.cfg", "id_rsa", "id_ed25519", "id_ecdsa", "id_dsa", "authorized_keys",
	"known_hosts", ".env", ".npmrc", ".pypirc", ".netrc", ".pgpass", ".my.cnf", ".git-credentials",
	"ssh_host_*_key", "*encryption-key*.json", "environ", "credentials",
}

// recursiveSearchRoot is the only tree a recursive content search may cover.
// A recursive search reads every file below its root, including container
// root filesystems and application directories that no path list can vet.
const recursiveSearchRoot = "/var/log"

// CommandTouchesSensitivePath is a best-effort heuristic for blocking command execution that
// obviously reads secrets. This is not a full shell parser; it intentionally catches only
// high-confidence cases.
func CommandTouchesSensitivePath(command string) (bool, string) {
	cmdLower := strings.ToLower(command)
	if cmdLower == "" {
		return false, ""
	}

	// High-confidence substrings.
	for _, s := range []string{"/etc/shadow", "/etc/gshadow", "/etc/sudoers", "/run/secrets/", "/var/run/secrets/", "/.ssh/", "ai.enc"} {
		if strings.Contains(cmdLower, s) {
			return true, "references sensitive path"
		}
	}
	if strings.Contains(cmdLower, "/proc/") && strings.Contains(cmdLower, "environ") {
		return true, "references process environment file"
	}

	for _, words := range commandStages(command) {
		if reason := sensitiveStageReason(words); reason != "" {
			return true, reason
		}
	}
	return false, ""
}

// CommandReadsProcessEnvironment reports whether a command prints process
// environments: env, printenv, systemctl show-environment, or a ps form
// (directly or through docker top) that adds environment output.
// bsdOptionSemantics selects the BSD and macOS meaning of ps options, where
// -e and -E show the environment, instead of the Linux one.
func CommandReadsProcessEnvironment(command string, bsdOptionSemantics bool) bool {
	for _, words := range commandStages(command) {
		if stageReadsProcessEnvironment(words, bsdOptionSemantics) {
			return true
		}
	}
	return false
}

func sensitiveStageReason(words []string) string {
	words = unwrapTimeout(words)
	if len(words) == 0 {
		return ""
	}
	if stageReadsProcessEnvironment(words, false) {
		return "reads process environment values"
	}
	program := strings.ToLower(path.Base(words[0]))
	args := words[1:]
	switch program {
	case "getent":
		if database := GetentDatabase(args); database == "shadow" || database == "gshadow" {
			return "reads the system credential database"
		}
	case "kubectl":
		if kubectlPrintsSecretData(args) {
			return "prints Kubernetes secret values"
		}
	case "grep", "egrep", "fgrep", "zgrep", "rgrep":
		roots, recursive, understood := grepSearchRoots(args)
		if !understood {
			return "grep options are not recognised"
		}
		if recursive || program == "rgrep" {
			if reason := recursiveSearchReason(roots); reason != "" {
				return reason
			}
		}
	}
	for _, word := range words {
		if reason := sensitiveWordReason(word); reason != "" {
			return reason
		}
	}
	return ""
}

func stageReadsProcessEnvironment(words []string, bsdOptionSemantics bool) bool {
	words = unwrapTimeout(words)
	if len(words) == 0 {
		return false
	}
	args := words[1:]
	switch strings.ToLower(path.Base(words[0])) {
	case "env", "printenv":
		return true
	case "ps":
		return psPrintsEnvironment(args, bsdOptionSemantics)
	case "systemctl":
		return firstOperand(args) == "show-environment"
	case "docker", "podman":
		// docker top CONTAINER [ps options] runs ps on the daemon's Linux host.
		if len(args) >= 2 && args[0] == "top" {
			return psPrintsEnvironment(args[2:], false)
		}
	}
	return false
}

// psPrintsEnvironment detects ps invocations that may print process
// environments. With BSD semantics (macOS, FreeBSD) that is any e or E option.
// On Linux, procps prints environments for the BSD-style e option and re-reads
// a whole invocation as BSD syntax when any part fails SysV parsing, which
// turns any e in any argument (a -e flag, a format or a command name) into
// that option. Modelling procps exactly is not practical, so an invocation
// that mentions e is accepted only in canonical SysV shapes.
func psPrintsEnvironment(args []string, bsdOptionSemantics bool) bool {
	if bsdOptionSemantics {
		for i := 0; i < len(args); i++ {
			arg := args[i]
			switch {
			case strings.HasPrefix(arg, "--"):
				if !strings.Contains(arg, "=") && wordIn(arg, "--format --sort --pid --ppid --user --group --tty --sid --cols --columns --rows --width") {
					i++
				}
			case strings.HasPrefix(arg, "-"):
				cluster := arg[1:]
				if strings.ContainsAny(cluster, "eE") {
					return true
				}
				if valueAt := strings.IndexAny(cluster, "oOptUGMNJ"); valueAt >= 0 && valueAt == len(cluster)-1 {
					i++
				}
			default:
				if strings.IndexByte(arg, 'e') >= 0 {
					return true
				}
				if arg != "" && strings.IndexByte("oOptUkq", arg[len(arg)-1]) >= 0 {
					i++
				}
			}
		}
		return false
	}
	if psIsCanonicalSysV(args) {
		return false
	}
	for _, arg := range args {
		if strings.IndexByte(arg, 'e') >= 0 {
			return true
		}
	}
	return false
}

// psCanonicalFlagClusters are SysV option clusters ps diagnostics use that
// procps parses without falling back to BSD syntax.
var psCanonicalFlagClusters = map[string]bool{
	"-e": true, "-A": true, "-a": true, "-d": true, "-ef": true, "-eF": true, "-elf": true,
	"-ely": true, "-eLf": true, "-ejH": true, "-eH": true, "-ew": true, "-eww": true,
	"-efw": true, "-efww": true, "-Af": true, "-AF": true, "-Alf": true, "-ALf": true,
	"-AjH": true, "-f": true, "-F": true, "-l": true, "-L": true, "-H": true, "-w": true,
	"-ww": true,
}

// psIsCanonicalSysV accepts flag clusters from psCanonicalFlagClusters,
// -o FORMAT (also -eo and -Ao), -p PIDS, -C NAMES, --sort, --forest and header
// switches, and one output format. Format and sort values must name fields
// from psFormatFields, which leaves out environment fields.
func psIsCanonicalSysV(args []string) bool {
	formats := 0
	formatFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := ""
		takeValue := func() bool {
			if i+1 >= len(args) {
				return false
			}
			i++
			value = args[i]
			return true
		}
		switch {
		case psCanonicalFlagClusters[arg]:
			formatFlags = formatFlags || strings.ContainsAny(arg, "fFlj")
			continue
		case wordIn(arg, "--forest --headers --no-headers"):
			continue
		case wordIn(arg, "-o -eo -Ao"):
			formats++
			if !takeValue() || !psFieldList(value, false) {
				return false
			}
		case arg == "-p":
			if !takeValue() || strings.Trim(value, "0123456789,") != "" || value == "" {
				return false
			}
		case arg == "-C":
			if !takeValue() || !psNameListValue(value) {
				return false
			}
		case arg == "--sort" || strings.HasPrefix(arg, "--sort="):
			if arg == "--sort" {
				if !takeValue() {
					return false
				}
			} else {
				value = strings.TrimPrefix(arg, "--sort=")
			}
			if !psFieldList(value, true) {
				return false
			}
		default:
			return false
		}
	}
	return formats <= 1 && !(formats == 1 && formatFlags)
}

// psFormatFields are procps output fields used for diagnostics. Environment
// fields are deliberately absent.
var psFormatFields = wordSet(`pid ppid pgid pgrp sid sess tid spid lwp nlwp tgid
	user euser ruser suser uid euid ruid suid uname group egroup rgroup gid egid rgid
	comm ucomm ucmd fname cmd args command
	%cpu pcpu c cp %mem pmem rss rssize rsz vsz vsize sz size drs trs
	stat state s start lstart start_time bsdstart stime etime etimes time cputime cputimes bsdtime
	tty tt tname ni nice pri priority opri psr wchan nwchan cls class policy sched rtprio
	f flags cgroup label maj_flt min_flt majflt minflt`)

func wordSet(words string) map[string]bool {
	set := make(map[string]bool)
	for _, word := range strings.Fields(words) {
		set[word] = true
	}
	return set
}

// psFieldList checks a comma-separated -o or --sort value. Output fields may
// carry a =header suffix; sort fields may carry a + or - prefix.
func psFieldList(value string, sort bool) bool {
	if value == "" {
		return false
	}
	for _, item := range strings.Split(value, ",") {
		if sort {
			item = strings.TrimLeft(item, "+-")
		} else if key, header, found := strings.Cut(item, "="); found {
			if strings.ContainsAny(header, " \t") {
				return false
			}
			item = key
		}
		if !psFormatFields[strings.ToLower(item)] {
			return false
		}
	}
	return true
}

func psNameListValue(value string) bool {
	if value == "" || strings.HasPrefix(value, "-") {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("_,.:+-", c)) {
			return false
		}
	}
	return true
}

// GetentDatabase returns the database a getent invocation reads, skipping
// options and the value of -s/--service.
func GetentDatabase(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-s" || arg == "--service" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return strings.ToLower(arg)
	}
	return ""
}

// kubectlPrintsSecretData reports a get of Secrets with an output format that
// carries their data. The default table, wide and name forms do not.
func kubectlPrintsSecretData(args []string) bool {
	if len(args) == 0 || args[0] != "get" {
		return false
	}
	namesSecret := false
	output := ""
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-o" || arg == "--output":
			if i+1 < len(args) {
				i++
				output = args[i]
			}
		case strings.HasPrefix(arg, "--output="):
			output = strings.TrimPrefix(arg, "--output=")
		case strings.HasPrefix(arg, "-o") && !strings.HasPrefix(arg, "--"):
			output = strings.TrimPrefix(strings.TrimPrefix(arg, "-o"), "=")
		case strings.HasPrefix(arg, "-"):
			continue
		default:
			for _, ref := range strings.Split(arg, ",") {
				kind, _, _ := strings.Cut(strings.ToLower(ref), "/")
				kind, _, _ = strings.Cut(kind, ".")
				if kind == "secret" || kind == "secrets" {
					namesSecret = true
				}
			}
		}
	}
	output = strings.ToLower(output)
	return namesSecret && output != "" && output != "wide" && output != "name"
}

// grepLongOptions is GNU grep's long-option vocabulary with whether each
// requires a separate value. GNU getopt takes an exact name first, then a
// unique prefix; anything else is an error, which grepSearchRoots reports as
// not understood.
var grepLongOptions = map[string]bool{
	"extended-regexp": false, "fixed-strings": false, "basic-regexp": false, "perl-regexp": false,
	"ignore-case": false, "no-ignore-case": false, "word-regexp": false, "line-regexp": false,
	"null-data": false, "no-messages": false, "invert-match": false, "version": false, "help": false,
	"byte-offset": false, "line-number": false, "no-line-number": false, "line-buffered": false,
	"with-filename": false, "no-filename": false, "only-matching": false, "quiet": false,
	"silent": false, "binary": false, "text": false, "recursive": false,
	"dereference-recursive": false, "files-without-match": false, "files-with-matches": false,
	"count": false, "initial-tab": false, "null": false, "unix-byte-offsets": false,
	"no-group-separator": false, "color": false, "colour": false,
	"regexp": true, "file": true, "max-count": true, "label": true, "binary-files": true,
	"directories": true, "devices": true, "include": true, "exclude": true, "exclude-from": true,
	"exclude-dir": true, "after-context": true, "before-context": true, "context": true,
	"group-separator": true,
}

func resolveGrepLongOption(name string) (string, bool) {
	if _, exact := grepLongOptions[name]; exact {
		return name, true
	}
	match := ""
	for option := range grepLongOptions {
		if name != "" && strings.HasPrefix(option, name) {
			if match != "" {
				return "", false
			}
			match = option
		}
	}
	return match, match != ""
}

// grepSearchRoots returns the file operands of a grep invocation, whether it
// searches recursively, and false when an option cannot be read exactly.
func grepSearchRoots(args []string) ([]string, bool, bool) {
	recursive := false
	patternGiven := false
	var operands []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			name, value, attached := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
			option, ok := resolveGrepLongOption(name)
			if !ok {
				return nil, false, false
			}
			if grepLongOptions[option] && !attached {
				if i+1 >= len(args) {
					return nil, false, false
				}
				i++
				value = args[i]
			}
			switch option {
			case "recursive", "dereference-recursive":
				recursive = true
			case "regexp", "file":
				patternGiven = true
			case "directories":
				recursive = recursive || value == "recurse"
			}
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			cluster := arg[1:]
			for j := 0; j < len(cluster); j++ {
				option := cluster[j]
				if option == 'r' || option == 'R' {
					recursive = true
				}
				if strings.IndexByte("efmABCdD", option) >= 0 {
					value := cluster[j+1:]
					if value == "" {
						if i+1 >= len(args) {
							return nil, false, false
						}
						i++
						value = args[i]
					}
					if option == 'e' || option == 'f' {
						patternGiven = true
					}
					if option == 'd' && value == "recurse" {
						recursive = true
					}
					break
				}
			}
			continue
		}
		operands = append(operands, arg)
	}
	if !patternGiven && len(operands) > 0 {
		operands = operands[1:]
	}
	return operands, recursive, true
}

// recursiveSearchReason refuses a recursive content search unless every root
// is an explicit path under /var/log.
func recursiveSearchReason(roots []string) string {
	if len(roots) == 0 {
		return "recursive search needs an explicit path under " + recursiveSearchRoot
	}
	for _, root := range roots {
		clean := path.Clean(root)
		if clean != recursiveSearchRoot && !strings.HasPrefix(clean, recursiveSearchRoot+"/") {
			return "recursive search is limited to " + recursiveSearchRoot
		}
	}
	return ""
}

// sensitiveWordReason checks one shell word, plus the value of an --opt=value
// or @file word, as a path.
func sensitiveWordReason(word string) string {
	candidates := []string{word}
	if strings.HasPrefix(word, "-") {
		if _, value, ok := strings.Cut(word, "="); ok {
			candidates = append(candidates, value)
		}
		if len(word) > 2 && word[1] != '-' && (word[2] == '/' || word[2] == '~') {
			candidates = append(candidates, word[2:])
		}
	}
	if strings.HasPrefix(word, "@") {
		candidates = append(candidates, word[1:])
	}
	for _, candidate := range candidates {
		if hasParentSegment(candidate) {
			return "parent-directory path segments are not accepted"
		}
		for _, absolute := range absolutePathCandidates(candidate) {
			if sensitive, reason := IsSensitivePath(absolute); sensitive {
				return reason
			}
			if pathCanReachSensitiveLocation(absolute) {
				return "path is or can match a credential location"
			}
			if !strings.ContainsAny(absolute, "*?[") {
				continue
			}
			if globCanNameParent(absolute) {
				return "parent-directory path segments are not accepted"
			}
			if globDirectoryCanBeSecrets(absolute) {
				return "path pattern can match a secrets directory"
			}
			if globNameCanMatchCredentialFile(absolute) {
				return "path pattern can match a credential file"
			}
		}
	}
	return ""
}

// absolutePathCandidates resolves a word that looks like a path to the
// absolute paths it can name. Agents run commands from /, so a relative path
// names a file there; ~ is root's home or any user's.
func absolutePathCandidates(word string) []string {
	switch {
	case strings.HasPrefix(word, "/"):
		return []string{word}
	case strings.HasPrefix(word, "~"):
		rest := ""
		if slash := strings.IndexByte(word, '/'); slash >= 0 {
			rest = word[slash:]
		}
		// "~user" is a placeholder for any user's home directory: the
		// location list matches it through its "*" segments, and it is not
		// a shell pattern the glob rules would read.
		return []string{"/root" + rest, "/home/~user" + rest}
	case strings.Contains(word, "/"):
		return []string{"/" + word}
	}
	return nil
}

// pathCanReachSensitiveLocation reports whether a path, literal or glob, is
// or can expand to a fixed credential location or something inside one, at
// the root or, for locations of two or more segments, below any prefix.
func pathCanReachSensitiveLocation(p string) bool {
	segments := shellGlobSegments(p)
	for _, location := range sensitiveLocations {
		locationSegments := pathSegments(location)
		for offset := 0; offset < len(segments); offset++ {
			if offset > 0 && len(locationSegments) < 2 && strings.Trim(segments[offset], "*?") == "" {
				// Below a prefix, a one-segment location needs a literal
				// character in the pattern, or every * would match it.
				continue
			}
			tail := segments[offset:]
			if len(tail) >= len(locationSegments) && segmentsOverlap(tail, locationSegments) {
				return true
			}
		}
	}
	return false
}

// globCanNameParent reports a glob segment that could expand to "." or "..".
// POSIX shells only match a leading dot explicitly, so only segments that
// start with a dot or a bracket expression can.
func globCanNameParent(p string) bool {
	for _, segment := range strings.Split(p, "/") {
		if !strings.ContainsAny(segment, "*?[") || !(strings.HasPrefix(segment, ".") || strings.HasPrefix(segment, "[")) {
			continue
		}
		pattern := shellGlobPattern(segment)
		if globsIntersect(pattern, "..") || globsIntersect(pattern, ".") {
			return true
		}
	}
	return false
}

// globDirectoryCanBeSecrets reports a glob directory segment, outside
// /var/log, that could expand to a secrets directory such as one inside a
// container root filesystem.
func globDirectoryCanBeSecrets(p string) bool {
	clean := strings.ToLower(p)
	if clean == recursiveSearchRoot || strings.HasPrefix(clean, recursiveSearchRoot+"/") {
		return false
	}
	segments := pathSegments(clean)
	for i := 0; i+1 < len(segments); i++ {
		if strings.ContainsAny(segments[i], "*?[") && globsIntersect(shellGlobPattern(segments[i]), "secrets") {
			return true
		}
	}
	return false
}

// globNameCanMatchCredentialFile reports a glob whose last segment could
// expand to a credential file name. /var/log holds logs, which the read path
// exists to show, so globs there are left to the location rules.
func globNameCanMatchCredentialFile(p string) bool {
	clean := strings.ToLower(p)
	if clean == recursiveSearchRoot || strings.HasPrefix(clean, recursiveSearchRoot+"/") {
		return false
	}
	name := path.Base(clean)
	if !strings.ContainsAny(name, "*?[") {
		return false
	}
	pattern := shellGlobPattern(name)
	for _, credential := range sensitiveNamePatterns {
		if globsIntersect(pattern, credential) {
			return true
		}
	}
	return false
}

func shellGlobSegments(p string) []string {
	segments := pathSegments(strings.ToLower(p))
	for i, segment := range segments {
		segments[i] = shellGlobPattern(segment)
	}
	return segments
}

// shellGlobPattern turns a shell glob segment into the simple pattern
// language globsIntersect reads (* and ? are wildcards, everything else is
// literal) so that it matches at least everything the shell pattern can.
// Bracket expressions differ between shells (negation with ! or ^, POSIX
// classes), so each becomes ?; an unterminated bracket matches the rest.
func shellGlobPattern(segment string) string {
	var out strings.Builder
	for i := 0; i < len(segment); i++ {
		c := segment[i]
		if c != '[' {
			out.WriteByte(c)
			continue
		}
		end := bracketExpressionEnd(segment, i)
		if end < 0 {
			out.WriteByte('*')
			return out.String()
		}
		out.WriteByte('?')
		i = end
	}
	return out.String()
}

// bracketExpressionEnd returns the index of the ] that closes the bracket
// expression opened at start, or -1.
func bracketExpressionEnd(segment string, start int) int {
	i := start + 1
	if i < len(segment) && (segment[i] == '!' || segment[i] == '^') {
		i++
	}
	if i < len(segment) && segment[i] == ']' {
		i++
	}
	for ; i < len(segment); i++ {
		if segment[i] == '[' && i+1 < len(segment) && strings.IndexByte(":.=", segment[i+1]) >= 0 {
			closer := string(segment[i+1]) + "]"
			if end := strings.Index(segment[i+2:], closer); end >= 0 {
				i += 2 + end + 1
				continue
			}
		}
		if segment[i] == ']' {
			return i
		}
	}
	return -1
}

func hasParentSegment(word string) bool {
	for _, segment := range strings.Split(word, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func pathSegments(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	var segments []string
	for _, segment := range strings.Split(p, "/") {
		if segment != "" && segment != "." {
			segments = append(segments, segment)
		}
	}
	return segments
}

// segmentsOverlap reports whether two paths, either of which may hold glob
// segments, agree on every segment they both have.
func segmentsOverlap(a, b []string) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if !segmentsMatch(a[i], b[i]) {
			return false
		}
	}
	return true
}

func segmentsMatch(x, y string) bool {
	return globsIntersect(x, y)
}

// globsIntersect reports whether two patterns, in which * matches any run of
// characters and ? any one character, can match a common string.
func globsIntersect(a, b string) bool {
	memo := make(map[[2]int]bool)
	seen := make(map[[2]int]bool)
	var intersect func(i, j int) bool
	intersect = func(i, j int) bool {
		key := [2]int{i, j}
		if seen[key] {
			return memo[key]
		}
		seen[key] = true
		result := false
		switch {
		case i == len(a) && j == len(b):
			result = true
		case i < len(a) && a[i] == '*':
			result = intersect(i+1, j) || (j < len(b) && intersect(i, j+1))
		case j < len(b) && b[j] == '*':
			result = intersect(i, j+1) || (i < len(a) && intersect(i+1, j))
		case i < len(a) && j < len(b):
			result = (a[i] == '?' || b[j] == '?' || a[i] == b[j]) && intersect(i+1, j+1)
		}
		memo[key] = result
		return result
	}
	return intersect(0, 0)
}

func unwrapTimeout(words []string) []string {
	if len(words) == 0 || strings.ToLower(path.Base(words[0])) != "timeout" {
		return words
	}
	i := 1
	for i < len(words) && strings.HasPrefix(words[i], "-") {
		if words[i] == "--" {
			i++
			break
		}
		if wordIn(words[i], "-s --signal -k --kill-after") {
			i++
		}
		i++
	}
	if i < len(words) {
		i++ // duration
	}
	if i >= len(words) {
		return nil
	}
	return words[i:]
}

func firstOperand(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

func wordIn(word, set string) bool {
	for _, candidate := range strings.Fields(set) {
		if word == candidate {
			return true
		}
	}
	return false
}

// commandStages splits a command into the words of each pipeline or list
// stage. Quotes and backslash escapes are honoured; expansions are not
// interpreted, so callers must also refuse commands that use them.
func commandStages(command string) [][]string {
	var stages [][]string
	var words []string
	var word strings.Builder
	inWord := false
	quote := byte(0)
	endWord := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}
	endStage := func() {
		endWord()
		if len(words) > 0 {
			stages = append(stages, words)
			words = nil
		}
	}
	for i := 0; i < len(command); i++ {
		c := command[i]
		if quote == '\'' {
			if c == '\'' {
				quote = 0
			} else {
				word.WriteByte(c)
			}
			continue
		}
		if c == '\\' && i+1 < len(command) {
			i++
			word.WriteByte(command[i])
			inWord = true
			continue
		}
		if quote == '"' {
			if c == '"' {
				quote = 0
			} else {
				word.WriteByte(c)
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			inWord = true
		case ' ', '\t', '<', '>':
			endWord()
		case '\n', '\r', '|', ';', '&', '(', ')':
			endStage()
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	endStage()
	return stages
}
