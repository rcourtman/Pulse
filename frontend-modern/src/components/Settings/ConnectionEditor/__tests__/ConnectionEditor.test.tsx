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

    render(() => (
      <ConnectionEditor
        renderCredentialSlot={renderSlot}
        onClose={() => {}}
      />
    ));

    const input = screen.getByPlaceholderText(
      /pve01\.lan/,
    ) as HTMLInputElement;
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

  it('falls back to manual type selection when probe returns no match', async () => {
    mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 203 });

    const renderSlot = vi.fn(({ type }) => <div data-testid="slot">slot:{type}</div>);

    render(() => (
      <ConnectionEditor
        renderCredentialSlot={renderSlot}
        onClose={() => {}}
      />
    ));

    const input = screen.getByPlaceholderText(
      /pve01\.lan/,
    ) as HTMLInputElement;
    fireEvent.input(input, { target: { value: '192.168.1.50' } });

    fireEvent.click(screen.getByRole('button', { name: /probe address/i }));
    await waitFor(() => expect(mockedProbe).toHaveBeenCalled());

    await screen.findByText(/no supported product detected/i);

    fireEvent.click(screen.getByRole('button', { name: /enter credentials manually/i }));

    fireEvent.click(screen.getByText('TrueNAS SCALE'));

    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:truenas'));
    const lastCall = renderSlot.mock.calls.at(-1)![0];
    expect(lastCall.type).toBe('truenas');
    expect(lastCall.candidate).toBeNull();
  });

  it('surfaces Platform API and Pulse Unified Agent as the two add-paths, mirroring the explainer', () => {
    render(() => (
      <ConnectionEditor
        renderCredentialSlot={() => <div />}
        onClose={() => {}}
      />
    ));

    expect(screen.getByText('Platform API')).toBeInTheDocument();
    expect(screen.getByText('Pulse Unified Agent')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /install the unified agent on a host/i }),
    ).toBeInTheDocument();
  });

  it('routes the Install Unified Agent button straight to the agent credential slot', () => {
    const renderSlot = vi.fn(({ type }) => <div data-testid="slot">slot:{type}</div>);

    render(() => (
      <ConnectionEditor
        renderCredentialSlot={renderSlot}
        onClose={() => {}}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: /install the unified agent on a host/i }));

    expect(screen.getByTestId('slot').textContent).toBe('slot:agent');
    const call = renderSlot.mock.calls.at(-1)![0];
    expect(call.type).toBe('agent');
    expect(call.candidate).toBeNull();
  });

  it('keeps agent out of the Platform API manual picker (it has its own path)', async () => {
    mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 150 });

    render(() => (
      <ConnectionEditor
        renderCredentialSlot={() => <div />}
        onClose={() => {}}
      />
    ));

    const input = screen.getByPlaceholderText(/pve01\.lan/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: 'example.lan' } });
    fireEvent.click(screen.getByRole('button', { name: /probe address/i }));
    await waitFor(() => expect(mockedProbe).toHaveBeenCalled());

    fireEvent.click(screen.getByRole('button', { name: /enter credentials manually/i }));

    expect(screen.getByText(/choose platform api type manually/i)).toBeInTheDocument();
    expect(screen.queryByText(/Agent \(install on host\)/i)).toBeNull();
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
});
