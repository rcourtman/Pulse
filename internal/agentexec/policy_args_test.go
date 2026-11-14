package agentexec

import "testing"

// Display forms of the auto-approved programs keep running without approval.
func TestDefaultPolicyAutoApprovesDisplayForms(t *testing.T) {
	policy := DefaultPolicy()
	for _, command := range []string{
		// ip and ifconfig
		"ip addr",
		"ip address show",
		"ip addr show dev eth0",
		"ip route",
		"ip route show table all",
		"ip route list",
		"ip route get 192.0.2.1",
		"ip link",
		"ip link show eth0",
		"ifconfig",
		"ifconfig -a",
		"ifconfig eth0",
		"ifconfig -a eth0",
		// find
		"find / -type f -size +100M",
		"find /var -type f -mtime +30 -name '*.log'",
		"find /var/lib -type d -mtime -1 ! -path '/var/lib/docker/*'",
		"find /var -type f -size +10M \\( -name '*.gz' -o -name '*.tar' \\) -printf '%s %p\\n'",
		"find /home -xdev -type f -size +1G -ls",
		// du, file, last, blkid, lsblk
		"du -sh /var/lib/docker",
		"du -xh -d 3 /mnt/nvr",
		"du -sh --exclude=*.iso /srv",
		"du -sh ~",
		"file /usr/bin/ls",
		"file -bi /var/log/syslog",
		"file -s /dev/sda1",
		"last -n 20",
		"last -F reboot",
		"blkid",
		"blkid -o value -s UUID /dev/sda1",
		"blkid -p /dev/sda1",
		"blkid -c /dev/null -o export",
		"blkid --cache-file=/dev/null",
		"lsblk -o NAME,SIZE,TYPE",
		// ss, lsof, netstat
		"ss -tulpn",
		"ss -tan state established '( dport = :443 )'",
		"ss -s",
		"ss -tn --family=inet",
		"lsof -nP -iTCP -sTCP:LISTEN",
		"lsof +D /var/log",
		"lsof +L1",
		"lsof -p 1234",
		// journalctl and systemctl
		"journalctl -u nginx -n 100 --no-pager",
		"journalctl -xeu nginx",
		"journalctl --since \"1 hour ago\" -p err",
		"journalctl --since=\"1 hour ago\" --until=now",
		"journalctl -b -1 -n 50",
		"journalctl --disk-usage",
		"journalctl --cursor=s=abc -n 10",
		"journalctl --verify",
		"journalctl -k --reverse",
		"systemctl status nginx --no-pager",
		"systemctl status nginx -n 50",
		"systemctl is-active nginx",
		"systemctl list-units --failed",
		"service nginx status",
		// sar, ps, apt-cache, dpkg, pvesh
		"sar -u 1 3",
		"sar -n DEV 1 1",
		"sar -A",
		"sar -f /var/log/sa/sa12 -u",
		"sar -f /var/log/sysstat/sa12 -r",
		"ps aux",
		"ps -ef",
		"ps auxf",
		"ps -eo pid,user,%cpu,%mem,etime,cmd --sort=-%cpu",
		"ps -p 1234 -o pid,etime,cmd",
		"ps axo pid,user,comm",
		"ps --format pid,user,etime",
		"ps ax --sort etime",
		"ps -aux",
		"ps -eLf",
		"ps -efww",
		"ps -ejH",
		"ps -ef -p1234",
		"ps -eo pid,cmd -p1234,5678",
		"apt-cache policy nginx",
		"apt-cache show nginx",
		"dpkg -l",
		"dpkg -l nginx 'linux-image*'",
		"dpkg --list",
		"dpkg -L nginx",
		"pvesh get /nodes --output-format json",
		"pvesh get /nodes/pve/qemu/100/agent/get-osinfo",
		// Anchored patterns still match their commands.
		"docker ps -a",
		"docker images",
		"docker network ls",
		"docker stats --no-stream",
		"podman ps",
		"podman ps -a --format '{{.Names}} {{.Status}}'",
		"podman logs --tail 100 -t web",
		"podman inspect --type container web",
		"podman images -a",
		"vmstat 1 5",
		"iostat -x",
		"mpstat -P ALL",
		"sudo ss -tulpn",
	} {
		if got := policy.Evaluate(command); got != PolicyAllow {
			t.Errorf("Evaluate(%q) = %q, want %q", command, got, PolicyAllow)
		}
	}
}

