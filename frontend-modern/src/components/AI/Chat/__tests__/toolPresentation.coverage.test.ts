import { describe, expect, it } from 'vitest';
import {
  parseToolCommandPreview,
  parseToolInputSummary,
  pendingToolActionLabel,
  pendingToolActionState,
  toolValueText,
} from '../toolPresentation';

const summary = (record: Record<string, unknown>, tool: string, rawInput?: string): string =>
  parseToolInputSummary(JSON.stringify(record), tool, rawInput);

const readSummary = (record: Record<string, unknown>): string => summary(record, 'pulse_read');
const alertsSummary = (record: Record<string, unknown>): string => summary(record, 'pulse_alerts');

const partialRaw = (tool: string, rawInput: string): string =>
  parseToolInputSummary('{}', tool, rawInput);

const commandPreview = (input: string, tool: string, rawInput?: string): string =>
  parseToolCommandPreview(input, tool, rawInput);

describe('pendingToolActionLabel', () => {
  it('maps every known tool to its in-progress verb, honoring pulse_ normalization', () => {
    expect(pendingToolActionLabel('pulse_run_command')).toBe('Writing command...');
    expect(pendingToolActionLabel('pulse_control')).toBe('Writing command...');
    expect(pendingToolActionLabel('pulse_read')).toBe('Preparing read...');
    expect(pendingToolActionLabel('pulse_query')).toBe('Preparing query...');
    expect(pendingToolActionLabel('pulse_fetch_url')).toBe('Fetching URL...');
    expect(pendingToolActionLabel('pulse_get_infrastructure_state')).toBe(
      'Reading infrastructure...',
    );
    expect(pendingToolActionLabel('pulse_get_active_alerts')).toBe('Reading alerts...');
    expect(pendingToolActionLabel('alerts')).toBe('Reading alerts...');
    expect(pendingToolActionLabel('pulse_get_metrics_history')).toBe('Reading metrics...');
    expect(pendingToolActionLabel('pulse_get_baselines')).toBe('Reading baselines...');
    expect(pendingToolActionLabel('pulse_get_patterns')).toBe('Reading patterns...');
    expect(pendingToolActionLabel('pulse_get_disk_health')).toBe('Checking disks...');
    expect(pendingToolActionLabel('pulse_get_storage')).toBe('Reading storage...');
    expect(pendingToolActionLabel('pulse_get_storage_config')).toBe('Reading storage...');
    expect(pendingToolActionLabel('pulse_get_resource_details')).toBe('Reading resource...');
  });

  it('recognizes finding tools by substring and falls back for unknown tools', () => {
    expect(pendingToolActionLabel('pulse_finding_scan')).toBe('Preparing finding...');
    expect(pendingToolActionLabel('pulse_unknown_tool')).toBe('Preparing tool...');
  });

  it('returns the default for empty and undefined names and trims surrounding whitespace', () => {
    expect(pendingToolActionLabel(undefined)).toBe('Preparing tool...');
    expect(pendingToolActionLabel('')).toBe('Preparing tool...');
    expect(pendingToolActionLabel('   ')).toBe('Preparing tool...');
    expect(pendingToolActionLabel('  pulse_query  ')).toBe('Preparing query...');
  });
});

describe('pendingToolActionState', () => {
  it('classifies write, prepare, fetch, and check tools distinctly', () => {
    expect(pendingToolActionState('pulse_run_command')).toBe('writing');
    expect(pendingToolActionState('pulse_control')).toBe('writing');
    expect(pendingToolActionState('pulse_query')).toBe('preparing');
    expect(pendingToolActionState('pulse_fetch_url')).toBe('fetching');
    expect(pendingToolActionState('pulse_get_disk_health')).toBe('checking');
  });

  it('classifies the reading tool family as reading', () => {
    expect(pendingToolActionState('pulse_read')).toBe('reading');
    expect(pendingToolActionState('pulse_get_infrastructure_state')).toBe('reading');
    expect(pendingToolActionState('pulse_get_active_alerts')).toBe('reading');
    expect(pendingToolActionState('alerts')).toBe('reading');
    expect(pendingToolActionState('pulse_get_metrics_history')).toBe('reading');
    expect(pendingToolActionState('pulse_get_baselines')).toBe('reading');
    expect(pendingToolActionState('pulse_get_patterns')).toBe('reading');
    expect(pendingToolActionState('pulse_get_storage')).toBe('reading');
    expect(pendingToolActionState('pulse_get_storage_config')).toBe('reading');
    expect(pendingToolActionState('pulse_get_resource_details')).toBe('reading');
  });

  it('falls back to preparing for unknown and undefined tool names', () => {
    expect(pendingToolActionState('pulse_unknown')).toBe('preparing');
    expect(pendingToolActionState(undefined)).toBe('preparing');
  });
});

