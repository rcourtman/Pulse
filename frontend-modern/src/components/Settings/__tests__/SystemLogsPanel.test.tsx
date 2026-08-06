import { describe, expect, it } from 'vitest';
import diagnosticsPanelSource from '../DiagnosticsPanel.tsx?raw';
import systemLogsPanelSource from '../SystemLogsPanel.tsx?raw';
import systemLogsPanelStateSource from '../useSystemLogsPanelState.ts?raw';

describe('SystemLogsPanel architecture', () => {
  it('keeps system logs split into shell and runtime owners', () => {
    expect(systemLogsPanelSource).toContain('./useSystemLogsPanelState');
    expect(systemLogsPanelSource).not.toContain('createSignal(');
    expect(systemLogsPanelSource).not.toContain('new EventSource(');
    expect(systemLogsPanelSource).not.toContain("apiFetchJSON('/api/logs/level'");
    expect(systemLogsPanelStateSource).toContain('new EventSource');
    expect(systemLogsPanelStateSource).toContain("apiFetchJSON('/api/logs/level'");
    expect(systemLogsPanelStateSource).toContain("window.location.href = '/api/logs/download'");
    expect(systemLogsPanelStateSource).toContain('notificationStore.success');
  });

  it('keeps support-page controls touch-sized on phones', () => {
    expect(diagnosticsPanelSource).toContain('min-h-11 sm:min-h-9 min-w-11 sm:min-w-10');
    expect(diagnosticsPanelSource).toContain('flex min-h-11 sm:min-h-9 items-center');
    expect(systemLogsPanelSource).toContain('form-select min-h-11 sm:min-h-9');
    expect(systemLogsPanelSource).toContain('min-h-11 sm:min-h-9 min-w-11 sm:min-w-9');
    expect(systemLogsPanelSource).toContain('min-h-11 sm:min-h-9 flex items-center');
  });
});
