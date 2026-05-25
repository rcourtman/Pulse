import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import type { Resource } from '@/types/resource';
import commandPaletteModalSource from '@/components/shared/CommandPaletteModal.tsx?raw';
import commandPaletteModelSource from '@/components/shared/commandPaletteModel.ts?raw';
import commandPaletteStateSource from '@/components/shared/useCommandPaletteState.ts?raw';
import { buildPrimaryInfrastructureNavigationVisibility } from '@/features/infrastructureNavigation/infrastructureNavigationModel';

const navigateMock = vi.fn();

vi.mock('@solidjs/router', () => ({
  useNavigate: () => navigateMock,
}));

vi.mock('@/components/shared/Dialog', () => ({
  Dialog: (props: { isOpen: boolean; children: JSX.Element }) =>
    props.isOpen ? <div>{props.children}</div> : null,
}));

import { CommandPaletteModal } from '@/components/shared/CommandPaletteModal';

const makeResource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'resource-1',
    name: overrides.name ?? overrides.id ?? 'resource-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'resource-1',
    type: overrides.type ?? 'agent',
    platformId: overrides.platformId ?? 'platform-1',
    platformType: overrides.platformType ?? 'agent',
    sourceType: overrides.sourceType ?? 'api',
    status: overrides.status ?? 'online',
    lastSeen: overrides.lastSeen ?? 1_700_000_000_000,
    ...overrides,
  }) as Resource;

const infrastructureVisibility = () =>
  buildPrimaryInfrastructureNavigationVisibility([
    makeResource({ id: 'agent-1', type: 'agent', platformType: 'agent' }),
    makeResource({ id: 'pve-1', type: 'agent', platformType: 'proxmox-pve' }),
    makeResource({ id: 'docker-1', type: 'docker-host', platformType: 'docker' }),
    makeResource({ id: 'k8s-1', type: 'k8s-cluster', platformType: 'kubernetes' }),
    makeResource({ id: 'vc-1', type: 'network', platformType: 'vmware-vsphere' }),
  ]);

