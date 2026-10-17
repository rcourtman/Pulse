package tools

import (
	"strconv"
	"strings"
)

// literalCommandBase receives a decoded word, so quote characters are part of
// the executable name and must not be trimmed as shell syntax.
func literalCommandBase(word string) string {
	if i := strings.LastIndex(word, "/"); i >= 0 {
		return word[i+1:]
	}
	return word
}

// readCommandWords decodes the literal shell words we can inspect. Expansion is
// deliberately unsupported: its result could introduce options or extra operands.
// Glob expansion is retained only for binaries whose operands cannot enable writes.
func readCommandWords(command string) (words []string, glob bool, ok bool) {
	var word strings.Builder
	quote := byte(0)
	started := false
	flush := func() {
		if started {
			words = append(words, word.String())
			word.Reset()
			started = false
		}
	}
	for i := 0; i < len(command); i++ {
		c := command[i]
		if quote == '\'' {
			if c == quote {
				quote = 0
			} else {
				word.WriteByte(c)
			}
			continue
		}
		if c == '\\' {
			if i+1 == len(command) || command[i+1] == '\n' {
				return nil, false, false
			}
			i++
			// Inside double quotes, backslashes only escape these characters.
			if quote == '"' && !strings.ContainsRune("$`\"\\", rune(command[i])) {
				word.WriteByte('\\')
			}
			word.WriteByte(command[i])
			started = true
			continue
		}
		if c == '$' || c == '`' || c == 0 {
			return nil, false, false
		}
		if quote == '"' {
			if c == quote {
				quote = 0
			} else {
				word.WriteByte(c)
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			started = true
		case ' ', '\t':
			flush()
		case '\n', '\r', '|', '&', ';', '<', '>', '(', ')', '{', '}', '#':
			return nil, false, false
		case '*', '?', '[':
			glob = true
			fallthrough
		default:
			word.WriteByte(c)
			started = true
		}
	}
	flush()
	return words, glob, quote == 0 && len(words) > 0
}

// classifyReadPipeline checks every stage, rather than inheriting the first
// binary's permissions. Keep the original quoting when recursing into a stage.
func classifyReadPipeline(command string, opts executionClassificationOptions) (IntentResult, bool) {
	var stages []string
	quote := byte(0)
	start := 0
	for i := 0; i < len(command); i++ {
		c := command[i]
		if c == '\\' && quote != '\'' {
			i++
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
		} else if c == '|' {
			stages = append(stages, strings.TrimSpace(command[start:i]))
			start = i + 1
		}
	}
	if len(stages) == 0 {
		return IntentResult{}, false
	}
	stages = append(stages, strings.TrimSpace(command[start:]))
	for _, stage := range stages {
		if stage == "" {
			return IntentResult{Intent: IntentWriteOrUnknown, Reason: "empty pipeline stage"}, true
		}
		result := classifySingleCommandWithOptions(stage, opts)
		if result.Intent == IntentWriteOrUnknown {
			result.Reason = "pipeline stage not read-only: " + result.Reason
			return result, true
		}
	}
	return IntentResult{Intent: IntentReadOnlyCertain, Reason: "all pipeline stages are read-only"}, true
}

// safeReadOptions accepts exact options and short clusters, preserving case.
// A trailing '=' means one required literal argument. '?' allows an optional
// attached argument, for tools such as lsof whose bare -i is also useful.
// Unknown options, abbreviations and missing arguments fail closed.
func safeReadOptions(args []string, specification string, maxOperands int) bool {
	_, ok := readOptionOperands(args, specification, maxOperands)
	return ok
}

func readOptionOperands(args []string, specification string, maxOperands int, validators ...func(string, string) bool) ([]string, bool) {
	options := make(map[string]byte)
	for _, option := range strings.Fields(specification) {
		mode := byte(0)
		if strings.HasSuffix(option, "=") || strings.HasSuffix(option, "?") {
			mode = option[len(option)-1]
			option = option[:len(option)-1]
		}
		options[option] = mode
	}
	var operands []string
	validate := func(name, value string) bool {
		for _, validator := range validators {
			if !validator(name, value) {
				return false
			}
		}
		return true
	}
	end := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" && !end {
			end = true
			continue
		}
		if end || arg == "-" || !strings.HasPrefix(arg, "-") {
			operands = append(operands, arg)
			continue
		}
		name, value, attached := strings.Cut(arg, "=")
		if mode, found := options[name]; found {
			if attached && mode == 0 {
				return nil, false
			}
			if mode == '=' && !attached {
				i++
				if i >= len(args) || !isLiteralReadOptionValue(args[i]) {
					return nil, false
				}
				value = args[i]
			}
			if !validate(name, value) {
				return nil, false
			}
			continue
		}
		if strings.HasPrefix(arg, "--") || attached {
			return nil, false
		}
		for j := 1; j < len(arg); j++ {
			mode, found := options["-"+arg[j:j+1]]
			if !found {
				return nil, false
			}
			if mode != 0 {
				value := arg[j+1:]
				if mode == '=' && j == len(arg)-1 {
					i++
					if i >= len(args) || !isLiteralReadOptionValue(args[i]) {
						return nil, false
					}
					value = args[i]
				}
				if !validate("-"+arg[j:j+1], value) {
					return nil, false
				}
				break
			}
			if !validate("-"+arg[j:j+1], "") {
				return nil, false
			}
		}
	}
	return operands, maxOperands < 0 || len(operands) <= maxOperands
}

