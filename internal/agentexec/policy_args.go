package agentexec

import (
	"regexp"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/secretredact"
)

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
	// file -s (--special-files) is allowed for block devices only (see
	// fileArgsSafe).
	fileSpecialOptions = deniedOptions{
		short: "s",
		long:  []string{"special-files"},
	}
	lastDeniedOptions = deniedOptions{
		short: "f",
		long:  []string{"file"},
	}
	blkidDeniedOptions = deniedOptions{
		short: "cg",
		long:  []string{"cache-file", "garbage-collect"},
	}
	// lsblk --sysroot reads device attributes (model, vendor, serial) from
	// files below the named directory, following links, and prints them.
	lsblkDeniedOptions = deniedOptions{
		long: []string{"sysroot"},
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
	case "find", "du", "df", "fdisk", "file", "last", "blkid", "lsblk", "ss", "lsof", "journalctl",
		"systemctl", "apt-cache", "sar", "ps", "ip", "ifconfig", "dpkg", "pvesh", "docker", "podman":
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
	case "df":
		return dfArgsSafe(args)
	case "fdisk":
		return fdiskArgsSafe(args)
	case "file":
		return fileArgsSafe(args)
	case "last":
		return lastDeniedOptions.allows(args)
	case "blkid":
		return blkidDeniedOptions.allows(withoutNullBlkidCache(args))
	case "lsblk":
		return lsblkDeniedOptions.allows(args)
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
	case "docker":
		return formatTemplatesSelectFields(args)
	case "podman":
		return podmanArgsSafe(args) && formatTemplatesSelectFields(args)
	}
	return false
}