describe('CommandPaletteModal', () => {
  afterEach(() => {
    cleanup();
    navigateMock.mockReset();
  });

  it('keeps the command palette on shell, runtime, and model owners', () => {
    expect(commandPaletteModalSource).toContain('useCommandPaletteState');
    expect(commandPaletteModalSource).not.toContain('useNavigate');
    expect(commandPaletteModalSource).not.toContain('createSignal');
    expect(commandPaletteModalSource).not.toContain('buildInfrastructurePath');

    expect(commandPaletteStateSource).toContain('useNavigate');
    expect(commandPaletteStateSource).toContain('createSignal');
    expect(commandPaletteStateSource).toContain('buildStandalonePath');
    expect(commandPaletteStateSource).toContain('buildProxmoxPath');
    expect(commandPaletteStateSource).toContain('export function useCommandPaletteState');

    // Infrastructure and aggregate workspaces are not command-palette routes;
    // platform/runtime pages own those workflows.
    expect(commandPaletteStateSource).not.toContain('buildInfrastructurePath');
    expect(commandPaletteStateSource).not.toContain('buildWorkloadsPath');
    expect(commandPaletteStateSource).not.toContain('buildStoragePath');
    expect(commandPaletteStateSource).not.toContain('buildRecoveryPath');

    expect(commandPaletteModelSource).toContain('buildCommandPaletteCommands');
    expect(commandPaletteModelSource).toContain('normalizeCommandPaletteQuery');
    expect(commandPaletteModelSource).toContain('filterCommandPaletteCommands');
    expect(commandPaletteModelSource).toContain("id: 'nav-standalone'");
    expect(commandPaletteModelSource).toContain("id: 'nav-proxmox'");
    expect(commandPaletteModelSource).toContain("id: 'nav-docker'");
    expect(commandPaletteModelSource).toContain("id: 'nav-kubernetes'");
    expect(commandPaletteModelSource).toContain("id: 'nav-kubernetes-workloads'");
    expect(commandPaletteModelSource).toContain("id: 'nav-truenas'");
    expect(commandPaletteModelSource).toContain("id: 'nav-vmware'");
    expect(commandPaletteModelSource).toContain("id: 'nav-vmware-networks'");
    expect(commandPaletteModelSource).not.toContain("id: 'nav-infrastructure'");
    expect(commandPaletteModelSource).not.toContain("id: 'nav-workloads'");
    expect(commandPaletteModelSource).not.toContain("id: 'nav-storage'");
    expect(commandPaletteModelSource).not.toContain("id: 'nav-recovery'");
  });

  it('renders platform entries, runtime lens commands, and vSphere network inventory', () => {
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={vi.fn()}
        infrastructureVisibility={infrastructureVisibility}
      />
    ));

    expect(screen.getByText('Go to Standalone')).toBeInTheDocument();
    expect(screen.getByText('/standalone/machines')).toBeInTheDocument();
    expect(screen.getByText('Go to Proxmox')).toBeInTheDocument();
    expect(screen.getByText('Go to Containers')).toBeInTheDocument();
    expect(screen.getByText('Go to Kubernetes Workloads')).toBeInTheDocument();
    expect(screen.getByText('/kubernetes/workloads')).toBeInTheDocument();
    expect(screen.queryByText('Go to Workloads')).not.toBeInTheDocument();
    expect(screen.queryByText('/workloads')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Storage')).not.toBeInTheDocument();
    expect(screen.queryByText('/storage')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Recovery')).not.toBeInTheDocument();
    expect(screen.queryByText('/recovery')).not.toBeInTheDocument();
    expect(screen.getByText('Go to vSphere')).toBeInTheDocument();
    expect(screen.getByText('Go to vSphere Networks')).toBeInTheDocument();
    expect(screen.getByText('/vmware/networks')).toBeInTheDocument();

    const commandLabels = screen.getAllByRole('button').map((button) => button.textContent ?? '');
    expect(commandLabels.findIndex((label) => label.includes('Go to Proxmox'))).toBeLessThan(
      commandLabels.findIndex((label) => label.includes('Go to Standalone')),
    );
  });

  it('navigates to the Kubernetes workloads tab', async () => {
    const onClose = vi.fn();
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={onClose}
        infrastructureVisibility={infrastructureVisibility}
      />
    ));

    await fireEvent.click(screen.getByText('Go to Kubernetes Workloads'));

    expect(navigateMock).toHaveBeenCalledWith('/kubernetes/workloads');
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('navigates to the vSphere networks sub-tab', async () => {
    const onClose = vi.fn();
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={onClose}
        infrastructureVisibility={infrastructureVisibility}
      />
    ));

    await fireEvent.click(screen.getByText('Go to vSphere Networks'));

    expect(navigateMock).toHaveBeenCalledWith('/vmware/networks');
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('uses the shared search input and keeps Enter selection behavior', async () => {
    const onClose = vi.fn();
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={onClose}
        infrastructureVisibility={infrastructureVisibility}
      />
    ));

    const input = screen.getByPlaceholderText('Type a command or search...');
    await fireEvent.input(input, { target: { value: 'kubernetes pods' } });
    await fireEvent.keyDown(input, { key: 'Enter' });

    expect(navigateMock).toHaveBeenCalledWith('/kubernetes/workloads');
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('hides platform commands without supported infrastructure evidence', () => {
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={vi.fn()}
        infrastructureVisibility={() =>
          buildPrimaryInfrastructureNavigationVisibility([
            makeResource({ id: 'pve-1', type: 'agent', platformType: 'proxmox-pve' }),
          ])
        }
      />
    ));

    expect(screen.getByText('Go to Proxmox')).toBeInTheDocument();
    expect(screen.queryByText('Go to Workloads')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Storage')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Recovery')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Standalone')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Containers')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Kubernetes')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to TrueNAS')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to vSphere')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to vSphere Networks')).not.toBeInTheDocument();
  });
});