describe('toolValueText', () => {
  it('returns strings as-is and empty for null or undefined', () => {
    expect(toolValueText('hello')).toBe('hello');
    expect(toolValueText('')).toBe('');
    expect(toolValueText(null)).toBe('');
    expect(toolValueText(undefined)).toBe('');
  });

  it('JSON-stringifies numbers, booleans, arrays, and objects', () => {
    expect(toolValueText(42)).toBe('42');
    expect(toolValueText(true)).toBe('true');
    expect(toolValueText([1, 2])).toBe('[1,2]');
    expect(toolValueText({ a: 1 })).toBe('{"a":1}');
  });

  it('falls back to String(value) when JSON.stringify throws on a circular reference', () => {
    const circular: Record<string, unknown> = {};
    circular.self = circular;
    expect(toolValueText(circular)).toBe('[object Object]');
  });
});

describe('formatAlertsInputSummary (alerts tool)', () => {
  it('lists alerts by severity, with no severity, and treats whitespace severity as absent', () => {
    expect(alertsSummary({ action: 'list', severity: 'critical' })).toBe('list critical alerts');
    expect(alertsSummary({ action: 'list' })).toBe('list active alerts');
    expect(alertsSummary({ action: 'list', severity: '   ' })).toBe('list active alerts');
  });

  it('coerces a numeric severity to a string label', () => {
    expect(alertsSummary({ action: 'list', severity: 5 })).toBe('list 5 alerts');
  });

  it('summarizes get with and without a resource, and labels other actions', () => {
    expect(alertsSummary({ action: 'get', resource: 'web-01' })).toBe('get alert for web-01');
    expect(alertsSummary({ action: 'get' })).toBe('get alert');
    expect(alertsSummary({ action: 'acknowledge' })).toBe('acknowledge');
  });

  it('reads alerts when no recognizable action is present', () => {
    expect(alertsSummary({ note: 'x' })).toBe('read alerts');
  });
});

describe('formatStructuredInputSummary routing arms', () => {
  it('returns request for an empty record with no recoverable raw input', () => {
    expect(parseToolInputSummary('{}', 'pulse_read')).toBe('request');
  });

  it('routes a non-special tool to an action label, a command label, or a generic request', () => {
    expect(summary({ action: 'reboot' }, 'pulse_custom')).toBe('reboot');
    expect(summary({ command: 'restart-nginx' }, 'pulse_custom')).toBe('restart-nginx');
    expect(summary({ foo: 'bar' }, 'pulse_custom')).toBe('request');
  });
});

