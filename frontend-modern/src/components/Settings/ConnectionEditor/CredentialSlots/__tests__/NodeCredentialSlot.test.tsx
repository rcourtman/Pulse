import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { InfrastructurePlatformSettingsProps } from '@/components/Settings/proxmoxSettingsModel';
import type { NodeConfigWithStatus } from '@/types/nodes';
import { NodeCredentialSlot } from '../NodeCredentialSlot';

const importApprovalRequiredMessage =
  'Approve the import plan before generating setup commands or adding this source.';

const createSettings = (): InfrastructurePlatformSettingsProps =>
  ({
    securityStatus: () => null,
    resolveTemperatureMonitoringEnabled: () => true,
    temperatureMonitoringLocked: () => false,
    savingTemperatureSetting: () => false,
    handleNodeTemperatureMonitoringChange: vi.fn().mockResolvedValue(undefined),
    handleTemperatureMonitoringChange: vi.fn().mockResolvedValue(undefined),
    saveNode: vi.fn().mockResolvedValue(true),
  }) as unknown as InfrastructurePlatformSettingsProps;

describe('NodeCredentialSlot', () => {
  afterEach(() => cleanup());

  it('requires candidate import plan approval before guided setup handoff or manual save', () => {
    render(() => (
      <NodeCredentialSlot
        nodeType="pve"
        settings={createSettings()}
        prefillNode={{
          type: 'pve',
          name: 'tower',
          host: 'https://tower.local:8006',
        }}
        importCandidate={{
          kind: 'discovery',
          server: {
            type: 'pve',
            ip: '10.0.0.10',
            hostname: 'tower.local',
            port: 8006,
            version: 'Proxmox VE',
            release: '9.0',
          },
        }}
        onCancel={vi.fn()}
        onSaved={vi.fn()}
      />
    ));

    expect(screen.getByText('Candidate import plan')).toBeInTheDocument();
    expect(screen.getByText('Proxmox VE 9.0')).toBeInTheDocument();

    const blockedSetupButton = screen.getAllByTitle(importApprovalRequiredMessage)[0];
    expect(blockedSetupButton).toBeDisabled();

    fireEvent.click(screen.getByLabelText(/Approve this import plan/i));

    expect(screen.getByTitle('Copy command')).not.toBeDisabled();

    fireEvent.click(screen.getByRole('button', { name: /^Manual Token Setup$/i }));

    const addButton = screen.getByRole('button', { name: 'Add node' });
    expect(addButton).toBeDisabled();

    fireEvent.click(screen.getByLabelText(/Approve this import plan/i));

    expect(addButton).not.toBeDisabled();
  });

  it('lets the user set a per-member connection address on an existing cluster', async () => {
    const settings = createSettings();
    const editingNode = {
      id: 'pve-0',
      type: 'pve',
      name: 'homelab',
      host: 'https://pve1.local:8006',
      user: '',
      tokenName: 'root@pam!pulse',
      hasToken: true,
      verifySSL: true,
      monitorVMs: true,
      monitorContainers: true,
      monitorStorage: true,
      monitorBackups: true,
      monitorPhysicalDisks: true,
      status: 'connected',
      isCluster: true,
      clusterName: 'homelab',
      clusterEndpoints: [
        {
          nodeId: 'node/pve1',
          nodeName: 'pve1',
          host: 'https://pve1.local:8006',
          ip: '10.0.0.1',
          online: true,
          lastSeen: '',
        },
        {
          nodeId: 'node/pve2',
          nodeName: 'pve2',
          host: 'https://pve2.local:8006',
          ip: '192.168.100.2',
          ipOverride: '10.0.0.2',
          online: true,
          lastSeen: '',
          pulseReachable: false,
          pulseError: 'connection refused',
        },
      ],
    } as unknown as NodeConfigWithStatus;

    render(() => (
      <NodeCredentialSlot
        nodeType="pve"
        settings={settings}
        editingNode={editingNode}
        onCancel={vi.fn()}
        onSaved={vi.fn()}
      />
    ));

    expect(screen.getByText('Cluster members')).toBeInTheDocument();
    expect(screen.getByText('Unreachable')).toBeInTheDocument();

    const pve1Input = screen.getByLabelText('Connection address for pve1') as HTMLInputElement;
    const pve2Input = screen.getByLabelText('Connection address for pve2') as HTMLInputElement;
    expect(pve1Input.value).toBe('');
    expect(pve2Input.value).toBe('10.0.0.2');

    fireEvent.input(pve1Input, { target: { value: '10.0.0.11' } });
    fireEvent.submit(pve1Input.closest('form')!);

    await vi.waitFor(() => {
      expect(settings.saveNode).toHaveBeenCalledTimes(1);
    });
    const payload = vi.mocked(settings.saveNode).mock.calls[0][0];
    expect(payload).toMatchObject({
      clusterEndpointOverrides: [{ nodeName: 'pve1', ipOverride: '10.0.0.11' }],
    });
  });

  it('omits clusterEndpointOverrides when no member address changed', async () => {
    const settings = createSettings();
    const editingNode = {
      id: 'pve-0',
      type: 'pve',
      name: 'homelab',
      host: 'https://pve1.local:8006',
      user: '',
      tokenName: 'root@pam!pulse',
      hasToken: true,
      verifySSL: true,
      status: 'connected',
      isCluster: true,
      clusterName: 'homelab',
      clusterEndpoints: [
        {
          nodeId: 'node/pve2',
          nodeName: 'pve2',
          host: 'https://pve2.local:8006',
          ip: '192.168.100.2',
          ipOverride: '10.0.0.2',
          online: true,
          lastSeen: '',
        },
      ],
    } as unknown as NodeConfigWithStatus;

    render(() => (
      <NodeCredentialSlot
        nodeType="pve"
        settings={settings}
        editingNode={editingNode}
        onCancel={vi.fn()}
        onSaved={vi.fn()}
      />
    ));

    const input = screen.getByLabelText('Connection address for pve2') as HTMLInputElement;
    fireEvent.submit(input.closest('form')!);

    await vi.waitFor(() => {
      expect(settings.saveNode).toHaveBeenCalledTimes(1);
    });
    const payload = vi.mocked(settings.saveNode).mock.calls[0][0];
    expect(payload).not.toHaveProperty('clusterEndpointOverrides');
  });

  it('passes the edited node to saveNode so edits hit the update path', async () => {
    const settings = createSettings();
    const onSaved = vi.fn();
    const editingNode = {
      id: 'pve-0',
      type: 'pve',
      name: 'homelab',
      host: 'https://pve1.local:8006',
      user: '',
      tokenName: 'root@pam!pulse',
      hasToken: true,
      verifySSL: true,
      status: 'connected',
    } as unknown as NodeConfigWithStatus;

    render(() => (
      <NodeCredentialSlot
        nodeType="pve"
        settings={settings}
        editingNode={editingNode}
        onCancel={vi.fn()}
        onSaved={onSaved}
      />
    ));

    fireEvent.submit(screen.getByRole('button', { name: 'Save changes' }).closest('form')!);

    await vi.waitFor(() => {
      expect(settings.saveNode).toHaveBeenCalledTimes(1);
    });
    expect(vi.mocked(settings.saveNode).mock.calls[0][1]).toBe(editingNode);
    await vi.waitFor(() => {
      expect(onSaved).toHaveBeenCalledTimes(1);
    });
  });

  it('keeps the editor open when the save fails', async () => {
    const settings = createSettings();
    vi.mocked(settings.saveNode).mockResolvedValue(false);
    const onSaved = vi.fn();
    const editingNode = {
      id: 'pve-0',
      type: 'pve',
      name: 'homelab',
      host: 'https://pve1.local:8006',
      user: '',
      tokenName: 'root@pam!pulse',
      hasToken: true,
      verifySSL: true,
      status: 'connected',
    } as unknown as NodeConfigWithStatus;

    render(() => (
      <NodeCredentialSlot
        nodeType="pve"
        settings={settings}
        editingNode={editingNode}
        onCancel={vi.fn()}
        onSaved={onSaved}
      />
    ));

    fireEvent.submit(screen.getByRole('button', { name: 'Save changes' }).closest('form')!);

    await vi.waitFor(() => {
      expect(settings.saveNode).toHaveBeenCalledTimes(1);
    });
    expect(onSaved).not.toHaveBeenCalled();
  });
});
