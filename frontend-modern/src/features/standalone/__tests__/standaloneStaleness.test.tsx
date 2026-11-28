import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createStore } from 'solid-js/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { AgentsMachinesTable } from '../AgentsMachinesTable';
import { StandaloneVirtualMachinesTable } from '../StandaloneVirtualMachinesTable';
import {
  describeStaleStandaloneMachines,
  describeStaleStandaloneVMs,
  getStandaloneMachineStaleness,
  getStandaloneMachineStatusIndicator,
  getStandaloneVMStaleness,
} from '../standaloneStaleness';
import { filterStandaloneVMs, mapStandaloneVMStatus } from '../standaloneVmTableModel';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: (props: { resource: Resource }) => (
    <div data-testid={`mock-drawer-${props.resource.id}`} />
  ),
}));

vi.mock('@/api/agentMetadata', () => ({
  AgentMetadataAPI: {
    getAllMetadata: vi.fn(async () => ({})),
    deleteMetadata: vi.fn(async () => undefined),
  },
}));

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: {
    deleteAgent: vi.fn(async () => undefined),
  },
}));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: {} as Record<string, Alert> }),
}));

const NOW = Date.parse('2026-11-28T05:00:00Z');

const makeMachine = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: overrides.id ?? 'agent:edge-nas',
    name: overrides.name ?? 'edge-nas',
    displayName: overrides.displayName ?? overrides.name ?? 'edge-nas',
    type: 'agent',
    platformId: 'agent',
    platformType: 'agent',
    sourceType: 'agent',
    sources: ['agent'],
    status: 'online',
    lastSeen: NOW - 15_000,
    cpu: { current: 31 },
    memory: { current: 54, used: 16 * 1024 ** 3, total: 32 * 1024 ** 3 },
    agent: {
      agentId: 'edge-nas',
      agentVersion: '6.10.0',
      osName: 'Ubuntu',
      osVersion: '24.04',
      stale: false,
      lastReportAt: new Date(NOW - 15_000).toISOString(),
    },
    ...overrides,
  }) as Resource;

