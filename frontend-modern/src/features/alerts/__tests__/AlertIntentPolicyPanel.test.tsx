import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertIntentPoliciesAPI, type AlertIntentPolicyDocument } from '@/api/alertIntentPolicies';
import type { Resource } from '@/types/resource';
import { AlertIntentPolicyPanel } from '../AlertIntentPolicyPanel';

vi.mock('@/api/alertIntentPolicies', () => ({
  AlertIntentPoliciesAPI: {
    get: vi.fn(),
    update: vi.fn(),
    preview: vi.fn(),
  },
}));

const baseDocument = (): AlertIntentPolicyDocument => ({
  schemaVersion: 1,
  revision: 0,
  defaults: {
    'state.offline': {
      graceSeconds: 30,
      honorOperatorState: true,
      backupOffline: { enabled: true, postGraceSeconds: 60, maxDeferralSeconds: 3600 },
    },
  },
  resourceTypes: {},
  resources: {},
});

const resource: Resource = {
  id: 'vm-canonical',
  type: 'vm',
  name: 'database',
  displayName: 'database',
  platformId: 'node-a',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'running',
  lastSeen: Date.now(),
};

const dockerHost = (sourceId: string, type: Resource['type'] = 'docker-host'): Resource =>
  ({
    id: `agent-${sourceId}`,
    type,
    name: 'dock',
    displayName: 'dock',
    platformType: 'docker',
    sourceType: 'agent',
    status: 'online',
    lastSeen: Date.now(),
    docker: { hostname: 'dock', hostSourceId: sourceId, runtime: 'docker' },
  }) as unknown as Resource;

