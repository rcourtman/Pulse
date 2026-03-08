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

  it('renders detail field in a collapsible details element', () => {
    const onRemove = vi.fn();
    render(() => (
      <Toast
        toast={{
          id: 'err-1',
          type: 'error',
          title: 'Could not connect to the server',
          detail: 'dial tcp smtp.gmail.com:587: connect: connection refused',
        }}
        onRemove={onRemove}
      />
    ));

    // Title is visible
    expect(screen.getByText('Could not connect to the server')).toBeInTheDocument();

    // Detail is inside a <details> element (collapsed by default)
    const details = document.querySelector('details');
    expect(details).toBeInTheDocument();
    expect(details!.open).toBe(false);

    // The "Details" summary is visible
    expect(screen.getByText('Details')).toBeInTheDocument();

    // The raw error text is in the DOM but inside the collapsed details
    expect(
      screen.getByText('dial tcp smtp.gmail.com:587: connect: connection refused'),
    ).toBeInTheDocument();
  });

  it('renders message as visible subtitle without collapsing', () => {
    const onRemove = vi.fn();
    render(() => (
      <Toast
        toast={{
          id: 'err-2',
          type: 'error',
          title: 'Unable to report merge',
          message: 'The node is currently offline',
        }}
        onRemove={onRemove}
      />
    ));

    // No details element when only message is set (no detail)
    expect(document.querySelector('details')).not.toBeInTheDocument();

    // Message is directly visible as a subtitle
    expect(screen.getByText('The node is currently offline')).toBeInTheDocument();
  });
});