// Forms that change the system, write files, read another file's contents or
// hide their arguments need approval.
func TestDefaultPolicyRequiresApprovalForMutatingForms(t *testing.T) {
	policy := DefaultPolicy()
	for _, command := range []string{
		// ip and ifconfig mutations
		"ip link set eth0 down",
		"ip link s eth0 down",
		"ip link del br0",
		"ip link add dummy0 type dummy",
		"ip route del default",
		"ip route add 10.0.0.0/8 via 192.0.2.1",
		"ip route flush cache",
		"ip route replace default via 192.0.2.1",
		"ip route save",
		"ip addr add 192.0.2.10/24 dev eth0",
		"ip addr del 192.0.2.10/24 dev eth0",
		"ip addr flush dev eth0",
		"ip addrlabel add prefix ::/0 label 1",
		"ip link show eth0 -batch /tmp/x",
		"ip addr show; ip link set eth0 down",
		"ifconfig eth0 down",
		"ifconfig eth0 up",
		"ifconfig eth0 192.0.2.10 netmask 255.255.255.0",
		"ifconfig eth0 mtu 1400",
		"ifconfigx",
		// find actions and file output
		"find / -type f -delete",
		"find / -type f '-delete'",
		"find / -type f -del''ete",
		"find / -type f -del\\ete",
		"find / -type f -DELETE",
		"find / -type f -exec rm {} +",
		"find / -type f -execdir id {} +",
		"find / -type f -ok rm {} +",
		"find / -type f -fprint /etc/cron.d/x",
		"find / -type f -fprint0 /tmp/x",
		"find / -type f -fprintf /tmp/x '%p'",
		"find / -type f -fls /tmp/x",
		"find / -type f -files0-from /etc/shadow",
		"find / -type f ${x:--delete}",
		"find / -type f $'-delete'",
		"find / -type f {-delete,}",
		"find / -type f -*",
		"find / -type f *",
		"find / -type f -delet?",
		"du -s* /var",
		// du, file, last, blkid
		"du --files0-from=/etc/shadow",
		"du --files0-from /etc/shadow",
		"du --files0=/etc/shadow",
		"du -X /etc/shadow /var",
		"du -shX/etc/shadow /var",
		"du --exclude-from=/etc/shadow /var",
		"file -f /etc/shadow",
		"file -bf /etc/shadow",
		"file --files-from /etc/shadow",
		"file -m /etc/shadow /bin/ls",
		"file --magic-file=/etc/shadow /bin/ls",
		"file --mag=/etc/shadow /bin/ls",
		"file -C -m /etc/shadow",
		"last -f /etc/shadow",
		"last --file=/etc/shadow",
		"last --fi /etc/shadow",
		"blkid -c /etc/x",
		"blkid --cache-file=/etc/x",
		"blkid -g",
		// ss and lsof
		"ss -K dst 192.0.2.10",
		"ss -tK dst 192.0.2.10",
		"ss --kill dst 192.0.2.10",
		"ss --ki dst 192.0.2.10",
		"ss -D /tmp/x",
		"ss --diag=/tmp/x",
		"ss -F /etc/shadow",
		"ss --filter=/etc/shadow",
		"lsof -Db/tmp/x",
		"lsof -D b",
		"lsof -k /tmp/x",
		"lsof -m /tmp/x",
		"lsof +m /tmp/x",
		// journalctl and systemctl
		"journalctl --vacuum-time=1s",
		"journalctl -q --vacuum-size=1M",
		"journalctl --vac=1M",
		"journalctl --rotate",
		"journalctl --rot",
		"journalctl --flush",
		"journalctl --sync",
		"journalctl --relinquish-var",
		"journalctl --smart-relinquish-var",
		"journalctl --setup-keys",
		"journalctl --update-catalog",
		"journalctl --cursor-file=/etc/x -n 10",
		"journalctl --cursor-f=/etc/x",
		"journalctl --file=/tmp/x.journal",
		"journalctl -D /tmp/journal",
		"journalctl -xD/tmp/journal",
		"journalctl --directory=/tmp/journal",
		"journalctl --root=/mnt",
		"journalctl --image=/tmp/disk.img",
		"systemctl status -H root@192.0.2.1 nginx",
		"systemctl status --host=root@192.0.2.1 nginx",
		"service ../../tmp/x status",
		"service nginx status extra",
		// sar, ps, apt-cache, dpkg, pvesh
		"sar -o /tmp/x.bin 1 1",
		"sar -A -o/tmp/x.bin 1 1",
		"sar -uo /tmp/x.bin 1 1",
		"sar -f /etc/shadow",
		"sar -f /var/log/sa/../../../etc/shadow",
		"sar -f /var/log/sa/.*",
		"sar -f",
		"sar -fx",
		"ps aux e",
		"ps auxe",
		"ps eww -p 1",
		"ps -p 1 e",
		"ps -o pid e",
		"ps -ex",
		"ps -axe",
		"ps -auxeww",
		"ps -ewwx",
		"ps -aux -e",
		"ps -p1e",
		"ps -p 1 -o pid,environ",
		"ps -eo pid,environ",
		"ps --format=pid,ENVIRON",
		"ps -o environ",
		"ps --format environ",
		"ps -e -pe",
		"apt-cache gen*",
		"ps a*",
		"ifconfig d*",
		"ip addr show e*",
		"file -d /etc/shadow",
		"file --debug /etc/shadow",
		"file -bd /etc/shadow",
		"apt-cache gencaches",
		"apt-cache --with-source=/etc/shadow show nginx",
		"apt-cache -o Dir::Cache::pkgcache=/etc/x show nginx",
		"apt-cache -oDir::Cache=/etc show nginx",
		"apt-cache --option=Dir::Cache=/etc show nginx",
		"apt-cache -c /etc/shadow show nginx",
		"apt-cache --config-file=/etc/shadow show nginx",
		"apt-cache -p /etc/x show nginx",
		"apt-cache -s /etc/x show nginx",
		"dpkg -l --admindir=/tmp/x",
		"dpkg -l --root=/mnt",
		"dpkg -la",
		"dpkg -L nginx --admindir=/tmp/x",
		"pvesh get /nodes/pve/qemu/100/agent/file-read --file /etc/shadow",
		"pvesh get nodes/pve/qemu/100/agent//file-read --file /etc/shadow",
		"pvesh get /nodes/pve/storage/pbs/file-restore/download --volume x --filepath /",
		// podman global flags after the subcommand
		"podman ps --root /tmp/x",
		"podman ps --runroot=/tmp/x",
		"podman ps --runtime /tmp/x",
		"podman ps --out /tmp/x",
		"podman logs --url ssh://root@192.0.2.1/run/podman.sock web",
		"podman ps --connection remote",
		"podman ps -c remote",
		"podman ps -r",
		// Words the policy cannot read are not guessed.
		"ss -tan $OPTS",
		"ps aux `echo e`",
		"journalctl -u nginx ~-",
	} {
		if got := policy.Evaluate(command); got != PolicyRequireApproval {
			t.Errorf("Evaluate(%q) = %q, want %q", command, got, PolicyRequireApproval)
		}
	}
}

