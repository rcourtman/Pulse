import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SystemLogsPanel } from '../SystemLogsPanel';

const apiFetchJSON = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON }));
vi.mock('@/utils/logger', () => ({ logger: { debug: vi.fn(), error: vi.fn() } }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));

class DisplayStream {
  static instances: DisplayStream[] = [];
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();

  constructor(public readonly url: string) {
    DisplayStream.instances.push(this);
  }

  receive(data: string) {
    this.onmessage?.({ data });
  }
}

async function mountPanel() {
  const view = render(() => <SystemLogsPanel />);
  await waitFor(() => expect(DisplayStream.instances).toHaveLength(1));
  return { ...view, stream: DisplayStream.instances[0] };
}

describe('SystemLogsPanel control safety', () => {
  beforeEach(() => {
    DisplayStream.instances = [];
    vi.stubGlobal('EventSource', DisplayStream);
    apiFetchJSON.mockReset();
    apiFetchJSON.mockResolvedValue({ level: 'info' });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('describes the server-wide threshold at the level control without changing it on mount', async () => {
    await mountPanel();
    const select = screen.getByRole('combobox', { name: 'Server Log Level:' });
    expect(select).toHaveValue('info');
    expect(select).toHaveAccessibleDescription(/whole Pulse server/);
    expect(select).toHaveAccessibleDescription(/Debug can include private guest names and paths/);
    const help = document.getElementById(select.getAttribute('aria-describedby')!);
    expect(help).toBeVisible();
    expect(apiFetchJSON.mock.calls).toEqual([['/api/logs/level']]);
  });

  it('describes display-only pause and still receives later messages on the same stream', async () => {
    const { stream } = await mountPanel();
    const pause = screen.getByRole('button', { name: 'Pause Stream' });
    expect(pause).toHaveAccessibleDescription(/Monitoring and server logging continue/);
    expect(pause).toHaveAttribute('aria-pressed', 'false');
    stream.receive('INFO before display pause');
    expect(screen.getByText('INFO before display pause')).toBeVisible();
    fireEvent.click(pause);
    const resume = screen.getByRole('button', { name: 'Resume Stream' });
    expect(resume).toHaveAttribute('aria-pressed', 'true');
    expect(resume).toHaveAccessibleDescription(/Messages received while paused are not added/);
    stream.receive('INFO received while paused');
    expect(screen.queryByText('INFO received while paused')).toBeNull();
    expect(stream.close).not.toHaveBeenCalled();
    fireEvent.click(resume);
    stream.receive('INFO after display resume');
    expect(screen.getByText('INFO after display resume')).toBeVisible();
    expect(DisplayStream.instances).toHaveLength(1);
    expect(stream.url).toBe('/api/logs/stream');
    expect(apiFetchJSON.mock.calls).toEqual([['/api/logs/level']]);
  });

  it('describes view-only clear without changing the level or closing the stream', async () => {
    const { stream } = await mountPanel();
    stream.receive('INFO displayed before clear');
    const clear = screen.getByRole('button', { name: 'Clear Log Output' });
    expect(clear).toHaveAccessibleDescription(/Pause and Clear affect this display only/);
    fireEvent.click(clear);
    expect(screen.queryByText('INFO displayed before clear')).toBeNull();
    expect(screen.getByText('Waiting for log output.')).toBeVisible();
    stream.receive('INFO displayed after clear');
    expect(screen.getByText('INFO displayed after clear')).toBeVisible();
    expect(stream.close).not.toHaveBeenCalled();
    expect(screen.getByRole('combobox', { name: 'Server Log Level:' })).toHaveValue('info');
    expect(apiFetchJSON.mock.calls).toEqual([['/api/logs/level']]);
  });

  it('keeps the full archive privacy warning visible and attached after pause and clear', async () => {
    await mountPanel();
    const bundle = screen.getByRole('button', { name: 'Support Bundle' });
    expect(bundle).toHaveAccessibleDescription(
      /server logs, configuration and environment details/,
    );
    expect(bundle).toHaveAccessibleDescription(/not just the lines shown here/);
    expect(bundle).toHaveAccessibleDescription(
      /Pausing or clearing this display does not remove them/,
    );
    expect(bundle).toHaveAccessibleDescription(/Keep the archive private/);
    expect(bundle).toHaveAccessibleDescription(/manually reviewed, redacted excerpts/);
    const warning = document.getElementById(bundle.getAttribute('aria-describedby')!);
    expect(warning).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: 'Pause Stream' }));
    fireEvent.click(screen.getByRole('button', { name: 'Clear Log Output' }));
    expect(warning).toBeVisible();
    expect(bundle).toHaveAccessibleDescription(/Keep the archive private/);
    expect(bundle).not.toHaveAccessibleDescription(/safe to (publish|share)/);
    expect(apiFetchJSON.mock.calls).toEqual([['/api/logs/level']]);
  });

  it('does not turn display actions into submission of a surrounding settings form', async () => {
    const submit = vi.fn((event: SubmitEvent) => event.preventDefault());
    render(() => (
      <form onSubmit={submit}>
        <SystemLogsPanel />
      </form>
    ));
    await waitFor(() => expect(DisplayStream.instances).toHaveLength(1));
    for (const name of ['Pause Stream', 'Clear Log Output', 'Support Bundle']) {
      expect(screen.getByRole('button', { name })).toHaveAttribute('type', 'button');
    }
    fireEvent.click(screen.getByRole('button', { name: 'Pause Stream' }));
    fireEvent.click(screen.getByRole('button', { name: 'Clear Log Output' }));
    expect(submit).not.toHaveBeenCalled();
  });

  it('keeps descriptions instance-local when more than one panel is mounted', async () => {
    render(() => (
      <>
        <SystemLogsPanel />
        <SystemLogsPanel />
      </>
    ));
    await waitFor(() => expect(DisplayStream.instances).toHaveLength(2));
    const selects = screen.getAllByRole('combobox', { name: 'Server Log Level:' });
    expect(new Set(selects.map((select) => select.id)).size).toBe(2);
    for (const select of selects) {
      expect(select).toHaveAccessibleDescription(/whole Pulse server/);
    }
    for (const name of ['Pause Stream', 'Clear Log Output', 'Support Bundle']) {
      const descriptions = screen
        .getAllByRole('button', { name })
        .map((button) => button.getAttribute('aria-describedby'));
      expect(descriptions.every(Boolean)).toBe(true);
      expect(new Set(descriptions).size).toBe(2);
      for (const id of descriptions) expect(document.getElementById(id!)).toBeVisible();
    }
  });
});
