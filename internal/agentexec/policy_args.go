package agentexec

import "strings"

// Auto-approve patterns match a command's start, but several approved
// programs also take arguments that change the system, write files, or read
// another file and echo its lines in error messages. autoApprovedArgsSafe
// checks the arguments of those programs after a pattern has matched, so only
// their display forms run without approval.

// deniedOptions lists the options of a getopt-style program that must not run
// without approval.
type deniedOptions struct {
	// short letters refuse any single-dash word that holds one of them, so a
	// cluster such as -xD/path or an attached value cannot hide the letter.
	short string
	// long names (without the leading --) are refused, together with every
	// abbreviation getopt_long would expand to them.
	long []string
	// exact names are safe options that are themselves an abbreviation of a
	// refused name (du --exclude, journalctl --cursor); getopt_long prefers
	// the exact match.
	exact []string
}

var (
	duDeniedOptions = deniedOptions{
		short: "X",
		long:  []string{"files0-from", "exclude-from"},
		exact: []string{"exclude"},
	}
	// file -d (--debug) prints the bytes it tests.
	fileDeniedOptions = deniedOptions{
		short: "fmCd",
		long:  []string{"files-from", "magic-file", "compile", "debug"},
	}
	lastDeniedOptions = deniedOptions{
		short: "f",
		long:  []string{"file"},
	}
	blkidDeniedOptions = deniedOptions{
		short: "cg",
		long:  []string{"cache-file", "garbage-collect"},
	}
	ssDeniedOptions = deniedOptions{
		short: "KDF",
		long:  []string{"kill", "diag", "filter"},
	}
	journalctlDeniedOptions = deniedOptions{
		short: "D",
		long: []string{
			"vacuum-size", "vacuum-files", "vacuum-time", "rotate", "flush", "sync",
			"synchronize-on-exit", "relinquish-var", "smart-relinquish-var",
			"setup-keys", "update-catalog", "cursor-file",
			"file", "directory", "root", "image",
		},
		exact: []string{"cursor"},
	}
	systemctlDeniedOptions = deniedOptions{
		short: "H",
		long:  []string{"host"},
	}
	aptCacheDeniedOptions = deniedOptions{
		short: "ocps",
		long:  []string{"option", "config-file", "pkg-cache", "src-cache", "with-source"},
	}
)

// autoApprovedArgsSafe reports whether a command an auto-approve pattern
// matched keeps to forms that neither change the system nor read or write
// files the pattern did not name. name is the lower-case program from the
// policy's view of the command; command is the same command in its original
// case without a plain sudo prefix, since many options differ only by case.
func autoApprovedArgsSafe(name, command string) bool {
	switch name {
	case "find", "du", "file", "last", "blkid", "ss", "lsof", "journalctl",
		"systemctl", "apt-cache", "sar", "ps", "ip", "ifconfig", "dpkg", "pvesh", "podman":
	default:
		return true
	}
	words, ok := splitPolicyWords(command)
	if !ok || len(words) == 0 || strings.ToLower(words[0].text) != name {
		return false
	}
	for _, word := range words[1:] {
		if !word.glob {
			continue
		}
		// These programs read meaning from how many words there are or
		// what an operand says, so a glob in the working directory could
		// add a word such as down (ifconfig d*) or e (ps a*).
		if globSensitivePrograms[name] || !globKeepsOption(word.text) {
			return false
		}
	}
	args := make([]string, 0, len(words)-1)
	for _, word := range words[1:] {
		args = append(args, word.text)
	}

	switch name {
	case "find":
		return findArgsSafe(args)
	case "du":
		return duDeniedOptions.allows(args)
	case "file":
		return fileDeniedOptions.allows(args)
	case "last":
		return lastDeniedOptions.allows(args)
	case "blkid":
		return blkidDeniedOptions.allows(withoutNullBlkidCache(args))
	case "ss":
		return ssDeniedOptions.allows(args)
	case "lsof":
		return lsofArgsSafe(args)
	case "journalctl":
		return journalctlDeniedOptions.allows(args)
	case "systemctl":
		return systemctlDeniedOptions.allows(args)
	case "apt-cache":
		// gencaches only rebuilds the caches.
		return !containsString(args, "gencaches") && aptCacheDeniedOptions.allows(args)
	case "sar":
		return sarArgsSafe(args)
	case "ps":
		return psArgsSafe(args)
	case "ip":
		return ipArgsSafe(args)
	case "ifconfig":
		return ifconfigArgsSafe(args)
	case "dpkg":
		return dpkgArgsSafe(args)
	case "pvesh":
		return pveshArgsSafe(args)
	case "podman":
		return podmanArgsSafe(args)
	}
	return false
}

