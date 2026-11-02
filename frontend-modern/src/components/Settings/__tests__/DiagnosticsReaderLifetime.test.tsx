import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DiagnosticsPanel } from '../DiagnosticsPanel';
import { DIAGNOSTICS_PANEL_COPY } from '@/utils/diagnosticsPresentation';

const api = vi.hoisted(() => ({ read: vi.fn(), org: 'diagnostics-a' }));
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: api.read, getOrgID: () => api.org }));
vi.mock('@/utils/toast', () => ({
  showError: () => {
    throw new Error('Diagnostics feedback must stay local');
  },
  showSuccess: () => {
    throw new Error('Diagnostics feedback must stay local');
  },
}));
const data = {
  version: '6.7.0',
  runtime: 'go',
  uptime: 3600,
  nodes: [
    {
      id: 'node-a',
      name: 'private-reader-a',
      host: 'private-a.fixture.test',
      type: 'pve',
      authMethod: 'token',
      connected: true,
    },
  ],
  pbs: [],
  errors: [],
  system: {
    os: 'linux',
    arch: 'amd64',
    goVersion: 'go1.26',
    numCPU: 4,
    numGoroutine: 32,
    memoryMB: 128,
  },
};
const run = () => fireEvent.click(screen.getAllByRole('button', { name: 'Run Diagnostics' })[0]);

beforeEach(() => {
  api.read.mockReset();
  api.org = 'diagnostics-a';
});
afterEach(cleanup);

describe('Diagnostics reader lifetime', () => {
  it('withdraws stale cards and export controls during a denied refresh with local safe feedback', async () => {
    api.read.mockResolvedValueOnce(data).mockRejectedValueOnce(new Error('private-error-payload'));
    render(() => (
      <Router>
        <Route path="/" component={() => <DiagnosticsPanel canRead={true} />} />
      </Router>
    ));
    expect(api.read).not.toHaveBeenCalled();
    run();
    await screen.findByText('private-reader-a');
    expect(screen.getByRole('status')).toHaveTextContent('Diagnostics completed');
    expect(screen.getByRole('button', { name: 'Full (private)' })).toBeEnabled();
    run();
    await screen.findByRole('alert');
    expect(screen.getByRole('alert')).toHaveTextContent(DIAGNOSTICS_PANEL_COPY.runError);
    expect(screen.queryByText('private-reader-a')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Full (private)' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'GitHub (review first)' })).not.toBeInTheDocument();
    expect(screen.queryByText('private-error-payload')).not.toBeInTheDocument();
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
    expect(api.read).toHaveBeenCalledTimes(2);
  });

  it('keeps live checks off on reader or permission changes and only recovers after an explicit run', async () => {
    const [reader, setReader] = createSignal('reader-a');
    const [allowed, setAllowed] = createSignal(true);
    api.read.mockResolvedValue(data);
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => <DiagnosticsPanel currentUser={reader()} canRead={allowed()} />}
        />
      </Router>
    ));
    run();
    await screen.findByText('private-reader-a');
    setReader('reader-b');
    await waitFor(() => expect(screen.queryByText('private-reader-a')).not.toBeInTheDocument());
    expect(api.read).toHaveBeenCalledOnce();
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
    setAllowed(false);
    await screen.findByText(DIAGNOSTICS_PANEL_COPY.accessUnavailable);
    expect(screen.queryByRole('button', { name: 'Run Diagnostics' })).not.toBeInTheDocument();
    setAllowed(true);
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: 'Run Diagnostics' })).toHaveLength(2),
    );
    expect(api.read).toHaveBeenCalledOnce();
    run();
    await screen.findByText('private-reader-a');
    expect(api.read).toHaveBeenCalledTimes(2);
  });

  it('aborts a disposed panel without showing its eventual completion in a new panel', async () => {
    let resolve!: (value: typeof data) => void;
    api.read.mockReturnValue(
      new Promise<typeof data>((done) => {
        resolve = done;
      }),
    );
    const [visible, setVisible] = createSignal(true);
    const { Show } = await import('solid-js');
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <Show when={visible()}>
              <DiagnosticsPanel canRead={true} />
            </Show>
          )}
        />
      </Router>
    ));
    run();
    const signal = api.read.mock.calls[0][1].signal as AbortSignal;
    setVisible(false);
    await waitFor(() => expect(signal.aborted).toBe(true));
    setVisible(true);
    resolve(data);
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: 'Run Diagnostics' })).toHaveLength(2),
    );
    expect(screen.queryByText('private-reader-a')).not.toBeInTheDocument();
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
    expect(api.read).toHaveBeenCalledOnce();
  });
});
