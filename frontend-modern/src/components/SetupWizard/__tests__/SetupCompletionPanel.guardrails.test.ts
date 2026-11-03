import { describe, expect, it } from 'vitest';
import setupCompletionPanelSource from '../SetupCompletionPanel.tsx?raw';

describe('SetupCompletionPanel guardrails', () => {
  it('keeps setup preview copy actions accessible at phone widths', () => {
    expect(setupCompletionPanelSource).toContain('min-h-10 min-w-10');
  });
});
