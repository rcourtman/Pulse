import { beforeEach, describe, expect, it, vi } from 'vitest';
import { runPatrolPreflight } from '../patrol';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('runPatrolPreflight', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('POSTs to the canonical preflight endpoint and returns the structured result', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      success: true,
      provider: 'deepseek',
      model: 'deepseek-v4-flash',
      tool_call_observed: true,
      duration_ms: 842,
      message: 'Provider accepted the preflight request and the model emitted a tool call.',
    });

    const result = await runPatrolPreflight();

    expect(apiFetchJSON).toHaveBeenCalledWith('/api/ai/patrol/preflight', {
      method: 'POST',
      body: '{}',
      headers: { 'Content-Type': 'application/json' },
    });
    expect(result.success).toBe(true);
    expect(result.tool_call_observed).toBe(true);
    expect(result.provider).toBe('deepseek');
    expect(result.duration_ms).toBe(842);
  });

  it('forwards provider and model overrides verbatim', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      success: false,
      tool_call_observed: false,
      duration_ms: 312,
      message: 'Provider rejected forced tool selection',
      cause: 'tool_choice_rejected',
      recommendation: 'Pulse will retry with automatic tool selection on the next Patrol run.',
    });

    const result = await runPatrolPreflight({ provider: 'deepseek', model: 'deepseek-v4-flash' });

    expect(apiFetchJSON).toHaveBeenCalledWith('/api/ai/patrol/preflight', {
      method: 'POST',
      body: JSON.stringify({ provider: 'deepseek', model: 'deepseek-v4-flash' }),
      headers: { 'Content-Type': 'application/json' },
    });
    expect(result.success).toBe(false);
    expect(result.cause).toBe('tool_choice_rejected');
  });

  it('round-trips the recorded_at fields used to render the "last verified" indicator', async () => {
    // Pulse caches preflight outcomes server-side and surfaces them
    // through /api/settings/ai. The same response shape is also
    // returned from the live POST so the inline result panel can render
    // a "last verified" timestamp without forking shapes.
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      success: true,
      provider: 'deepseek',
      model: 'deepseek-v4-flash',
      tool_call_observed: true,
      duration_ms: 1948,
      message: 'Provider accepted the preflight request and the model emitted a tool call.',
      recorded_at: '2026-05-10T13:54:11Z',
      recorded_at_unix: 1778421251,
    });

    const result = await runPatrolPreflight();

    expect(result.recorded_at).toBe('2026-05-10T13:54:11Z');
    expect(result.recorded_at_unix).toBe(1778421251);
  });

  it('handles auto-triggered preflight outcomes with the same shape as manual ones', async () => {
    // The backend dispatches auto-preflight in the background after a
    // settings save when the Patrol model or its provider key changes.
    // The cached result then shows up on /api/settings/ai with the
    // same shape this client uses for manual preflight, so the UI can
    // render it through one code path. Verify the recorded_at fields
    // on a successful auto-trigger result round-trip cleanly.
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      success: true,
      provider: 'deepseek',
      model: 'deepseek-v4-flash',
      tool_call_observed: true,
      duration_ms: 1948,
      message: 'Provider accepted the preflight request and the model emitted a tool call.',
      recorded_at: '2026-05-10T13:54:11Z',
      recorded_at_unix: 1778421251,
    });

    const result = await runPatrolPreflight();

    expect(result.success).toBe(true);
    expect(result.tool_call_observed).toBe(true);
    expect(result.recorded_at_unix).toBe(1778421251);
  });

  it('exposes the soft-warning shape when the model accepted the request but did not call the tool', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      success: false,
      tool_call_observed: false,
      duration_ms: 412,
      message:
        'Provider accepted the preflight request but the model did not emit a tool call. Patrol may still work in practice.',
      cause: 'model_tool_support_unverified',
      recommendation:
        'Trigger a real Patrol run to confirm tool calling. If that fails, switch to a model with stronger tool-following behaviour.',
    });

    const result = await runPatrolPreflight();

    expect(result.success).toBe(false);
    expect(result.tool_call_observed).toBe(false);
    expect(result.cause).toBe('model_tool_support_unverified');
    expect(result.recommendation).toContain('real Patrol');
  });
});
