import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal, type JSX } from 'solid-js';
import type { Resource } from '@/types/resource';
import commandPaletteModalSource from '@/components/shared/CommandPaletteModal.tsx?raw';
import commandPaletteModelSource from '@/components/shared/commandPaletteModel.ts?raw';
import commandPaletteStateSource from '@/components/shared/useCommandPaletteState.ts?raw';
import { buildPrimaryPlatformNavigationVisibility } from '@/features/platformNavigation/platformNavigationModel';
import { buildDockerPath } from '@/routing/resourceLinks';
import { aiAvailabilityStore } from '@/stores/aiAvailability';

const navigateMock = vi.fn();

vi.mock('@solidjs/router', () => ({
  useNavigate: () => navigateMock,
  useLocation: () => ({ pathname: window.location.pathname }),
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
    vi.restoreAllMocks();
    aiAvailabilityStore.setEnabled(false);
  });

  beforeEach(() => {
    window.history.replaceState({}, '', '/proxmox/overview');
    aiAvailabilityStore.setEnabled(true);
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
    expect(commandPaletteStateSource).not.toContain('aiChatStore');

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
    expect(commandPaletteModelSource).not.toContain("id: 'assistant-");
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
        platformVisibility={platformVisibility}
      />
    ));

    expect(screen.queryByText('Ask about Proxmox')).not.toBeInTheDocument();
    expect(screen.getByText('Go to Machines')).toBeInTheDocument();
    expect(screen.getByText('/standalone/machines')).toBeInTheDocument();
    expect(screen.getByText('Go to Proxmox')).toBeInTheDocument();
    expect(screen.getByText('Go to Docker')).toBeInTheDocument();
    expect(screen.getByText('Go to Kubernetes Workloads')).toBeInTheDocument();
    expect(screen.getByText('/kubernetes/workloads')).toBeInTheDocument();
    expect(screen.queryByText('Go to Workloads')).not.toBeInTheDocument();
    expect(screen.queryByText('/workloads')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Storage')).not.toBeInTheDocument();
    expect(screen.queryByText('/storage')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Recovery')).not.toBeInTheDocument();
    expect(screen.queryByText('/recovery')).not.toBeInTheDocument();
    expect(screen.getByText('Go to Patrol')).toBeInTheDocument();
    expect(screen.queryByText('Go to Needs Attention')).not.toBeInTheDocument();
    expect(screen.getByText('Go to vSphere')).toBeInTheDocument();
    expect(screen.getByText('Go to vSphere Networks')).toBeInTheDocument();
    expect(screen.getByText('/vmware/networks')).toBeInTheDocument();

    const commandLabels = screen.getAllByRole('option').map((button) => button.textContent ?? '');
    expect(commandLabels.findIndex((label) => label.includes('Go to Proxmox'))).toBe(0);
    expect(commandLabels.findIndex((label) => label.includes('Go to Proxmox'))).toBeLessThan(
      commandLabels.findIndex((label) => label.includes('Go to Machines')),
    );
  });

  it('moves the selected command before Enter selection', async () => {
    const onClose = vi.fn();
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={onClose}
        platformVisibility={platformVisibility}
      />
    ));

    const input = screen.getByPlaceholderText('Type a command or search...');
    const firstOption = screen.getByRole('option', { name: /Go to Proxmox/ });
    expect(input).toHaveAttribute('role', 'combobox');
    expect(input).toHaveAttribute('aria-autocomplete', 'list');
    expect(input).toHaveAttribute('aria-controls', 'command-palette-results');
    expect(input).toHaveAttribute('aria-expanded', 'true');
    expect(firstOption).toHaveAttribute('aria-selected', 'true');
    expect(firstOption).toHaveAttribute('tabindex', '-1');
    expect(input).toHaveAttribute('aria-activedescendant', firstOption.id);

    await fireEvent.keyDown(input, { key: 'ArrowDown' });

    const secondOption = screen.getByRole('option', { name: /Go to Docker/ });
    expect(secondOption).toHaveAttribute('aria-selected', 'true');
    expect(input).toHaveFocus();
    expect(input).toHaveAttribute('aria-activedescendant', secondOption.id);

    await fireEvent.keyDown(input, { key: 'Enter' });

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(navigateMock).toHaveBeenCalledWith(buildDockerPath());
  });

  it('keeps the active command visible while keyboard focus remains on the combobox', async () => {
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={vi.fn()}
        platformVisibility={platformVisibility}
      />
    ));

    const input = screen.getByRole('combobox', { name: 'Type a command or search...' });
    const secondOption = screen.getByRole('option', { name: /Go to Docker/ });
    const scrollIntoView = vi.fn();
    Object.defineProperty(secondOption, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    });

    input.focus();
    fireEvent.keyDown(input, { key: 'ArrowDown' });

    await waitFor(() => expect(scrollIntoView).toHaveBeenCalledWith({ block: 'nearest' }));
    expect(input).toHaveFocus();
  });

  it('clears active-option semantics when filtering produces no matches', async () => {
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={vi.fn()}
        platformVisibility={platformVisibility}
      />
    ));

    const input = screen.getByRole('combobox', { name: 'Type a command or search...' });
    fireEvent.input(input, { target: { value: 'definitely-no-command-matches-this' } });

    await waitFor(() => expect(screen.getByText('No matches found.')).toBeInTheDocument());
    expect(input).toHaveAttribute('aria-expanded', 'false');
    expect(input).not.toHaveAttribute('aria-activedescendant');
    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('keeps the selected command across command-list recomputes before Enter selection', async () => {
    const onClose = vi.fn();
    const [visibilityVersion, setVisibilityVersion] = createSignal(0);
    const dynamicPlatformVisibility = () => {
      visibilityVersion();
      return platformVisibility();
    };
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={onClose}
        platformVisibility={dynamicPlatformVisibility}
      />
    ));

    const input = screen.getByPlaceholderText('Type a command or search...');
    await fireEvent.keyDown(input, { key: 'ArrowDown' });

    expect(screen.getByRole('option', { name: /Go to Docker/ })).toHaveAttribute(
      'aria-selected',
      'true',
    );

    setVisibilityVersion(1);

    await waitFor(() => {
      expect(screen.getByRole('option', { name: /Go to Docker/ })).toHaveAttribute(
        'aria-selected',
        'true',
      );
    });

    await fireEvent.keyDown(input, { key: 'Enter' });

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(navigateMock).toHaveBeenCalledWith(buildDockerPath());
  });

  it('wraps command palette keyboard movement', async () => {
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={vi.fn()}
        platformVisibility={platformVisibility}
      />
    ));

    const input = screen.getByPlaceholderText('Type a command or search...');

    await fireEvent.keyDown(input, { key: 'ArrowUp' });

    expect(screen.getByRole('option', { name: /Go to Settings/ })).toHaveAttribute(
      'aria-selected',
      'true',
    );

    await fireEvent.keyDown(input, { key: 'Home' });

    expect(screen.getByRole('option', { name: /Go to Proxmox/ })).toHaveAttribute(
      'aria-selected',
      'true',
    );

    await fireEvent.keyDown(input, { key: 'End' });

    expect(screen.getByRole('option', { name: /Go to Settings/ })).toHaveAttribute(
      'aria-selected',
      'true',
    );
  });

  it('navigates to the Kubernetes workloads tab', async () => {
    const onClose = vi.fn();
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={onClose}
        platformVisibility={platformVisibility}
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
        platformVisibility={platformVisibility}
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
        platformVisibility={platformVisibility}
      />
    ));

    const input = screen.getByPlaceholderText('Type a command or search...');
    await fireEvent.input(input, { target: { value: 'kubernetes pods' } });
    await fireEvent.keyDown(input, { key: 'Enter' });

    expect(navigateMock).toHaveBeenCalledWith('/kubernetes/workloads');
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('does not offer Patrol while AI is off (#905)', () => {
    aiAvailabilityStore.setEnabled(false);
    render(() => (
      <CommandPaletteModal
        isOpen={true}
        onClose={vi.fn()}
        platformVisibility={platformVisibility}
      />
    ));

    expect(screen.getByText('Go to Alerts')).toBeInTheDocument();
    expect(screen.getByText('Go to Settings')).toBeInTheDocument();
    expect(screen.queryByText('Go to Patrol')).not.toBeInTheDocument();
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
    expect(screen.queryByText('Go to Workloads')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Storage')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Recovery')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Machines')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Docker')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to Kubernetes')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to TrueNAS')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to vSphere')).not.toBeInTheDocument();
    expect(screen.queryByText('Go to vSphere Networks')).not.toBeInTheDocument();
  });
});