describe('formatPartialRawInputSummary (empty record + raw input)', () => {
  it('summarizes read and write commands from partial raw JSON', () => {
    expect(partialRaw('pulse_read', '{"command":"df -h"}')).toBe('Inspect filesystems');
    expect(partialRaw('pulse_run_command', '{"command":"reboot"}')).toBe('Run command');
    expect(partialRaw('pulse_control', '{"command":"reboot"}')).toBe('Run command');
  });

  it('summarizes read actions for file, tail, find, and logs', () => {
    expect(partialRaw('pulse_read', '{"action":"file","path":"/etc/hosts"}')).toBe(
      'read /etc/hosts',
    );
    expect(partialRaw('pulse_read', '{"action":"tail","path":"/var/log/syslog"}')).toBe(
      'tail /var/log/syslog',
    );
    expect(partialRaw('pulse_read', '{"action":"find","pattern":"ERROR"}')).toBe('find "ERROR"');
    expect(partialRaw('pulse_read', '{"action":"logs"}')).toBe('read logs');
  });

  it('appends a target suffix for partial read file actions', () => {
    expect(
      partialRaw('pulse_read', '{"action":"file","path":"/etc/hosts","target_host":"web-01"}'),
    ).toBe('read /etc/hosts on web-01');
  });

  it('summarizes query actions for search, list, and other verbs', () => {
    expect(partialRaw('pulse_query', '{"action":"search","query":"web-01"}')).toBe(
      'search "web-01"',
    );
    expect(partialRaw('pulse_query', '{"action":"list"}')).toBe('list resources');
    expect(partialRaw('pulse_query', '{"action":"rebuild"}')).toBe('rebuild');
  });

  it('surfaces an action label for unrelated tools and reports receiving input otherwise', () => {
    expect(partialRaw('pulse_custom', '{"action":"reboot"}')).toBe('reboot');
    expect(partialRaw('pulse_custom', '{"note":"x"}')).toBe('receiving input');
  });
});

describe('shellCommandIntentLabel (read exec commands)', () => {
  it('labels device, disk, zfs, log, service, container, k8s, proxmox, and network intents', () => {
    expect(readSummary({ action: 'exec', command: 'lsblk' })).toBe('Inspect devices');
    expect(readSummary({ action: 'exec', command: 'smartctl --scan' })).toBe('Check disk health');
    expect(readSummary({ action: 'exec', command: 'zpool status' })).toBe('Inspect ZFS storage');
    expect(readSummary({ action: 'exec', command: 'journalctl -u nginx' })).toBe('Read logs');
    expect(readSummary({ action: 'exec', command: 'systemctl status nginx' })).toBe(
      'Check service status',
    );
    expect(readSummary({ action: 'exec', command: 'docker ps' })).toBe('Inspect containers');
    expect(readSummary({ action: 'exec', command: 'kubectl get pods' })).toBe(
      'Inspect Kubernetes resources',
    );
    expect(readSummary({ action: 'exec', command: 'qm list' })).toBe('Inspect Proxmox resources');
    expect(readSummary({ action: 'exec', command: 'ping 10.0.0.1' })).toBe('Check network state');
  });

  it('appends a target suffix to a read intent', () => {
    expect(readSummary({ action: 'exec', command: 'lsblk', target_host: 'web-01' })).toBe(
      'Inspect devices on web-01',
    );
  });

  it('falls back to a generic read-only label for unrecognized commands', () => {
    expect(readSummary({ action: 'exec', command: 'echo hello' })).toBe('Run read-only command');
  });
});

describe('parseFunctionStyleToolInput', () => {
  it('parses a well-formed multi-argument call with quoted string values', () => {
    expect(parseToolInputSummary('read(action="file", path="/etc/hosts")', 'pulse_read')).toBe(
      'read /etc/hosts',
    );
  });

  it('types unquoted scalar values as boolean, number, null, and bareword', () => {
    expect(parseToolInputSummary('query(action=topology, summary_only=true)', 'pulse_query')).toBe(
      'topology summary',
    );
    expect(parseToolInputSummary('query(action=topology, summary_only=false)', 'pulse_query')).toBe(
      'topology',
    );
    expect(parseToolInputSummary('query(action=get, resource_id=101)', 'pulse_query')).toBe(
      'get 101',
    );
    expect(parseToolInputSummary('query(action=get, resource_id=vm101)', 'pulse_query')).toBe(
      'get vm101',
    );
    expect(parseToolInputSummary('query(action=list, type=null)', 'pulse_query')).toBe(
      'list resources',
    );
    expect(parseToolInputSummary('query(action=list, type=vm)', 'pulse_query')).toBe('list vm');
  });

  it('returns null for input without a function-call shape so the caller echoes the label', () => {
    expect(parseToolInputSummary('plain text', 'pulse_read')).toBe('plain text');
  });

  it('yields no structured result when the body has no parseable key', () => {
    expect(parseToolInputSummary('read(123)', 'pulse_read')).toBe('read(123)');
    expect(parseToolInputSummary('read(', 'pulse_read')).toBe('read(');
  });

  it('skips a leading comma before the first argument', () => {
    expect(parseToolInputSummary('read(,action="file")', 'pulse_read')).toBe('read file');
  });
});

