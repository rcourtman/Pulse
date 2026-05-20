import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import type { Resource } from '@/types/resource';
import commandPaletteModalSource from '@/components/shared/CommandPaletteModal.tsx?raw';
import commandPaletteModelSource from '@/components/shared/commandPaletteModel.ts?raw';
import commandPaletteStateSource from '@/components/shared/useCommandPaletteState.ts?raw';
import { buildPrimaryPlatformNavigationVisibility } from '@/features/platformNavigation/platformNavigationModel';

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

const platformVisibility = () =>
  buildPrimaryPlatformNavigationVisibility([
    makeResource({ id: 'pve-1', type: 'agent', platformType: 'proxmox-pve' }),
    makeResource({ id: 'docker-1', type: 'docker-host', platformType: 'docker' }),
    makeResource({ id: 'k8s-1', type: 'k8s-cluster', platformType: 'kubernetes' }),
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
    expect(commandPaletteStateSource).toContain('buildProxmoxPath');
    expect(commandPaletteStateSource).toContain('export function useCommandPaletteState');

    // Legacy navigation entries (Infrastructure / Workloads / Storage / Recovery)
    // were retired when primary nav moved to platform-first. The palette
    // exposes the platform pages directly instead.
    expect(commandPaletteStateSource).not.toContain('buildInfrastructurePath');
    expect(commandPaletteStateSource).not.toContain('buildWorkloadsPath');
    expect(commandPaletteStateSource).not.toContain('buildStoragePath');
    expect(commandPaletteStateSource).not.toContain('buildRecoveryPath');

    expect(commandPaletteModelSource).toContain('buildCommandPaletteCommands');
    expect(commandPaletteModelSource).toContain('normalizeCommandPaletteQuery');
    expect(commandPaletteModelSource).toContain('filterCommandPaletteCommands');
    expect(commandPaletteModelSource).toContain("id: 'nav-proxmox'");
    expect(commandPaletteModelSource).toContain("id: 'nav-docker'");
    expect(commandPaletteModelSource).toContain("id: 'nav-kubernetes'");
    expect(commandPaletteModelSource).toContain("id: 'nav-truenas'");
    expect(commandPaletteModelSource).toContain("id: 'nav-vmware'");
    expect(commandPaletteModelSource).not.toContain("id: 'nav-infrastructure'");
    expect(commandPaletteModelSource).not.toContain("id: 'nav-workloads'");
    expect(commandPaletteModelSource).not.toContain("id: 'nav-storage'");
    expect(commandPaletteModelSource).not.toContain("id: 'nav-recovery'");
  });

  it('renders the platform entries, container runtime lens, and dedicated Kubernetes pods command', () => {
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={vi.fn()}
        platformVisibility={platformVisibility}
      />
    ));

    expect(screen.getByText('Go to Proxmox')).toBeInTheDocument();
    expect(screen.getByText('Go to Containers')).toBeInTheDocument();
    expect(screen.getByText('Go to Kubernetes Pods')).toBeInTheDocument();
    expect(screen.getByText('/kubernetes/pods')).toBeInTheDocument();
  });

  it('navigates to the Kubernetes pods sub-tab', async () => {
    const onClose = vi.fn();
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={onClose}
        platformVisibility={platformVisibility}
      />
    ));

    await fireEvent.click(screen.getByText('Go to Kubernetes Pods'));

    expect(navigateMock).toHaveBeenCalledWith('/kubernetes/pods');
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('uses the shared search input and keeps Enter selection behavior', async () => {
    const onClose = vi.fn();
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={onClose}
        platformVisibility={platformVisibility}
      />
    ));

    const input = screen.getByPlaceholderText('Type a command or search...');
    await fireEvent.input(input, { target: { value: 'kubernetes pods' } });
    await fireEvent.keyDown(input, { key: 'Enter' });

    expect(navigateMock).toHaveBeenCalledWith('/kubernetes/pods');
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('hides platform commands without supported infrastructure evidence', () => {
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={vi.fn()}
        platformVisibility={() =>
          buildPrimaryPlatformNavigationVisibility([
            makeResource({ id: 'pve-1', type: 'agent', platformType: 'proxmox-pve' }),
          ])
        }
      />
    ));

    expect(screen.getByText('Go to Proxmox')).toBeInTheDocument();
    expect(screen.queryByText('Go to Containers')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Kubernetes')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to TrueNAS')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to vSphere')).not.toBeInTheDocument();
  });
});
