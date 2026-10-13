import { execFileSync, spawnSync } from 'node:child_process';
import {
  chmodSync,
  existsSync,
  mkdirSync,
  symlinkSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { unixBootstrapTestSource } from './unixBootstrapTestSource';
import {
  buildUnixAgentInstallCommand,
  buildWindowsAgentInstallCommand,
  buildWindowsAgentUninstallCommand,
  normalizeInstallerBaseUrl,
  resolveAgentCommandPlatform,
  resolveInstallerBaseUrl,
} from '../agentInstallCommand';

describe('agentInstallCommand', () => {
  it('includes insecure transport continuity for plain-http Pulse URLs', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'http://pulse.example:7655',
      token: 'token-123',
    });

    expect(command).toContain("--url 'http://pulse.example:7655'");
    expect(command).toContain('token_dir=$(mktemp -d /tmp/pulse-agent-bootstrap.XXXXXX)');
    expect(command).toContain('sudo bash -c');
    expect(command).not.toContain('token-123');
    expect(command).toContain('--token-file "$token_file"');
    expect(command).toContain('--insecure');
  });

  it('shell-quotes canonical URL and token-file bootstrap transport', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: "https://pulse.example/base path/agent's",
      token: "tok'en",
    });

    expect(command).toContain(
      "curl -fsSL 'https://pulse.example/base path/agent'\"'\"'s/install.sh' -o \"$install_script\"",
    );
    expect(command).toContain("--url 'https://pulse.example/base path/agent'\"'\"'s'");
    expect(command).not.toContain("tok'en");
    expect(command).not.toContain("tok'\"'\"'en");
    expect(command).toContain('--token-file "$token_file"');
    expect(command).not.toContain("--token 'tok");
  });

  it('runs a non-root preflight before privilege escalation for Unix installs', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: 'token-123',
    });

    const preflightIndex = command.indexOf('--preflight-only');
    const sudoIndex = command.indexOf('sudo bash -c');

    expect(command).toContain('bootstrap_dir=$(mktemp -d /tmp/pulse-bootstrap.XXXXXX)');
    expect(command).toContain('trap cleanup EXIT');
    expect(command).toContain('bash "$install_script" --url');
    expect(command).toContain('--output json');
    expect(command).toContain('--non-interactive');
    expect(preflightIndex).toBeGreaterThan(-1);
    expect(sudoIndex).toBeGreaterThan(preflightIndex);
    expect(
      command.indexOf('token_dir=$(mktemp -d /tmp/pulse-agent-bootstrap.XXXXXX)'),
    ).toBeGreaterThan(preflightIndex);
  });

  it('normalizes trailing slashes before building installer transport', () => {
    expect(normalizeInstallerBaseUrl('https://pulse.example/base///')).toBe(
      'https://pulse.example/base',
    );

    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example/base/',
      token: 'token-123',
    });

    expect(command).toContain("curl -fsSL 'https://pulse.example/base/install.sh'");
    expect(command).toContain("--url 'https://pulse.example/base'");
    expect(command).not.toContain('//install.sh');
    expect(command).not.toContain("--url 'https://pulse.example/base/'");
  });

  it('falls back to the canonical endpoint when the custom override is only whitespace', () => {
    expect(resolveInstallerBaseUrl('   ', 'https://pulse.example/base/')).toBe(
      'https://pulse.example/base',
    );
  });

  it('preserves explicit custom CA transport for the first installer fetch and runtime', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: 'token-123',
      caCertPath: '/etc/pulse/custom-ca.pem',
    });

    expect(command).toContain(
      "curl -fsSL --cacert '/etc/pulse/custom-ca.pem' 'https://pulse.example/install.sh' -o \"$install_script\"",
    );
    expect(command).toContain("--url 'https://pulse.example'");
    expect(command).toContain('--token-file "$token_file"');
    expect(command).toContain("--cacert '/etc/pulse/custom-ca.pem'");
  });

  it('preserves explicit insecure transport for self-signed https installs', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: 'token-123',
      insecure: true,
    });

    expect(command).toContain(
      'curl -kfsSL \'https://pulse.example/install.sh\' -o "$install_script"',
    );
    expect(command).toContain("--url 'https://pulse.example'");
    expect(command).toContain('--token-file "$token_file"');
    expect(command).toContain('--insecure');
  });

  it('omits token transport entirely when optional auth uses tokenless install commands', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: null,
    });

    expect(command).toContain(
      'curl -fsSL \'https://pulse.example/install.sh\' -o "$install_script"',
    );
    expect(command).toContain("--url 'https://pulse.example'");
    expect(command).not.toContain('--token');
    expect(command).not.toContain('token_file=');
  });

  it('keeps every copied Windows command on a single line', () => {
    // Console hosts execute pasted input line by line, so a literal newline
    // anywhere in the copied command breaks it in PowerShell 5.1 and 7 alike.
    const variants = [
      buildWindowsAgentInstallCommand({ baseUrl: 'https://pulse.example' }),
      buildWindowsAgentInstallCommand({
        baseUrl: 'https://pulse.example',
        token: 'token-123',
        insecure: true,
      }),
      buildWindowsAgentInstallCommand({
        baseUrl: 'https://pulse.example',
        token: 'token-123',
        caCertPath: 'C:\\Pulse\\custom-ca.cer',
      }),
    ];
    for (const command of variants) {
      expect(command).not.toMatch(/[\r\n]/);
    }
  });

  it('builds shared Windows install transport with token, insecure TLS, and custom CA continuity', () => {
    const command = buildWindowsAgentInstallCommand({
      baseUrl: 'https://pulse.example/base/',
      token: 'token-123',
      insecure: true,
      caCertPath: 'C:\\Pulse\\custom-ca.cer',
    });

    expect(command).toContain(
      '$pulseTmp=Join-Path ([System.IO.Path]::GetTempPath()) ("pulse-agent-install-"+[System.Guid]::NewGuid().ToString("N"))',
    );
    expect(command).toContain('$pulseScriptUrl="https://pulse.example/base/install.ps1"');
    expect(command).toContain(
      '[System.IO.File]::WriteAllText($pulseTokenFile, "token-123", [System.Text.Encoding]::ASCII)',
    );
    expect(command).toContain('-TokenFile $pulseTokenFile');
    expect(command).toContain('$env:PULSE_PREFLIGHT_ONLY="true"');
    expect(command).toContain('$env:PULSE_OUTPUT="json"');
    expect(command).toContain('$env:PULSE_NON_INTERACTIVE="true"');
    expect(command).toContain('$env:PULSE_INSECURE_SKIP_VERIFY="true"');
    expect(command).toContain('$env:PULSE_CACERT="C:\\Pulse\\custom-ca.cer"');
    expect(command).toContain(
      'Invoke-WebRequest -Uri $pulseScriptUrl -UseBasicParsing -OutFile $pulseInstallScript',
    );
    expect(command).toContain(
      'ServerCertificateValidationCallback=[PulseInstallerCertificateValidator]::AcceptAnyCallback',
    );
    expect(command).toContain('Add-Type -TypeDefinition');
    expect(command).not.toContain('ServerCertificateValidationCallback={ param(');
    expect(command).not.toContain('$pulseAllowInsecure');
    expect(command).toContain(
      '& $pulsePowerShell -NoProfile -ExecutionPolicy Bypass -File $pulseInstallScript -Url',
    );
    expect(command).toContain('if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }');
    expect(command).toContain('Remove-Item Env:PULSE_PREFLIGHT_ONLY -ErrorAction SilentlyContinue');
    expect(command).not.toContain('-PreflightOnly $true');
    expect(command).not.toContain('-NonInteractive $true');
    expect(command).not.toContain('-Insecure $true');
    expect(command).not.toContain('$env:PULSE_TOKEN=');
  });

  it('uses a runspace-independent custom CA callback when TLS verification stays enabled', () => {
    const command = buildWindowsAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: null,
      caCertPath: 'C:\\Pulse\\custom-ca.cer',
    });

    expect(command).toContain(
      'ServerCertificateValidationCallback=[PulseInstallerCertificateValidator]::ValidateCustomCaCallback',
    );
    expect(command).toContain(
      '[PulseInstallerCertificateValidator]::LoadCertificate($pulseCaCertPath)',
    );
    expect(command).not.toContain('CreateFromPem');
    expect(command).not.toContain('GetNewClosure');
  });

  it('supports tokenless shared Windows install transport for optional auth', () => {
    const command = buildWindowsAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: null,
    });

    expect(command).toContain('$pulseScriptUrl="https://pulse.example/install.ps1"');
    expect(command).not.toContain('$env:PULSE_TOKEN=');
    expect(command).not.toContain('-TokenFile $pulseTokenFile');
    expect(command).toContain('$env:PULSE_PREFLIGHT_ONLY="true"');
    expect(command).toContain('$env:PULSE_NON_INTERACTIVE="true"');
    expect(command).not.toContain('-PreflightOnly $true');
    expect(command).not.toContain('-NonInteractive $true');
  });

  it('fails closed when the install endpoint URL is blank', () => {
    expect(() =>
      buildUnixAgentInstallCommand({
        baseUrl: '   ',
        token: 'token-123',
      }),
    ).toThrow('Pulse install endpoint URL is required.');

    expect(() =>
      buildWindowsAgentInstallCommand({
        baseUrl: '   ',
        token: 'token-123',
      }),
    ).toThrow('Pulse install endpoint URL is required.');
  });

  it('preserves extra installer args for shared Unix install transport', () => {
    const command = buildUnixAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: 'token-123',
      extraArgs: ['--enable-docker', '--disable-host', '--enable-commands'],
    });

    expect(command).toContain('--token-file "$token_file"');
    expect(command).toContain('--enable-docker --disable-host --enable-commands');
  });

  it('preserves extra env assignments for shared Windows install transport', () => {
    const command = buildWindowsAgentInstallCommand({
      baseUrl: 'https://pulse.example',
      token: 'token-123',
      extraEnvAssignments: [
        '$env:PULSE_ENABLE_PROXMOX="true"',
        '$env:PULSE_PROXMOX_TYPE="pbs"',
        '$env:PULSE_ENABLE_COMMANDS="true"',
      ],
    });

    expect(command).not.toContain('$env:PULSE_TOKEN="token-123"');
    expect(command).toContain('-TokenFile $pulseTokenFile');
    expect(command).toContain('$env:PULSE_ENABLE_PROXMOX="true"');
    expect(command).toContain('$env:PULSE_PROXMOX_TYPE="pbs"');
    expect(command).toContain('$env:PULSE_ENABLE_COMMANDS="true"');
    expect(command).toContain(
      '& $pulsePowerShell -NoProfile -ExecutionPolicy Bypass -File $pulseInstallScript -Url',
    );
  });

  it('passes insecure runtime continuity for plain-http Windows installs', () => {
    const command = buildWindowsAgentInstallCommand({
      baseUrl: 'http://pulse.example:7655',
      token: 'token-123',
    });

    expect(command).toContain('-Url "http://pulse.example:7655"');
    expect(command).toContain('$env:PULSE_INSECURE_SKIP_VERIFY="true"');
    expect(command).toContain('$env:PULSE_PREFLIGHT_ONLY="true"');
    expect(command).not.toContain('-Insecure $true');
    expect(command).not.toContain('-PreflightOnly $true');
    expect(command).not.toContain('$env:PULSE_TOKEN=');
  });
});

