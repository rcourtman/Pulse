import { describe, expect, it } from 'vitest';
import { parseToolCommandPreview, parseToolInputSummary } from '../toolPresentation';

// Branch-coverage companion to toolPresentation.test.ts and
// toolPresentation.coverage.test.ts. The seven target functions
// (parseFunctionStyleToolInput, extractPartialJSONStringField,
// commandPreviewValue, formatShellCommandPreview, shellCommandIntentLabel,
// parseStructuredInputRecord, partialCommandPreview) are all module-private,
// so every branch is driven through the two exported entry points
// `parseToolInputSummary` and `parseToolCommandPreview`, matching the sibling
// suites' convention.

const readSummary = (record: Record<string, unknown>): string =>
  parseToolInputSummary(JSON.stringify(record), 'pulse_read');
const commandPreview = (input: string, tool: string, rawInput?: string): string =>
  parseToolCommandPreview(input, tool, rawInput);

// ---------------------------------------------------------------------------
// parseFunctionStyleToolInput — partial-mode branches the sibling suites never
// reach: the unquoted-value `)` stop (line 279 ternary, allowPartial arm), the
// post-value `)` break (line 298), end-of-body right after `=` (line 267),
// empty unquoted rawValue rescue (line 284), and the negative/decimal arms of
// the numeric literal matcher (line 288).
// ---------------------------------------------------------------------------

describe('parseFunctionStyleToolInput — partial-mode branches', () => {
  it('stops an unquoted value at a mid-body ")" and breaks (lines 279 + 298)', () => {
    // Strict regex rejects `read(command=df -h)z` (no trailing `)`), so the
    // partial pass runs: the unquoted scan hits `)` (line 279 allowPartial
    // arm), then the post-value `)` break fires (line 298). The captured
    // command flows through to the read-exec intent label.
    expect(parseToolInputSummary('read(command=df -h)z', 'pulse_read')).toBe('Inspect filesystems');
  });

  it('breaks on ")" after a quoted value when strict mode rejected the call (line 298)', () => {
    // `read(action="file")extra)` — strict consumes the inner body then fails
    // at the leftover "extra"; the partial pass slices the trailing ")" and
    // breaks at the next ")" after the quoted value.
    expect(parseToolInputSummary('read(action="file")extra)', 'pulse_read')).toBe('read file');
  });

  it('rescues earlier args when the body ends right after "=" (line 267)', () => {
    // No closing ")" → strict regex fails; partial parses `action="file"`,
    // reaches `path=`, then end-of-body and returns the partial result.
    expect(parseToolInputSummary('read(action="file", path=', 'pulse_read')).toBe('read file');
  });

  it('rescues earlier args when an unquoted value is empty (line 284)', () => {
    // `x=,` yields an empty rawValue; partialResult returns the prior `action`.
    expect(parseToolInputSummary('read(action="file", x=,)', 'pulse_read')).toBe('read file');
  });

  it('types negative-integer and decimal unquoted values via the numeric matcher arms', () => {
    expect(parseToolInputSummary('query(action=get, resource_id=-5)', 'pulse_query')).toBe(
      'get -5',
    );
    expect(parseToolInputSummary('query(action=get, resource_id=1.5)', 'pulse_query')).toBe(
      'get 1.5',
    );
  });
});

// ---------------------------------------------------------------------------
// extractPartialJSONStringField — the sibling suites only exercise happy-path
// command/cmd/path extraction. These cases drive the in-value unescape arms
// (`\"` → `"`, `\\` → `\`) and the trailing `.trim()` of a space-padded value.
// Reached via partialCommandPreview → parseToolCommandPreview using incomplete
// JSON so the full-JSON path is bypassed.
// ---------------------------------------------------------------------------

describe('extractPartialJSONStringField — escape and trim arms', () => {
  it('unescapes an embedded escaped quote in a partial JSON command value', () => {
    // Raw chars: {"command":"a\"b  — the capture group holds a\"b which the
    // `\"` → `"` replacement turns into a"b.
    expect(commandPreview('{"command":"a\\"b', 'pulse_run_command')).toBe('$ a"b');
  });

  it('unescapes a JSON-encoded backslash in a partial command value', () => {
    // Raw chars: {"command":"a\\b — capture holds a\\b (two backslashes) which
    // the `\\` → `\` replacement collapses to a single backslash.
    expect(commandPreview('{"command":"a\\\\b', 'pulse_run_command')).toBe('$ a\\b');
  });

  it('trims surrounding whitespace inside a partial JSON string value', () => {
    expect(commandPreview('{"command":"  df  ', 'pulse_run_command')).toBe('$ df');
  });
});