var globSensitivePrograms = map[string]bool{
	"ip": true, "ifconfig": true, "ps": true, "sar": true, "pvesh": true,
	"apt-cache": true,
}

// globKeepsOption reports whether every expansion of an unquoted glob is
// read as the same kind of word: a path or other operand, or one long option
// whose name ends before the wildcard (--exclude=*.iso). A glob that starts
// with a wildcard or a short option can expand to a file named like any
// option.
func globKeepsOption(text string) bool {
	literal := text[:strings.IndexAny(text, "*?[")]
	if literal == "" {
		return false
	}
	if literal[0] != '-' {
		return true
	}
	return strings.HasPrefix(literal, "--") && strings.Contains(literal, "=")
}

// splitPolicyWords splits a command into shell words after quote removal.
// A word may join unquoted text, backslash escapes, single-quoted text and
// double-quoted text without expansions, as in --since="1 hour ago". Anything
// the shell would expand into other words or text the policy cannot see
// ($, backticks, braces, a ~ other than the home directory, comments) or
// parse as syntax is refused rather than interpreted.
func splitPolicyWords(command string) ([]logReadWord, bool) {
	var words []logReadWord
	i := 0
	for i < len(command) {
		if command[i] == ' ' || command[i] == '\t' {
			i++
			continue
		}
		if command[i] == '~' && !strings.HasPrefix(command[i:], "~/") &&
			i+1 < len(command) && command[i+1] != ' ' && command[i+1] != '\t' {
			// ~- and ~+ expand to $OLDPWD and $PWD.
			return nil, false
		}
		var text strings.Builder
		glob := false
		for i < len(command) && command[i] != ' ' && command[i] != '\t' {
			c := command[i]
			switch {
			case c == '\'' || c == '"':
				end := strings.IndexByte(command[i+1:], c)
				if end < 0 {
					return nil, false
				}
				quoted := command[i+1 : i+1+end]
				if c == '"' && strings.ContainsAny(quoted, "$`\\") {
					return nil, false
				}
				if strings.ContainsAny(quoted, "\x00\n\r") {
					return nil, false
				}
				text.WriteString(quoted)
				i += end + 2
			case c == '\\':
				if i+1 >= len(command) || command[i+1] == '\n' || command[i+1] == '\r' || command[i+1] == 0 {
					return nil, false
				}
				text.WriteByte(command[i+1])
				i += 2
			case strings.IndexByte(policyBareWordChars, c) >= 0:
				if c == '*' || c == '?' || c == '[' {
					glob = true
				}
				text.WriteByte(c)
				i++
			default:
				return nil, false
			}
		}
		words = append(words, logReadWord{text: text.String(), glob: glob})
	}
	return words, true
}

// policyBareWordChars are the characters an unquoted word may hold. ! and ^
// are literal in a non-interactive sh; [ and ] form globs, which the caller
// checks. ~ is literal inside a word.
const policyBareWordChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789/._+=:,@%*?[]!^~-"

// allows checks every word, wherever it sits: getopt moves options found
// after operands, and a word refused because it was really another option's
// value only costs an approval.
func (d deniedOptions) allows(args []string) bool {
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--"):
			name, _, _ := strings.Cut(strings.ToLower(arg[2:]), "=")
			if name == "" || containsString(d.exact, name) {
				continue
			}
			for _, denied := range d.long {
				if strings.HasPrefix(denied, name) {
					return false
				}
			}
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			if strings.ContainsAny(arg[1:], d.short) {
				return false
			}
		}
	}
	return true
}