func TestSplitPolicyWords(t *testing.T) {
	for _, tc := range []struct {
		command string
		words   []string
		ok      bool
	}{
		{"journalctl --since=\"1 hour ago\" -u x", []string{"journalctl", "--since=1 hour ago", "-u", "x"}, true},
		{"find / -name '*.log' -del''ete", []string{"find", "/", "-name", "*.log", "-delete"}, true},
		{"find / \\( -name a \\)", []string{"find", "/", "(", "-name", "a", ")"}, true},
		{"du -sh ~/x ~", []string{"du", "-sh", "~/x", "~"}, true},
		{"ps -o \"$x\"", nil, false},
		{"ps {a,b}", nil, false},
		{"ps 'unterminated", nil, false},
		{"ps ~root", nil, false},
		{"ps a#b", nil, false},
	} {
		words, ok := splitPolicyWords(tc.command)
		if ok != tc.ok {
			t.Errorf("splitPolicyWords(%q) ok = %v, want %v", tc.command, ok, tc.ok)
			continue
		}
		if !ok {
			continue
		}
		var got []string
		for _, word := range words {
			got = append(got, word.text)
		}
		if len(got) != len(tc.words) {
			t.Errorf("splitPolicyWords(%q) = %q, want %q", tc.command, got, tc.words)
			continue
		}
		for i := range got {
			if got[i] != tc.words[i] {
				t.Errorf("splitPolicyWords(%q) = %q, want %q", tc.command, got, tc.words)
				break
			}
		}
	}
}