describe('resolveAgentCommandPlatform', () => {
  it('maps legacy gopsutil Windows captions to windows (refs #1555)', () => {
    expect(resolveAgentCommandPlatform('microsoft windows 11 pro')).toBe('windows');
    expect(resolveAgentCommandPlatform('Microsoft Windows Server 2022 Standard')).toBe('windows');
    expect(resolveAgentCommandPlatform('windows')).toBe('windows');
  });

  it('maps macOS variants to macos', () => {
    expect(resolveAgentCommandPlatform('darwin')).toBe('macos');
    expect(resolveAgentCommandPlatform('macos')).toBe('macos');
    expect(resolveAgentCommandPlatform('Mac OS X')).toBe('macos');
  });

  it('maps FreeBSD variants to freebsd', () => {
    expect(resolveAgentCommandPlatform('freebsd')).toBe('freebsd');
    expect(resolveAgentCommandPlatform('FreeBSD 14.1-RELEASE')).toBe('freebsd');
  });

  it('defaults Linux distros and unknown values to linux', () => {
    expect(resolveAgentCommandPlatform('ubuntu')).toBe('linux');
    expect(resolveAgentCommandPlatform('')).toBe('linux');
    expect(resolveAgentCommandPlatform(undefined)).toBe('linux');
    expect(resolveAgentCommandPlatform(null)).toBe('linux');
  });
});

