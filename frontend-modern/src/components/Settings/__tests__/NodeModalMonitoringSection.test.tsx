import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { NodeConfig } from '@/types/nodes';
import type { NodeModalProps } from '../nodeModalModel';
import { useNodeModalState } from '../useNodeModalState';
import { NodeModalMonitoringSection } from '../NodeModalMonitoringSection';

function renderMonitoring(nodeType: NodeModalProps['nodeType'] = 'pve') {
  const onSave = vi.fn();
  const fetch = vi.fn();
  vi.stubGlobal('fetch', fetch);
  const baseNode = {
    id: 'fixture-source',
    name: 'Fixture source',
    host: 'https://pve.example.invalid:8006',
    user: 'monitoring@pam',
    tokenName: 'monitoring@pam!fixture',
    hasToken: true,
    verifySSL: true,
  };
  const editingNode: NodeConfig =
    nodeType === 'pve'
      ? {
          ...baseNode,
          type: 'pve',
          monitorVMs: true,
          monitorContainers: true,
          monitorStorage: true,
          monitorBackups: true,
          monitorPhysicalDisks: false,
        }
      : nodeType === 'pbs'
        ? {
            ...baseNode,
            type: 'pbs',
            monitorDatastores: true,
            monitorSyncJobs: true,
            monitorVerifyJobs: true,
            monitorPruneJobs: true,
            monitorGarbageJobs: true,
            excludeDatastores: [],
          }
        : {
            ...baseNode,
            type: 'pmg',
            monitorMailStats: true,
            monitorQueues: true,
            monitorQuarantine: true,
            monitorDomainStats: true,
          };
  const Harness = () => {
    const modalProps: NodeModalProps = {
      nodeType,
      isOpen: true,
      onClose: vi.fn(),
      onSave,
      // Edit an ordinary saved source, without issuing or replacing a token.
      editingNode,
    };
    const state = useNodeModalState(modalProps);
    return (
      <form onSubmit={state.handleSubmit}>
        <NodeModalMonitoringSection modalProps={modalProps} state={state} />
        <button type="submit">Save fixture</button>
      </form>
    );
  };
  render(() => <Harness />);
  return { onSave, fetch };
}

describe('NodeModalMonitoringSection collection safety', () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('explains missing guest readings and independent recovery beside PVE collection choices', () => {
    const { fetch } = renderMonitoring();
    const scope = within(screen.getByRole('region', { name: 'Collection scope' }));
    expect(
      scope.getByText(/Guest details depend on guest-agent availability and permissions/),
    ).toBeVisible();
    expect(
      scope.getByText(/Some readings can be unavailable or temporarily deferred/),
    ).toBeVisible();
    const precaution = scope.getByRole('note', { name: 'Guest-agent backup precaution' });
    expect(precaution).toHaveTextContent('stop Pulse monitoring and leave it stopped');
    expect(precaution).toHaveTextContent('Do not run live diagnostics or restart the guest agent');
    expect(precaution).toHaveTextContent(
      'independent checks confirm thaw and writes to every filesystem covered by the backup',
    );
    expect(precaution).toHaveTextContent('An OK backup or a running VM does not prove recovery');
    expect(scope.queryByText(/always get full visibility/)).not.toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
  });

  it('keeps collector choices, certificate verification and saved credentials unchanged', () => {
    const { fetch, onSave } = renderMonitoring();
    const vms = screen.getByRole('checkbox', { name: /^Virtual machines/ });
    expect(vms).toBeChecked();
    fireEvent.click(vms);
    expect(vms).not.toBeChecked();
    expect(screen.getByRole('note', { name: 'Guest-agent backup precaution' })).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: 'Save fixture' }));
    expect(onSave).toHaveBeenCalledOnce();
    expect(onSave.mock.calls[0][0]).toMatchObject({
      type: 'pve',
      monitorVMs: false,
      monitorContainers: true,
      monitorStorage: true,
      monitorBackups: true,
      monitorPhysicalDisks: false,
      verifySSL: true,
    });
    expect(onSave.mock.calls[0][0]).not.toHaveProperty('tokenValue');
    expect(onSave.mock.calls[0][0]).not.toHaveProperty('password');
    expect(fetch).not.toHaveBeenCalled();
  });

  it.each(['pbs', 'pmg'] as const)('does not imply %s makes QEMU guest-agent reads', (nodeType) => {
    const { fetch } = renderMonitoring(nodeType);
    expect(
      screen.queryByRole('note', { name: 'Guest-agent backup precaution' }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/Guest details depend on guest-agent/)).not.toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
  });
});
