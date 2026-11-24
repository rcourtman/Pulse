import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SystemLogsPanel } from '../SystemLogsPanel';
import diagnosticsPanelSource from '../DiagnosticsPanel.tsx?raw';
import systemLogsPanelSource from '../SystemLogsPanel.tsx?raw';

let mockLogs: string[] = [];

vi.mock('../useSystemLogsPanelState', () => ({
  useSystemLogsPanelState: () => ({
    logs: () => mockLogs,
    level: () => 'info',
    isPaused: () => false,
    connectionState: () => 'streaming',
    isLoading: () => false,
    canUseControls: () => true,
    changingLevel: () => false,
    canReconnect: () => false,
    reconnect: vi.fn(),
    error: () => '',
    notice: () => '',
    maxLogs: 1000,
    clearLogs: vi.fn(),
    togglePaused: vi.fn(),
    handleDownload: vi.fn(),
    handleLevelChange: vi.fn(),
    handleLogScroll: vi.fn(),
    setLogContainer: vi.fn(),
  }),
}));

afterEach(() => {
  mockLogs = [];
  cleanup();
});

describe('SystemLogsPanel architecture', () => {
  it('keeps support-page controls touch-sized on phones', () => {
    expect(diagnosticsPanelSource).toContain('min-h-11 sm:min-h-9 min-w-11 sm:min-w-10');
    expect(diagnosticsPanelSource).toContain('flex min-h-11 sm:min-h-9 items-center');
    expect(systemLogsPanelSource).toContain('form-select min-h-11 sm:min-h-9');
    expect(systemLogsPanelSource).toContain('min-h-11 sm:min-h-9 min-w-11 sm:min-w-9');
  });

  it('keeps new log lines immediately readable', () => {
    expect(systemLogsPanelSource).not.toContain('animate-enter');
  });

  it('preserves accessible scope and privacy descriptions on the composed controls', () => {
    render(() => <SystemLogsPanel />);

    expect(screen.getByRole('combobox', { name: 'Server Log Level:' })).toHaveAccessibleDescription(
      /whole Pulse server.*private guest names and paths/,
    );
    for (const name of ['Pause Stream', 'Clear Log Output']) {
      const control = screen.getByRole('button', { name });
      expect(control).toHaveAttribute('type', 'button');
      expect(control).toHaveAccessibleDescription(/not a backup safety pause/);
    }
    const download = screen.getByRole('button', { name: 'Support Bundle' });
    expect(download).toHaveAttribute('type', 'button');
    expect(download).toHaveAccessibleDescription(/Keep the archive private.*redacted excerpts/);
    expect(screen.getByRole('button', { name: 'Pause Stream' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });

  it('uses explicit dark-surface border and hover contrast inside the terminal and styles Diagnostics link', () => {
    expect(systemLogsPanelSource).toContain('border-slate-800/80');
    expect(systemLogsPanelSource).toContain('hover:bg-slate-900/70');
    expect(systemLogsPanelSource).not.toContain('border-border-subtle');
    expect(diagnosticsPanelSource).not.toContain('text-link');
    expect(diagnosticsPanelSource).toContain('text-blue-600 dark:text-blue-400 hover:underline');
  });

  it('filters streamed log lines client-side and allows clearing the filter', () => {
    mockLogs = [
      '2026-11-23T19:00:00Z INFO scheduler started',
      '2026-11-23T19:00:01Z WARN pve-01 poll retry',
      '2026-11-23T19:00:02Z ERROR timeout reaching pbs-02',
    ];
    render(() => <SystemLogsPanel />);

    const input = screen.getByPlaceholderText('Filter log lines...');
    fireEvent.input(input, { target: { value: 'pbs-02' } });

    expect(screen.getByText(/timeout reaching pbs-02/)).toBeInTheDocument();
    expect(screen.queryByText(/scheduler started/)).not.toBeInTheDocument();
    expect(screen.getByText('Showing 1 of 3 lines')).toBeInTheDocument();

    fireEvent.input(input, { target: { value: 'no-such-line' } });
    expect(screen.getByText(/No log lines match/)).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole('button', { name: 'Clear filter' })[0]);
    expect(screen.getByText(/scheduler started/)).toBeInTheDocument();
    expect(screen.getByText(/timeout reaching pbs-02/)).toBeInTheDocument();
  });
});