const variants = [
  { baseUrl: 'https://pulse.invalid' },
  { baseUrl: 'http://pulse.invalid/base/', token: 'invented-token' },
  { baseUrl: "https://pulse.invalid/agent's path", token: "tok'en;$(false)`false`\\value" },
  { baseUrl: 'https://pulse.invalid', insecure: true, token: 'invented-token' },
  {
    baseUrl: 'https://pulse.invalid',
    caCertPath: "/tmp/agent's ca.pem",
    extraArgs: ['--enable-docker', '--disable-host'],
  },
];

describe('single-line Unix install commands', () => {
  it.each(variants)('survives text-input normalization: %j', (options) => {
    const command = buildUnixAgentInstallCommand(options);
    const input = document.createElement('input');
    input.type = 'text';
    input.value = command;
    expect(command).not.toMatch(/[\r\n]/);
    expect(input.value).toBe(command);
    for (const shell of ['sh', 'bash']) {
      expect(() => execFileSync(shell, ['-n', '-c', input.value])).not.toThrow();
    }
  });

  it.each([
    { baseUrl: 'https://pulse.invalid/a\nb' },
    { baseUrl: 'https://pulse.invalid', tokenFilePath: '/tmp/a\rb' },
    { baseUrl: 'https://pulse.invalid', caCertPath: '/tmp/a\nb' },
    { baseUrl: 'https://pulse.invalid', extraArgs: ['--a\n--b'] },
  ])('rejects embedded line breaks without silently changing values: %j', (options) => {
    expect(() => buildUnixAgentInstallCommand(options)).toThrow('must not contain line breaks');
  });

  it.each(['sh', 'bash'])('runs tokenless commands under %s', (shell) => {
    const fixture = makeUnixFixture();
    try {
      for (const uid of ['0', '1000']) {
        const command = buildUnixAgentInstallCommand({ baseUrl: 'https://pulse.invalid' });
        const result = spawnSync(shell, ['-c', command], {
          env: fixture.env(uid),
          encoding: 'utf8',
        });
        expect(result.status, result.stderr).toBe(0);
        expect(readFileSync(fixture.trace, 'utf8')).toContain('install');
        assertUnixCleanup(fixture.dir);
      }
    } finally {
      fixture.close();
    }
  });
});

const unixPTYRunner = `
import errno, fcntl, json, os, pty, select, signal, subprocess, sys, termios, time
p = json.load(sys.stdin)
master, slave = pty.openpty()
tty_watch = os.dup(slave)
initial_tty = termios.tcgetattr(tty_watch)
def session():
    os.setsid()
    fcntl.ioctl(0, termios.TIOCSCTTY, 0)
history = p.get("history")
env = os.environ.copy()
if history:
    env.update(HISTFILE=history, HISTSIZE="100", HISTFILESIZE="100", HISTCONTROL="", PS1="PULSE_TEST$ ")
program = ["bash", "--noprofile", "--norc", "-i"] if history else ["sh", "-c", p["command"]]
child = subprocess.Popen(program, stdin=slave, stdout=slave, stderr=slave, preexec_fn=session, env=env)
if history: os.write(master, (p["command"] + "\\n").encode())
os.close(slave)
output = b""
sent = False
echo_at_prompt = False
deadline = time.monotonic() + 15
while time.monotonic() < deadline:
    ready, _, _ = select.select([master], [], [], .1)
    if ready:
        try: chunk = os.read(master, 65536)
        except OSError as e:
            if e.errno == errno.EIO: break
            raise
        if not chunk: break
        output += chunk
        # The interactive shell's echo of the command line quotes the prompt
        # text too, so a chunk can end there long before the bootstrap runs.
        # Only the bootstrap's own stty -echo (recorded by the stty fixture)
        # marks the real prompt.
        if (output.endswith(b"(paste at this prompt, not in the command): ") and not sent
                and os.path.exists(env["READER_PID"])):
            echo_at_prompt = bool(termios.tcgetattr(tty_watch)[3] & termios.ECHO)
            if p.get("read_signal"):
                time.sleep(.1)  # read has displayed its own prompt and is waiting
                with open(env["READER_PID"]) as f: reader = int(f.read())
                os.kill(reader, getattr(signal, "SIG" + p["read_signal"]))
            elif p.get("slow"):
                for piece in [p["input"][:4], p["input"][4:], "\\n"]:
                    os.write(master, piece.encode())
                    time.sleep(.05)
            else:
                os.write(master, (p["input"] + "\\n").encode())
            sent = True
            if history: os.write(master, b"exit\\n")
    if child.poll() is not None and not ready: break
else:
    os.killpg(child.pid, signal.SIGKILL)
    output += b"\\nPTY fixture deadline expired\\n"
code = child.wait()
final_tty = termios.tcgetattr(tty_watch)
if initial_tty != final_tty:
    output += b"\\nBootstrap did not restore terminal echo/canonical mode\\n"
    code = 1
if echo_at_prompt:
    output += b"\\nBootstrap prompted for the token with terminal echo enabled\\n"
    code = 1
os.close(tty_watch)
os.close(master)
sys.stdout.buffer.write(output)
sys.exit(code if code >= 0 else 128 - code)
`;

