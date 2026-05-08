import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { ConnectionsAPI, type ProbeResponse } from '@/api/connections';
import { ConnectionEditor } from '../ConnectionEditor';

const connectionsApiMock = vi.hoisted(() => ({
  list: vi.fn(),
  probe: vi.fn(),
  setEnabled: vi.fn(),
  remove: vi.fn(),
}));

vi.mock('@/api/connections', () => ({
  ConnectionsAPI: connectionsApiMock,
}));

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

    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);

    const input = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: 'pve.lab' } });

    fireEvent.click(screen.getByRole('button', { name: /probe api endpoint/i }));

    await waitFor(() => expect(mockedProbe).toHaveBeenCalledWith('pve.lab'));

    fireEvent.click((await screen.findByText('https://pve.lab:8006')).closest('button')!);

    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:pve'));
    const lastCall = renderSlot.mock.calls.at(-1)![0];
    expect(lastCall.type).toBe('pve');
    expect(lastCall.candidate?.host).toBe('https://pve.lab:8006');
    expect(lastCall.mode).toBe('add');
  });

  it('renders the detect landing and can return to the source picker', () => {
    const onBackToCatalog = vi.fn();

    render(() => (
      <ConnectionEditor
        renderCredentialSlot={() => <div />}
        onBackToCatalog={onBackToCatalog}
        onClose={() => {}}
      />
    ));

    expect(screen.getByText('API platform probe')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Back to source types/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Install Pulse Agent/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Back to source types/i }));
    expect(onBackToCatalog).toHaveBeenCalledTimes(1);
  });

  it('offers the source picker as a no-match escape hatch when provided', async () => {
    mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 203 });
    const onBackToCatalog = vi.fn();

    render(() => (
      <ConnectionEditor
        renderCredentialSlot={() => <div />}
        onBackToCatalog={onBackToCatalog}
        onClose={() => {}}
      />
    ));

    const input = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: '192.168.1.50' } });
    fireEvent.click(screen.getByRole('button', { name: /probe api endpoint/i }));

    await waitFor(() =>
      expect(screen.getByText(/No supported API-backed platform detected/i)).toBeInTheDocument(),
    );
    const normalizedBodyText = document.body.textContent?.replace(/\s+/g, ' ') ?? '';
    expect(normalizedBodyText).toContain(
      'Choose a source type instead. If this is one of the supported Linux, macOS, Windows, FreeBSD, and Unraid host/appliance profiles',
    );
    expect(normalizedBodyText).not.toContain('if this is this is');

    fireEvent.click(screen.getByRole('button', { name: /Choose a source type instead/i }));
    expect(onBackToCatalog).toHaveBeenCalledTimes(1);
  });

  it('offers the agent path contextually when a probe returns no match', async () => {
    mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 180 });

    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);

    const input = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: 'baremetal.lan' } });
    fireEvent.click(screen.getByRole('button', { name: /probe api endpoint/i }));
    await waitFor(() => expect(mockedProbe).toHaveBeenCalled());

    const agentButton = await screen.findByRole('button', {
      name: /install pulse agent instead/i,
    });
    const normalizedBodyText = document.body.textContent?.replace(/\s+/g, ' ') ?? '';
    expect(normalizedBodyText).toContain(
      'If this is one of the supported Linux, macOS, Windows, FreeBSD, and Unraid host/appliance profiles',
    );
    expect(normalizedBodyText).not.toContain('catalog below, or if this is');

    fireEvent.click(agentButton);

    expect(screen.getByTestId('slot').textContent).toBe('slot:agent');
    const call = renderSlot.mock.calls.at(-1)![0];
    expect(call.type).toBe('agent');
    expect(call.candidate).toBeNull();
  });

  it('reopens the detect form from a selected credential slot', async () => {
    mockedProbe.mockResolvedValueOnce({
      candidates: [
        {
          type: 'truenas',
          host: 'https://truenas.lab',
          port: 443,
          hints: { product: 'TrueNAS SCALE' },
        },
      ],
      probedMs: 142,
    });

    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);

    const input = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: 'truenas.lab' } });
    fireEvent.click(screen.getByRole('button', { name: /probe api endpoint/i }));

    fireEvent.click((await screen.findByText('https://truenas.lab')).closest('button')!);
    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:truenas'));

    fireEvent.click(screen.getByRole('button', { name: /Back to API probe/i }));

    const resetInput = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    expect(resetInput.value).toBe('');
    expect(screen.queryByTestId('slot')).toBeNull();
    expect(screen.getByText('API platform probe')).toBeInTheDocument();
  });

  it('opens direct type routes without the detect setup', async () => {
    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => (
      <ConnectionEditor
        initialType="truenas"
        renderCredentialSlot={renderSlot}
        onClose={() => {}}
      />
    ));

    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:truenas'));
    expect(screen.queryByRole('button', { name: /probe api endpoint/i })).not.toBeInTheDocument();
  });

  it('skips probe setup when an initialType is supplied in edit mode', () => {
    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

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
