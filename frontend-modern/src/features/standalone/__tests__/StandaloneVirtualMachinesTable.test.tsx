import { Show } from 'solid-js';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { StandaloneVirtualMachinesTable } from '../StandaloneVirtualMachinesTable';

beforeAll(() => {
  if (typeof globalThis.ResizeObserver === 'undefined') {
    globalThis.ResizeObserver = class ResizeObserver {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
});

vi.mock('@/features/platformPage/PlatformResourceDetailTableRow', async () => {
  const actual = await vi.importActual<
    typeof import('@/features/platformPage/PlatformResourceDetailTableRow')
  >('@/features/platformPage/PlatformResourceDetailTableRow');
  return {
    ...actual,
    PlatformResourceDetailTableRow: (props: {
      resource: Resource;
      open: boolean;
      detailRowId: string;
    }) => (
      <Show when={props.open}>
        <tr id={props.detailRowId} data-testid={`vm-drawer-${props.resource.id}`}>
          <td colSpan={7}>Drawer for {props.resource.name}</td>
        </tr>
      </Show>
    ),
  };
});

const makeResource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'vm-1',
    name: overrides.name ?? 'pulse-dev',
    displayName: overrides.displayName ?? overrides.name ?? 'pulse-dev',
    type: overrides.type ?? 'vm',
    platformId: overrides.platformId ?? 'agent',
    platformType: overrides.platformType ?? 'agent',
    sourceType: overrides.sourceType ?? 'agent',
    sources: overrides.sources ?? ['agent'],
    status: overrides.status ?? 'online',
    lastSeen: overrides.lastSeen ?? 1_700_000_000_000,
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('StandaloneVirtualMachinesTable', () => {
  const tower = makeResource({ id: 'agent:tower', name: 'Tower', type: 'agent' });
  const workstation = makeResource({ id: 'agent:box', name: 'LinuxBox', type: 'agent' });
  const pulseDev = makeResource({
    id: 'agent:tower:libvirt:domain-1',
    name: 'pulse-dev',
    parentId: 'agent:tower',
    parentName: 'Tower',
    technology: 'libvirt',
    virtualMachine: { runtimeState: 'running', hypervisor: 'libvirt', vcpus: 8 },
    cpu: { current: 14 },
    memory: { current: 25, used: 5 * 1024 ** 3, total: 20 * 1024 ** 3 },
    diskIO: { readRate: 1024 * 512, writeRate: 1024 * 256 },
    network: { rxBytes: 1024 * 128, txBytes: 1024 * 64 },
  });
  const win11 = makeResource({
    id: 'agent:box:libvirt:domain-2',
    name: 'win11',
    parentId: 'agent:box',
    parentName: 'LinuxBox',
    status: 'offline',
    technology: 'libvirt',
    virtualMachine: { runtimeState: 'shutoff', hypervisor: 'libvirt', vcpus: 4 },
    memory: { current: 0, used: 0, total: 16 * 1024 ** 3 },
  });

  it('renders VMs, narrows by host scope, clears host scope, and opens the VM detail drawer', () => {
    const onClearHostScope = vi.fn();
    render(() => (
      <StandaloneVirtualMachinesTable
        vms={[pulseDev, win11]}
        machines={[tower, workstation]}
        selectedHostId={() => 'agent:tower'}
        onClearHostScope={onClearHostScope}
        emptyIcon={<span />}
        emptyTitle="No virtual machines"
        emptyDescription="Empty"
      />
    ));

    expect(screen.getByText('Virtual Machines on Tower')).toBeInTheDocument();
    expect(screen.getByText('pulse-dev')).toBeInTheDocument();
    expect(screen.queryByText('win11')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Show all hosts' }));
    expect(onClearHostScope).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByText('pulse-dev'));
    expect(screen.getByTestId('vm-drawer-agent:tower:libvirt:domain-1')).toBeInTheDocument();
  });

  it('renders crashed and paused states cleanly without duplicate state text', () => {
    const crashed = makeResource({
      id: 'agent:box:libvirt:crashed',
      name: 'kvm-legacy-erp',
      parentId: 'agent:box',
      parentName: 'LinuxBox',
      status: 'degraded',
      technology: 'libvirt',
      virtualMachine: { runtimeState: 'crashed', hypervisor: 'libvirt', vcpus: 4 },
    });
    const paused = makeResource({
      id: 'agent:box:libvirt:paused',
      name: 'kvm-build-runner',
      parentId: 'agent:box',
      parentName: 'LinuxBox',
      status: 'degraded',
      technology: 'libvirt',
      virtualMachine: { runtimeState: 'paused', hypervisor: 'libvirt', vcpus: 4 },
    });

    render(() => (
      <StandaloneVirtualMachinesTable
        vms={[crashed, paused]}
        machines={[workstation]}
        emptyIcon={<span />}
        emptyTitle="No virtual machines"
        emptyDescription="Empty"
      />
    ));

    const crashedRow = document.querySelector(
      '[data-standalone-vm-row="agent:box:libvirt:crashed"]',
    );
    expect(crashedRow?.textContent).toContain('Crashed');
    expect(crashedRow?.textContent).not.toContain('Crashedcrashed');
    expect(crashedRow?.textContent).not.toContain('Crashed crashed');

    const pausedRow = document.querySelector('[data-standalone-vm-row="agent:box:libvirt:paused"]');
    expect(pausedRow?.textContent).toContain('Paused');
  });

  it('shows the empty state when no standalone VMs are reported', () => {
    render(() => (
      <StandaloneVirtualMachinesTable
        vms={[]}
        machines={[tower]}
        emptyIcon={<span />}
        emptyTitle="No virtual machines"
        emptyDescription="Libvirt and XCP-ng VMs appear here."
      />
    ));

    expect(screen.getByText('No virtual machines')).toBeInTheDocument();
  });
});