type TtyTool = 'stty' | 'busybox' | 'both' | 'none' | 'busybox-without-stty';
const makeUnixFixture = (ttyTool: TtyTool = 'stty', isolatedPath = false) => {
  const dir = mkdtempSync(join(tmpdir(), 'pulse-private-unix-'));
  chmodSync(dir, 0o700);
  const trace = join(dir, 'trace');
  writeFileSync(trace, '');
  const bin = join(dir, 'bin');
  mkdirSync(bin, { mode: 0o700 });
  const fake = (name: string, body: string) => {
    writeFileSync(join(bin, name), `#!/bin/sh\n${body}\n`, { mode: 0o700 });
  };
  const ttyBody = `
printf '%s\\n' "$0 $*" >> "$TTY_TRACE"
[ "$FAIL_AT:$1" != tty-state:-g ] || exit 1
[ "$FAIL_AT:$1" != tty-state-empty:-g ] || exit 0
[ "$FAIL_AT:$1" != tty-echo:-echo ] || exit 1
if [ "$FAIL_AT" = tty-restore ] && [ "$1" != -g ] && [ "$1" != -echo ] && [ ! -e "$RESTORE_FAILED" ]; then
  : > "$RESTORE_FAILED"; exit 1
fi
if [ "$1" = -echo ]; then echo "$PPID" > "$READER_PID"; fi
`;
  if (ttyTool === 'stty' || ttyTool === 'both') fake('stty', ttyBody + 'exec /bin/stty "$@"');
  if (ttyTool === 'busybox' || ttyTool === 'both')
    fake(
      'busybox',
      `[ "$1" = stty ] || exit 127; shift
${ttyBody}
# CI's shim exercises dispatch/modes without adding a package dependency.
# Source proof supplies an actual static BusyBox binary as an explicit input.
if [ -n "\${PULSE_TEST_BUSYBOX:-}" ]; then exec "$PULSE_TEST_BUSYBOX" stty "$@"; fi
exec /bin/stty "$@"`,
    );
  if (ttyTool === 'busybox-without-stty') fake('busybox', 'exit 127');
  if (isolatedPath) {
    // No /bin in PATH: an accidentally inherited standalone stty must not
    // turn the absent-tool control into a successful prompt.
    for (const program of ['bash', 'sh', 'python3', 'cp', 'cat', 'stat', 'dirname', 'rm', 'rmdir'])
      symlinkSync(
        execFileSync('which', [program], { encoding: 'utf8' }).trim(),
        join(bin, program),
      );
  }
  fake('id', 'if [ "$1" = -u ]; then echo "$FAKE_UID"; else exec /usr/bin/id "$@"; fi');
  fake(
    'sudo',
    'printf "%s\\n" "$@" >> "$ARGV"; [ "$FAIL_AT" != sudo ] || exit 1; [ "$1" != -v ] || exit 0; exec "$@"',
  );
  fake(
    'mktemp',
    'path=$(/usr/bin/mktemp "$@") || exit 1; printf "%s\\n" "$path" >> "$DIRS"; printf "%s\\n" "$path"',
  );
  fake(
    'curl',
    `printf '%s\\n' "$@" >> "$ARGV"
while [ "$#" -gt 0 ]; do
  if [ "$1" = -o ]; then output="$2"; shift; fi
  shift
done
cp "$INSTALLER" "$output"
echo fetch >> "$TRACE"
[ "$FAIL_AT" != fetch ] || exit 22`,
  );
  writeFileSync(
    join(dir, 'installer'),
    `#!/bin/bash
set -eu
printf '%s\\n' "$@" >> "$ARGV"
case " $* " in *' --preflight-only '*) echo preflight >> "$TRACE"; [ "$FAIL_AT" != preflight ]; exit $?;; esac
echo install >> "$TRACE"
printf '%s:%s' "\${pulse_token:-}" "\${PULSE_TOKEN:-}" > "$SECRET_ENV"
token_file=''
while [ "$#" -gt 0 ]; do
  if [ "$1" = --token-file ]; then token_file="$2"; shift; fi
  shift
done
if [ -n "$token_file" ]; then
  [ "$(stat -c %a "$token_file")" = 600 ]
  [ "$(stat -c %a "$(dirname "$token_file")")" = 700 ]
  [ "$(stat -c %u "$token_file")" = "$(stat -c %u "$(dirname "$token_file")")" ]
  cat "$token_file" > "$CAPTURE"
fi
if [ "$FAIL_AT" = TERM ] || [ "$FAIL_AT" = HUP ]; then kill -s "$FAIL_AT" "$PPID"; fi
[ "$FAIL_AT" != install ]
`,
    { mode: 0o700 },
  );
  return {
    dir,
    trace,
    env: (uid = '0', failure = '') => ({
      ...process.env,
      PATH: isolatedPath ? bin : `${bin}:${process.env.PATH}`,
      FAKE_UID: uid,
      FAIL_AT: failure,
      TRACE: trace,
      ARGV: join(dir, 'argv'),
      DIRS: join(dir, 'dirs'),
      INSTALLER: join(dir, 'installer'),
      CAPTURE: join(dir, 'capture'),
      SECRET_ENV: join(dir, 'secret-env'),
      READER_PID: join(dir, 'reader-pid'),
      TTY_TRACE: join(dir, 'tty-trace'),
      RESTORE_FAILED: join(dir, 'restore-failed'),
      PULSE_TOKEN: '',
    }),
    close: () => rmSync(dir, { recursive: true, force: true }),
  };
};

const assertUnixCleanup = (dir: string) => {
  if (existsSync(join(dir, 'dirs')))
    for (const path of readFileSync(join(dir, 'dirs'), 'utf8').trim().split('\n'))
      expect(existsSync(path), `temporary directory left behind: ${path}`).toBe(false);
};
const runPrivateUnix = (
  command: string,
  input: string,
  env: NodeJS.ProcessEnv,
  history?: string,
  readSignal?: string,
  slow = false,
) =>
  spawnSync('python3', ['-c', unixPTYRunner], {
    input: JSON.stringify({ command, input, history, read_signal: readSignal, slow }),
    env,
    encoding: 'utf8',
    timeout: 20000,
  });

