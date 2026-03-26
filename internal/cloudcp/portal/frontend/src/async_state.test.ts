import { describe, expect, it } from 'vitest';

import {
  beginMutationState,
  beginQueryState,
  failMutationState,
  failQueryState,
  resetMutationState,
  resolveQueryState,
  succeedMutationState,
} from './async_state';
import { createMutationState, createQueryState } from './state';

describe('portal async state helpers', function() {
  it('models mutation lifecycle transitions', function() {
    var mutation = createMutationState();

    beginMutationState(mutation);
    expect(mutation.pending).toBe(true);
    expect(mutation.error).toBe('');

    failMutationState(mutation, 'Broken');
    expect(mutation.pending).toBe(false);
    expect(mutation.error).toBe('Broken');

    succeedMutationState(mutation);
    expect(mutation.pending).toBe(false);
    expect(mutation.error).toBe('');

    mutation.pending = true;
    mutation.error = 'Stale';
    resetMutationState(mutation);
    expect(mutation.pending).toBe(false);
    expect(mutation.error).toBe('');
  });

  it('models query lifecycle transitions', function() {
    var query = createQueryState<string[]>(['stale']);

    beginQueryState(query, []);
    expect(query.status).toBe('loading');
    expect(query.error).toBe('');
    expect(query.data).toEqual([]);

    resolveQueryState(query, ['fresh']);
    expect(query.status).toBe('ready');
    expect(query.error).toBe('');
    expect(query.data).toEqual(['fresh']);

    failQueryState(query, [], 'Nope');
    expect(query.status).toBe('error');
    expect(query.error).toBe('Nope');
    expect(query.data).toEqual([]);
  });
});