func isLiteralReadOptionValue(value string) bool {
	return !strings.HasPrefix(value, "-") || (len(value) > 1 && value[1] >= '0' && value[1] <= '9')
}

func wordIn(word, set string) bool {
	for _, candidate := range strings.Fields(set) {
		if word == candidate {
			return true
		}
	}
	return false
}

func isReadOnlyByConstruction(command string) bool {
	// Only these fixed fd redirects are admitted by the shell guard.
	words, glob, ok := readCommandWords(stripReadRedirects(command))
	if !ok {
		return false
	}
	// These programs only inspect data. Their options do not select write or
	// execution modes. Other allowlisted programs require argument validation.
	if wordIn(words[0], "cat head tail ls ll dir ps free du iostat vmstat mpstat grep locate which whereis whoami id groups echo printf uptime printenv locale netstat ping traceroute tracepath dig host stat wc cut tr getent nproc lscpu lsmem lsblk lxc-ls lxc-info zcat zgrep bzcat xzcat md5sum sha256sum sha1sum test pgrep pidof pstree last who w lspci lsusb pveversion") {
		return true
	}
	if words[0] == "[" {
		return words[len(words)-1] == "]"
	}
	if glob {
		return false
	}
	args := words[1:]
	switch words[0] {
	case "uname":
		return safeReadOptions(args, "-a -s -n -r -v -m -p -i -o --all --kernel-name --nodename --kernel-release --kernel-version --machine --processor --hardware-platform --operating-system --help --version", 0)
	case "nslookup":
		// Interactive variants interpret stdin and may support output files.
		operands, valid := readOptionOperands(args, "-type= -query= -querytype= -class= -domain= -timeout= -retry= -port= -debug -d2 -nodebug -recurse -norecurse -vc -novc -search -nosearch -defname -nodefname -fail -nofail", 2)
		return valid && len(operands) > 0 && !wordIn(strings.ToLower(operands[0]), "- ls view finger set server lserver root exit help")
	case "df":
		return safeReadOptions(args, "-a -B= -h -H -i -k -l -P -t= -T -x= --all --block-size= --human-readable --si --inodes --local --no-sync --portability --type= --print-type --exclude-type= --total --output? --help --version", -1)
	case "date":
		operands, valid := readOptionOperands(args, "-d= --date= -f= --file= -I? --iso-8601? -R --rfc-email --rfc-3339= -r= --reference= -u --utc --universal --help --version", 1)
		return valid && (len(operands) == 0 || strings.HasPrefix(operands[0], "+"))
	case "hostname":
		return safeReadOptions(args, "-a --alias -A --all-fqdns -d --domain -f --fqdn --long -i --ip-address -I --all-ip-addresses -s --short -y --yp --nis -v --verbose -h --help -V --version", 0)
	case "sort":
		return safeReadOptions(args, "-b -d -f -g -h -i -M -n -R -r -s -u -V -z -c -C -m -k= -t= -S= --ignore-leading-blanks --dictionary-order --ignore-case --general-numeric-sort --human-numeric-sort --ignore-nonprinting --month-sort --numeric-sort --random-sort --reverse --stable --unique --version-sort --zero-terminated --check? --merge --key= --field-separator= --buffer-size= --parallel= --batch-size= --help --version", -1)
	case "uniq":
		return safeReadOptions(args, "-c -d -D -u -i -z -f= -s= -w= --count --repeated --all-repeated? --unique --ignore-case --zero-terminated --skip-fields= --skip-chars= --check-chars= --help --version", 1)
	case "journalctl":
		// journalctl also accepts a negative boot offset as a separate word.
		args = append([]string(nil), args...)
		for i := 0; i+1 < len(args); i++ {
			if wordIn(args[i], "-b --boot") && strings.HasPrefix(args[i+1], "-") {
				if _, err := strconv.ParseInt(args[i+1], 10, 64); err == nil {
					args[i] += "=" + args[i+1]
					args = append(args[:i+1], args[i+2:]...)
				}
			}
		}
		return safeReadOptions(args, "-a -b? -k -f -e -r -q -x -l -m -n? -u= -t= -p= -o= -S= -U= -D= -M= --all --boot? --dmesg --follow --pager-end --reverse --quiet --catalog --full --merge --lines? --unit= --user-unit= --identifier= --priority= --output= --output-fields= --since= --until= --directory= --file= --machine= --namespace= --cursor= --after-cursor= --grep= --case-sensitive= --no-pager --no-hostname --no-full --no-tail --utc --system --user --list-boots --disk-usage --list-invocations --verify --header --field= -F= --fields --facility= --help --version", -1)
	case "dmesg":
		return safeReadOptions(args, "-d -H -k -L? -P -r -S -T -t -u -w -x -J -l= -f= -s= --show-delta --human --kernel --color? --nopager --raw --syslog --ctime --notime --userspace --follow --decode --json --level= --facility= --buffer-size= --since= --until= --time-format= --help --version", 0)
	case "sar":
		return safeReadOptions(args, "-A -B -b -C -d -F -H -h -p -r -S -t -u -v -W -w -y -z -q -R -n= -I= -P= -f= -s= -e= -i= --human --help -V", -1)
	case "fuser":
		return safeReadOptions(args, "-a -c -f -m -M -n= -s -u -v -4 -6 --all --mount --ismountpoint --namespace= --silent --user --verbose --ipv4 --ipv6 --help --version", -1)
	case "ss":
		return safeReadOptions(args, "-4 -6 -0 -a -d -e -f= -F= -H -i -l -m -n -N= -o -p -r -s -t -u -w -x -A= -O -T -b --ipv4 --ipv6 --all --dccp --extended --family= --filter= --no-header --info --listening --memory --numeric --net= --options --processes --query= --resolve --summary --tcp --udp --raw --unix --oneline --threads --bpf --help --version", -1)
	case "ifconfig":
		return safeReadOptions(args, "-a -s -v --help --version", 1)
	case "route":
		return safeReadOptions(args, "-n -e -v -A= -4 -6 -F -C --numeric --extend --verbose --family= --help --version", 0)
	case "arp":
		return safeReadOptions(args, "-a -e -n -v -D -i= -A= -H= --all --numeric --verbose --device= --protocol= --hw-type= --help --version", 1)
	case "ip":
		return isReadOnlyIP(args)
	case "lsof":
		// -D selects cache build/update modes on platforms with a device cache.
		return safeReadOptions(args, "-a -b -c= -d= -e= -f -g? -i? -l -n -N -o? -p= -P -R -s? -t -u= -U -v -V -w -X -F? -h", -1)
	case "blkid":
		// Low-level probing does not update blkid's persistent cache.
		return len(args) > 0 && wordIn(args[0], "-p --probe") && safeReadOptions(args, "-p --probe -i --info -o= --output= -s= --match-tag= -n= --match-types= -u= --usages= -O= --offset= -S= --size= --help --version", -1)
	case "file":
		return safeReadOptions(args, "-b -c -E -i -k -L -l -N -n -r -s -z -Z -0 -e= -f= -F= -m= -P= --brief --checking-printout --mime --mime-type --mime-encoding --keep-going --dereference --list --no-buffer --no-pad --raw --special-files --uncompress --uncompress-noreport --print0 --exclude= --files-from= --separator= --magic-file= --parameter= --help --version", -1)
	case "lastlog":
		return safeReadOptions(args, "-b= -t= -u= --before= --time= --user= --help --version", 0)
	case "dmidecode":
		return safeReadOptions(args, "-d= -s= -t= -H= -u -q --dev-mem= --string= --type= --handle= --dump --quiet --from-dump= --no-sysfs --help --version", 0)
	case "hwinfo":
		return safeReadOptions(args, "--all --short --listmd --only= --cpu --bios --memory --disk --partition --block --network --netcard --wlan --usb --pci --storage-ctrl --gfxcard --monitor --sound --bridge --system --help --version", 0)
	case "inxi":
		_, valid := readOptionOperands(args, "-A -b -B -c= --color= -C -d -D -E -f -F -G -h -i -I -j -J -l -L -m -M -n -N -o -p -P -r -R -s -S -t= -u -v= -V -w -W= -x -y= -z --admin --basic --full --extra --filter --no-host --no-color --help --version", 0, func(name, value string) bool {
			if wordIn(name, "-c --color") {
				color, err := strconv.Atoi(value)
				return err == nil && color >= 0 && color <= 42
			}
			return true
		})
		return valid
	case "sensors":
		return safeReadOptions(args, "-A -u -j -f -c= --no-adapter --fahrenheit --config-file= --help --version", -1)
	case "hddtemp":
		return safeReadOptions(args, "-n -q -u= --numeric --quiet --unit= --help --version", -1)
	case "smartctl":
		// Some --log values set SCT parameters or reset device counters.
		_, valid := readOptionOperands(args, "-a -x -i -H -A -c -d= -l= -v= -j --all --xall --info --health --attributes --capabilities --device= --log= --vendorattribute= --json --scan --scan-open --help --version", -1, func(name, value string) bool {
			return !wordIn(name, "-l --log") || wordIn(value, "error selftest selective directory scttempsts scttemphist scterc devstat sataphy")
		})
		return valid
	case "nvme":
		return len(args) > 0 && wordIn(args[0], "list list-subsys id-ctrl id-ns smart-log error-log fw-log effects-log self-test-log endurance-log") && safeReadOptions(args[1:], "-o= --output-format= -v --verbose -H --human-readable -b --raw-binary -n= --namespace-id= -e= --log-entries= --help", 1)
	case "ffprobe":
		return safeReadOptions(args, "-v= -loglevel= -hide_banner -show_format -show_streams -show_programs -show_chapters -show_packets -show_frames -show_error -show_entries= -show_data -show_data_hash= -select_streams= -of= -print_format= -read_intervals= -count_frames -count_packets -pretty -sexagesimal -unit -prefix -byte_binary_prefix -i= -h -help -version", 1)
	case "mediainfo":
		return safeReadOptions(args, "--Output= --Inform= --Full --Details= --Language= --ParseSpeed= --BOM --Help --Version -f -h", -1)
	case "exiftool":
		return safeReadOptions(args, "-a -u -g -G -s -S -n -j -json -csv -t -T -b -e -fast -fast2 -r -ext= -ver -h", -1)
	case "nc", "netcat":
		zeroIO := false
		_, valid := readOptionOperands(args, "-z -v -n -4 -6 -w= -G= -i= -p= -s=", 2, func(name, _ string) bool {
			zeroIO = zeroIO || name == "-z"
			return true
		})
		return valid && zeroIO
	}
	return isReadOnlySubcommand(words)
}