// Join the real generated terminal prompt to the canonical installer's writer,
// rather than stopping at the recording installer's copy of the hand-off file.
// Downloads, preflight and root/sudo remain simulated. This does not run the
// agent, authenticate a server request or establish installed recovery.
describe('terminal-to-runtime token-file preservation', { timeout: 25_000 }, () => {
  const original = '0123456789abcdef'.repeat(4);
  const duplicated = original.slice(0, 24) + original.slice(20, 24) + original.slice(24);
  const legacy = 'legacy-synthetic-token.with-dashes_12345';

  it.each(['0', '1000'])(
    'preserves correct, same-hint duplicated and legacy input through the real writer (uid=%s)',
    (uid) => {
      const source = readFileSync(join(process.cwd(), '..', 'scripts', 'install.sh'), 'utf8');
      const start = source.indexOf('ensure_runtime_token_file() {');
      const end = source.indexOf('\nclear_proxmox_state_if_needed() {', start);
      expect(start).toBeGreaterThanOrEqual(0);
      expect(end).toBeGreaterThan(start);
      const writer = source.slice(start, end);
      expect(duplicated.slice(0, 6)).toBe(original.slice(0, 6));
      expect(duplicated.slice(-4)).toBe(original.slice(-4));
      expect(duplicated).not.toBe(original);

      for (const [name, inputs] of [
        ['correct', [original, original]],
        ['duplicated-middle then original restoration', [duplicated, original, original]],
        ['legacy', [legacy, legacy]],
      ] as const) {
        const fixture = makeUnixFixture();
        try {
          const installerPath = join(fixture.dir, 'installer');
          const recorder = readFileSync(installerPath, 'utf8');
          const copy = '  cat "$token_file" > "$CAPTURE"';
          expect(recorder).toContain(copy);
          writeFileSync(
            installerPath,
            recorder.replace(
              copy,
              `${copy}
  PULSE_TOKEN=$(cat "$token_file")
  TMP_FILES=(); ENROLL=false; EXIT_GENERAL=1
  log_info() { :; }; fail() { exit 1; }
  ${writer}
  ensure_runtime_token_file "$SAVED_STATE"
  printf %s "$RUNTIME_TOKEN_CHANGED" > "$SAVED_CHANGED"`,
            ),
          );
          const state = join(fixture.dir, 'saved-state');
          const command = buildUnixAgentInstallCommand({
            baseUrl: 'https://pulse.invalid',
            token: original,
            extraArgs: ['--enable-host'],
          });
          for (const [phase, input] of inputs.entries()) {
            const history = join(fixture.dir, `history-${phase}`);
            const result = runPrivateUnix(
              command,
              input,
              {
                ...fixture.env(uid),
                SAVED_STATE: state,
                SAVED_CHANGED: join(fixture.dir, 'saved-changed'),
              },
              history,
            );
            expect(result.status, result.stdout + result.stderr).toBe(0);
            expect(readFileSync(join(fixture.dir, 'capture'), 'utf8')).toBe(input);
            expect(readFileSync(join(state, 'token'), 'utf8')).toBe(input);
            expect(readFileSync(join(fixture.dir, 'saved-changed'), 'utf8')).toBe(
              phase === 0 || input !== inputs[phase - 1] ? 'true' : 'false',
            );
            expect(execFileSync('stat', ['-c', '%a', state], { encoding: 'utf8' }).trim()).toBe(
              '700',
            );
            expect(
              execFileSync('stat', ['-c', '%a', join(state, 'token')], { encoding: 'utf8' }).trim(),
            ).toBe('600');
            expect(readFileSync(join(fixture.dir, 'secret-env'), 'utf8')).toBe(':');
            for (const text of [
              command,
              readFileSync(join(fixture.dir, 'argv'), 'utf8'),
              readFileSync(history, 'utf8'),
              result.stdout,
              result.stderr,
            ]) {
              expect(text).not.toContain(input);
            }
            assertUnixCleanup(fixture.dir);
            console.info(
              `terminal/writer uid=${uid} case=${name} phase=${phase} bytes=${input.length} exact=true private=true cleaned=true`,
            );
          }
        } finally {
          fixture.close();
        }
      }
    },
  );
});

