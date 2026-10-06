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
import {
  buildUnixAgentInstallCommand,
  buildWindowsAgentInstallCommand,
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
mode_mask = termios.ECHO | termios.ICANON
if (initial_tty[3] & mode_mask) != (final_tty[3] & mode_mask):
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

const makeUnixFixture = () => {
  const dir = mkdtempSync(join(tmpdir(), 'pulse-private-unix-'));
  chmodSync(dir, 0o700);
  const trace = join(dir, 'trace');
  writeFileSync(trace, '');
  const bin = join(dir, 'bin');
  mkdirSync(bin, { mode: 0o700 });
  const fake = (name: string, body: string) => {
    writeFileSync(join(bin, name), `#!/bin/sh\n${body}\n`, { mode: 0o700 });
  };
  fake(
    'stty',
    'if [ \"$1\" = -echo ]; then echo \"$PPID\" > \"$READER_PID\"; fi; exec /bin/stty \"$@\"',
  );
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
      PATH: `${bin}:${process.env.PATH}`,
      FAKE_UID: uid,
      FAIL_AT: failure,
      TRACE: trace,
      ARGV: join(dir, 'argv'),
      DIRS: join(dir, 'dirs'),
      INSTALLER: join(dir, 'installer'),
      CAPTURE: join(dir, 'capture'),
      SECRET_ENV: join(dir, 'secret-env'),
      READER_PID: join(dir, 'reader-pid'),
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