// withoutNullBlkidCache drops a cache file of /dev/null, the usual way to
// probe without the cache, so only other cache files are refused.
func withoutNullBlkidCache(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case (args[i] == "-c" || args[i] == "--cache-file") && i+1 < len(args) && args[i+1] == "/dev/null":
			i++
		case args[i] == "-c/dev/null" || args[i] == "--cache-file=/dev/null":
		default:
			out = append(out, args[i])
		}
	}
	return out
}

// findDeniedPredicates run commands, delete files, write files or read file
// names from another file. find matches predicate names exactly.
var findDeniedPredicates = []string{
	"-exec", "-execdir", "-ok", "-okdir", "-delete",
	"-fprint", "-fprint0", "-fprintf", "-fls", "-files0-from",
}

func findArgsSafe(args []string) bool {
	for _, arg := range args {
		if containsString(findDeniedPredicates, strings.ToLower(arg)) {
			return false
		}
	}
	return true
}

// lsofArgsSafe refuses -D (device cache build and update modes), -k (kernel
// name list file) and -m/+m (kernel memory or mount supplement file). +D is
// the read-only directory search and stays.
func lsofArgsSafe(args []string) bool {
	for _, arg := range args {
		if len(arg) < 2 {
			continue
		}
		if arg[0] == '-' && strings.ContainsAny(arg[1:], "Dkm") {
			return false
		}
		if arg[0] == '+' && strings.ContainsRune(arg[1:], 'm') {
			return false
		}
	}
	return true
}

// sarDataDirs hold the sysstat daily files sar -f may read.
var sarDataDirs = []string{"/var/log/sa/", "/var/log/sysstat/"}

// sarArgsSafe refuses -o, which writes collected data to a file, and allows
// -f only for a sysstat daily file.
func sarArgsSafe(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") || !strings.HasPrefix(arg, "-") {
			continue
		}
		if arg == "-f" {
			i++
			if i >= len(args) || !isSarDataFile(args[i]) {
				return false
			}
			continue
		}
		if strings.ContainsAny(arg[1:], "of") {
			return false
		}
	}
	return true
}

func isSarDataFile(path string) bool {
	if strings.ContainsAny(path, "*?[") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == ".." {
			return false
		}
	}
	for _, dir := range sarDataDirs {
		if strings.HasPrefix(path, dir) && len(path) > len(dir) {
			return true
		}
	}
	return false
}

// psArgsSafe refuses what makes ps print process environments, as reading
// /proc/PID/environ would: the BSD e modifier, the environ format, and a
// UNIX-style cluster with e that procps re-reads as BSD options because it
// does not parse as UNIX options (ps -ex and ps -auxeww print environments).
// UNIX -e on its own (every process) stays. Words that are another option's
// value are skipped; a value that cannot be told apart is treated as an
// option, which only costs an approval.
func psArgsSafe(args []string) bool {
	const unixFlagLetters = "AacdefFHjlLMmNTwyZ"
	const unixValueLetters = "CGUgpqstuOo"
	// BSD p, t and U are left out: if ps ever read their value as more
	// options, a skipped word could carry e.
	const bsdValueLetters = "Oo"
	longValues := []string{
		"--format", "--sort", "--pid", "--ppid", "--user", "--User", "--group", "--Group",
		"--tty", "--sid", "--cols", "--columns", "--width", "--rows", "--lines", "--quick-pid",
	}
	// Checked on every word first, since the loop below skips the words it
	// takes for option values (-o environ).
	for _, arg := range args {
		if strings.Contains(strings.ToLower(arg), "environ") {
			return false
		}
	}
	unparsedCluster := false
	clusterWithE := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "--"):
			if !strings.Contains(arg, "=") && containsString(longValues, arg) {
				i++
			}
		case strings.HasPrefix(arg, "-"):
			cluster := arg[1:]
			parses := unixClusterParses(cluster, unixFlagLetters, unixValueLetters)
			if strings.ContainsRune(cluster, 'e') {
				if !parses {
					return false
				}
				clusterWithE = true
			}
			if !parses {
				unparsedCluster = true
			}
			if takesNextWord(cluster, unixValueLetters) {
				i++
			}
		default:
			if strings.ContainsRune(arg, 'e') {
				return false
			}
			if takesNextWord(arg, bsdValueLetters) {
				i++
			}
		}
	}
	// A cluster procps cannot read as UNIX options may send the whole
	// command line to the BSD parser, where -e is e.
	return !(unparsedCluster && clusterWithE)
}

