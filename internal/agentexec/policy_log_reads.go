package agentexec

import (
	"regexp"
	"strings"
)

// logReadRoot is the only directory the default policy lets cat, head, tail
// and grep read without approval.
const logReadRoot = "/var/log"

// logReadWord is one shell word of a log read after quote removal.
type logReadWord struct {
	text string
	// glob is true for an unquoted word holding * or ?, which the shell
	// expands against the filesystem.
	glob bool
}

var (
	// logReadBareWordRe is the character set an unquoted word may use. It
	// leaves out everything the shell treats specially in a word: quoting,
	// escapes, expansions ($ ~ { } [ ]), comments and operators.
	logReadBareWordRe = regexp.MustCompile(`^[A-Za-z0-9/._+=:,@%*?-]+$`)
	logReadCountRe    = regexp.MustCompile(`^[+-]?[0-9]+[A-Za-z]{0,3}$`)
	logReadNumberRe   = regexp.MustCompile(`^[0-9]+$`)
	logReadSecondsRe  = regexp.MustCompile(`^([0-9]+(\.[0-9]*)?|\.[0-9]+)$`)
	logReadObsoleteRe = regexp.MustCompile(`^-[0-9]+$`)
)

// isAutoApprovedLogRead reports whether command is a cat, head, tail or grep
// whose every file operand is a path under /var/log. The command must already
// be free of shell control operators. It works on the original case, because
// grep and tail give upper- and lower-case options different meanings.
func isAutoApprovedLogRead(command string) bool {
	words, ok := splitLogReadWords(command)
	if !ok || len(words) < 2 {
		return false
	}
	args := words[1:]
	switch words[0].text {
	case "cat":
		return parseLogReadArgs(args, catLogReadOptions)
	case "head":
		return parseLogReadArgs(args, headLogReadOptions)
	case "tail":
		return parseLogReadArgs(args, tailLogReadOptions)
	case "grep":
		return parseLogReadArgs(args, grepLogReadOptions)
	default:
		return false
	}
}

// splitLogReadWords splits a command into shell words. A word is unquoted
// with only safe characters, or wholly inside one pair of single quotes, or
// wholly inside one pair of double quotes with nothing the shell expands
// there. Any other shape is refused rather than interpreted.
func splitLogReadWords(command string) ([]logReadWord, bool) {
	var words []logReadWord
	i := 0
	for i < len(command) {
		c := command[i]
		if c == ' ' || c == '\t' {
			i++
			continue
		}
		start := i
		switch c {
		case '\'', '"':
			end := strings.IndexByte(command[i+1:], c)
			if end < 0 {
				return nil, false
			}
			end += i + 1
			if end+1 < len(command) && command[end+1] != ' ' && command[end+1] != '\t' {
				return nil, false
			}
			text := command[start+1 : end]
			if c == '"' && strings.ContainsAny(text, "$`\\") {
				return nil, false
			}
			if strings.ContainsAny(text, "\x00\n\r") {
				return nil, false
			}
			words = append(words, logReadWord{text: text})
			i = end + 1
		default:
			for i < len(command) && command[i] != ' ' && command[i] != '\t' {
				i++
			}
			text := command[start:i]
			if !logReadBareWordRe.MatchString(text) {
				return nil, false
			}
			words = append(words, logReadWord{text: text, glob: strings.ContainsAny(text, "*?")})
		}
	}
	return words, true
}

// isLogReadPath reports whether a file operand stays under /var/log. It
// refuses parent segments, and globs in a segment that starts with a dot,
// since only those can expand to "..".
func isLogReadPath(word logReadWord) bool {
	p := word.text
	if p != logReadRoot && !strings.HasPrefix(p, logReadRoot+"/") {
		return false
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == ".." {
			return false
		}
		if word.glob && strings.HasPrefix(segment, ".") && strings.ContainsAny(segment, "*?") {
			return false
		}
	}
	return true
}

// logReadValueKind says which values an option accepts.
type logReadValueKind int

const (
	logReadNoValue      logReadValueKind = iota
	logReadCount                         // line or byte count, optionally signed or suffixed
	logReadNumber                        // plain non-negative integer
	logReadSeconds                       // non-negative interval, fractions allowed
	logReadText                          // free text such as a pattern; never a glob
	logReadOptionalWhen                  // --color[=WHEN]
)

type logReadOption struct {
	kind    logReadValueKind
	choices []string // allowed values, when the option takes a fixed set
}