func stripReadRedirects(command string) string {
	// Match complete, unquoted redirect tokens. A prefix match would also
	// accept destinations such as /dev/null.log, which create real files.
	var out strings.Builder
	quote := byte(0)
	for i := 0; i < len(command); i++ {
		c := command[i]
		if c == '\\' && quote != '\'' && i+1 < len(command) {
			out.WriteByte(c)
			i++
			out.WriteByte(command[i])
			continue
		}
		if quote != 0 {
			out.WriteByte(c)
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
		} else if i == 0 || command[i-1] == ' ' || command[i-1] == '\t' {
			matched := false
			for _, redirect := range []string{"2>/dev/null", "2>&1"} {
				end := i + len(redirect)
				if strings.HasPrefix(command[i:], redirect) && (end == len(command) || command[end] == ' ' || command[end] == '\t') {
					i = end - 1
					matched = true
					break
				}
			}
			if matched {
				continue
			}
		}
		out.WriteByte(c)
	}
	return out.String()
}

func isSafeCurlRead(args []string) bool {
	operands, valid := readOptionOperands(args, "-s -S -f -L -k -i -I -v -4 -6 --silent --show-error --fail --fail-with-body --location --insecure --include --head --verbose --ipv4 --ipv6 --compressed --connect-timeout= --max-time= --retry= --retry-delay= -m= -H= --header= -u= --user= -A= --user-agent= -b= --cookie= --cacert= --cert= --key= --url= -X= --request=", -1, func(name, value string) bool {
		if name == "-X" || name == "--request" {
			return wordIn(strings.ToUpper(value), "GET HEAD OPTIONS")
		}
		return name != "--url" || isHTTPReadURL(value)
	})
	if !valid {
		return false
	}
	for _, operand := range operands {
		if !isHTTPReadURL(operand) {
			return false
		}
	}
	return true
}

