import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { InfrastructurePlatformSettingsProps } from '@/components/Settings/proxmoxSettingsModel';
import type { NodeConfigWithStatus } from '@/types/nodes';
import { MonitoredSystemLedgerAPI } from '@/api/monitoredSystemLedger';
import { NodesAPI } from '@/api/nodes';
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
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it.each([false, true])(
    'shows truthful PMG account guidance in the real password form (editing: %s)',
    (editing) => {
      const settings = createSettings();
      const formTest = vi.spyOn(NodesAPI, 'testConnection');
      const savedTest = vi.spyOn(NodesAPI, 'testExistingNode');
      render(() => (
        <NodeCredentialSlot
          nodeType="pmg"
          settings={settings}
          editingNode={
            editing
              ? ({
                  id: 'pmg-fixture',
                  type: 'pmg',
                  name: 'Fixture',
                  host: 'https://saved.example.invalid',
                  user: 'existing@pmg',
                  hasPassword: true,
                  verifySSL: true,
                  status: 'connected',
                } as NodeConfigWithStatus)
              : undefined
          }
          onCancel={vi.fn()}
          onSaved={vi.fn()}
        />
      ));

      expect(screen.getByText('Username and password')).toBeInTheDocument();
      expect(screen.getAllByRole('radio')).toHaveLength(1);
      expect(screen.getByRole('radio', { name: 'Username & Password' })).toBeChecked();
      expect(screen.getAllByRole('note', { name: 'Mail Gateway account guidance' })).toHaveLength(
        1,
      );
      expect(screen.queryByLabelText(/Token ID|Token Value/)).not.toBeInTheDocument();
      const username = screen.getByRole('textbox', { name: /^Username/ });
      expect(username).toHaveAttribute('placeholder', 'pulse-monitor@pmg');
      expect(username).toHaveValue(editing ? 'existing@pmg' : '');
      const password = screen.getByLabelText(/^Password/);
      expect(password).toHaveValue('');
      expect(password).toHaveAttribute('type', 'password');
      expect(password.hasAttribute('required')).toBe(!editing);
      fireEvent.input(username, { target: { value: 'pulse-monitor@pmg' } });
      fireEvent.input(password, { target: { value: 'fixture-only-secret' } });
      expect(username).toHaveValue('pulse-monitor@pmg');
      expect(screen.getByRole('checkbox', { name: 'Verify SSL certificate' })).toBeChecked();
      expect(formTest).not.toHaveBeenCalled();
      expect(savedTest).not.toHaveBeenCalled();
      expect(settings.saveNode).not.toHaveBeenCalled();
    },
  );

  it.each(['pve', 'pbs', 'pmg'] as const)(
    'discloses a saved %s check beside edited trust, then removes obsolete success',
    async (nodeType) => {
      const savedTest = vi.spyOn(NodesAPI, 'testExistingNode').mockResolvedValue({
        status: 'success',
        message: 'Fixture saved connection responds',
      });
      const formTest = vi.spyOn(NodesAPI, 'testConnection');
      const settings = createSettings();
      render(() => (
        <NodeCredentialSlot
          nodeType={nodeType}
          settings={settings}
          editingNode={
            {
              id: `${nodeType}-fixture`,
              type: nodeType,
              name: 'Fixture',
              host: 'https://saved.example.invalid',
              user: 'fixture@pam',
              tokenName: 'fixture@pam!read',
              hasPassword: nodeType === 'pmg',
              verifySSL: true,
              status: 'connected',
            } as NodeConfigWithStatus
          }
          onCancel={vi.fn()}
          onSaved={vi.fn()}
        />
      ));
      fireEvent.input(screen.getByLabelText(/^Endpoint URL/), {
        target: { value: 'https://entered.example.invalid' },
      });
      fireEvent.input(screen.getByLabelText('SSL Fingerprint (optional)'), {
        target: { value: 'ab'.repeat(32) },
      });
      const testButton = screen.getByRole('button', { name: 'Test saved connection' });
      expect(testButton).toHaveAccessibleDescription(/Unsaved changes are not tested/);
      fireEvent.click(testButton);
      const result = await screen.findByRole('status', { name: 'Connection test result' });
      expect(result).toHaveTextContent('Saved connection test');
      expect(result).toHaveTextContent('Fixture saved connection responds');
      expect(result).toHaveTextContent('Unsaved changes are not tested');
      expect(savedTest).toHaveBeenCalledExactlyOnceWith(`${nodeType}-fixture`);
      expect(formTest).not.toHaveBeenCalled();
      fireEvent.click(screen.getByRole('checkbox', { name: 'Verify SSL certificate' }));
      expect(
        screen.queryByRole('status', { name: 'Connection test result' }),
      ).not.toBeInTheDocument();
      expect(screen.getByLabelText(/^Endpoint URL/)).toHaveValue('https://entered.example.invalid');
      expect(settings.saveNode).not.toHaveBeenCalled();
    },
  );

  it('discloses and retires an entered-password test through the real form', async () => {
    const formTest = vi.spyOn(NodesAPI, 'testConnection').mockResolvedValue({
      status: 'success',
      message: 'Fixture entered connection responds',
    });
    const savedTest = vi.spyOn(NodesAPI, 'testExistingNode');
    render(() => (
      <NodeCredentialSlot
        nodeType="pmg"
        settings={createSettings()}
        editingNode={
          {
            id: 'pmg-fixture',
            type: 'pmg',
            name: 'Fixture',
            host: 'https://saved.example.invalid',
            user: 'fixture@pam',
            hasPassword: true,
            verifySSL: true,
            status: 'connected',
          } as NodeConfigWithStatus
        }
        onCancel={vi.fn()}
        onSaved={vi.fn()}
      />
    ));
    fireEvent.input(screen.getByLabelText('Password'), {
      target: { value: 'fixture-only-secret' },
    });
    const button = screen.getByRole('button', { name: 'Test entered settings' });
    expect(button).toHaveAccessibleDescription(/No settings are saved/);
    fireEvent.click(button);
    const result = await screen.findByRole('status', { name: 'Connection test result' });
    expect(result).toHaveTextContent('Entered settings test');
    expect(result).toHaveTextContent('No settings are saved');
    expect(formTest).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({ password: 'fixture-only-secret' }),
    );
    expect(savedTest).not.toHaveBeenCalled();
    fireEvent.input(screen.getByLabelText('Password'), { target: { value: '' } });
    expect(
      screen.queryByRole('status', { name: 'Connection test result' }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Test saved connection' })).toBeEnabled();
  });

  it('syncs an untouched mounted editor to a refreshed node snapshot', async () => {
    const settings = createSettings();
    const initialNode = {
      id: 'pve-0',
      type: 'pve',
      name: 'homelab',
      host: 'https://pve1.local:8006',
      user: '',
      tokenName: 'root@pam!pulse',
      hasToken: true,
      verifySSL: false,
      status: 'connected',
    } as unknown as NodeConfigWithStatus;
    const [editingNode, setEditingNode] = createSignal(initialNode);

    render(() => (
      <NodeCredentialSlot
        nodeType="pve"
        settings={settings}
        editingNode={editingNode()}
        onCancel={vi.fn()}
        onSaved={vi.fn()}
      />
    ));

    expect(screen.getByLabelText(/^Endpoint URL/)).toHaveValue('https://pve1.local:8006');
    expect(screen.getByRole('checkbox', { name: 'Verify SSL certificate' })).not.toBeChecked();
    expect(screen.getByRole('status', { name: 'Form TLS policy' })).toHaveTextContent(
      'Certificate identity not verified',
    );

    setEditingNode({
      ...initialNode,
      host: 'https://pve1-refreshed.local:8006',
      verifySSL: true,
    } as NodeConfigWithStatus);

    await waitFor(() => {
      expect(screen.getByLabelText(/^Endpoint URL/)).toHaveValue(
        'https://pve1-refreshed.local:8006',
      );
      expect(screen.getByRole('checkbox', { name: 'Verify SSL certificate' })).toBeChecked();
      expect(screen.getByRole('status', { name: 'Form TLS policy' })).toHaveTextContent(
        'Trusted certificate chain required',
      );
    });
  });

  it('preserves unsaved connection edits across polls and saves to the refreshed edit target', async () => {
    const settings = createSettings();
    const onSaved = vi.fn();
    const initialNode = {
      id: 'pve-0',
      type: 'pve',
      name: 'homelab',
      host: 'https://pve1.local:8006',
      user: '',
      tokenName: 'root@pam!pulse',
      hasToken: true,
      verifySSL: true,
      fingerprint: 'ab'.repeat(32),
      status: 'connected',
    } as unknown as NodeConfigWithStatus;
    const [editingNode, setEditingNode] = createSignal(initialNode);

    render(() => (
      <NodeCredentialSlot
        nodeType="pve"
        settings={settings}
        editingNode={editingNode()}
        onCancel={vi.fn()}
        onSaved={onSaved}
      />
    ));

    const endpoint = screen.getByLabelText(/^Endpoint URL/);
    const verifySSL = screen.getByRole('checkbox', { name: 'Verify SSL certificate' });
    fireEvent.input(endpoint, { target: { value: 'https://operator-edit.local:8006' } });
    fireEvent.click(verifySSL);
    expect(screen.getByRole('status', { name: 'Form TLS policy' })).toHaveTextContent(
      'Certificate fingerprint required',
    );

    for (let poll = 1; poll <= 2; poll += 1) {
      setEditingNode({
        ...initialNode,
        host: `https://server-poll-${poll}.local:8006`,
        fingerprint: 'cd'.repeat(32),
      });
      await waitFor(() => {
        expect(screen.getByLabelText(/^Endpoint URL/)).toBe(endpoint);
        expect(endpoint).toHaveValue('https://operator-edit.local:8006');
        expect(verifySSL).not.toBeChecked();
        expect(screen.getByLabelText('SSL Fingerprint (optional)')).toHaveValue('ab'.repeat(32));
        expect(screen.getByRole('status', { name: 'Form TLS policy' })).toHaveTextContent(
          'Certificate fingerprint required',
        );
      });
    }

    expect(settings.saveNode).not.toHaveBeenCalled();
    fireEvent.submit(endpoint.closest('form')!);
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(settings.saveNode).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({
        host: 'https://operator-edit.local:8006',
        verifySSL: false,
        fingerprint: 'ab'.repeat(32),
      }),
      editingNode(),
    );
  });

  it('keeps the selected authentication strategy across polls but resets for another node', async () => {
    const settings = createSettings();
    const initialNode = {
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
    const [editingNode, setEditingNode] = createSignal(initialNode);

    render(() => (
      <NodeCredentialSlot
        nodeType="pve"
        settings={settings}
        editingNode={editingNode()}
        onCancel={vi.fn()}
        onSaved={vi.fn()}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: /^Host Telemetry Agent$/i }));
    const agentPanel = screen.getByText('Host telemetry agent');
    for (let poll = 1; poll <= 2; poll += 1) {
      setEditingNode({ ...initialNode, host: `https://poll-${poll}.local:8006` });
      await waitFor(() => {
        expect(screen.getByText('Host telemetry agent')).toBe(agentPanel);
        expect(screen.getByLabelText(/^Endpoint URL/)).toHaveValue(initialNode.host);
      });
    }

    // A genuinely different edit target must not inherit the previous strategy.
    setEditingNode({ ...initialNode, id: 'pve-1', host: 'https://pve2.local:8006' });
    await waitFor(() => {
      expect(screen.queryByText('Host telemetry agent')).not.toBeInTheDocument();
      expect(screen.getByLabelText(/^Endpoint URL/)).toHaveValue('https://pve2.local:8006');
    });
    expect(settings.saveNode).not.toHaveBeenCalled();
  });

  it('discards an obsolete preview error after the endpoint changes', async () => {
    let rejectPreview!: (error: Error) => void;
    const pending = new Promise<never>((_, reject) => {
      rejectPreview = reject;
    });
    vi.spyOn(MonitoredSystemLedgerAPI, 'preview').mockReturnValueOnce(pending);
    render(() => (
      <NodeCredentialSlot
        nodeType="pve"
        settings={createSettings()}
        prefillNode={{ name: 'tower', host: 'https://tower.local:8006' }}
        importCandidate={{
          kind: 'discovery',
          server: {
            type: 'pve',
            ip: '10.0.0.10',
            hostname: 'tower.local',
            port: 8006,
            version: '9.0',
          },
        }}
        onCancel={vi.fn()}
        onSaved={vi.fn()}
      />
    ));
    fireEvent.click(screen.getByRole('button', { name: 'Preview impact' }));
    fireEvent.input(screen.getByLabelText(/^Endpoint URL/), {
      target: { value: 'https://replacement.local:8006' },
    });
    rejectPreview(new Error('Obsolete endpoint failure'));
    await pending.catch(() => undefined);
    await waitFor(() => {
      expect(screen.queryByText('Obsolete endpoint failure')).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Preview impact' })).toBeEnabled();
    });
  });

  it('asks for the endpoint first and names the node from it', () => {
    render(() => (
      <NodeCredentialSlot
        nodeType="pve"
        settings={createSettings()}
        onCancel={vi.fn()}
        onSaved={vi.fn()}
      />
    ));

    expect(screen.getByText('API address and name')).toBeInTheDocument();
    const endpoint = screen.getByLabelText(/^Endpoint URL/);
    const name = screen.getByLabelText(/^Node Name/);
    expect(endpoint.compareDocumentPosition(name) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(endpoint).toBeRequired();
    expect(name).not.toBeRequired();
    expect(screen.queryByText(/must be unique/)).not.toBeInTheDocument();

    fireEvent.input(endpoint, { target: { value: 'https://pve1.local:8006' } });
    expect(name).toHaveValue('pve1.local');

    fireEvent.input(name, { target: { value: 'Rack A' } });
    fireEvent.input(endpoint, { target: { value: 'https://pve2.local:8006' } });
    expect(name).toHaveValue('Rack A');
  });

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
    expect(screen.getByText(/Unreachable — details/, { selector: 'summary' })).toHaveTextContent(
      'Unreachable — details for pve2',
    );

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

  it('saves a standalone PVE rename to the original connection after a poll', async () => {
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

    const [node, setNode] = createSignal(editingNode);

    render(() => (
      <NodeCredentialSlot
        nodeType="pve"
        settings={settings}
        editingNode={node()}
        onCancel={vi.fn()}
        onSaved={onSaved}
      />
    ));

    const name = screen.getByLabelText(/^Node Name/);
    fireEvent.input(name, { target: { value: 'renamed-connection' } });
    setNode({ ...editingNode, name: 'server-refresh' });
    await waitFor(() => {
      expect(screen.getByLabelText(/^Node Name/)).toBe(name);
      expect(name).toHaveValue('renamed-connection');
    });
    expect(settings.saveNode).not.toHaveBeenCalled();

    fireEvent.submit(screen.getByRole('button', { name: 'Save changes' }).closest('form')!);

    await vi.waitFor(() => {
      expect(settings.saveNode).toHaveBeenCalledTimes(1);
    });
    expect(settings.saveNode).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({ name: 'renamed-connection', host: editingNode.host }),
      node(),
    );
    expect(node().id).toBe('pve-0');
    await vi.waitFor(() => {
      expect(onSaved).toHaveBeenCalledTimes(1);
    });
  });

  it('edits PBS datastore exclusions and saves normalized patterns', async () => {
    const settings = createSettings();
    const editingNode = {
      id: 'pbs-0',
      type: 'pbs',
      name: 'backup',
      host: 'https://pbs.local:8007',
      user: '',
      tokenName: 'root@pam!pulse',
      hasToken: true,
      verifySSL: true,
      monitorDatastores: true,
      monitorSyncJobs: true,
      monitorVerifyJobs: true,
      monitorPruneJobs: true,
      monitorGarbageJobs: true,
      excludeDatastores: ['archive-*'],
      status: 'connected',
    } as unknown as NodeConfigWithStatus;

    render(() => (
      <NodeCredentialSlot
        nodeType="pbs"
        settings={settings}
        editingNode={editingNode}
        onCancel={vi.fn()}
        onSaved={vi.fn()}
      />
    ));

    const input = screen.getByLabelText('Excluded datastores (optional)') as HTMLInputElement;
    expect(input.value).toBe('archive-*');

    fireEvent.input(input, { target: { value: 'exthdd*, *removable*, exact-store' } });
    fireEvent.submit(input.closest('form')!);

    await vi.waitFor(() => {
      expect(settings.saveNode).toHaveBeenCalledTimes(1);
    });
    expect(vi.mocked(settings.saveNode).mock.calls[0][0]).toMatchObject({
      excludeDatastores: ['exthdd*', '*removable*', 'exact-store'],
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