describe('AlertIntentPolicyPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(AlertIntentPoliciesAPI.get).mockResolvedValue(baseDocument());
    vi.mocked(AlertIntentPoliciesAPI.update).mockImplementation(async (document) => ({
      ...document,
      revision: document.revision + 1,
    }));
    vi.mocked(AlertIntentPoliciesAPI.preview).mockResolvedValue({
      resourceId: resource.id,
      resourceType: resource.type,
      signal: 'state.offline',
      status: 'pending_grace',
      reason: 'grace_period',
      effective: {
        graceSeconds: 30,
        honorOperatorState: true,
        sources: { graceSeconds: 'defaults.state.offline' },
        explicit: true,
      },
      remainingSeconds: 30,
      contexts: [],
      warnings: [],
    });
  });

  afterEach(() => cleanup());

  it('tells same-named Docker hosts apart in the resource picker', async () => {
    render(() => (
      <AlertIntentPolicyPanel
        resources={[
          dockerHost('proxmox-lxc-docker:lab:pve1:200'),
          dockerHost('proxmox-lxc-docker:site:pve2:300'),
          resource,
          {
            ...resource,
            id: 'container-1',
            type: 'app-container',
            name: 'nginx',
            displayName: 'nginx',
            docker: { hostname: 'dock', hostSourceId: 'proxmox-lxc-docker:lab:pve1:200' },
          } as unknown as Resource,
        ]}
      />
    ));
    fireEvent.click(screen.getByRole('button', { name: 'Configure policies' }));

    await screen.findByRole('option', { name: 'dock (lab) · docker-host' });
    expect(screen.getByRole('option', { name: 'dock (site) · docker-host' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'database · vm' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'nginx · app-container' })).toBeInTheDocument();
  });

  it('counts a machine agent that also runs Docker as a Docker host in the picker', async () => {
    const withAddress = (host: Resource, address: string): Resource => ({
      ...host,
      identity: { ips: [address] } as Resource['identity'],
    });
    render(() => (
      <AlertIntentPolicyPanel
        resources={[
          withAddress(dockerHost('machine-home', 'agent'), '192.168.1.50'),
          withAddress(dockerHost('machine-work', 'agent'), '10.20.0.7'),
        ]}
      />
    ));
    fireEvent.click(screen.getByRole('button', { name: 'Configure policies' }));

    await screen.findByRole('option', { name: 'dock (192.168.1.50) · agent' });
    expect(screen.getByRole('option', { name: 'dock (10.20.0.7) · agent' })).toBeInTheDocument();
  });

  it('selects the first canonical resource when inventory arrives after policy load', async () => {
    const [resources, setResources] = createSignal<Resource[]>([]);
    render(() => <AlertIntentPolicyPanel resources={resources()} />);

    await waitFor(() => expect(AlertIntentPoliciesAPI.get).toHaveBeenCalledTimes(1));
    setResources([resource]);
    fireEvent.click(screen.getByRole('button', { name: 'Configure policies' }));

    expect(await screen.findByLabelText('Resource')).toHaveValue(resource.id);
  });

  it('opens directly on a row-selected resource and signal', async () => {
    render(() => (
      <AlertIntentPolicyPanel
        resources={[resource]}
        selectionTarget={{
          resourceId: resource.id,
          signal: 'metric.memory',
          requestId: 1,
        }}
      />
    ));

    expect(await screen.findByRole('button', { name: 'Hide policies' })).toBeInTheDocument();
    expect(screen.getByLabelText('Resource')).toHaveValue(resource.id);
    expect(screen.getByLabelText('Signal')).toHaveValue('metric.memory');
  });

  it('keeps a row-selected machine sensor that the resource list does not hold', async () => {
    const sensorId = 'agent:host-1/sensor:6170706c65736d632d6973612d303330302f74656d7031';
    render(() => (
      <AlertIntentPolicyPanel
        resources={[resource]}
        selectionTarget={{
          resourceId: sensorId,
          signal: 'metric.temperature',
          label: 'TB0T · macbook-pve',
          requestId: 1,
        }}
      />
    ));

    expect(await screen.findByRole('button', { name: 'Hide policies' })).toBeInTheDocument();
    const select = screen.getByLabelText('Resource');
    expect(select).toHaveValue(sensorId);
    expect(screen.getByRole('option', { name: 'TB0T · macbook-pve' })).toBeInTheDocument();
    expect(screen.getByLabelText('Signal')).toHaveValue('metric.temperature');
    expect(screen.getByRole('button', { name: 'Preview current policy' })).toBeDisabled();

    fireEvent.input(await screen.findByLabelText(/^Grace override \(seconds\)/), {
      target: { value: '600' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save override' }));

    await waitFor(() => expect(AlertIntentPoliciesAPI.update).toHaveBeenCalledTimes(1));
    const saved = vi.mocked(AlertIntentPoliciesAPI.update).mock.calls[0][0];
    expect(saved.resources?.[sensorId]?.['metric.temperature']).toEqual({ graceSeconds: 600 });
  });

  it('does not replay a row handoff when the resource list changes', async () => {
    const sensorId = 'agent:host-1/sensor:6162';
    const [resources, setResources] = createSignal<Resource[]>([resource]);
    render(() => (
      <AlertIntentPolicyPanel
        resources={resources()}
        selectionTarget={{
          resourceId: sensorId,
          signal: 'metric.temperature',
          label: 'TB0T · macbook-pve',
          requestId: 1,
        }}
      />
    ));

    const select = await screen.findByLabelText('Resource');
    fireEvent.change(select, { target: { value: resource.id } });
    fireEvent.change(screen.getByLabelText('Signal'), { target: { value: 'metric.cpu' } });
    setResources([resource, { ...resource, id: 'vm-other', name: 'other', displayName: 'other' }]);

    await waitFor(() => expect(screen.getByLabelText('Resource')).toHaveValue(resource.id));
    expect(screen.getByLabelText('Signal')).toHaveValue('metric.cpu');
  });

  it('saves field-level resource overrides without disabling inherited policy', async () => {
    render(() => <AlertIntentPolicyPanel resources={[resource]} />);

    await waitFor(() => expect(AlertIntentPoliciesAPI.get).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole('button', { name: 'Configure policies' }));
    const grace = await screen.findByLabelText(/^Grace override \(seconds\)/);
    expect(grace).toHaveValue(null);
    expect(screen.getByLabelText('Operator state override')).toHaveValue('inherit');
    expect(screen.getByLabelText('Backup handling override')).toHaveValue('inherit');

    fireEvent.input(grace, { target: { value: '75' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save override' }));

    await waitFor(() => expect(AlertIntentPoliciesAPI.update).toHaveBeenCalledTimes(1));
    const saved = vi.mocked(AlertIntentPoliciesAPI.update).mock.calls[0][0];
    expect(saved.resources?.[resource.id]?.['state.offline']).toEqual({ graceSeconds: 75 });
  });

  it('stores the powered-off default on the guest resource type', async () => {
    render(() => <AlertIntentPolicyPanel resources={[resource]} />);

    await waitFor(() => expect(AlertIntentPoliciesAPI.get).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole('button', { name: 'Configure policies' }));
    const tolerance = await screen.findByLabelText(
      /^VM \/ container powered-off tolerance \(seconds\)/,
    );
    expect(tolerance).toHaveValue(30);
    fireEvent.input(tolerance, { target: { value: '300' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save defaults' }));

    await waitFor(() => expect(AlertIntentPoliciesAPI.update).toHaveBeenCalledTimes(1));
    const saved = vi.mocked(AlertIntentPoliciesAPI.update).mock.calls[0][0];
    expect(saved.resourceTypes?.guest?.['state.offline']).toEqual(
      expect.objectContaining({
        graceSeconds: 300,
        honorOperatorState: true,
        backupOffline: { enabled: true, postGraceSeconds: 60, maxDeferralSeconds: 3600 },
      }),
    );
  });

  it('preserves explicit zero and rejects non-integer overrides locally', async () => {
    render(() => <AlertIntentPolicyPanel resources={[resource]} />);

    await waitFor(() => expect(AlertIntentPoliciesAPI.get).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole('button', { name: 'Configure policies' }));
    const grace = await screen.findByLabelText(/^Grace override \(seconds\)/);

    fireEvent.input(grace, { target: { value: '0' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save override' }));
    await waitFor(() => expect(AlertIntentPoliciesAPI.update).toHaveBeenCalledTimes(1));
    expect(
      vi.mocked(AlertIntentPoliciesAPI.update).mock.calls[0][0].resources?.[resource.id]?.[
        'state.offline'
      ],
    ).toEqual({ graceSeconds: 0 });

    vi.mocked(AlertIntentPoliciesAPI.update).mockClear();
    fireEvent.input(grace, { target: { value: '1.5' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save override' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Grace override must be a whole number of seconds.',
    );
    expect(AlertIntentPoliciesAPI.update).not.toHaveBeenCalled();
  });

  it('previews the selected canonical resource', async () => {
    render(() => <AlertIntentPolicyPanel resources={[resource]} />);
    await waitFor(() => expect(AlertIntentPoliciesAPI.get).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole('button', { name: 'Configure policies' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Preview current policy' }));

    await waitFor(() =>
      expect(AlertIntentPoliciesAPI.preview).toHaveBeenCalledWith(
        expect.objectContaining({
          resourceId: resource.id,
          resourceType: resource.type,
          signal: 'state.offline',
          conditionActive: true,
        }),
      ),
    );
    expect(await screen.findByText(/grace period · 30s grace/)).toBeInTheDocument();
  });
});