type logReadOptionSet struct {
	short map[byte]logReadOption
	long  map[string]logReadOption
	// obsoleteCount accepts the historical -NUM form (head -20, grep -3).
	obsoleteCount bool
	// patternOperand means the first operand is a pattern, unless -e set one.
	patternOperand bool
	// patternOptions name the options that supply a pattern.
	patternOptions map[string]bool
	// recursiveOptions turn on recursive search; recursiveValueOptions do
	// so when their value is "recurse".
	recursiveOptions      map[string]bool
	recursiveValueOptions map[string]bool
}

var catLogReadOptions = logReadOptionSet{
	short: shortFlags("AbeEnstTuv"),
	long: longFlags("--show-all", "--number-nonblank", "--show-ends", "--number",
		"--squeeze-blank", "--show-tabs", "--show-nonprinting"),
}

var headLogReadOptions = logReadOptionSet{
	short: withShortValues(shortFlags("qvz"), map[byte]logReadOption{
		'n': {kind: logReadCount},
		'c': {kind: logReadCount},
	}),
	long: withLongValues(longFlags("--quiet", "--silent", "--verbose", "--zero-terminated"),
		map[string]logReadOption{
			"--lines": {kind: logReadCount},
			"--bytes": {kind: logReadCount},
		}),
	obsoleteCount: true,
}

var tailLogReadOptions = logReadOptionSet{
	short: withShortValues(shortFlags("fFqvz"), map[byte]logReadOption{
		'n': {kind: logReadCount},
		'c': {kind: logReadCount},
		's': {kind: logReadSeconds},
	}),
	long: withLongValues(longFlags("--quiet", "--silent", "--verbose", "--zero-terminated", "--retry", "--follow"),
		map[string]logReadOption{
			"--lines":               {kind: logReadCount},
			"--bytes":               {kind: logReadCount},
			"--sleep-interval":      {kind: logReadSeconds},
			"--pid":                 {kind: logReadNumber},
			"--max-unchanged-stats": {kind: logReadNumber},
		}),
	obsoleteCount: true,
}

// grepLogReadOptions leaves out options that read another file (-f, --file,
// --exclude-from), follow every symlink (-R) or filter by file-name globs.
var grepLogReadOptions = logReadOptionSet{
	short: withShortValues(shortFlags("EFGPiyvwxclLoqsbHhnTZzaIrUV"), map[byte]logReadOption{
		'e': {kind: logReadText},
		'm': {kind: logReadNumber},
		'A': {kind: logReadNumber},
		'B': {kind: logReadNumber},
		'C': {kind: logReadNumber},
		'd': {kind: logReadText, choices: []string{"read", "skip", "recurse"}},
		'D': {kind: logReadText, choices: []string{"read", "skip"}},
	}),
	long: withLongValues(longFlags(
		"--extended-regexp", "--fixed-strings", "--basic-regexp", "--perl-regexp",
		"--ignore-case", "--no-ignore-case", "--invert-match", "--word-regexp", "--line-regexp",
		"--count", "--files-with-matches", "--files-without-match", "--only-matching",
		"--quiet", "--silent", "--no-messages", "--byte-offset", "--with-filename",
		"--no-filename", "--line-number", "--initial-tab", "--null", "--null-data",
		"--text", "--binary", "--recursive", "--line-buffered", "--no-group-separator",
	), map[string]logReadOption{
		"--regexp":          {kind: logReadText},
		"--max-count":       {kind: logReadNumber},
		"--after-context":   {kind: logReadNumber},
		"--before-context":  {kind: logReadNumber},
		"--context":         {kind: logReadNumber},
		"--label":           {kind: logReadText},
		"--group-separator": {kind: logReadText},
		"--binary-files":    {kind: logReadText, choices: []string{"binary", "text", "without-match"}},
		"--directories":     {kind: logReadText, choices: []string{"read", "skip", "recurse"}},
		"--devices":         {kind: logReadText, choices: []string{"read", "skip"}},
		"--color":           {kind: logReadOptionalWhen, choices: []string{"never", "always", "auto"}},
		"--colour":          {kind: logReadOptionalWhen, choices: []string{"never", "always", "auto"}},
	}),
	obsoleteCount:         true,
	patternOperand:        true,
	patternOptions:        map[string]bool{"-e": true, "--regexp": true},
	recursiveOptions:      map[string]bool{"-r": true, "--recursive": true},
	recursiveValueOptions: map[string]bool{"-d": true, "--directories": true},
}

func shortFlags(letters string) map[byte]logReadOption {
	flags := make(map[byte]logReadOption, len(letters))
	for i := 0; i < len(letters); i++ {
		flags[letters[i]] = logReadOption{}
	}
	return flags
}

func longFlags(names ...string) map[string]logReadOption {
	flags := make(map[string]logReadOption, len(names))
	for _, name := range names {
		flags[name] = logReadOption{}
	}
	return flags
}