// unixClusterParses reports whether every letter of a dash cluster is a UNIX
// flag, up to a value-taking letter that ends the cluster (its value is the
// next word) or carries a numeric list such as -p1234.
func unixClusterParses(cluster, flagLetters, valueLetters string) bool {
	if cluster == "" {
		return false
	}
	for j := 0; j < len(cluster); j++ {
		if strings.IndexByte(valueLetters, cluster[j]) >= 0 {
			return j == len(cluster)-1 || strings.Trim(cluster[j+1:], "0123456789,") == ""
		}
		if strings.IndexByte(flagLetters, cluster[j]) < 0 {
			return false
		}
	}
	return true
}

// takesNextWord reports whether the first value-taking letter of a cluster
// is its last letter, so its value is the next word.
func takesNextWord(cluster, valueLetters string) bool {
	index := strings.IndexAny(cluster, valueLetters)
	return index >= 0 && index == len(cluster)-1
}

// ipArgsSafe allows ip addr, ip link and ip route alone or with show or
// list, and ip route get. Every other verb, abbreviations included (ip link
// s means set), changes the system or needs approval.
func ipArgsSafe(args []string) bool {
	if len(args) == 0 || !containsString([]string{"addr", "address", "link", "route"}, args[0]) {
		return false
	}
	if len(args) == 1 {
		return true
	}
	verb := args[1]
	if verb != "show" && verb != "list" && (verb != "get" || args[0] != "route") {
		return false
	}
	return noOptionWords(args[2:])
}

// ifconfigArgsSafe allows the display forms: listing options and at most one
// interface name. A second operand (an address, up, down, mtu) configures it.
func ifconfigArgsSafe(args []string) bool {
	operands := 0
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			if !containsString([]string{"-a", "-s", "-v"}, arg) {
				return false
			}
			continue
		}
		operands++
	}
	return operands <= 1
}

// dpkgArgsSafe allows dpkg -l and dpkg -L with package names only. Options
// such as --admindir or --root would read another database.
func dpkgArgsSafe(args []string) bool {
	if len(args) == 0 || !containsString([]string{"-l", "--list", "-L", "--listfiles"}, args[0]) {
		return false
	}
	return noOptionWords(args[1:])
}

// pveshArgsSafe refuses API paths that return a file's contents from a guest
// or a backup (qemu agent file-read, storage file-restore).
func pveshArgsSafe(args []string) bool {
	for _, arg := range args {
		for _, segment := range strings.Split(strings.ToLower(arg), "/") {
			if segment == "file-read" || segment == "file-restore" {
				return false
			}
		}
	}
	return true
}

// podmanReadFlags are the display flags of the podman subcommands the policy
// approves. podman's global flags (--root, --runtime, --out, --url,
// --connection and others) are persistent, so they are accepted after the
// subcommand too, and they select another storage root, runtime binary,
// output file or remote host. Only known display flags pass.
var podmanReadFlags = []string{
	"all", "quiet", "size", "filter", "format", "last", "latest", "no-trunc",
	"no-stream", "no-reset", "details", "follow", "since", "until", "tail",
	"timestamps", "type", "names", "noheading", "sort", "pod", "external", "ns",
	"digests", "interval", "watch", "verbose",
}

// podmanReadShortFlags are the short forms of podmanReadFlags; digits are
// attached values such as -n5.
const podmanReadShortFlags = "aqsnlfptwiv0123456789"

func podmanArgsSafe(args []string) bool {
	for _, arg := range args {
		switch {
		case arg == "--":
			return true
		case strings.HasPrefix(arg, "--"):
			name, _, _ := strings.Cut(arg[2:], "=")
			if !containsString(podmanReadFlags, name) {
				return false
			}
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			for i := 1; i < len(arg); i++ {
				if strings.IndexByte(podmanReadShortFlags, arg[i]) < 0 {
					return false
				}
			}
		}
	}
	return true
}

func noOptionWords(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return false
		}
	}
	return true
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