// The PTY has a 15s deadline and its caller a 20s ceiling. Let both return
// their terminal/cleanup assertions before Vitest's enclosing budget expires.
describe('credential-free Unix lifecycle execution', { timeout: 25_000 }, () => {
  it.each(['0', '1000'])(
    'keeps private input out of root/sudo argv, environment, output and history (uid=%s)',
    (uid) => {
      const fixture = makeUnixFixture();
      const marker = join(fixture.dir, 'must-not-execute');
      const token = `synthetic-' ; $(touch ${marker}) \`false\` \\value`;
      try {
        const command = buildUnixAgentInstallCommand({
          baseUrl: "https://pulse.invalid/agent's path",
          token,
          extraArgs: ['--enable-docker', "--agent-id 'agent-42'", "--hostname 'node.example'"],
        });
        const history = join(fixture.dir, 'history');
        const result = runPrivateUnix(command, token, fixture.env(uid), history);
        expect(result.status, result.stdout + result.stderr).toBe(0);
        expect(readFileSync(join(fixture.dir, 'capture'), 'utf8')).toBe(token);
        expect(existsSync(marker)).toBe(false);
        expect(readFileSync(join(fixture.dir, 'secret-env'), 'utf8')).toBe(':');
        const argv = readFileSync(join(fixture.dir, 'argv'), 'utf8');
        expect(argv).toContain("--url\nhttps://pulse.invalid/agent's path\n");
        expect(argv).toContain('--agent-id\nagent-42\n');
        expect(argv).toContain('--hostname\nnode.example\n');
        const hist = readFileSync(history, 'utf8');
        expect(hist).toContain(command);
        for (const text of [command, argv, hist, result.stdout, result.stderr])
          expect(text).not.toContain(token);
        assertUnixCleanup(fixture.dir);
      } finally {
        fixture.close();
      }
    },
  );

  it('preserves every character of the maximum-size accepted token', () => {
    const fixture = makeUnixFixture();
    try {
      const input = 'a'.repeat(4096);
      const result = runPrivateUnix(
        buildUnixAgentInstallCommand({ baseUrl: 'https://pulse.invalid', token: 'present' }),
        input,
        fixture.env(),
      );
      expect(result.status, result.stdout + result.stderr).toBe(0);
      expect(readFileSync(join(fixture.dir, 'capture'), 'utf8')).toBe(input);
      assertUnixCleanup(fixture.dir);
    } finally {
      fixture.close();
    }
  });

  it('drains oversized input without echoing it or returning it to interactive history', () => {
    const fixture = makeUnixFixture();
    try {
      const input = 'x'.repeat(10000) + '-OVERFLOW_HISTORY_MARKER';
      const history = join(fixture.dir, 'history');
      const result = runPrivateUnix(
        buildUnixAgentInstallCommand({ baseUrl: 'https://pulse.invalid', token: 'present' }),
        input,
        fixture.env(),
        history,
      );
      expect(result.status, result.stdout + result.stderr).not.toBe(0);
      expect(result.stdout).toContain('at most 4096');
      expect(existsSync(join(fixture.dir, 'capture'))).toBe(false);
      for (const text of [result.stdout, readFileSync(history, 'utf8')]) {
        expect(text).not.toContain('OVERFLOW_HISTORY_MARKER');
        expect(text).not.toContain('x'.repeat(512));
      }
      assertUnixCleanup(fixture.dir);
    } finally {
      fixture.close();
    }
  });

  it.each([
    'fetch',
    'preflight',
    'sudo',
    'install',
    'HUP',
    'TERM',
    'blank',
    'interrupt',
    'no-terminal',
  ])('stops and cleans up on %s', (failure) => {
    const fixture = makeUnixFixture();
    try {
      const command = buildUnixAgentInstallCommand({
        baseUrl: 'https://pulse.invalid',
        token: 'present',
      });
      const env = fixture.env(failure === 'sudo' ? '1000' : '0', failure);
      const result =
        failure === 'no-terminal'
          ? spawnSync('sh', ['-c', command], { env, encoding: 'utf8' })
          : runPrivateUnix(
              command,
              failure === 'blank' ? '' : failure === 'interrupt' ? '\x03' : 'test-input-secret',
              env,
            );
      expect(result.status, result.stdout + result.stderr).not.toBe(0);
      if (!['install', 'HUP', 'TERM'].includes(failure))
        expect(existsSync(join(fixture.dir, 'capture'))).toBe(false);
      expect(result.stdout).not.toContain('test-input-secret');
      expect(result.stdout).not.toContain('did not restore');
      assertUnixCleanup(fixture.dir);
    } finally {
      fixture.close();
    }
  });

  it.each(['HUP', 'TERM', 'INT'])(
    'restores the terminal when %s interrupts private input',
    (readSignal) => {
      const fixture = makeUnixFixture();
      try {
        const command = buildUnixAgentInstallCommand({
          baseUrl: 'https://pulse.invalid',
          token: 'present',
        });
        const result = runPrivateUnix(command, '', fixture.env(), undefined, readSignal);
        expect(result.status, result.stdout + result.stderr).not.toBe(0);
        expect(result.stdout).not.toContain('did not restore');
        expect(result.stdout).not.toContain('deadline expired');
        expect(existsSync(join(fixture.dir, 'capture'))).toBe(false);
        assertUnixCleanup(fixture.dir);
      } finally {
        fixture.close();
      }
    },
  );

  it('accepts deliberate slow token input without a timeout or truncation', () => {
    const fixture = makeUnixFixture();
    try {
      const command = buildUnixAgentInstallCommand({
        baseUrl: 'https://pulse.invalid',
        token: 'present',
      });
      const input = 'deliberate-slow-input';
      const result = runPrivateUnix(command, input, fixture.env(), undefined, undefined, true);
      expect(result.status, result.stdout + result.stderr).toBe(0);
      expect(readFileSync(join(fixture.dir, 'capture'), 'utf8')).toBe(input);
      expect(result.stdout).not.toContain(input);
      assertUnixCleanup(fixture.dir);
    } finally {
      fixture.close();
    }
  });

  it('supports non-terminal private-file input without deleting the operator file', () => {
    const fixture = makeUnixFixture();
    try {
      const file = join(fixture.dir, "operator's token");
      writeFileSync(file, 'private-file-token', { mode: 0o600 });
      const command = buildUnixAgentInstallCommand({
        baseUrl: 'https://pulse.invalid',
        tokenFilePath: file,
      });
      expect(command).not.toContain('read -r');
      expect(command).not.toContain('private-file-token');
      const result = spawnSync('sh', ['-c', command], { env: fixture.env(), encoding: 'utf8' });
      expect(result.status, result.stdout + result.stderr).toBe(0);
      expect(readFileSync(join(fixture.dir, 'capture'), 'utf8')).toBe('private-file-token');
      expect(readFileSync(file, 'utf8')).toBe('private-file-token');
      assertUnixCleanup(fixture.dir);
    } finally {
      fixture.close();
    }
  });

  it.each(['missing', 'symlink', 'fifo', 'directory', 'public-file', 'public-parent'])(
    'rejects unsafe private-file input: %s',
    (kind) => {
      const fixture = makeUnixFixture();
      try {
        const file = join(fixture.dir, 'provided-token');
        const real = join(fixture.dir, 'actual-token');
        writeFileSync(real, 'private-file-token', { mode: 0o600 });
        if (kind === 'symlink') symlinkSync(real, file);
        if (kind === 'fifo') execFileSync('mkfifo', [file]);
        if (kind === 'directory') mkdirSync(file);
        if (kind === 'public-file' || kind === 'public-parent') {
          writeFileSync(file, 'private-file-token', {
            mode: kind === 'public-file' ? 0o644 : 0o600,
          });
          if (kind === 'public-parent') chmodSync(fixture.dir, 0o755);
        }
        const command = buildUnixAgentInstallCommand({
          baseUrl: 'https://pulse.invalid',
          tokenFilePath: file,
        });
        const result = spawnSync('sh', ['-c', command], {
          env: fixture.env(),
          encoding: 'utf8',
          timeout: 3000,
        });
        expect(result.status, result.stdout + result.stderr).not.toBe(0);
        expect(existsSync(join(fixture.dir, 'capture'))).toBe(false);
        assertUnixCleanup(fixture.dir);
      } finally {
        fixture.close();
      }
    },
  );
});

