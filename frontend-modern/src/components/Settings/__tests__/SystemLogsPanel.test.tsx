import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SystemLogsPanel } from '../SystemLogsPanel';
import diagnosticsPanelSource from '../DiagnosticsPanel.tsx?raw';
import systemLogsPanelSource from '../SystemLogsPanel.tsx?raw';

vi.mock('../useSystemLogsPanelState', () => ({
  useSystemLogsPanelState: () => ({
    logs: () => [],
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
    setLogContainer: vi.fn(),
  }),
}));

afterEach(cleanup);

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
});