func isHTTPReadURL(value string) bool {
	value = strings.ToLower(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func isSafeWgetSpiderRead(args []string) bool {
	spider := false
	operands, valid := readOptionOperands(args, "--spider -q --quiet -S --server-response --no-check-certificate -T= --timeout= -t= --tries= -U= --user-agent= --header= --user= --password= --max-redirect=", -1, func(name, _ string) bool {
		spider = spider || name == "--spider"
		return true
	})
	if !valid || !spider {
		return false
	}
	for _, operand := range operands {
		if !isHTTPReadURL(operand) {
			return false
		}
	}
	return true
}

func isReadOnlyIP(args []string) bool {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if !wordIn(args[0], "-4 -6 -0 -s -stats -statistics -d -details -h -human -r -resolve -o -oneline -j -json -p -pretty -br -brief") {
			return false
		}
		args = args[1:]
	}
	if len(args) == 0 || !wordIn(args[0], "a addr address r route l link n neigh neighbor neighbour") {
		return false
	}
	if len(args) == 1 {
		return true
	}
	verb := args[1]
	if !wordIn(verb, "show list lst get") || (verb == "get" && !wordIn(args[0], "r route")) {
		return false
	}
	// Remaining words are filters for an explicitly selected read operation.
	return safeReadOptions(args[2:], "", -1)
}

func isReadOnlySubcommand(words []string) bool {
	if len(words) < 2 {
		return false
	}
	args := words[2:]
	verb := words[1]
	switch words[0] {
	case "systemctl":
		return wordIn(verb, "status is-active is-enabled is-failed is-system-running list-units list-unit-files list-jobs list-sockets list-timers list-dependencies show show-environment cat") && safeReadOptions(args, "-a -l -q -p= -n= -o= -t= --all --full --quiet --property= --lines= --output= --type= --state= --no-pager --no-legend --plain --failed --user --system --value --after --before --reverse --recursive --with-dependencies", -1)
	case "service":
		return (verb == "--status-all" && len(args) == 0) || (len(args) == 1 && args[0] == "status" && !strings.HasPrefix(verb, "-"))
	case "docker", "docker-compose":
		if words[0] == "docker-compose" && verb != "ps" {
			return false
		}
		if verb == "compose" {
			if len(args) == 0 || args[0] != "ps" {
				return false
			}
			args = args[1:]
		} else if wordIn(verb, "network volume") {
			if len(args) == 0 || !wordIn(args[0], "ls inspect") {
				return false
			}
			args = args[1:]
		} else if !wordIn(verb, "ps logs inspect stats images info version top port") {
			return false
		}
		options := "-a -q -s -f= -n= --all --quiet --size --filter= --format= --last= --latest --no-trunc --no-stream --no-cache --details --follow --since= --until= --tail= --timestamps --type="
		if verb == "logs" {
			options = strings.ReplaceAll(options, "-f=", "-f -t")
		}
		return safeReadOptions(args, options, -1)
	case "pct", "qm":
		if words[0] == "qm" && verb == "guest" {
			return len(args) == 3 && args[0] == "cmd" && wordIn(args[2], "ping get-host-name get-osinfo get-users get-time get-fsinfo get-memory-block-info get-memory-blocks get-vcpus network-get-interfaces")
		}
		return (wordIn(verb, "list status config") || (words[0] == "pct" && verb == "df")) && safeReadOptions(args, "--current --snapshot= --verbose --full", 1)
	case "pvesh":
		return verb == "get" && safeReadOptions(args, "--output-format= --human-readable= --noborder= --noheader= --verbose", 1)
	case "pvecm":
		return wordIn(verb, "status nodes") && len(args) == 0
	case "pvesm":
		return wordIn(verb, "status list") && safeReadOptions(args, "--content= --storage= --vmid= --enabled= --target=", 1)
	case "zpool", "zfs":
		return (wordIn(verb, "list get") || (words[0] == "zpool" && verb == "status")) && safeReadOptions(args, "-H -p -v -P -L -r -x -t= -o= -s= -S= -d= -T=", -1)
	case "mdadm":
		return wordIn(verb, "--detail -D") && safeReadOptions(args, "--brief --export --scan --test -b -s -t", -1)
	case "apt", "yum", "dnf":
		return (verb == "list" || (words[0] == "apt" && verb == "show")) && safeReadOptions(args, "--installed --upgradable --all-versions -a", -1)
	case "apt-cache":
		return wordIn(verb, "show showpkg depends rdepends policy search pkgnames stats madison") && safeReadOptions(args, "--installed --names-only --recurse --no-recurse --all-versions -a -n", -1)
	case "dpkg":
		return wordIn(verb, "-l --list -s --status -L --listfiles -S --search") && safeReadOptions(args, "", -1)
	case "rpm":
		return wordIn(verb, "-q -qa --query") && safeReadOptions(args, "-a -i -l -R -s -f= -p= --all --info --list --requires --state --file= --package=", -1)
	case "kubectl":
		if verb == "config" {
			return len(args) == 1 && args[0] == "view"
		}
		if verb == "cluster-info" {
			return len(args) == 0
		}
		return wordIn(verb, "get describe logs top api-resources api-versions version") && safeReadOptions(args, "-A -a -c= -f -l= -n= -o= -p --all-namespaces --all-containers --container= --selector= --namespace= --output= --previous --follow --tail= --since= --since-time= --timestamps --prefix --limit-bytes= --show-labels --sort-by= --field-selector= --no-headers --containers --use-protocol-buffers --client --short", -1)
	}
	return false
}