describe('terminal-control portability', { timeout: 25_000 }, () => {
  const command = () =>
    buildUnixAgentInstallCommand({ baseUrl: 'https://pulse.invalid', token: 'present' });

  it.each(['stty', 'busybox', 'both'] as const)(
    'keeps maximum input private and restores modes with %s',
    (tool) => {
      const fixture = makeUnixFixture(tool, true);
      const input = 'portable-private-'.padEnd(4096, 'x');
      const history = join(fixture.dir, 'history');
      try {
        const result = runPrivateUnix(command(), input, fixture.env(), history);
        expect(result.status, result.stdout + result.stderr).toBe(0);
        expect(readFileSync(join(fixture.dir, 'capture'), 'utf8')).toBe(input);
        expect(readFileSync(join(fixture.dir, 'secret-env'), 'utf8')).toBe(':');
        const trace = readFileSync(join(fixture.dir, 'tty-trace'), 'utf8');
        expect(trace).toContain(`${tool === 'busybox' ? 'busybox' : 'stty'} -g`);
        if (tool === 'both') expect(trace).not.toContain('busybox');
        for (const text of [
          result.stdout,
          result.stderr,
          readFileSync(history, 'utf8'),
          readFileSync(join(fixture.dir, 'argv'), 'utf8'),
        ])
          expect(text).not.toContain(input);
        assertUnixCleanup(fixture.dir);
      } finally {
        fixture.close();
      }
    },
  );

  it.each(['blank', 'overflow', 'HUP', 'INT', 'TERM'])(
    'refuses %s with applet-only input, without leaks or leftover modes/files',
    (failure) => {
      const fixture = makeUnixFixture('busybox', true);
      const input = failure === 'overflow' ? 'x'.repeat(10000) + 'PRIVATE_OVERFLOW_SENTINEL' : '';
      const history = join(fixture.dir, 'history');
      try {
        const result = runPrivateUnix(
          command(),
          input,
          fixture.env(),
          history,
          ['HUP', 'INT', 'TERM'].includes(failure) ? failure : undefined,
        );
        expect(result.status, result.stdout + result.stderr).not.toBe(0);
        expect(result.stdout).not.toMatch(/deadline expired|did not restore/);
        expect(existsSync(join(fixture.dir, 'capture'))).toBe(false);
        for (const text of [result.stdout, result.stderr, readFileSync(history, 'utf8')])
          expect(text).not.toMatch(/PRIVATE_OVERFLOW_SENTINEL|x{512}/);
        assertUnixCleanup(fixture.dir);
      } finally {
        fixture.close();
      }
    },
  );

  it.each([
    ['none', '', 'Silent token input requires stty or BusyBox stty'],
    ['busybox-without-stty', '', 'Cannot read terminal settings'],
    ['stty', 'tty-state', 'Cannot read terminal settings'],
    ['busybox', 'tty-state', 'Cannot read terminal settings'],
    ['stty', 'tty-state-empty', 'Cannot read terminal settings'],
    ['busybox', 'tty-state-empty', 'Cannot read terminal settings'],
    ['stty', 'tty-echo', 'Cannot disable terminal echo'],
    ['busybox', 'tty-echo', 'Cannot disable terminal echo'],
    ['stty', 'tty-restore', 'Cannot restore terminal settings'],
    ['busybox', 'tty-restore', 'Cannot restore terminal settings'],
  ] as const)(
    'diagnoses %s/%s without pretending a real TTY is missing',
    (tool, failure, error) => {
      const fixture = makeUnixFixture(tool, true);
      try {
        const result = runPrivateUnix(command(), 'private-input', fixture.env('0', failure));
        expect(result.status, result.stdout + result.stderr).not.toBe(0);
        expect(result.stdout).toContain(error);
        expect(result.stdout).toContain('No installation was attempted');
        expect(result.stdout).not.toMatch(
          /A terminal is required|command not found|deadline expired|did not restore|private-input/,
        );
        expect(existsSync(join(fixture.dir, 'capture'))).toBe(false);
        assertUnixCleanup(fixture.dir);
      } finally {
        fixture.close();
      }
    },
  );

  it.each(['stty', 'busybox', 'none'] as const)('no TTY fails before selecting %s', (tool) => {
    const fixture = makeUnixFixture(tool, true);
    try {
      const result = spawnSync('sh', ['-c', command()], { env: fixture.env(), encoding: 'utf8' });
      expect(result.status, result.stdout + result.stderr).not.toBe(0);
      expect(result.stderr).toContain('A terminal is required');
      expect(result.stderr).toContain('private token-file command');
      expect(existsSync(join(fixture.dir, 'tty-trace'))).toBe(false);
      expect(existsSync(join(fixture.dir, 'capture'))).toBe(false);
      assertUnixCleanup(fixture.dir);
    } finally {
      fixture.close();
    }
  });

  it('uses the existing private-file handoff with neither TTY nor terminal utilities', () => {
    const fixture = makeUnixFixture('none', true);
    try {
      const file = join(fixture.dir, 'operator-token');
      writeFileSync(file, 'file-input', { mode: 0o600 });
      const cmd = buildUnixAgentInstallCommand({
        baseUrl: 'https://pulse.invalid',
        tokenFilePath: file,
      });
      const result = spawnSync('sh', ['-c', cmd], { env: fixture.env(), encoding: 'utf8' });
      expect(result.status, result.stdout + result.stderr).toBe(0);
      expect(readFileSync(join(fixture.dir, 'capture'), 'utf8')).toBe('file-input');
      expect(readFileSync(file, 'utf8')).toBe('file-input');
      assertUnixCleanup(fixture.dir);
    } finally {
      fixture.close();
    }
  });
});