// ---------------------------------------------------------------------------
// commandPreviewValue / formatShellCommandPreview — the sibling suite only
// redacts an `Authorization: Bearer` header. These drive the remaining redaction
// arms in redactShellCommandPreview (sk- token, --flag=, Bearer w/o header,
// ENV=, lowercase key=) which all flow through commandPreviewValue's
// non-truncating branch and formatShellCommandPreview's `$ ` prefix arm.
// ---------------------------------------------------------------------------

describe('commandPreviewValue / formatShellCommandPreview — redaction arms', () => {
  it('redacts a bare sk- style API key', () => {
    expect(
      commandPreview(JSON.stringify({ command: 'echo sk-abcdefghijkl' }), 'pulse_run_command'),
    ).toBe('$ echo [redacted-secret]');
  });

  it('redacts an --api-key=<value> flag', () => {
    expect(
      commandPreview(
        JSON.stringify({ command: 'curl --api-key=secret12345 https://x' }),
        'pulse_run_command',
      ),
    ).toBe('$ curl --api-key=[redacted-secret] https://x');
  });

  it('redacts a Bearer token passed without an Authorization: prefix', () => {
    expect(
      commandPreview(
        JSON.stringify({ command: 'curl -H Bearer abc12345678 https://x' }),
        'pulse_run_command',
      ),
    ).toBe('$ curl -H Bearer [redacted-secret] https://x');
  });

  it('redacts an uppercase ENV-var assignment', () => {
    expect(
      commandPreview(
        JSON.stringify({ command: 'API_KEY=secret1234 curl https://x' }),
        'pulse_run_command',
      ),
    ).toBe('$ API_KEY=[redacted-secret] curl https://x');
  });

  it('redacts a lowercase password=<value> assignment', () => {
    expect(
      commandPreview(
        JSON.stringify({ command: 'curl -d password=hunter2222 https://x' }),
        'pulse_run_command',
      ),
    ).toBe('$ curl -d password=[redacted-secret] https://x');
  });

  it('returns empty when formatShellCommandPreview receives an empty preview', () => {
    // A whitespace-only command normalizes to "" inside commandPreviewValue,
    // which formatShellCommandPreview maps to "" (falsy ternary arm).
    expect(commandPreview(JSON.stringify({ command: '   ' }), 'pulse_run_command')).toBe('');
  });
});

// ---------------------------------------------------------------------------
// shellCommandIntentLabel — the sibling suite covers one command per family.
// These exercise the remaining regex alternatives inside each intent bucket so
// every alternation arm is hit at least once. All flow through read exec.
// ---------------------------------------------------------------------------

