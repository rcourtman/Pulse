import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { NodeConfig } from '@/types/nodes';
import type { NodeModalProps } from '../nodeModalModel';
import { useNodeModalState } from '../useNodeModalState';
import { NodeModalMonitoringSection } from '../NodeModalMonitoringSection';

function renderMonitoring(
  nodeType: NodeModalProps['nodeType'] = 'pve',
  trust: { verifySSL?: boolean; fingerprint?: string; host?: string } = {},
) {
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
    ...trust,
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

  it('gives VM collection a concise name and the full independent-recovery precaution', () => {
    const { fetch } = renderMonitoring();
    const vms = screen.getByRole('checkbox', { name: 'Virtual machines' });
    expect(vms).toHaveAccessibleName('Virtual machines');
    const precaution = screen.getByRole('note', { name: 'Guest-agent backup precaution' });
    // Refer to the warning text, not the named note: its accessible name would
    // replace the actual recovery instructions in the browser description.
    const warning = within(precaution).getByText(/^Guest-agent reads share the channel/);
    const help = screen.getByText(/^VM state and resource usage/);
    expect(vms).toHaveAccessibleDescription(`${help.textContent} ${precaution.textContent}`);
    expect(vms.getAttribute('aria-describedby')?.split(' ')).toContain(warning.id);
    fireEvent.click(vms);
    expect(vms).not.toBeChecked();
    expect(vms).toHaveAccessibleDescription(`${help.textContent} ${precaution.textContent}`);
    expect(fetch).not.toHaveBeenCalled();
  });

  it.each([
    ['Containers', 'LXC container state and resource usage.'],
    ['Storage', 'Per-storage capacity, status, and content type breakdown.'],
    ['Backups', 'vzdump schedules and recent backup activity.'],
  ])('keeps %s help distinct from the VM guest-agent precaution', (name, description) => {
    renderMonitoring();
    const collector = screen.getByRole('checkbox', { name });
    expect(collector).toHaveAccessibleName(name);
    expect(collector).toHaveAccessibleDescription(description);
    const precaution = screen.getByRole('note', { name: 'Guest-agent backup precaution' });
    const warning = within(precaution).getByText(/^Guest-agent reads share the channel/);
    expect(collector.getAttribute('aria-describedby')?.split(' ')).not.toContain(warning.id);
    fireEvent.click(screen.getByText(name, { exact: true }));
    expect(collector).not.toBeChecked();
  });

  it('exposes the drive-standby warning before and after changing SMART collection', () => {
    const { fetch } = renderMonitoring();
    const warning = 'This will spin up idle HDDs. Leave disabled if you rely on drive standby.';
    const enable = screen.getByRole('button', { name: 'Enable physical disk monitoring' });
    expect(enable).toHaveAttribute('aria-pressed', 'false');
    expect(enable).toHaveAccessibleDescription(warning);
    fireEvent.click(enable);
    const disable = screen.getByRole('button', { name: 'Disable physical disk monitoring' });
    expect(disable).toHaveAttribute('aria-pressed', 'true');
    expect(disable).toHaveAccessibleDescription(warning);
    expect(fetch).not.toHaveBeenCalled();
  });

  it('keeps safety descriptions local to each mounted editor', () => {
    renderMonitoring();
    renderMonitoring();
    const scopes = screen.getAllByRole('region', { name: 'Collection scope' });
    const ids = scopes.map((scope) => {
      const editor = within(scope);
      const vms = editor.getByRole('checkbox', { name: 'Virtual machines' });
      const precaution = editor.getByRole('note', { name: 'Guest-agent backup precaution' });
      const warning = within(precaution).getByText(/^Guest-agent reads share the channel/);
      expect(vms.getAttribute('aria-describedby')?.split(' ')).toContain(warning.id);
      return warning.id;
    });
    expect(new Set(ids).size).toBe(2);
  });

  it.each(['pbs', 'pmg'] as const)('does not imply %s makes QEMU guest-agent reads', (nodeType) => {
    const { fetch } = renderMonitoring(nodeType);
    expect(
      screen.queryByRole('note', { name: 'Guest-agent backup precaution' }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/Guest details depend on guest-agent/)).not.toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
  });

  it.each(['pve', 'pbs', 'pmg'] as const)(
    'explains all four %s trust combinations without changing them or testing the endpoint',
    (nodeType) => {
      const pin = 'ab:'.repeat(31) + 'ab';
      for (const verifySSL of [true, false]) {
        for (const fingerprint of ['', pin]) {
          const { fetch, onSave } = renderMonitoring(nodeType, { verifySSL, fingerprint });
          const region = within(screen.getByRole('region', { name: 'TLS trust' }));
          const policy = region.getByRole('status', { name: 'Form TLS policy' });
          expect(policy).toHaveAttribute('aria-live', 'polite');
          expect(policy).toHaveAttribute('aria-atomic', 'true');
          expect(policy).toHaveTextContent(
            fingerprint
              ? 'Certificate fingerprint required'
              : verifySSL
                ? 'Trusted certificate chain required'
                : 'Certificate identity not verified',
          );
          if (fingerprint) {
            expect(policy).toHaveTextContent('The fingerprint takes precedence over the checkbox');
            expect(policy).toHaveTextContent('even if its certificate chain is trusted');
          } else if (verifySSL) {
            expect(policy).toHaveTextContent('a chain trusted by the Pulse runtime');
          } else {
            expect(policy).toHaveTextContent('do not authenticate the server');
            expect(policy).toHaveTextContent('impersonated endpoint');
          }
          expect(region.getByText(/Form settings only/)).toHaveTextContent(
            'not a connection-test result',
          );
          if (nodeType === 'pve') {
            expect(region.getByText(/Cluster members may have/)).toBeInTheDocument();
          } else {
            expect(region.queryByText(/Cluster members may have/)).not.toBeInTheDocument();
          }
          const verification = region.getByRole('checkbox', { name: 'Verify SSL certificate' });
          expect(verification).toHaveAccessibleDescription(
            policy.querySelector('[id]')!.textContent!,
          );
          expect(verification).toHaveProperty('checked', verifySSL);
          expect(region.getByRole('textbox', { name: 'SSL Fingerprint (optional)' })).toHaveValue(
            fingerprint,
          );
          fireEvent.click(screen.getByRole('button', { name: 'Save fixture' }));
          expect(onSave).toHaveBeenCalledOnce();
          expect(onSave.mock.calls[0][0]).toMatchObject({
            type: nodeType,
            verifySSL,
            fingerprint,
          });
          expect(onSave.mock.calls[0][0]).not.toHaveProperty('password');
          expect(onSave.mock.calls[0][0]).not.toHaveProperty('tokenValue');
          expect(fetch).not.toHaveBeenCalled();
          cleanup();
        }
      }
    },
  );

  it('keeps an invalid pin visible and blocks save rather than bypassing it with the checkbox', () => {
    const { fetch, onSave } = renderMonitoring('pve', { fingerprint: 'not-a-valid-pin' });
    const verification = screen.getByRole('checkbox', { name: 'Verify SSL certificate' });
    fireEvent.click(verification);
    expect(verification).not.toBeChecked();
    expect(screen.getByRole('status', { name: 'Form TLS policy' })).toHaveTextContent(
      'Certificate fingerprint required',
    );
    fireEvent.click(screen.getByRole('button', { name: 'Save fixture' }));
    expect(onSave).not.toHaveBeenCalled();
    expect(screen.getByRole('textbox', { name: 'SSL Fingerprint (optional)' })).toHaveValue(
      'not-a-valid-pin',
    );
    expect(fetch).not.toHaveBeenCalled();
  });

  it.each(['pve', 'pbs', 'pmg'] as const)(
    'associates the %s pin error with its input and clears native invalidity after correction',
    (nodeType) => {
      const { fetch, onSave } = renderMonitoring(nodeType);
      const fingerprint = screen.getByRole('textbox', { name: 'SSL Fingerprint (optional)' });
      fireEvent.input(fingerprint, { target: { value: 'SHA256:' + 'ab'.repeat(32) } });
      expect(fingerprint).toHaveAttribute('aria-invalid', 'true');
      expect(fingerprint).toHaveAccessibleDescription(/exactly 64 hexadecimal characters/);
      expect(fingerprint).toHaveAccessibleDescription(/Valid format does not prove trust/);
      expect(fingerprint).toBeInvalid();
      expect(screen.getByRole('alert')).toHaveTextContent(
        'Do not include spaces or a SHA256: label',
      );
      fireEvent.click(screen.getByRole('button', { name: 'Save fixture' }));
      expect(onSave).not.toHaveBeenCalled();

      const corrected = 'CD:'.repeat(31) + 'CD';
      fireEvent.input(fingerprint, { target: { value: corrected } });
      expect(fingerprint).not.toHaveAttribute('aria-invalid');
      expect(fingerprint).toBeValid();
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', { name: 'Save fixture' }));
      expect(onSave).toHaveBeenCalledExactlyOnceWith(
        expect.objectContaining({ type: nodeType, fingerprint: corrected, verifySSL: true }),
      );
      expect(fetch).not.toHaveBeenCalled();
    },
  );

  it.each(['pve', 'pbs', 'pmg'] as const)(
    'does not present a checked box or a pin as TLS protection for an HTTP %s endpoint',
    (nodeType) => {
      const { fetch, onSave } = renderMonitoring(nodeType, {
        host: 'HTTP://fixture.example.invalid',
        verifySSL: true,
        fingerprint: 'ab'.repeat(32),
      });
      const policy = screen.getByRole('status', { name: 'Form TLS policy' });
      expect(policy).toHaveTextContent('TLS is not used for HTTP');
      expect(policy).toHaveTextContent('neither the checkbox nor a fingerprint authenticates');
      expect(policy).toHaveTextContent('Do not switch to HTTP to bypass a certificate error');
      const verification = screen.getByRole('checkbox', { name: 'Verify SSL certificate' });
      expect(verification).toHaveAccessibleDescription(/does not use TLS/);
      expect(verification).toBeChecked();
      expect(screen.getByRole('textbox', { name: 'SSL Fingerprint (optional)' })).toHaveValue(
        'ab'.repeat(32),
      );
      expect(onSave).not.toHaveBeenCalled();
      expect(fetch).not.toHaveBeenCalled();
    },
  );

  it('updates the policy and descriptions as the operator edits, without silently changing trust', () => {
    const { fetch } = renderMonitoring();
    const policy = screen.getByRole('status', { name: 'Form TLS policy' });
    const verification = screen.getByRole('checkbox', { name: 'Verify SSL certificate' });
    const fingerprint = screen.getByRole('textbox', { name: 'SSL Fingerprint (optional)' });
    const safeHelp = /Independently verify the replacement through an already-trusted/;
    expect(screen.getByText(safeHelp)).toHaveTextContent(
      'Do not disable verification or clear a pin to bypass an error',
    );
    expect(fingerprint).toHaveAccessibleDescription(/ACME or Let's Encrypt/);
    fireEvent.click(verification);
    expect(policy).toHaveTextContent('Certificate identity not verified');
    fireEvent.input(fingerprint, { target: { value: 'ab'.repeat(32) } });
    expect(policy).toHaveTextContent('Certificate fingerprint required');
    expect(verification).not.toBeChecked();
    fireEvent.input(fingerprint, { target: { value: '' } });
    expect(policy).toHaveTextContent('Certificate identity not verified');
    fireEvent.click(verification);
    expect(policy).toHaveTextContent('Trusted certificate chain required');
    expect(fingerprint).toHaveValue('');
    expect(fetch).not.toHaveBeenCalled();
  });

  it('keeps TLS labels and descriptions local to each editor', () => {
    renderMonitoring('pve', { fingerprint: 'ab'.repeat(32) });
    renderMonitoring('pbs', { verifySSL: false });
    const ids = screen.getAllByRole('region', { name: 'TLS trust' }).map((region) => {
      const editor = within(region);
      const pin = editor.getByRole('textbox', { name: 'SSL Fingerprint (optional)' });
      const checkbox = editor.getByRole('checkbox', { name: 'Verify SSL certificate' });
      const descriptionId = checkbox.getAttribute('aria-describedby')!;
      expect(region.querySelector(`[id="${descriptionId}"]`)).toBeInTheDocument();
      expect(pin).toHaveAccessibleDescription(/Independently verify/);
      return pin.id;
    });
    expect(new Set(ids).size).toBe(2);
  });
});