describe('saved-credential uninstall commands', () => {
  it.each(['unused-new-install-token', null])(
    'never asks for installation credentials (%s)',
    (token) => {
      const command = buildUnixAgentInstallCommand({
        baseUrl: 'https://download.invalid',
        token,
        tokenFilePath: 'unused-relative-file',
        extraArgs: ['--uninstall'],
      });
      const child = unixBootstrapTestSource(command);
      expect(command).not.toContain('unused-new-install-token');
      expect(command).not.toContain('unused-relative-file');
      expect(command).not.toContain('Pulse agent token');
      expect(command).not.toContain('--token');
      expect(command).not.toContain('--url');
      expect(command).not.toContain('--preflight-only');
      expect(child).toContain(
        'unset PULSE_TOKEN PULSE_TOKEN_FILE PULSE_URL PULSE_AGENT_ID PULSE_HOSTNAME',
      );
      expect(command).toContain('https://download.invalid/install.sh');
    },
  );

  it.each([0, 17])(
    'executes the actual Unix command without a terminal and retains installer exit %s',
    (exit) => {
      const dir = mkdtempSync(join(tmpdir(), 'pulse-uninstall-command-'));
      const bin = join(dir, 'bin');
      mkdirSync(bin);
      try {
        writeFileSync(join(bin, 'id'), '#!/bin/sh\nprintf 0\n', { mode: 0o700 });
        writeFileSync(
          join(bin, 'curl'),
          `#!/bin/sh
while [ "$#" -gt 0 ]; do
  if [ "$1" = -o ]; then cp "$RECORDER" "$2"; exit 0; fi
  shift
done
exit 31
`,
          { mode: 0o700 },
        );
        // Only a recorder is downloaded. No production installer or sudo runs.
        const recorder = join(dir, 'recorder');
        writeFileSync(
          recorder,
          `#!/bin/bash
printf '%s\\n' "$@" > "$ARGV"
printf '%s:%s:%s:%s:%s:%s:%s:%s' "\${PULSE_TOKEN-}" "\${PULSE_TOKEN_FILE-}" "\${PULSE_URL-}" "\${PULSE_AGENT_ID-}" "\${PULSE_HOSTNAME-}" "\${PULSE_INSECURE_SKIP_VERIFY-}" "\${PULSE_CACERT-}" "\${PULSE_SERVER_FINGERPRINT-}" > "$ENV_CAPTURE"
exit "$EXPECTED_EXIT"
`,
        );
        const command = buildUnixAgentInstallCommand({
          baseUrl: 'https://download.invalid',
          token: 'not-an-uninstall-credential',
          caCertPath: '/explicit/ca.pem',
          extraArgs: ['--uninstall', "--agent-id 'saved-id'", "--hostname 'saved.example'"],
        });
        const result = spawnSync('bash', ['-c', command], {
          encoding: 'utf8',
          timeout: 5000,
          env: {
            ...process.env,
            PATH: `${bin}:${process.env.PATH}`,
            RECORDER: recorder,
            ARGV: join(dir, 'argv'),
            ENV_CAPTURE: join(dir, 'environment'),
            EXPECTED_EXIT: String(exit),
            PULSE_TOKEN: 'ambient-install-secret',
            PULSE_TOKEN_FILE: '/wrong/file',
            PULSE_URL: 'https://wrong.invalid',
            PULSE_AGENT_ID: 'wrong-id',
            PULSE_HOSTNAME: 'wrong-host',
            PULSE_INSECURE_SKIP_VERIFY: 'true',
            PULSE_CACERT: '/wrong/ca',
            PULSE_SERVER_FINGERPRINT: 'wrong-pin',
          },
        });
        expect(result.status, result.stdout + result.stderr).toBe(exit);
        expect(result.stdout + result.stderr).not.toContain('ambient-install-secret');
        expect(readFileSync(join(dir, 'environment'), 'utf8')).toBe(':::::::');
        expect(readFileSync(join(dir, 'argv'), 'utf8').trim().split('\n')).toEqual([
          '--uninstall',
          '--agent-id',
          'saved-id',
          '--hostname',
          'saved.example',
          '--non-interactive',
          '--cacert',
          '/explicit/ca.pem',
        ]);
        // The parent shell retains its inherited value; clearing is child-only.
        const sentinel = execFileSync('bash', ['-c', command + '; printf %s "$PULSE_TOKEN"'], {
          env: {
            ...process.env,
            PATH: `${bin}:${process.env.PATH}`,
            RECORDER: recorder,
            ARGV: join(dir, 'argv'),
            ENV_CAPTURE: join(dir, 'environment'),
            EXPECTED_EXIT: '0',
            PULSE_TOKEN: 'parent-sentinel',
          },
          encoding: 'utf8',
          timeout: 5000,
        });
        expect(sentinel).toBe('parent-sentinel');
      } finally {
        rmSync(dir, { recursive: true, force: true });
      }
    },
  );

  it('Windows binds empty credential parameters and escaped identity without mutating environment', () => {
    const command = buildWindowsAgentUninstallCommand({
      baseUrl: 'https://download.invalid/',
      agentId: "agent'$()",
      hostname: "host'$name",
      caCertPath: "C:\\trust\\host's.pem",
    });
    expect(command).toContain('https://download.invalid/install.ps1');
    expect(command).toContain("-Uninstall $true -Url '' -Token '' -TokenFile ''");
    expect(command).toContain("-AgentId 'agent''$()' -Hostname 'host''$name'");
    expect(command).toContain(
      "-Insecure $false -CACertPath 'C:\\trust\\host''s.pem' -ServerFingerprint ''",
    );
    expect(command).toContain("LoadCertificate('C:\\trust\\host''s.pem')");
    expect(command).toContain('-PreflightOnly $false');
    expect(command).not.toContain('$env:');
    expect(command).not.toContain('\n');
    expect(command).toContain('ForEach-Object { & ([scriptblock]::Create($_))');
  });

  it('HTTP download does not grant an insecure saved-server uninstall', () => {
    expect(
      buildUnixAgentInstallCommand({
        baseUrl: 'http://download.invalid',
        extraArgs: ['--uninstall'],
      }),
    ).not.toContain(' --insecure');
    expect(buildWindowsAgentUninstallCommand({ baseUrl: 'http://download.invalid' })).toContain(
      '-Insecure $false',
    );
  });
});
