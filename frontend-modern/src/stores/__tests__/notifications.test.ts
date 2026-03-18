import { describe, expect, it, vi, beforeEach } from 'vitest';
import { notificationStore } from '../notifications';

vi.mock('@/utils/toast', () => ({
  showToast: vi.fn().mockReturnValue('toast-id'),
}));

import { showToast } from '@/utils/toast';

describe('notificationStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('success', () => {
    it('shows success toast with default duration', () => {
      vi.mocked(showToast).mockReturnValue('toast-id');

      const result = notificationStore.success('Operation completed');

      expect(showToast).toHaveBeenCalledWith('success', 'Operation completed', undefined, 5000);
      expect(result).toBe('toast-id');
    });

    it('shows success toast with custom duration', () => {
      vi.mocked(showToast).mockReturnValue('toast-id');

      const result = notificationStore.success('Operation completed', 10000);

      expect(showToast).toHaveBeenCalledWith('success', 'Operation completed', undefined, 10000);
      expect(result).toBe('toast-id');
    });
  });

  describe('error', () => {
    it('shows error toast with default duration', () => {
      vi.mocked(showToast).mockReturnValue('toast-id');

      const result = notificationStore.error('Something went wrong');

      expect(showToast).toHaveBeenCalledWith('error', 'Something went wrong', undefined, 10000);
      expect(result).toBe('toast-id');
    });

    it('shows error toast with custom duration', () => {
      vi.mocked(showToast).mockReturnValue('toast-id');

      const result = notificationStore.error('Error', 20000);

      expect(showToast).toHaveBeenCalledWith('error', 'Error', undefined, 20000);
      expect(result).toBe('toast-id');
    });
  });

  describe('info', () => {
    it('shows info toast with default duration', () => {
      vi.mocked(showToast).mockReturnValue('toast-id');

      const result = notificationStore.info('Information');

      expect(showToast).toHaveBeenCalledWith('info', 'Information', undefined, 5000);
      expect(result).toBe('toast-id');
    });
  });

  describe('warning', () => {
    it('shows warning toast with info duration', () => {
      vi.mocked(showToast).mockReturnValue('toast-id');

      const result = notificationStore.warning('Warning message');

      expect(showToast).toHaveBeenCalledWith('warning', 'Warning message', undefined, 5000);
      expect(result).toBe('toast-id');
    });
  });
});
