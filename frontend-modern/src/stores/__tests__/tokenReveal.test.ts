import { describe, expect, it, beforeEach } from 'vitest';
import {
  tokenRevealStore,
  useTokenRevealState,
  showTokenReveal,
  dismissTokenReveal,
} from '../tokenReveal';

describe('tokenRevealStore', () => {
  beforeEach(() => {
    tokenRevealStore.dismiss();
  });

  describe('state', () => {
    it('starts with null state', () => {
      expect(tokenRevealStore.state()).toBeNull();
    });

    it('returns null after dismiss', () => {
      showTokenReveal({
        token: 'abc',
        record: { id: '1', name: 'test', prefix: 'pmp_', suffix: 'x', createdAt: '' },
      });
      expect(tokenRevealStore.state()).not.toBeNull();

      tokenRevealStore.dismiss();
      expect(tokenRevealStore.state()).toBeNull();
    });
  });

  describe('show', () => {
    it('sets state with payload', () => {
      const payload = {
        token: 'secret-token',
        record: {
          id: 'token-1',
          name: 'My Token',
          prefix: 'pmp_',
          suffix: 'x',
          createdAt: '2024-01-01',
        },
        source: 'settings',
        note: 'Test note',
      };

      showTokenReveal(payload);

      const state = tokenRevealStore.state();
      expect(state).not.toBeNull();
      expect(state!.token).toBe('secret-token');
      expect(state!.record.id).toBe('token-1');
      expect(state!.source).toBe('settings');
      expect(state!.note).toBe('Test note');
      expect(state!.issuedAt).toBeDefined();
    });

    it('sets issuedAt to current timestamp', () => {
      const before = Date.now();
      showTokenReveal({
        token: 'abc',
        record: { id: '1', name: 'test', prefix: 'pmp_', suffix: 'x', createdAt: '' },
      });
      const after = Date.now();

      const state = tokenRevealStore.state();
      expect(state!.issuedAt).toBeGreaterThanOrEqual(before);
      expect(state!.issuedAt).toBeLessThanOrEqual(after);
    });
  });

  describe('dismiss', () => {
    it('clears state to null', () => {
      showTokenReveal({
        token: 'abc',
        record: { id: '1', name: 'test', prefix: 'pmp_', suffix: 'x', createdAt: '' },
      });
      dismissTokenReveal();

      expect(tokenRevealStore.state()).toBeNull();
    });
  });

  describe('useTokenRevealState', () => {
    it('returns current state', () => {
      expect(useTokenRevealState()()).toBeNull();

      showTokenReveal({
        token: 'abc',
        record: { id: '1', name: 'test', prefix: 'pmp_', suffix: 'x', createdAt: '' },
      });
      expect(useTokenRevealState()()).not.toBeNull();
    });
  });
});
