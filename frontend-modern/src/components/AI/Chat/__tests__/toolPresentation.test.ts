import { describe, expect, it } from 'vitest';
import { parseToolInputSummary } from '../toolPresentation';

const readSummary = (record: Record<string, unknown>) =>
  parseToolInputSummary(JSON.stringify(record), 'pulse_read');
const querySummary = (record: Record<string, unknown>) =>
  parseToolInputSummary(JSON.stringify(record), 'pulse_query');
const runCommandSummary = (record: Record<string, unknown>) =>
  parseToolInputSummary(JSON.stringify(record), 'pulse_run_command');
const controlSummary = (record: Record<string, unknown>) =>
  parseToolInputSummary(JSON.stringify(record), 'pulse_control');

describe('formatPulseReadInputSummary (read tool)', () => {
  it('summarizes exec with a command via its read intent and falls back when command is absent', () => {
    expect(readSummary({ action: 'exec', command: 'df -h' })).toBe('Inspect filesystems');
    expect(readSummary({ action: 'exec' })).toBe('run read-only command');
  });

  it('summarizes file with and without a path', () => {
    expect(readSummary({ action: 'file', path: '/etc/hosts' })).toBe('read /etc/hosts');
    expect(readSummary({ action: 'file' })).toBe('read file');
  });

  it('appends a target suffix when target_host is present', () => {
    expect(readSummary({ action: 'file', path: '/etc/hosts', target_host: 'web-01' })).toBe(
      'read /etc/hosts on web-01',
    );
  });

  it('summarizes tail with and without a path', () => {
    expect(readSummary({ action: 'tail', path: '/var/log/syslog' })).toBe(
      'tail /var/log/syslog',
    );
    expect(readSummary({ action: 'tail' })).toBe('tail file');
  });

  it('summarizes find with pattern+path, pattern only, and neither', () => {
    expect(readSummary({ action: 'find', pattern: 'ERROR', path: '/var/log' })).toBe(
      'find "ERROR" in /var/log',
    );
    expect(readSummary({ action: 'find', pattern: 'OOM' })).toBe('find "OOM"');
    expect(readSummary({ action: 'find' })).toBe('find files');
  });

  it('summarizes logs by container, then source, then a generic fallback', () => {
    expect(readSummary({ action: 'logs', container: 'nginx' })).toBe('logs nginx');
    expect(readSummary({ action: 'logs', source: 'journald' })).toBe('journald logs');
    expect(readSummary({ action: 'logs' })).toBe('read logs');
  });

  it('uses the type field as an action alias and labels unrecognized actions', () => {
    expect(readSummary({ type: 'file', path: '/etc/hosts' })).toBe('read /etc/hosts');
    expect(readSummary({ action: 'stat' })).toBe('stat');
  });
});

describe('formatQueryInputSummary (query tool)', () => {
  it('summarizes search with and without a query term', () => {
    expect(querySummary({ action: 'search', query: 'web-101' })).toBe('search "web-101"');
    expect(querySummary({ action: 'search' })).toBe('search resources');
  });

  it('summarizes list with a type, a resource_type alias, and no type', () => {
    expect(querySummary({ action: 'list', type: 'vm' })).toBe('list vm');
    expect(querySummary({ action: 'list', resource_type: 'storage_pool' })).toBe(
      'list storage pool',
    );
    expect(querySummary({ action: 'list' })).toBe('list resources');
  });

  it('summarizes get with and without a resource id', () => {
    expect(querySummary({ action: 'get', resource_id: 'vm:101' })).toBe('get vm:101');
    expect(querySummary({ action: 'get' })).toBe('get resource');
  });

  it('summarizes config with id+node, id only, and neither', () => {
    expect(querySummary({ action: 'config', resource_id: 'vm:101', node: 'pve01' })).toBe(
      'config vm:101 on pve01',
    );
    expect(querySummary({ action: 'config', resource_id: 'vm:101' })).toBe('config vm:101');
    expect(querySummary({ action: 'config' })).toBe('resource config');
  });

  it('summarizes topology honoring summary_only and health as a fixed label', () => {
    expect(querySummary({ action: 'topology' })).toBe('topology');
    expect(querySummary({ action: 'topology', summary_only: true })).toBe('topology summary');
    expect(querySummary({ action: 'topology', summary_only: false })).toBe('topology');
    expect(querySummary({ action: 'health' })).toBe('health summary');
  });

  it('labels unknown actions and falls back to resource-type then a generic query label', () => {
    expect(querySummary({ action: 'custom' })).toBe('custom');
    expect(querySummary({ resource_type: 'vm' })).toBe('query vm');
    expect(querySummary({ foo: 'bar' })).toBe('query resources');
  });
});

describe('formatStructuredInputSummary mode split (read vs run_command vs control)', () => {
  it('routes run_command and control to write mode, producing "Run command"', () => {
    expect(runCommandSummary({ command: 'systemctl restart nginx' })).toBe('Run command');
    expect(controlSummary({ command: 'reboot', target_host: 'node-1' })).toBe(
      'Run command on node-1',
    );
  });

  it('falls back to "run command" for write tools when no command is present', () => {
    expect(runCommandSummary({ foo: 'bar' })).toBe('run command');
    expect(controlSummary({ foo: 'bar' })).toBe('run command');
  });

  it('falls back to "read resource" when the read tool yields no specific summary', () => {
    expect(readSummary({ foo: 'bar' })).toBe('read resource');
  });

  it('splits the same command between read intent and write execution by tool', () => {
    expect(readSummary({ command: 'df -h' })).toBe('Inspect filesystems');
    expect(runCommandSummary({ command: 'df -h' })).toBe('Run command');
  });
});

describe('partialResult rescue during function-style input parsing', () => {
  it('recovers args parsed before a malformed token in a partial function call', () => {
    expect(parseToolInputSummary('read(action="file", 123)', 'pulse_read')).toBe('read file');
  });

  it('yields no structured summary when no args are recoverable', () => {
    expect(parseToolInputSummary('read(=', 'pulse_read')).toBe('read(=');
  });
});
