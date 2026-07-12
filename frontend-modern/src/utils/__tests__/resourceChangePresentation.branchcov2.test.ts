import { describe, expect, it } from 'vitest';

import {
  formatResourceChangeHeadline,
  formatResourceChangeKind,
  getResourceChangeKindPresentation,
  getResourceChangeSourceAdapterPresentation,
  getResourceChangeSourceTypePresentation,
} from '@/utils/resourceChangePresentation';
import type { ResourceChange } from '@/types/resource';

const makeChange = (overrides: Partial<ResourceChange> = {}): ResourceChange => ({
  id: 'change-1',
  observedAt: '2026-03-18T12:00:00Z',
  resourceId: 'vm:42',
  kind: 'activity',
  sourceType: 'platform_event',
  confidence: 'high',
  ...overrides,
});

describe('formatResourceChangeKind (branch coverage)', () => {
  it('returns the canonical label for the remaining named switch arms', () => {
    expect(formatResourceChangeKind('state_transition')).toBe('State transition');
    expect(formatResourceChangeKind('restart')).toBe('Restart');
    expect(formatResourceChangeKind('capability_change')).toBe('Capability change');
    expect(formatResourceChangeKind('alert_acknowledged')).toBe('Alert acknowledged');
    expect(formatResourceChangeKind('alert_unacknowledged')).toBe('Alert unacknowledged');
    expect(formatResourceChangeKind('alert_resolved')).toBe('Alert resolved');
    expect(formatResourceChangeKind('command_executed')).toBe('Command executed');
  });

  it('humanizes an unmapped kind through the default switch arm', () => {
    expect(
      formatResourceChangeKind(
        'custom_unmapped_kind' as unknown as Parameters<typeof formatResourceChangeKind>[0],
      ),
    ).toBe('Custom Unmapped Kind');
  });

  it('returns an empty string for an empty kind via the default arm fallback', () => {
    expect(
      formatResourceChangeKind(
        '' as unknown as Parameters<typeof formatResourceChangeKind>[0],
      ),
    ).toBe('');
  });
});

describe('getResourceChangeKindPresentation (branch coverage)', () => {
  it('returns the full strict shape for known kinds not already strictly asserted', () => {
    expect(getResourceChangeKindPresentation('state_transition')).toStrictEqual({
      label: 'State transition',
      plural: 'State transitions',
      className: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300',
    });
    expect(getResourceChangeKindPresentation('metric_anomaly')).toStrictEqual({
      label: 'Anomaly',
      plural: 'Anomalies',
      className: 'bg-rose-100 text-rose-700 dark:bg-rose-900 dark:text-rose-300',
    });
    expect(getResourceChangeKindPresentation('alert_unacknowledged')).toStrictEqual({
      label: 'Alert unacknowledged',
      plural: 'Alerts unacknowledged',
      className: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
    });
  });

  it('falls back to a humanized presentation for an unknown kind (?? branch)', () => {
    expect(getResourceChangeKindPresentation('totally_unknown_kind')).toStrictEqual({
      label: 'Totally Unknown Kind',
      plural: 'Totally Unknown Kinds',
      className: 'bg-surface text-muted',
    });
  });

  it('falls back to the Unknown sentinel for an empty kind string', () => {
    expect(getResourceChangeKindPresentation('')).toStrictEqual({
      label: 'Unknown',
      plural: 'Unknowns',
      className: 'bg-surface text-muted',
    });
  });
});

describe('getResourceChangeSourceTypePresentation (branch coverage)', () => {
  it('returns the full strict shape for every canonical source type', () => {
    expect(getResourceChangeSourceTypePresentation('platform_event')).toStrictEqual({
      label: 'Platform event',
      plural: 'Platform events',
      className: 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
    });
    expect(getResourceChangeSourceTypePresentation('pulse_diff')).toStrictEqual({
      label: 'Pulse diff',
      plural: 'Pulse diffs',
      className: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300',
    });
    expect(getResourceChangeSourceTypePresentation('heuristic')).toStrictEqual({
      label: 'Heuristic',
      plural: 'Heuristics',
      className: 'bg-rose-100 text-rose-700 dark:bg-rose-900 dark:text-rose-300',
    });
    expect(getResourceChangeSourceTypePresentation('user_action')).toStrictEqual({
      label: 'User action',
      plural: 'User actions',
      className: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
    });
    expect(getResourceChangeSourceTypePresentation('agent_action')).toStrictEqual({
      label: 'Agent action',
      plural: 'Agent actions',
      className: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
    });
  });

  it('falls back to a humanized presentation for an unknown source type (?? branch)', () => {
    expect(getResourceChangeSourceTypePresentation('mystery_source')).toStrictEqual({
      label: 'Mystery Source',
      plural: 'Mystery Sources',
      className: 'bg-surface text-muted',
    });
  });
});

