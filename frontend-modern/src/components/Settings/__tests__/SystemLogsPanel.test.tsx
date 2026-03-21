import { describe, expect, it } from 'vitest';
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
});
