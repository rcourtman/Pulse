import { afterEach, describe, expect, it, vi } from 'vitest';
import { batch } from 'solid-js';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Toast, ToastContainer } from '@/components/Toast/Toast';

describe('Toast', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllTimers();
    vi.useRealTimers();
  });

  it('keeps all toasts created within a batch', () => {
    render(() => <ToastContainer />);

    batch(() => {
      window.showToast('info', 'First toast');
      window.showToast('success', 'Second toast');
    });

    expect(screen.getByText('First toast')).toBeInTheDocument();
    expect(screen.getByText('Second toast')).toBeInTheDocument();
  });

  it('does not schedule a second removal after manual close', () => {
    vi.useFakeTimers();
    const onRemove = vi.fn();

    render(() => (
      <Toast
        toast={{ id: 'toast-1', type: 'info', title: 'Manual close toast', duration: 1000 }}
        onRemove={onRemove}
      />
    ));

    fireEvent.click(screen.getByRole('button'));

    vi.advanceTimersByTime(300);
    expect(onRemove).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(2000);
    expect(onRemove).toHaveBeenCalledTimes(1);
  });
});
