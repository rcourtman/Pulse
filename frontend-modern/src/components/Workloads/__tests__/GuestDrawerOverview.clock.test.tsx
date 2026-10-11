import { cleanup, render, screen } from '@solidjs/testing-library';
import type { ComponentProps } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Alert } from '@/types/api';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { METRIC_ALERT_STATUS_STALE_MS } from '@/features/alerts/metricAlertPresentation';
import { GuestDrawerOverview } from '../GuestDrawerOverview';

const start = Date.parse('2026-10-09T20:00:00Z');
const alert = {
  id: 'cpu-held',
  resourceId: 'lab:pve-a:101',
  resourceName: 'held-guest',
  type: 'cpu',
  level: 'warning',
  message: 'CPU above threshold',
  value: 92,
  threshold: 80,
  startTime: new Date(start - 60_000).toISOString(),
  acknowledged: false,
  node: 'pve-a',
  metricStatus: {
    phase: 'breaching',
    value: 91,
    unit: '%',
    observedAt: new Date(start - METRIC_ALERT_STATUS_STALE_MS + 5_000).toISOString(),
    trigger: 80,
    recovery: 75,
  },
} as Alert;
const props = {
  guest: {
    id: 'lab:pve-a:101',
    name: 'held-guest',
    vmid: 101,
    instance: 'lab',
    node: 'pve-a',
    type: 'qemu',
    status: 'running',
    memory: { total: 1024, used: 256, free: 768, usage: 25 },
    disk: { total: 1024, used: 256, usage: 25 },
    tags: [],
    cpu: 0.1,
    cpus: 1,
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    lock: '',
    lastSeen: new Date(start).toISOString(),
  },
  guestOsSummary: '',
  agentHeading: '',
  agentLabel: '',
  agentTitle: '',
  hasAgentInfo: false,
  hasFilesystemDetails: false,
  hasNetworkInterfaces: false,
  hasOsInfo: false,
  hasWorkloadActionAgent: false,
  showInGuestAgentInstallCue: false,
  ipAddresses: [],
  networkInterfaces: [],
  normalizedTags: [],
  backupPresentation: null,
  workloadActionAgentTitle: '',
  alerts: [alert],
} as ComponentProps<typeof GuestDrawerOverview>;

describe('mounted guest drawer attention freshness', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(start);
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });
  it('stops describing an old alert evaluation as now without changing the guest or alert', () => {
    render(() => <GuestDrawerOverview {...props} />);
    expect(screen.getByText('CPU 91%, above the 80% alert level')).toBeInTheDocument();
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(screen.queryByText('CPU 91%, above the 80% alert level')).not.toBeInTheDocument();
    expect(screen.getByText('Last reading: CPU 91%, 10 mins ago')).toBeInTheDocument();
    vi.advanceTimersByTime(60_000);
    expect(screen.getByText('Last reading: CPU 91%, 11 mins ago')).toBeInTheDocument();
  });
});
