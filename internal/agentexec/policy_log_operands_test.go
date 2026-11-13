package agentexec

import "testing"

func TestDefaultPolicyLogReadsAllowPathsUnderVarLog(t *testing.T) {
	p := DefaultPolicy()

	for _, command := range []string{
		"cat /var/log/syslog",
		"cat -n /var/log/syslog /var/log/kern.log",
		"head -n 20 /var/log/dpkg.log",
		"head -20 /var/log/dpkg.log",
		"head --lines=20 /var/log/dpkg.log",
		"tail -n 100 /var/log/syslog",
		"tail -n 100 '/var/log/syslog'",
		`tail -n 100 "/var/log/syslog"`,
		"tail -100 /var/log/syslog",
		"tail -fn 50 /var/log/syslog",
		"tail -F /var/log/syslog",
		"tail --follow=name --retry /var/log/syslog",
		"tail -n +1 -c 4K /var/log/syslog",
		"tail -n 100 /var/log/grafana/*.log",
		"tail -f -s 0.5 /var/log/syslog",
		"tail -f -s .5 /var/log/syslog",
		"grep -i error /var/log/syslog",
		"grep -F 'out of memory' /var/log/kern.log",
		`grep -E "oom killer" /var/log/syslog`,
		"grep -c error /var/log/syslog.1",
		"grep pattern /var/log/*.log",
		"grep -r error /var/log",
		"grep -r error /var/log/app/*.log /var/log/syslog",
		"grep -d recurse error /var/log/app",
		"grep -rn -e oom -e killed /var/log/",
		"grep --regexp=oom --color=never -m 5 /var/log/kern.log",
		"grep -i -C 3 timeout /var/log/pve/tasks/active",
		"grep -- -error /var/log/syslog",
		"grep -- - /var/log/syslog",
		"sudo tail -n 50 /var/log/auth.log",
		"cat /var/log/./syslog",
	} {
		if got := p.Evaluate(command); got != PolicyAllow {
			t.Errorf("Evaluate(%q) = %q, want %q", command, got, PolicyAllow)
		}
	}
}

func TestDefaultPolicyLogReadsRefuseOperandsOutsideVarLog(t *testing.T) {
	p := DefaultPolicy()

	for _, command := range []string{
		// Extra operands outside /var/log.
		"cat /var/log/syslog /etc/shadow",
		"tail -n 5 /etc/pve/priv/token.cfg /var/log/syslog",
		"head -n 5 /etc/pve/priv/token.cfg /var/log/syslog",
		`cat "/var/log/syslog" "/etc/shadow"`,
		"grep root /etc/shadow",
		"grep -e root /etc/shadow /var/log/syslog",
		// Log directories elsewhere are not under /var/log.
		"cat /opt/app/log/app.log",
		"tail -n 5 /srv/log/app.log",
		"cat /var/logs/app.log",
		// Parent segments and globs that could reach one.
		"cat /var/log/../../etc/shadow",
		"tail -n 5 /var/log/../lib/pulse-agent/token",
		"tail -n 5 /var/log/.*/lib/pulse-agent/token",
		"cat /var/log/.?/lib/pulse-agent/token",
		"cat '/var/log/../../etc/shadow'",
		// Shell expansion and quoting the check does not interpret.
		"cat /var/log/$HOME",
		`cat "/var/log/$x"`,
		"cat ~/x",
		"cat /var/log/{syslog,../../etc/shadow}",
		`cat /var/log/.\./.\./etc/shadow`,
		"cat '/var/log/x'/../../etc/shadow",
		"cat /var/log/[.][.]/lib/pulse-agent/token",
		"cat /var/log/syslog #",
		"grep * /var/log/syslog",
		"grep -e* /var/log/syslog",
		// Options that read another file, follow every symlink or read stdin.
		"grep -f /etc/shadow /var/log/syslog",
		"grep --file=/etc/shadow /var/log/syslog",
		"grep --exclude-from=/etc/shadow -r x /var/log",
		"grep -R x /var/log",
		"grep -r secret",
		"grep secret",
		"cat -",
		"tail -n 5 -",
		"tail --follow=everything /var/log/syslog",
		"tail -n lots /var/log/syslog",
		"head --unknown /var/log/syslog",
		// Options after an operand are file names to some getopt
		// implementations (POSIXLY_CORRECT, BusyBox on musl).
		"tail /var/log/syslog -n 5",
		"head /var/log/syslog -n 5",
		"grep -e root /var/log/syslog -e /etc/shadow",
		"grep error /var/log/syslog --file=/etc/shadow",
		// A recursive grep whose only operands are globs searches the
		// current directory if a nullglob shell drops them.
		"grep -r error /var/log/nothing*",
		"grep --recursive error /var/log/app/*.log",
		"grep -d recurse error /var/log/*",
		"grep --directories=recurse error /var/log/*",
		"grep --directories recurse error /var/log/nothing*",
		"grep -- - -",
		// sudo with options stays conservative.
		"sudo -u root cat /var/log/syslog",
		// Command separators.
		"cat /var/log/syslog & cat /etc/shadow",
		"tail -n 5 /var/log/syslog&cat /etc/shadow",
		"ps aux & cat /etc/shadow",
		// Other fixed cat rules take no extra operands.
		"cat /etc/os-release /etc/shadow",
	} {
		if got := p.Evaluate(command); got == PolicyAllow {
			t.Errorf("Evaluate(%q) = %q, want approval or block", command, got)
		}
	}
}

func TestDefaultPolicyFixedReadsStillAllowed(t *testing.T) {
	p := DefaultPolicy()

	for _, command := range []string{
		"cat /etc/os-release",
		"cat /proc/meminfo",
		"journalctl -u nginx -n 100 --no-pager",
		"ps aux",
	} {
		if got := p.Evaluate(command); got != PolicyAllow {
			t.Errorf("Evaluate(%q) = %q, want %q", command, got, PolicyAllow)
		}
	}
	if got := p.Evaluate("cat /proc/self/environ"); got != PolicyBlock {
		t.Errorf("Evaluate(cat /proc/self/environ) = %q, want %q", got, PolicyBlock)
	}
}