func withShortValues(flags map[byte]logReadOption, values map[byte]logReadOption) map[byte]logReadOption {
	for letter, option := range values {
		flags[letter] = option
	}
	return flags
}

func withLongValues(flags map[string]logReadOption, values map[string]logReadOption) map[string]logReadOption {
	for name, option := range values {
		flags[name] = option
	}
	return flags
}

func logReadValueAllowed(option logReadOption, value logReadWord) bool {
	if value.glob {
		return false
	}
	if len(option.choices) > 0 {
		for _, choice := range option.choices {
			if value.text == choice {
				return true
			}
		}
		return false
	}
	switch option.kind {
	case logReadCount:
		return logReadCountRe.MatchString(value.text)
	case logReadNumber:
		return logReadNumberRe.MatchString(value.text)
	case logReadSeconds:
		return logReadSecondsRe.MatchString(value.text)
	case logReadText:
		return true
	default:
		return false
	}
}

// parseLogReadArgs accepts only known options with valid values, all of
// them before the first operand, then at least one file operand, every one
// under /var/log. GNU getopt would also take options after operands, but
// with POSIXLY_CORRECT set, and in BusyBox on musl, those words are file
// names, so a word after the first operand that looks like an option is
// refused rather than guessed.
func parseLogReadArgs(args []logReadWord, set logReadOptionSet) bool {
	patternGiven := false
	recursive := false
	var operands []logReadWord
	endOfOptions := false

	for i := 0; i < len(args); i++ {
		word := args[i]
		text := word.text
		// "-" is an operand: grep's pattern, or standard input, which
		// isLogReadPath refuses.
		if endOfOptions || text == "" || text == "-" || text[0] != '-' {
			operands = append(operands, word)
			continue
		}
		if len(operands) > 0 || word.glob {
			return false
		}
		if text == "--" {
			endOfOptions = true
			continue
		}
		if set.obsoleteCount && logReadObsoleteRe.MatchString(text) {
			continue
		}

		if strings.HasPrefix(text, "--") {
			name, value, hasValue := strings.Cut(text, "=")
			option, known := set.long[name]
			if !known {
				return false
			}
			switch option.kind {
			case logReadNoValue:
				if hasValue && name == "--follow" {
					if value != "name" && value != "descriptor" {
						return false
					}
				} else if hasValue {
					return false
				}
			case logReadOptionalWhen:
				if hasValue && !logReadValueAllowed(logReadOption{kind: logReadText, choices: option.choices}, logReadWord{text: value}) {
					return false
				}
			default:
				if !hasValue {
					i++
					if i >= len(args) {
						return false
					}
					value = args[i].text
					if !logReadValueAllowed(option, args[i]) {
						return false
					}
				} else if !logReadValueAllowed(option, logReadWord{text: value}) {
					return false
				}
			}
			if set.patternOptions[name] {
				patternGiven = true
			}
			if set.recurses(name, value) {
				recursive = true
			}
			continue
		}

		// Short option cluster such as -in or -n100.
		for j := 1; j < len(text); j++ {
			option, known := set.short[text[j]]
			if !known {
				return false
			}
			if option.kind == logReadNoValue {
				if set.recurses("-"+string(text[j]), "") {
					recursive = true
				}
				continue
			}
			if set.patternOptions["-"+string(text[j])] {
				patternGiven = true
			}
			value := text[j+1:]
			if value == "" {
				i++
				if i >= len(args) {
					return false
				}
				value = args[i].text
				if !logReadValueAllowed(option, args[i]) {
					return false
				}
			} else if !logReadValueAllowed(option, logReadWord{text: value}) {
				return false
			}
			if set.recurses("-"+string(text[j]), value) {
				recursive = true
			}
			break
		}
	}

	if set.patternOperand && !patternGiven {
		if len(operands) == 0 || operands[0].glob {
			return false
		}
		operands = operands[1:]
	}
	if len(operands) == 0 {
		return false
	}
	literal := false
	for _, operand := range operands {
		if !isLogReadPath(operand) {
			return false
		}
		if !operand.glob {
			literal = true
		}
	}
	// A shell with nullglob drops a glob that matches nothing, and a
	// recursive grep left with no file operand searches the current
	// directory. One literal operand keeps the search under /var/log.
	if recursive && !literal {
		return false
	}
	return true
}

// recurses reports whether an option (with its value) makes grep search
// directories recursively.
func (set logReadOptionSet) recurses(name, value string) bool {
	if set.recursiveOptions[name] {
		return true
	}
	return set.recursiveValueOptions[name] && value == "recurse"
}
