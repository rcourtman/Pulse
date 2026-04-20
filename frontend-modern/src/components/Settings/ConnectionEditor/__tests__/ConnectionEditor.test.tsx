import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen, waitFor } from '@solidjs/testing-library';
import { ConnectionEditor } from '../ConnectionEditor';
import { ConnectionsAPI, type ProbeResponse } from '@/api/connections';

vi.mock('@/api/connections', async () => {
  const actual = await vi.importActual<typeof import('@/api/connections')>('@/api/connections');
  return {
    ...actual,
    ConnectionsAPI: {
      list: vi.fn(),
      probe: vi.fn(),
    },
  };
});

const mockedProbe = vi.mocked(ConnectionsAPI.probe);

function expectNodeBefore(a: Node, b: Node) {
  expect(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
}

describe('ConnectionEditor', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('runs a probe and dispatches the detected type into the credential slot', async () => {
    const response: ProbeResponse = {
      candidates: [
        { type: 'pve', host: 'https://pve.lab:8006', port: 8006, hints: { product: 'Proxmox VE' } },
      ],
      probedMs: 418,
    };
    mockedProbe.mockResolvedValueOnce(response);

    const renderSlot = vi.fn(({ type }) => <div data-testid="slot">slot:{type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);

    const input = screen.getByPlaceholderText(/pve01\.lan/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: 'pve.lab' } });

    const probeButton = screen.getByRole('button', { name: /probe address/i });
    fireEvent.click(probeButton);

    await waitFor(() => expect(mockedProbe).toHaveBeenCalledWith('pve.lab'));

    const candidateLabel = await screen.findAllByText('Proxmox VE');
    const candidateButton = candidateLabel[0].closest('button');
    expect(candidateButton).not.toBeNull();
    fireEvent.click(candidateButton!);

    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:pve'));
    expect(renderSlot).toHaveBeenCalled();
    const lastCall = renderSlot.mock.calls.at(-1)![0];
    expect(lastCall.type).toBe('pve');
    expect(lastCall.candidate?.host).toBe('https://pve.lab:8006');
    expect(lastCall.mode).toBe('add');
  });

  it('lets the user pick a product tile when probe returns no match', async () => {
    mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 203 });

    const renderSlot = vi.fn(({ type }) => <div data-testid="slot">slot:{type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);

    const input = screen.getByPlaceholderText(/pve01\.lan/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: '192.168.1.50' } });

    fireEvent.click(screen.getByRole('button', { name: /probe address/i }));
    await waitFor(() => expect(mockedProbe).toHaveBeenCalled());

    await screen.findByText(/no supported product detected/i);

    // The catalog grid is always visible below the probe, so the user picks a
    // tile directly — no intermediate "enter credentials manually" toggle.
    fireEvent.click(screen.getByRole('button', { name: /TrueNAS SCALE/i }));

    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:truenas'));
    const lastCall = renderSlot.mock.calls.at(-1)![0];
    expect(lastCall.type).toBe('truenas');
    expect(lastCall.candidate).toBeNull();
  });

  it('renders an agent-led catalog with the API fallback beneath it', () => {
    render(() => <ConnectionEditor renderCredentialSlot={() => <div />} onClose={() => {}} />);

    const agentButton = screen.getByRole('button', { name: /Install Pulse Agent/i });
    const apiHeading = screen.getByText('Or connect a platform API directly');
    const probeButton = screen.getByRole('button', { name: /probe address/i });

    // Catalog landing — lead with the agent card, then collapse the probe and
    // direct platform options into one API fallback section below it.
    expect(agentButton).toBeInTheDocument();
    expect(apiHeading).toBeInTheDocument();
    expect(probeButton).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Proxmox VE/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /TrueNAS SCALE/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /VMware vCenter \/ ESXi/i })).toBeInTheDocument();
    expectNodeBefore(agentButton, apiHeading);
    expectNodeBefore(apiHeading, probeButton);
    expect(
      screen.queryByRole('button', { name: /install the unified agent on a host/i }),
    ).toBeNull();
  });

  it('offers the agent path contextually when a probe returns no match', async () => {
    mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 180 });

    const renderSlot = vi.fn(({ type }) => <div data-testid="slot">slot:{type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);

    const input = screen.getByPlaceholderText(/pve01\.lan/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: 'baremetal.lan' } });
    fireEvent.click(screen.getByRole('button', { name: /probe address/i }));
    await waitFor(() => expect(mockedProbe).toHaveBeenCalled());

    // The no-match box names bare-metal Linux / Unraid / FreeBSD and offers
    // the agent as a first-class alternative, so a user who probed the wrong
    // thing isn't left in a Platform-API-only dead end.
    const agentButton = await screen.findByRole('button', {
      name: /install the unified agent instead/i,
    });
    fireEvent.click(agentButton);

    expect(screen.getByTestId('slot').textContent).toBe('slot:agent');
    const call = renderSlot.mock.calls.at(-1)![0];
    expect(call.type).toBe('agent');
    expect(call.candidate).toBeNull();
  });

  it('skips the probe step when an initialType is supplied (edit mode)', () => {
    const renderSlot = vi.fn(({ type }) => <div data-testid="slot">slot:{type}</div>);

    render(() => (
      <ConnectionEditor
        mode="edit"
        initialType="vmware"
        renderCredentialSlot={renderSlot}
        onClose={() => {}}
      />
    ));

    expect(screen.getByTestId('slot').textContent).toBe('slot:vmware');
    const call = renderSlot.mock.calls.at(0)![0];
    expect(call.mode).toBe('edit');
    expect(call.type).toBe('vmware');
    expect(call.candidate).toBeNull();
  });

  it('resets probe state when returning to the catalog from a credential slot', async () => {
    mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 203 });

    const renderSlot = vi.fn(({ type }) => <div data-testid="slot">slot:{type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);

    const input = screen.getByPlaceholderText(/pve01\.lan/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: '192.168.1.50' } });
    fireEvent.click(screen.getByRole('button', { name: /probe address/i }));

    await waitFor(() => expect(mockedProbe).toHaveBeenCalled());
    await screen.findByText(/no supported product detected/i);

    fireEvent.click(screen.getByRole('button', { name: /TrueNAS SCALE/i }));
    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:truenas'));

    fireEvent.click(screen.getByRole('button', { name: /back to catalog/i }));

    const resetInput = screen.getByPlaceholderText(/pve01\.lan/) as HTMLInputElement;
    expect(resetInput.value).toBe('');
    expect(screen.queryByText(/no supported product detected/i)).toBeNull();
    expect(screen.queryByTestId('slot')).toBeNull();
  });
});