describe('getResourceChangeSourceAdapterPresentation (branch coverage)', () => {
  it('returns the full strict shape for every canonical source adapter', () => {
    expect(getResourceChangeSourceAdapterPresentation('docker_adapter')).toStrictEqual({
      label: 'Docker adapter',
      plural: 'Docker adapters',
      className: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300',
    });
    expect(getResourceChangeSourceAdapterPresentation('proxmox_adapter')).toStrictEqual({
      label: 'Proxmox adapter',
      plural: 'Proxmox adapters',
      className: 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
    });
    expect(getResourceChangeSourceAdapterPresentation('truenas_adapter')).toStrictEqual({
      label: 'TrueNAS adapter',
      plural: 'TrueNAS adapters',
      className: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
    });
    expect(getResourceChangeSourceAdapterPresentation('vmware_adapter')).toStrictEqual({
      label: 'VMware adapter',
      plural: 'VMware adapters',
      className: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-300',
    });
    expect(getResourceChangeSourceAdapterPresentation('agent:ops-helper')).toStrictEqual({
      label: 'Ops helper',
      plural: 'Ops helpers',
      className: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
    });
  });

  it('falls back to a humanized presentation for an unknown source adapter (?? branch)', () => {
    expect(getResourceChangeSourceAdapterPresentation('synology_adapter')).toStrictEqual({
      label: 'Synology Adapter',
      plural: 'Synology Adapters',
      className: 'bg-surface text-muted',
    });
  });
});

describe('formatResourceChangeHeadline (branch coverage)', () => {
  it('uses the from/to format for a restart change with both endpoints', () => {
    expect(
      formatResourceChangeHeadline(makeChange({ kind: 'restart', from: 'stopped', to: 'running' })),
    ).toBe('Restart: stopped → running');
  });

  it('prefers the from/to format over reason when both endpoints are present', () => {
    expect(
      formatResourceChangeHeadline(
        makeChange({
          kind: 'state_transition',
          from: 'running',
          to: 'restarting',
          reason: 'should be ignored',
        }),
      ),
    ).toBe('State transition: running → restarting');
  });

  it('falls through to the reason branch when a state_transition is missing the "to" endpoint', () => {
    expect(
      formatResourceChangeHeadline(
        makeChange({ kind: 'state_transition', from: 'running', reason: 'Rebooted by scheduler' }),
      ),
    ).toBe('State transition: Rebooted by scheduler');
  });

  it('falls back to the resourceId when a restart has neither endpoints nor a reason', () => {
    expect(
      formatResourceChangeHeadline(makeChange({ kind: 'restart', resourceId: 'vm:99' })),
    ).toBe('Restart: vm:99');
  });

  it('trims surrounding whitespace from a reason before composing the headline', () => {
    expect(
      formatResourceChangeHeadline(
        makeChange({ kind: 'activity', reason: '  Deployed version 2.1  ' }),
      ),
    ).toBe('Activity: Deployed version 2.1');
  });

  it('returns the reason verbatim when it case-insensitively already starts with the kind label', () => {
    expect(
      formatResourceChangeHeadline(
        makeChange({ kind: 'activity', reason: 'ACTIVITY: deployed v3' }),
      ),
    ).toBe('ACTIVITY: deployed v3');
  });

  it('falls back to the resourceId when no reason and no endpoints are present', () => {
    expect(
      formatResourceChangeHeadline(
        makeChange({ kind: 'config_update', resourceId: 'storage:pool:data' }),
      ),
    ).toBe('Config update: storage:pool:data');
  });
});
