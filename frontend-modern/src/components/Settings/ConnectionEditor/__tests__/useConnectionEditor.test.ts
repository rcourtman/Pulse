import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ConnectionsAPI, type ProbeResponse } from '@/api/connections';
import { createConnectionEditorState } from '../useConnectionEditor';

vi.mock('@/api/connections', () => ({ ConnectionsAPI: { probe: vi.fn() } }));
const probe = vi.mocked(ConnectionsAPI.probe);
const match: ProbeResponse = {
  candidates: [{ type: 'pve', host: 'https://alpha.example.invalid:8006', port: 8006 }],
  probedMs: 42,
};
const deferred = () => {
  let resolve!: (value: ProbeResponse) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<ProbeResponse>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};

describe('address-bound platform probe evidence', () => {
  let dispose: () => void;
  const setup = () =>
    createRoot((cleanup) => {
      dispose = cleanup;
      const state = createConnectionEditorState();
      state.setAddress('alpha.example.invalid');
      return state;
    });
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => dispose?.());

  it.each(['detected', 'no-match', 'error'] as const)(
    'retires completed %s evidence on an address edit without probing again',
    async (phase) => {
      const state = setup();
      if (phase === 'error') probe.mockRejectedValueOnce(new Error('fixture failure'));
      else
        probe.mockResolvedValueOnce(
          phase === 'detected' ? match : { candidates: [], probedMs: 42 },
        );
      expect(await state.runProbe()).toBe(phase);
      expect(state.phase()).toBe(phase);
      state.setAddress('beta.example.invalid');
      expect(state.phase()).toBe('idle');
      expect(state.candidates()).toEqual([]);
      expect(state.errorMessage()).toBe('');
      expect(state.probedMs()).toBe(0);
      expect(state.probing()).toBe(false);
      state.setAddress('alpha.example.invalid');
      expect(state.phase()).toBe('idle');
      expect(state.candidates()).toEqual([]);
      expect(probe).toHaveBeenCalledTimes(1);
    },
  );

  it('keeps evidence on an identical value, but retires it on a whitespace edit', async () => {
    const state = setup();
    probe.mockResolvedValueOnce(match);
    await state.runProbe();
    state.setAddress('alpha.example.invalid');
    expect(state.phase()).toBe('detected');
    state.setAddress(' alpha.example.invalid ');
    expect(state.phase()).toBe('idle');
    probe.mockResolvedValueOnce(match);
    await state.runProbe();
    expect(probe).toHaveBeenLastCalledWith('alpha.example.invalid');
    expect(state.phase()).toBe('detected');
  });

  it.each(['detected', 'no-match', 'error'] as const)(
    'ignores late %s after an A-B-A edit and keeps the request single-flight',
    async (phase) => {
      const state = setup();
      const pending = deferred();
      probe.mockReturnValueOnce(pending.promise);
      const result = state.runProbe();
      expect(state.probing()).toBe(true);
      expect(await state.runProbe()).toBeNull();
      state.setAddress('beta.example.invalid');
      state.setAddress('alpha.example.invalid');
      expect(state.phase()).toBe('idle');
      expect(state.probing()).toBe(true);
      expect(await state.runProbe()).toBeNull();
      expect(probe).toHaveBeenCalledTimes(1);
      if (phase === 'error') pending.reject(new Error('obsolete failure'));
      else pending.resolve(phase === 'detected' ? match : { candidates: [], probedMs: 42 });
      expect(await result).toBeNull();
      expect(state.probing()).toBe(false);
      expect(state.phase()).toBe('idle');
      expect(state.candidates()).toEqual([]);
      expect(state.errorMessage()).toBe('');
      expect(state.probedMs()).toBe(0);

      probe.mockResolvedValueOnce(match);
      expect(await state.runProbe()).toBe('detected');
      expect(probe).toHaveBeenCalledTimes(2);
    },
  );

  it.each(['success', 'failure'] as const)(
    'does not restore evidence after a reset: %s',
    async (kind) => {
      const state = setup();
      const pending = deferred();
      probe.mockReturnValueOnce(pending.promise);
      const result = state.runProbe();
      state.reset();
      state.reset();
      expect(state.address()).toBe('');
      expect(state.phase()).toBe('idle');
      expect(state.probing()).toBe(true);
      state.setAddress('beta.example.invalid');
      expect(await state.runProbe()).toBeNull();
      if (kind === 'failure') pending.reject(new Error('obsolete failure'));
      else pending.resolve(match);
      expect(await result).toBeNull();
      expect(state.phase()).toBe('idle');
      expect(state.probing()).toBe(false);
      expect(state.candidates()).toEqual([]);
      expect(state.errorMessage()).toBe('');
      expect(state.probedMs()).toBe(0);
      expect(probe).toHaveBeenCalledTimes(1);
    },
  );

  it.each(['success', 'failure'] as const)('does not publish after disposal: %s', async (kind) => {
    const state = setup();
    const pending = deferred();
    probe.mockReturnValueOnce(pending.promise);
    const result = state.runProbe();
    dispose();
    if (kind === 'failure') pending.reject(new Error('obsolete failure'));
    else pending.resolve(match);
    expect(await result).toBeNull();
    expect(state.candidates()).toEqual([]);
    expect(state.errorMessage()).toBe('');
    expect(await state.runProbe()).toBeNull();
    expect(probe).toHaveBeenCalledTimes(1);
  });

  it('validates empty input locally and permits ordinary same-address recovery', async () => {
    const state = setup();
    state.setAddress('  ');
    expect(await state.runProbe()).toBe('error');
    expect(state.errorMessage()).toBe('Enter an API endpoint to probe.');
    expect(probe).not.toHaveBeenCalled();
    state.setAddress('alpha.example.invalid');
    probe.mockRejectedValueOnce(new Error('fixture failure')).mockResolvedValueOnce(match);
    expect(await state.runProbe()).toBe('error');
    expect(state.errorMessage()).toBe('fixture failure');
    expect(await state.runProbe()).toBe('detected');
    expect(state.errorMessage()).toBe('');
    expect(state.candidates()).toEqual(match.candidates);
  });
});