describe('shellCommandIntentLabel — remaining regex alternations', () => {
  const cases: Array<[string, string]> = [
    // Inspect devices: udevadm / lspci / lsusb / blkid / nvme list / /dev
    ['udevadm info /dev/sda', 'Inspect devices'],
    ['lspci -v', 'Inspect devices'],
    ['lsusb', 'Inspect devices'],
    ['blkid', 'Inspect devices'],
    ['nvme list', 'Inspect devices'],
    ['cat /dev/sda', 'Inspect devices'],
    // Check disk health: nvme smart-log / storcli / megacli.
    // (Note: a literal /dev path here would be shadowed by the device regex's
    // /dev alternative — see SUSPECTED SOURCE BUGS in GLM_REPORT.md — so these
    // omit /dev to actually reach the disk-health alternation.)
    ['nvme smart-log nvme0', 'Check disk health'],
    ['storcli /c0 show', 'Check disk health'],
    ['megacli -AdpAllInfo -a0', 'Check disk health'],
    // Inspect filesystems: findmnt / mount / ls /mnt|media
    ['findmnt /', 'Inspect filesystems'],
    ['mount /mnt/data', 'Inspect filesystems'],
    ['ls /mnt/data', 'Inspect filesystems'],
    ['ls /media/cdrom', 'Inspect filesystems'],
    // Inspect ZFS storage: bare `zfs`
    ['zfs list', 'Inspect ZFS storage'],
    // Read logs: docker logs / kubectl logs / tail -f /var/log / /var/log
    ['docker logs nginx', 'Read logs'],
    ['kubectl logs api', 'Read logs'],
    ['tail -f /var/log/syslog', 'Read logs'],
    ['grep ERROR /var/log/syslog', 'Read logs'],
    // Check service status: is-active / show / service ... status
    ['systemctl is-active nginx', 'Check service status'],
    ['systemctl show nginx', 'Check service status'],
    ['service nginx status', 'Check service status'],
    // Inspect containers: docker inspect|stats / podman
    ['docker inspect web', 'Inspect containers'],
    ['docker stats', 'Inspect containers'],
    ['podman ps', 'Inspect containers'],
    ['podman stats', 'Inspect containers'],
    // Inspect Kubernetes: describe|top / helm
    ['kubectl describe pod', 'Inspect Kubernetes resources'],
    ['kubectl top nodes', 'Inspect Kubernetes resources'],
    ['helm list', 'Inspect Kubernetes resources'],
    ['helm status release', 'Inspect Kubernetes resources'],
    // Inspect Proxmox: pct / qm variants / pvesh / pvesm
    ['pct list', 'Inspect Proxmox resources'],
    ['qm status 101', 'Inspect Proxmox resources'],
    ['qm config 101', 'Inspect Proxmox resources'],
    ['qm pending 101', 'Inspect Proxmox resources'],
    ['pvesh get nodes', 'Inspect Proxmox resources'],
    ['pvesm status', 'Inspect Proxmox resources'],
    // Check network state: ss / netstat / traceroute / curl / wget / dig / nslookup
    ['ss -tlnp', 'Check network state'],
    ['netstat -an', 'Check network state'],
    ['traceroute 8.8.8.8', 'Check network state'],
    ['curl http://example.com', 'Check network state'],
    ['wget http://example.com', 'Check network state'],
    ['dig example.com', 'Check network state'],
    ['nslookup example.com', 'Check network state'],
  ];

  it.each(cases)('labels %s as %j', (command, expected) => {
    expect(readSummary({ action: 'exec', command })).toBe(expected);
  });

  it('returns no intent for an unrecognized command, yielding the generic read-only label', () => {
    // shellCommandIntentLabel returns "" → formatCommandActivitySummary falls
    // through to "Run read-only command".
    expect(readSummary({ action: 'exec', command: 'uname -a' })).toBe('Run read-only command');
  });
});

// ---------------------------------------------------------------------------
// parseStructuredInputRecord — JSON.parse succeeds but yields a non-record
// (null / string), so the `parsed && typeof parsed === 'object'` guard is
// false and parsing falls through to the (also failing) function-call path.
// ---------------------------------------------------------------------------

describe('parseStructuredInputRecord — non-record JSON fall-through', () => {
  it('treats a JSON null literal as a non-record and falls back to the label', () => {
    // JSON.parse('null') === null → guard false → no function-call match → null.
    expect(parseToolInputSummary('null', 'pulse_read')).toBe('null');
  });

  it('treats a JSON string scalar as a non-record and falls back to the label', () => {
    // JSON.parse('"hello"') === 'hello' → typeof string → guard false → null.
    expect(parseToolInputSummary('"hello"', 'pulse_read')).toBe('"hello"');
  });
});

// ---------------------------------------------------------------------------
// partialCommandPreview — the sibling suite only covers run_command for the
// command-tool guard. These exercise the read and control arms of that guard
// plus a backslash-escaped partial value flowing through the helper.
// ---------------------------------------------------------------------------

describe('partialCommandPreview — read/control guard arms', () => {
  it('extracts a partial command for the read tool', () => {
    expect(commandPreview('{"command":"df', 'pulse_read')).toBe('$ df');
  });

  it('extracts a partial command for the control tool', () => {
    expect(commandPreview('{"command":"df', 'pulse_control')).toBe('$ df');
  });

  it('still returns empty for a non-command tool even when a command is present', () => {
    expect(commandPreview('{"command":"df"', 'pulse_query')).toBe('');
  });

  it('returns empty when the partial input has no command or cmd field', () => {
    expect(commandPreview('{"foo":"bar"', 'pulse_run_command')).toBe('');
  });

  it('uses the raw input when the trimmed input is empty', () => {
    expect(commandPreview('   ', 'pulse_run_command', '{"command":"uptime"}')).toBe('$ uptime');
  });
});