const makeVm = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: overrides.id ?? 'agent:edge-nas:libvirt:kvm-api',
    name: overrides.name ?? 'kvm-api',
    displayName: overrides.displayName ?? overrides.name ?? 'kvm-api',
    parentId: overrides.parentId ?? 'agent:edge-nas',
    parentName: overrides.parentName ?? 'edge-nas',
    type: 'vm',
    platformId: 'agent',
    platformType: 'agent',
    sourceType: 'agent',
    sources: ['agent'],
    technology: 'libvirt',
    status: 'online',
    lastSeen: NOW - 15_000,
    virtualMachine: { runtimeState: 'running', hypervisor: 'libvirt', vcpus: 4 },
    ...overrides,
  }) as Resource;

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: NOW });
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe = vi.fn();
      unobserve = vi.fn();
      disconnect = vi.fn();
    },
  );
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('standaloneStaleness helpers', () => {
  it('detects backend telemetry_stale and sourceStatus.agent impairment even when agent.stale is false and lastSeen looks recent', () => {
    const backendStale = makeMachine({
      health: { verdict: 'stale', reasons: [{ code: 'telemetry_stale', detail: '16m' }] },
      sourceStatus: {
        agent: {
          status: 'stale',
          lastSeen: new Date(NOW - 16 * 60_000).toISOString(),
          error: 'agent heartbeat timed out',
        },
      },
      agent: {
        agentVersion: '6.10.0',
        stale: false,
        lastReportAt: new Date(NOW - 16 * 60_000).toISOString(),
      },
    });

    const notice = getStandaloneMachineStaleness(backendStale, NOW);
    expect(notice).not.toBeNull();
    expect(notice?.badge).toBe('Stale · 16m');
    expect(notice?.compactBadge).toBe('Stale');
    expect(notice?.detail).toContain('Agent has stopped reporting.');
    expect(notice?.detail).toContain('Last error: agent heartbeat timed out.');
    expect(getStandaloneMachineStatusIndicator(backendStale, NOW)).toEqual({
      variant: 'warning',
      label: 'Stale',
    });
  });

  it('does not mark offline or awaiting-first-report machines as stale', () => {
    const offline = makeMachine({
      status: 'offline',
      health: { verdict: 'stale', reasons: [{ code: 'telemetry_stale', detail: '20m' }] },
    });
    const pending = makeMachine({
      status: 'unknown',
      sourceStatus: { agent: { status: 'pending' } },
      agent: { stale: true },
    });

    expect(getStandaloneMachineStaleness(offline, NOW)).toBeNull();
    expect(getStandaloneMachineStaleness(pending, NOW)).toBeNull();
    expect(getStandaloneMachineStatusIndicator(offline, NOW).variant).toBe('danger');
  });

  it('propagates parent host staleness and VM telemetry_stale to standalone VMs', () => {
    const staleHost = makeMachine({
      id: 'agent:lab-unraid',
      name: 'lab-unraid',
      lastSeen: NOW - 14 * 60_000,
      agent: { stale: true, lastReportAt: new Date(NOW - 14 * 60_000).toISOString() },
    });
    const vmOnStaleHost = makeVm({
      id: 'agent:lab-unraid:libvirt:kvm-billing',
      name: 'kvm-billing',
      parentId: 'agent:lab-unraid',
      parentName: 'lab-unraid',
      sourceStatus: { agent: { status: 'online' } },
    });

    const notice = getStandaloneVMStaleness(vmOnStaleHost, staleHost, NOW);
    expect(notice?.badge).toBe('stale · 14m');
    expect(mapStandaloneVMStatus(vmOnStaleHost, staleHost, NOW)).toBe('attention');

    const machinesById = new Map([[staleHost.id, staleHost]]);
    expect(
      filterStandaloneVMs(
        [vmOnStaleHost],
        'stale',
        'attention',
        machinesById,
        undefined,
        undefined,
        NOW,
      ).map((vm) => vm.id),
    ).toEqual([vmOnStaleHost.id]);
  });

  it('formats deduplicated stale source banners for machines and VMs', () => {
    expect(
      describeStaleStandaloneMachines([
        { name: 'edge-nas', age: '16m' },
        { name: 'lab-unraid', age: '14m' },
      ]),
    ).toContain('edge-nas (16m) and lab-unraid (14m)');
    expect(
      describeStaleStandaloneVMs([
        { name: 'lab-unraid', age: '14m' },
        { name: 'lab-unraid', age: '14m' },
      ]),
    ).toContain('lab-unraid (14m)');
  });
});

