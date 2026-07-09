import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  },
}));

import { logger } from '@/utils/logger';
import {
  showToast,
  showSuccess,
  showError,
  showErrorWithDetail,
  showWarning,
} from '@/utils/toast';

describe('toast', () => {
  const loggerInfo = vi.mocked(logger.info);

  beforeEach(() => {
    loggerInfo.mockReset();
    delete (window as unknown as { showToast?: unknown }).showToast;
  });

  afterEach(() => {
    delete (window as unknown as { showToast?: unknown }).showToast;
  });

  describe('showToast', () => {
    it('delegates to window.showToast and returns its id when available', () => {
      const mock = vi.fn(() => 'toast-id-1');
      (window as unknown as { showToast: typeof mock }).showToast = mock;

      const result = showToast('success', 'Saved', 'all good', 5000, 'details');

      expect(mock).toHaveBeenCalledWith('success', 'Saved', 'all good', 5000, 'details');
      expect(result).toBe('toast-id-1');
    });

    it('passes undefined for omitted args when delegating', () => {
      const mock = vi.fn(() => 'id');
      (window as unknown as { showToast: typeof mock }).showToast = mock;

      const result = showToast('info', 'Hello');

      expect(mock).toHaveBeenCalledWith('info', 'Hello', undefined, undefined, undefined);
      expect(result).toBe('id');
    });

    it('returns undefined when window.showToast returns undefined', () => {
      const mock = vi.fn(() => undefined);
      (window as unknown as { showToast: typeof mock }).showToast = mock;

      const result = showToast('info', 'Hello');

      expect(result).toBeUndefined();
    });

    it('delegates with an empty title', () => {
      const mock = vi.fn(() => 'id');
      (window as unknown as { showToast: typeof mock }).showToast = mock;

      showToast('info', '');

      expect(mock).toHaveBeenCalledWith('info', '', undefined, undefined, undefined);
    });

    it('falls back to logger.info and returns undefined when window.showToast is missing', () => {
      const result = showToast('error', 'Failed', 'something broke');

      expect(loggerInfo).toHaveBeenCalledWith('[toast:error] Failed: something broke');
      expect(result).toBeUndefined();
    });

    it('omits the message suffix when no message is provided (fallback)', () => {
      showToast('warning', 'Careful');

      expect(loggerInfo).toHaveBeenCalledWith('[toast:warning] Careful');
    });

    it('omits the message suffix when message is an empty string (fallback)', () => {
      showToast('success', 'Done', '');

      expect(loggerInfo).toHaveBeenCalledWith('[toast:success] Done');
    });

    it('includes the toast type in the fallback log line', () => {
      showToast('info', 'Hi', 'msg');

      expect(loggerInfo).toHaveBeenCalledWith('[toast:info] Hi: msg');
    });

    it('falls back with an empty title (fallback)', () => {
      showToast('info', '');

      expect(loggerInfo).toHaveBeenCalledWith('[toast:info] ');
    });
  });

  describe('convenience wrappers', () => {
    type ToastWrapper = (title: string, message?: string, duration?: number) => string | undefined;
    const wrapperCases: Array<{ name: string; fn: ToastWrapper; type: string }> = [
      { name: 'showSuccess', fn: showSuccess, type: 'success' },
      { name: 'showError', fn: showError, type: 'error' },
      { name: 'showWarning', fn: showWarning, type: 'warning' },
    ];

    it.each(wrapperCases)(
      '$name delegates to showToast with type "$type"',
      ({ fn, type }) => {
        const mock = vi.fn(() => 'id');
        (window as unknown as { showToast: typeof mock }).showToast = mock;

        const result = fn('Title', 'Message', 3000);

        expect(mock).toHaveBeenCalledWith(type, 'Title', 'Message', 3000, undefined);
        expect(result).toBe('id');
      },
    );

    it('showErrorWithDetail delegates with message undefined and detail as the 5th arg', () => {
      const mock = vi.fn(() => 'id');
      (window as unknown as { showToast: typeof mock }).showToast = mock;

      const result = showErrorWithDetail('Boom', 'stack trace', 4000);

      expect(mock).toHaveBeenCalledWith('error', 'Boom', undefined, 4000, 'stack trace');
      expect(result).toBe('id');
    });

    it('showErrorWithDetail works without duration or detail', () => {
      const mock = vi.fn(() => 'id');
      (window as unknown as { showToast: typeof mock }).showToast = mock;

      showErrorWithDetail('Boom');

      expect(mock).toHaveBeenCalledWith('error', 'Boom', undefined, undefined, undefined);
    });

    it('passes duration=0 through to window.showToast', () => {
      const mock = vi.fn(() => 'id');
      (window as unknown as { showToast: typeof mock }).showToast = mock;

      showSuccess('Title', 'Message', 0);

      expect(mock).toHaveBeenCalledWith('success', 'Title', 'Message', 0, undefined);
    });

    it('convenience wrappers fall back to logger when the toast system is not ready', () => {
      showSuccess('Saved', 'done');
      showError('Failed', 'broken');
      showWarning('Careful', 'careful');
      showErrorWithDetail('Boom', 'trace');

      expect(loggerInfo).toHaveBeenCalledWith('[toast:success] Saved: done');
      expect(loggerInfo).toHaveBeenCalledWith('[toast:error] Failed: broken');
      expect(loggerInfo).toHaveBeenCalledWith('[toast:warning] Careful: careful');
      expect(loggerInfo).toHaveBeenCalledWith('[toast:error] Boom');
    });
  });
});
