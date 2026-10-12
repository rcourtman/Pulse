import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal, type ComponentProps } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Alert } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { GuestDrawerOverview } from '../GuestDrawerOverview';

vi.mock('../GuestPhysicalDisks', () => ({ GuestPhysicalDisks: () => null }));
afterEach(cleanup);
const at = '2026-10-12T05:00:00Z';
const note = 'Alert percentages use guest allocation. The table row uses host capacity.';
const alert: Alert = {
  id: 'memory-alert',
  resourceId: 'lab:pve:101',
  resourceName: 'memory-guest',
  node: 'pve',
  instance: 'lab',
  type: 'memory',
  level: 'warning',
  message: 'VM memory at 86.6%',
  value: 86.6,
  threshold: 85,
  startTime: at,
  acknowledged: false,
};
const guest = (): WorkloadGuest => ({
  id: 'lab:pve:101',
  vmid: 101,
  name: 'memory-guest',
  node: 'pve',
  instance: 'lab',
  type: 'qemu',
  status: 'running',
  cpu: 0.1,
  cpus: 2,
  memory: {
    total: 4 * 1024 ** 3,
    used: 1024 ** 3,
    free: 3 * 1024 ** 3,
    usage: 25,
    observation: { state: 'current', source: 'agent', observedAt: at },
  },
  disk: { total: 1024, used: 512, usage: 50 },
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 3600,
  template: false,
  lastBackup: 0,
  tags: [],
  lock: '',
  lastSeen: at,
});
const props = (value: WorkloadGuest): ComponentProps<typeof GuestDrawerOverview> => ({
  guest: value,
  guestOsSummary: 'Linux',
  agentHeading: 'Guest agent',
  agentLabel: '',
  agentTitle: '',
  hasAgentInfo: false,
  hasFilesystemDetails: false,
  hasNetworkInterfaces: false,
  hasOsInfo: true,
  hasWorkloadActionAgent: false,
  showInGuestAgentInstallCue: false,
  ipAddresses: [],
  networkInterfaces: [],
  normalizedTags: [],
  backupPresentation: null,
  workloadActionAgentTitle: '',
});

describe('mounted guest drawer alert memory basis', () => {
  it('keeps alert-owned evidence when same-guest memory and capacity change independently', () => {
    const [value, setValue] = createSignal(guest());
    const [hostTotal, setHostTotal] = createSignal(8 * 1024 ** 3);
    const [basis, setBasis] = createSignal<'host' | 'guest'>('host');
    render(() => (
      <GuestDrawerOverview
        {...props(value())}
        parentMemoryTotal={hostTotal()}
        memoryDisplayBasis={basis()}
        alerts={[alert]}
      />
    ));
    const attention = screen.getByTestId('drawer-attention-section');
    expect(screen.getByText(alert.message)).toBeInTheDocument();
    expect(attention).toHaveTextContent(note);
    expect(attention).not.toHaveTextContent('13% of host capacity');
    for (const state of ['last-known', 'unavailable']) {
      setValue({
        ...value(),
        lastSeen: '2026-10-12T06:00:00Z',
        memory: {
          ...value().memory,
          used: 3 * 1024 ** 3,
          usage: 75,
          observation: { state, source: 'agent', observedAt: at },
        },
      });
      setHostTotal(16 * 1024 ** 3);
      expect(screen.getByText(alert.message)).toBeInTheDocument();
      expect(attention).toHaveTextContent(note);
      expect(attention).not.toHaveTextContent('19% of host capacity');
    }
    expect(screen.getByText('Memory reading').closest('tr')).toHaveTextContent('Unavailable');
    setBasis('guest');
    expect(attention).not.toHaveTextContent(note);
    expect(screen.getByText(alert.message)).toBeInTheDocument();
    setBasis('host');
    expect(attention).toHaveTextContent(note);
  });

  it('keeps live alert and clear-rule evidence separate from the row basis', () => {
    const evaluated: Alert = {
      ...alert,
      metricStatus: {
        phase: 'latched',
        value: 72,
        unit: '%',
        observedAt: new Date().toISOString(),
        trigger: 85,
        recovery: 70,
      },
    };
    const cpu = { ...alert, id: 'cpu-alert', type: 'cpu', message: 'CPU event stays unchanged' };
    render(() => (
      <GuestDrawerOverview
        {...props(guest())}
        parentMemoryTotal={8 * 1024 ** 3}
        memoryDisplayBasis="host"
        alerts={[evaluated, cpu]}
      />
    ));
    const attention = screen.getByTestId('drawer-attention-section');
    expect(attention).toHaveTextContent('Memory 72% now, back under the 85% alert level');
    expect(attention).toHaveTextContent(`Stays open until it reaches 70% or lower. ${note}`);
    expect(screen.getByText('CPU event stays unchanged')).toBeInTheDocument();
    expect(attention.textContent?.split(note)).toHaveLength(2);
  });
});