describe('parseQuotedValue escape handling', () => {
  it('unescapes embedded quotes inside a quoted command value', () => {
    expect(commandPreview('run_command(command="echo \\"hi\\"")', 'pulse_run_command')).toBe(
      '$ echo "hi"',
    );
  });

  it('rescues earlier args when a quote is left unterminated or a comma is missing', () => {
    expect(parseToolInputSummary('read(action="file" path="/x")', 'pulse_read')).toBe('read file');
    expect(parseToolInputSummary('read(action="file", path="unterminated)', 'pulse_read')).toBe(
      'read unterminated',
    );
  });

  it('stops cleanly at an early closing paren and preserves a trailing-backslash partial value', () => {
    expect(parseToolInputSummary('read(action="file"))', 'pulse_read')).toBe('read file');
    expect(commandPreview('run_command(command="df\\', 'pulse_run_command')).toBe('$ df');
  });

  it('treats a trailing backslash in a full call as a failed quote, rescued by the partial pass', () => {
    expect(parseToolInputSummary('read(action="abc\\)', 'pulse_read')).toBe('abc');
  });
});

describe('parseStructuredInputRecord', () => {
  it('rejects JSON arrays and primitives, then yields a label fallback', () => {
    expect(parseToolInputSummary('[1,2,3]', 'pulse_read')).toBe('[1,2,3]');
    expect(parseToolInputSummary('42', 'pulse_read')).toBe('42');
  });

  it('rescues a non-placeholder summary from raw input when the parsed input is a placeholder', () => {
    expect(
      parseToolInputSummary(JSON.stringify({ foo: 'bar' }), 'pulse_custom', '{"action":"reboot"}'),
    ).toBe('reboot');
  });
});

describe('commandPreviewValue', () => {
  it('returns an empty preview for a whitespace-only command', () => {
    expect(commandPreview(JSON.stringify({ command: '   ' }), 'pulse_run_command')).toBe('');
  });

  it('returns a short redacted command unchanged', () => {
    expect(commandPreview(JSON.stringify({ command: 'df -h' }), 'pulse_run_command')).toBe(
      '$ df -h',
    );
  });

  it('redacts bearer tokens wired through the preview', () => {
    const bearerToken = 'secret-token';
    expect(
      commandPreview(
        JSON.stringify({ command: `curl -H "Authorization: Bearer ${bearerToken}" https://x` }),
        'pulse_run_command',
      ),
    ).toBe('$ curl -H "Authorization: Bearer [redacted-secret]" https://x');
  });

  it('truncates a long command with an ellipsis', () => {
    const longCommand = `echo ${'x'.repeat(150)}`;
    const preview = commandPreview(JSON.stringify({ command: longCommand }), 'pulse_run_command');
    expect(preview).toMatch(/^\$ echo x{135}\.{3}$/);
  });
});

describe('partialCommandPreview', () => {
  it('returns empty for tools that are not command tools', () => {
    expect(commandPreview('{"command":"ls"}', 'pulse_query')).toBe('');
  });

  it('returns empty when no command field is present', () => {
    expect(commandPreview('{"foo":"bar"}', 'pulse_run_command')).toBe('');
  });

  it('extracts a command from partial JSON via the primary and alias keys', () => {
    expect(commandPreview('{"command":"df"', 'pulse_run_command')).toBe('$ df');
    expect(commandPreview('{"cmd":"ls"', 'pulse_run_command')).toBe('$ ls');
  });

  it('falls back to the raw input when the trimmed input is empty', () => {
    expect(commandPreview('', 'pulse_run_command', '{"command":"uptime"}')).toBe('$ uptime');
  });
});