var globSensitivePrograms = map[string]bool{
	"ip": true, "ifconfig": true, "ps": true, "sar": true, "pvesh": true,
	"apt-cache": true, "df": true,
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
// parse as syntax is refused rather than interpreted. Inside double quotes
// sh keeps a backslash as written unless it escapes $, `, ", \ or a newline,
// so "table {{.Names}}\t{{.Status}}" is read as docker receives it.
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
				if c == '"' && (strings.ContainsAny(quoted, "$`") ||
					strings.Contains(quoted, `\\`) || strings.HasSuffix(quoted, `\`)) {
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

// blockDeviceRe matches the kernel names of disks and their partitions.
// Character devices that share a prefix (the NVMe controller /dev/nvme0,
// /dev/loop-control, /dev/mmcblk0rpmb) do not match.
var blockDeviceRe = regexp.MustCompile(`^/dev/((sd|hd|vd|xvd)[a-z]+[0-9]*|(nvme[0-9]+n|mmcblk|md|loop|zd)[0-9]+(p[0-9]+)?|dm-[0-9]+)$`)

// fileArgsSafe also limits -s (--special-files) to block devices. Without -s
// file only stats a device or a zero-size /proc file; with it file opens and
// reads the operand, and for a character device that is an operation on the
// device: a watchdog is armed, a tape rewinds, a serial line or /proc/kmsg
// loses the bytes read. Every word that is not an option must therefore name
// a block device; a refused option value only costs an approval.
func fileArgsSafe(args []string) bool {
	if !fileDeniedOptions.allows(args) {
		return false
	}
	if fileSpecialOptions.allows(args) {
		return true
	}
	return operandsAllMatch(args, blockDeviceRe.MatchString)
}

// fdiskArgsSafe limits fdisk -l to block devices: fdisk opens every operand
// to read its partition table. A refused option value only costs an approval.
func fdiskArgsSafe(args []string) bool {
	return operandsAllMatch(args, blockDeviceRe.MatchString)
}

// operandsAllMatch reports whether every word that is not an option, and
// every word after --, satisfies allowed.
func operandsAllMatch(args []string, allowed func(string) bool) bool {
	operandsOnly := false
	for _, arg := range args {
		switch {
		case operandsOnly || !strings.HasPrefix(arg, "-") || arg == "-":
			if !allowed(arg) {
				return false
			}
		case arg == "--":
			operandsOnly = true
		}
	}
	return true
}

// dfValueOptionRe matches a GNU df option cluster whose last letter takes the
// next word as its value. A value option earlier in the cluster (-Bt) takes
// the rest of the word instead.
var dfValueOptionRe = regexp.MustCompile(`^-[aHhiklmPTv]*[BFtx]$`)

// dfArgsSafe limits the operands of df. GNU df opens every operand before it
// looks up its filesystem (so that an automounted one gets mounted), and for
// a character device, a FIFO or a link to one that open is an operation on
// the device, as with file -s: df /dev/watchdog arms the watchdog and the
// machine resets. The command text cannot show what a path leads to, since a
// link can be planted wherever someone may write, so an operand must be one
// the kernel opens only as a directory (see namesDirectory) or the name of a
// block device. df without operands reports every filesystem.
func dfArgsSafe(args []string) bool {
	operandsOnly, operandSeen := false, false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case operandsOnly || !strings.HasPrefix(arg, "-") || arg == "-":
			if !namesDirectory(arg) && !blockDeviceRe.MatchString(arg) {
				return false
			}
			operandSeen = true
		case arg == "--":
			operandsOnly = true
		case operandSeen:
			// With POSIXLY_CORRECT set every word after the first operand
			// is an operand, so a word that follows one is never skipped as
			// an option's value; that only costs an approval.
		case dfValueOptionRe.MatchString(arg), arg == "--block-size", arg == "--type", arg == "--exclude-type":
			i++
		}
	}
	return true
}

// namesDirectory reports whether a path can only be opened as a directory:
// it ends in a slash or in a . or .. component, and the kernel then refuses
// anything else with ENOTDIR before the open reaches a device driver,
// whatever links the path passes through.
func namesDirectory(path string) bool {
	if strings.HasSuffix(path, "/") {
		return true
	}
	last := path[strings.LastIndexByte(path, '/')+1:]
	return last == "." || last == ".."
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

// pveshReadPaths are the API subtrees pvesh get may read without approval:
// node, guest, storage and cluster state. pvesh acts as root@pam, so no
// Proxmox permission narrows the read, and other subtrees return stored
// credentials: ACME DNS plugin data (/cluster/acme), SDN DNS and IPAM keys
// (/cluster/sdn), OpenID client keys (/access/domains), a proxy password
// (/cluster/options).
var pveshReadPaths = []string{
	"version", "nodes", "storage", "pools",
	"cluster/status", "cluster/resources", "cluster/tasks", "cluster/log", "cluster/nextid",
	"cluster/ha", "cluster/backup", "cluster/backup-info", "cluster/replication", "cluster/ceph",
}

// pveshDeniedSegments are refused inside those subtrees too. They return a
// file's contents from a guest or a backup (qemu agent file-read, storage
// file-restore, vzdump extractconfig), a guest's generated cloud-init data,
// the node report with its raw configuration files, or the subscription key.
var pveshDeniedSegments = []string{
	"file-read", "file-restore", "extractconfig", "cloudinit", "report", "subscription",
}

// pveshArgsSafe allows "get PATH" for a PATH under pveshReadPaths. The path
// must be the word after get: pvesh reads its first operand as the path, and
// an option in front of it could take it as a value instead. Every later word
// is an option or the value of the option before it, so there is no second
// operand either.
func pveshArgsSafe(args []string) bool {
	if len(args) < 2 || args[0] != "get" {
		return false
	}
	for i, arg := range args[1:] {
		if pveshNamesPath(arg) {
			return false
		}
		// args[i] is the word before arg.
		if i > 0 && !strings.HasPrefix(arg, "-") &&
			(!strings.HasPrefix(args[i], "-") || strings.Contains(args[i], "=")) {
			return false
		}
		for _, segment := range strings.Split(strings.ToLower(arg), "/") {
			if containsString(pveshDeniedSegments, segment) {
				return false
			}
		}
	}
	var segments []string
	for _, segment := range strings.Split(args[1], "/") {
		switch {
		case segment == "":
			// pvesh skips empty segments.
			continue
		case segment == "." || segment == ".." || strings.Contains(segment, "%"):
			// Not a name; how pvesh would resolve it is not guessed.
			return false
		}
		segments = append(segments, segment)
	}
	path := strings.Join(segments, "/")
	for _, allowed := range pveshReadPaths {
		if path == allowed || strings.HasPrefix(path, allowed+"/") {
			return true
		}
	}
	return false
}

// pveshNamesPath reports whether arg names the path as an option. pvesh
// calls the operand api_path and parses options with Getopt::Long (one or two
// dashes, any case, abbreviations), so a second path given that way is
// refused rather than guessed at.
func pveshNamesPath(arg string) bool {
	if !strings.HasPrefix(arg, "-") {
		return false
	}
	name, _, _ := strings.Cut(strings.ToLower(strings.TrimLeft(arg, "-")), "=")
	return name != "" && strings.HasPrefix("api_path", name)
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

// formatTemplatesSelectFields checks the Go templates docker and podman take
// in --format (inspect -f). Output masking recognises a credential by the
// name in front of it (POSTGRES_PASSWORD=...), and a template chooses the
// output: {{range .Config.Env}}{{index (split . "=") 1}}{{end}} prints the
// values of a container's environment without their names. Only actions that
// select a field, alone or through json, run without approval:
// {{.State.Status}}, {{json .Mounts}}, {{.}}. They print what the plain
// command prints. Functions, range, with, variables and pipelines can drop
// the names and need approval. Every word is checked, so it does not matter
// how the option is spelled or attached.
func formatTemplatesSelectFields(args []string) bool {
	for _, arg := range args {
		for {
			open := strings.Index(arg, "{{")
			if open < 0 {
				break
			}
			end := strings.Index(arg[open:], "}}")
			if end < 0 {
				return false
			}
			action := strings.Fields(arg[open+2 : open+end])
			arg = arg[open+end+2:]
			if len(action) == 2 && action[0] == "json" {
				action = action[1:]
			}
			if len(action) != 1 || !templateFieldChainSafe(action[0]) {
				return false
			}
		}
	}
	return true
}

// templateFieldChars are the characters of a field name in a template.
const templateFieldChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_"

// templateFieldChainSafe accepts "." and chains such as .HostConfig.Memory.
// A field that masking would recognise by its own name (.Config.Labels.token)
// is refused, because the template prints its value without that name.
func templateFieldChainSafe(chain string) bool {
	if chain == "." {
		return true
	}
	if !strings.HasPrefix(chain, ".") {
		return false
	}
	for _, field := range strings.Split(chain[1:], ".") {
		if field == "" || strings.Trim(field, templateFieldChars) != "" {
			return false
		}
		if _, masked := secretredact.Text(`"` + field + `": "value"`); masked > 0 {
			return false
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
