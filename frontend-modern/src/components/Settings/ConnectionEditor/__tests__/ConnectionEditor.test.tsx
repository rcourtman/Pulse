import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { ConnectionsAPI, type ConnectionType, type ProbeResponse } from '@/api/connections';
import { ConnectionEditor } from '../ConnectionEditor';
import { createConnectionEditorState } from '../useConnectionEditor';

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

function deferredProbe() {
  let resolve!: (response: ProbeResponse) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<ProbeResponse>((accept, decline) => {
    resolve = accept;
    reject = decline;
  });
  return { promise, resolve, reject };
}

describe('ConnectionEditor', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it.each([
    [
      'certificate fingerprint mismatch: expected fixture-old, got fixture-new',
      'TLS fingerprint changed.',
      'Keep TLS verification enabled.',
    ],
    [
      'API error 403: authentication failed',
      'Permission denied.',
      'Keep intentional access limits',
    ],
  ])(
    'renders safe probe guidance and clears it on ordinary recovery: %s',
    async (raw, headline, hint) => {
      mockedProbe.mockRejectedValueOnce(new Error(raw));
      mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 10 });
      const renderSlot = vi.fn(() => <div />);
      render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);
      const input = screen.getByLabelText('API endpoint') as HTMLInputElement;
      fireEvent.input(input, { target: { value: 'https://pve.lab:8006' } });
      fireEvent.click(screen.getByRole('button', { name: /probe api endpoint/i }));
      const alert = await screen.findByRole('alert');
      expect(alert.textContent).toContain(headline);
      expect(alert.textContent).toContain(hint);
      expect(alert.textContent).not.toContain('credentials connected');
      expect(alert.textContent).not.toContain('disable TLS');
      expect(input.value).toBe('https://pve.lab:8006');
      expect(mockedProbe).toHaveBeenCalledTimes(1);
      expect(renderSlot).not.toHaveBeenCalled();

      fireEvent.click(screen.getByRole('button', { name: /probe api endpoint/i }));
      await screen.findByText('No supported API-backed platform detected at that address.');
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expect(mockedProbe).toHaveBeenCalledTimes(2);
      expect(input.value).toBe('https://pve.lab:8006');
      expect(renderSlot).not.toHaveBeenCalled();
    },
  );

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

  it('retains detection for whitespace only and hands off only a freshly probed endpoint after an edit', async () => {
    const firstHost = 'https://first.example.test:8006';
    const nextHost = 'https://second.example.test:8007';
    const nextCandidate = { type: 'pbs', host: nextHost, port: 8007 } as const;
    mockedProbe
      .mockResolvedValueOnce({
        candidates: [{ type: 'pve', host: firstHost, port: 8006 }],
        probedMs: 418,
      })
      .mockResolvedValueOnce({ candidates: [nextCandidate], probedMs: 215 });

    const chooseCandidate = vi.fn();
    render(() => (
      <ConnectionEditor
        onSelectCandidate={chooseCandidate}
        renderCredentialSlot={() => <div />}
        onClose={() => {}}
      />
    ));
    const input = screen.getByLabelText('API endpoint');
    const probe = screen.getByRole('button', { name: 'Probe API endpoint' });
    const edit = (value: string) => fireEvent.input(input, { target: { value } });

    edit(firstHost);
    fireEvent.click(probe);
    expect(await screen.findByText(firstHost)).toBeVisible();
    edit(`  ${firstHost}  `);
    expect(screen.getByText(firstHost)).toBeVisible();
    expect(screen.getByText('Probed in 418 ms')).toBeVisible();

    edit(nextHost);
    expect(screen.queryByText(firstHost)).not.toBeInTheDocument();
    expect(screen.queryByText('Probed in 418 ms')).not.toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('Address changed. Probe this endpoint');
    expect(probe).toBeEnabled();
    expect(chooseCandidate).not.toHaveBeenCalled();

    edit('');
    expect(probe).toBeDisabled();
    expect(screen.getByRole('status')).toBeEmptyDOMElement();
    edit(firstHost);
    expect(screen.queryByText(firstHost)).not.toBeInTheDocument();
    expect(mockedProbe.mock.calls).toEqual([[firstHost]]);

    edit(` ${nextHost} `);
    fireEvent.click(probe);
    fireEvent.click((await screen.findByText(nextHost)).closest('button')!);
    expect(chooseCandidate).toHaveBeenCalledExactlyOnceWith(nextCandidate);
    expect(mockedProbe.mock.calls).toEqual([[firstHost], [nextHost]]);
    expect(screen.getByRole('status')).toBeEmptyDOMElement();
  });

  it.each(['no-match', 'error'] as const)(
    'retains the source picker but retires a previous %s on address edit',
    async (outcome) => {
      if (outcome === 'no-match')
        mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 22 });
      else mockedProbe.mockRejectedValueOnce(new Error('Previous probe rejected the address'));

      const onBackToCatalog = vi.fn();
      render(() => (
        <ConnectionEditor
          onBackToCatalog={onBackToCatalog}
          renderCredentialSlot={() => <div />}
          onClose={() => {}}
        />
      ));
      const input = screen.getByLabelText('API endpoint');
      fireEvent.input(input, { target: { value: 'first.example.test' } });
      fireEvent.click(screen.getByRole('button', { name: 'Probe API endpoint' }));
      const previousResult = await (outcome === 'no-match'
        ? screen.findByRole('button', { name: 'install Pulse Agent instead', exact: true })
        : screen.findByRole('alert'));
      expect(previousResult).toBeVisible();

      fireEvent.input(input, { target: { value: 'second.example.test' } });
      expect(previousResult).not.toBeInTheDocument();
      const back = screen.getByRole('button', { name: /Back to source types/i });
      expect(back).toBeVisible();
      fireEvent.click(back);
      expect(onBackToCatalog).toHaveBeenCalledTimes(1);
    },
  );

  it.each(['edit', 'reset'] as const)(
    'ignores a late response after %s even if a new probe uses the same address',
    async (retirement) => {
      const retired = deferredProbe();
      const current = deferredProbe();
      mockedProbe.mockReturnValueOnce(retired.promise).mockReturnValueOnce(current.promise);
      const state = createConnectionEditorState();
      const address = 'https://first.example.test:8006';
      state.setAddress(address);
      const oldRun = state.runProbe();
      expect(state.phase()).toBe('probing');
      const duplicateRun = state.runProbe();
      expect(mockedProbe.mock.calls).toEqual([[address]]);
      expect(await duplicateRun).toBeUndefined();

      if (retirement === 'edit') state.setAddress('second.example.test');
      else state.reset();
      state.setAddress(address);
      const newRun = state.runProbe();
      if (retirement === 'edit') {
        retired.resolve({ candidates: [{ type: 'pve', host: address, port: 8006 }], probedMs: 99 });
      } else {
        retired.reject(new Error('Retired endpoint check failed'));
      }
      expect(await oldRun).toBeUndefined();
      expect(state.phase()).toBe('probing');
      expect(state.candidates()).toEqual([]);
      expect(state.errorMessage()).toBe('');

      current.resolve({ candidates: [], probedMs: 20 });
      expect(await newRun).toBe('no-match');
      expect(state.phase()).toBe('no-match');
      expect(state.probedMs()).toBe(20);
      expect(mockedProbe.mock.calls).toEqual([[address], [address]]);
    },
  );

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

  it('retargets the credential slot when a direct route changes type', async () => {
    const [initialType, setInitialType] = createSignal<ConnectionType>('agent');
    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => (
      <ConnectionEditor
        initialType={initialType()}
        renderCredentialSlot={renderSlot}
        onClose={() => {}}
      />
    ));

    expect(screen.getByTestId('slot').textContent).toBe('slot:agent');
    setInitialType('pve');
    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:pve'));
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