describe('AgentsMachinesTable stale telemetry presentation', () => {
  it('surfaces backend-stale hosts in Degraded with banner, inline cue, searchable error, drawer notice, and clears on recovery', async () => {
    const [machines, setMachines] = createStore<Resource[]>([
      makeMachine({
        id: 'agent:edge-nas',
        name: 'edge-nas',
        health: { verdict: 'stale', reasons: [{ code: 'telemetry_stale', detail: '16m' }] },
        sourceStatus: {
          agent: {
            status: 'stale',
            lastSeen: new Date(NOW - 16 * 60_000).toISOString(),
            error: 'agent heartbeat timed out',
          },
        },
        agent: {
          agentVersion: '6.10.0',
          stale: false,
          lastReportAt: new Date(NOW - 16 * 60_000).toISOString(),
        },
      }),
      makeMachine({ id: 'agent:prod-worker', name: 'prod-worker' }),
    ]);

    render(() => (
      <AgentsMachinesTable
        resources={machines}
        emptyIcon={<span />}
        emptyTitle="No machines"
        emptyDescription="Empty"
      />
    ));

    expect(screen.getByTestId('agent-machines-stale-banner')).toHaveTextContent(
      /Telemetry is stale for edge-nas \(16m\)/,
    );
    expect(screen.getByRole('button', { name: 'Healthy, 1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Degraded, 1' })).toBeInTheDocument();
    expect(document.querySelectorAll('[data-agent-machine-stale-cue]')).toHaveLength(1);

    // Search matches stale error text
    const search = screen.getByPlaceholderText('Search machines');
    await fireEvent.input(search, { target: { value: 'heartbeat timed out' } });
    expect(document.querySelectorAll('[data-agents-machine-row]')).toHaveLength(1);
    await fireEvent.input(search, { target: { value: '' } });

    // Expanding stale machine shows inline drawer notice
    await fireEvent.click(document.querySelector('[data-agents-machine-row="agent:edge-nas"]')!);
    expect(screen.getByTestId('agent-machine-stale-notice')).toHaveTextContent(
      /Last error: agent heartbeat timed out\./,
    );

    // Live recovery clears banner, row cue, and drawer notice
    setMachines(0, {
      health: { verdict: 'ok', reasons: [] },
      sourceStatus: { agent: { status: 'online', lastSeen: new Date(NOW).toISOString() } },
      lastSeen: NOW,
      agent: { agentVersion: '6.10.0', stale: false, lastReportAt: new Date(NOW).toISOString() },
    });
    expect(screen.queryByTestId('agent-machines-stale-banner')).not.toBeInTheDocument();
    expect(screen.queryByTestId('agent-machine-stale-notice')).not.toBeInTheDocument();
    expect(document.querySelectorAll('[data-agent-machine-stale-cue]')).toHaveLength(0);
    expect(screen.getByRole('button', { name: 'Healthy, 2' })).toBeInTheDocument();
  });
});

describe('StandaloneVirtualMachinesTable stale telemetry presentation', () => {
  it('flags VMs on stale hosts with banner, Attention bucket, stale dot title, drawer notice, and clears on recovery', async () => {
    const [machines, setMachines] = createStore<Resource[]>([
      makeMachine({
        id: 'agent:lab-unraid',
        name: 'lab-unraid',
        lastSeen: NOW - 14 * 60_000,
        agent: { stale: true, lastReportAt: new Date(NOW - 14 * 60_000).toISOString() },
      }),
      makeMachine({ id: 'agent:prod-worker', name: 'prod-worker' }),
    ]);
    const vms = [
      makeVm({
        id: 'agent:lab-unraid:libvirt:kvm-billing',
        name: 'kvm-billing',
        parentId: 'agent:lab-unraid',
        parentName: 'lab-unraid',
      }),
      makeVm({
        id: 'agent:prod-worker:libvirt:kvm-fresh-db',
        name: 'kvm-fresh-db',
        parentId: 'agent:prod-worker',
        parentName: 'prod-worker',
      }),
    ];

    render(() => (
      <StandaloneVirtualMachinesTable
        vms={vms}
        machines={machines}
        emptyIcon={<span />}
        emptyTitle="No VMs"
        emptyDescription="Empty"
      />
    ));

    expect(screen.getByTestId('standalone-vms-stale-banner')).toHaveTextContent(
      /Host agent telemetry is stale for lab-unraid \(14m\)/,
    );
    expect(screen.getByRole('button', { name: 'Running, 1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Attention, 1' })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Running · Stale' })).toBeInTheDocument();

    // Expanding stale VM shows inline drawer notice
    await fireEvent.click(
      document.querySelector('[data-standalone-vm-row="agent:lab-unraid:libvirt:kvm-billing"]')!,
    );
    expect(screen.getByTestId('standalone-vm-stale-notice')).toHaveTextContent(
      /Host agent has not reported recently \(14m ago\)/,
    );

    // Parent host recovery clears banner and moves VM back to Running
    setMachines(0, {
      lastSeen: NOW,
      agent: { stale: false, lastReportAt: new Date(NOW).toISOString() },
    });
    expect(screen.queryByTestId('standalone-vms-stale-banner')).not.toBeInTheDocument();
    expect(screen.queryByTestId('standalone-vm-stale-notice')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Running, 2' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Attention, 0' })).toBeInTheDocument();
  });
});
