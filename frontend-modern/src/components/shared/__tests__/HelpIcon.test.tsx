import { fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSignal, Show } from 'solid-js';
import { HelpIcon } from '@/components/shared/HelpIcon';

vi.mock('@/content/help', () => ({
  getHelpContent: vi.fn((id: string) =>
    id === 'alerts.thresholds.delay'
      ? {
          id,
          title: 'Delay threshold',
          description: 'Delay threshold help text',
          examples: ['5m', '10m'],
          docUrl: 'https://example.com/help',
        }
      : undefined,
  ),
}));

describe('HelpIcon', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  const deferOpeningFrames = () => {
    const frames = new Map<number, FrameRequestCallback>();
    let nextFrame = 0;
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      frames.set(++nextFrame, callback);
      return nextFrame;
    });
    vi.stubGlobal('cancelAnimationFrame', (id: number) => frames.delete(id));
    return () => {
      const pending = [...frames.values()];
      frames.clear();
      pending.forEach((callback) => callback(0));
    };
  };

  it('accepts Escape immediately, before any timer or opening focus frame', () => {
    vi.useFakeTimers();
    const flushFrames = deferOpeningFrames();
    render(() => <HelpIcon inline={{ title: 'Immediate help', description: 'Description' }} />);
    const trigger = screen.getByRole('button', { name: 'Help: Immediate help' });
    trigger.focus();
    fireEvent.click(trigger);
    expect(screen.getByRole('dialog', { name: 'Immediate help' })).toBeInTheDocument();
    expect(trigger).toHaveAttribute('aria-expanded', 'true');

    fireEvent.keyDown(trigger, { key: 'Escape' });

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    expect(trigger).not.toHaveAttribute('aria-controls');
    expect(trigger).toHaveFocus();
    vi.runOnlyPendingTimers();
    flushFrames();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it('honors an immediate outside click without a delayed focus steal', () => {
    vi.useFakeTimers();
    const flushFrames = deferOpeningFrames();
    render(() => (
      <>
        <button type="button">Outside help</button>
        <HelpIcon inline={{ title: 'Immediate help', description: 'Description' }} />
      </>
    ));
    const trigger = screen.getByRole('button', { name: 'Help: Immediate help' });
    const outside = screen.getByRole('button', { name: 'Outside help' });
    trigger.focus();
    fireEvent.click(trigger);
    outside.focus();
    fireEvent.click(outside);

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    vi.runOnlyPendingTimers();
    flushFrames();
    expect(outside).toHaveFocus();
  });

  it('keeps opening and inside clicks while allowing immediate trigger dismissal', () => {
    vi.useFakeTimers();
    const flushFrames = deferOpeningFrames();
    render(() => <HelpIcon inline={{ title: 'Immediate help', description: 'Description' }} />);
    const trigger = screen.getByRole('button', { name: 'Help: Immediate help' });
    trigger.focus();
    fireEvent.click(trigger);
    fireEvent.click(screen.getByText('Description'));
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    fireEvent.keyDown(document, { key: 'ArrowDown' });
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    fireEvent.click(trigger);

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    vi.runOnlyPendingTimers();
    flushFrames();
    expect(trigger).toHaveFocus();
    fireEvent.click(trigger);
    fireEvent.keyDown(trigger, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('removes immediate dismissal listeners and opening focus work on unmount', () => {
    vi.useFakeTimers();
    const flushFrames = deferOpeningFrames();
    const add = vi.spyOn(document, 'addEventListener');
    const remove = vi.spyOn(document, 'removeEventListener');
    const [shown, setShown] = createSignal(true);
    render(() => (
      <>
        <button type="button">Outside help</button>
        <Show when={shown()}>
          <HelpIcon inline={{ title: 'Removable help', description: 'Description' }} />
        </Show>
      </>
    ));
    fireEvent.click(screen.getByRole('button', { name: 'Help: Removable help' }));
    const listeners = add.mock.calls.filter(([type]) => type === 'click' || type === 'keydown');
    expect(listeners.some(([type]) => type === 'keydown')).toBe(true);
    expect(listeners.some(([type]) => type === 'click')).toBe(true);
    setShown(false);

    listeners.forEach(([type, handler]) => {
      expect(remove).toHaveBeenCalledWith(type, handler);
    });
    const outside = screen.getByRole('button', { name: 'Outside help' });
    outside.focus();
    vi.runOnlyPendingTimers();
    flushFrames();
    fireEvent.keyDown(document, { key: 'Escape' });
    fireEvent.click(outside);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(outside).toHaveFocus();
  });

  it('renders inline help content in the popover', async () => {
    render(() => (
      <HelpIcon
        inline={{
          title: 'Inline help',
          description: 'Inline description',
          examples: ['Example A'],
        }}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Help: Inline help' }));

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('Inline description')).toBeInTheDocument();
    expect(screen.getByText('Example A')).toBeInTheDocument();
  });

  it('moves focus into the portalled dialog and restores it when closed', async () => {
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      callback(0);
      return 1;
    });
    render(() => (
      <HelpIcon
        inline={{
          title: 'Inline help',
          description: 'Inline description',
        }}
      />
    ));

    const trigger = screen.getByRole('button', { name: 'Help: Inline help' });
    fireEvent.click(trigger);

    const dialog = await screen.findByRole('dialog', { name: 'Inline help' });
    const close = screen.getByRole('button', { name: 'Close help' });
    expect(close).toHaveFocus();
    expect(close).toHaveClass('min-h-11', 'min-w-11', 'sm:min-h-8', 'sm:min-w-8');
    expect(trigger).toHaveAttribute('aria-controls', dialog.id);
    expect(dialog).toHaveAttribute('aria-labelledby');
    expect(document.getElementById(dialog.getAttribute('aria-labelledby')!)).toHaveTextContent(
      'Inline help',
    );

    fireEvent.click(close);
    expect(screen.queryByRole('dialog', { name: 'Inline help' })).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it('uses distinct labels when moving between help triggers', async () => {
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      callback(0);
      return 1;
    });
    render(() => (
      <>
        <HelpIcon inline={{ title: 'First help', description: 'First description' }} />
        <HelpIcon inline={{ title: 'Second help', description: 'Second description' }} />
      </>
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Help: First help' }));
    const first = await screen.findByRole('dialog', { name: 'First help' });
    fireEvent.click(screen.getByRole('button', { name: 'Help: Second help' }));

    const second = await screen.findByRole('dialog', { name: 'Second help' });
    // The second trigger is outside the first help. Its click must dismiss
    // that help immediately, without relying on a listener-registration timer.
    expect(first).not.toBeInTheDocument();
    expect(first.id).not.toBe(second.id);
    expect(first.getAttribute('aria-labelledby')).not.toBe(second.getAttribute('aria-labelledby'));
    expect(document.getElementById(second.getAttribute('aria-labelledby')!)).toHaveTextContent(
      'Second help',
    );
  });

  it('contains the full dialog box through animation, resize and scrolling', async () => {
    let flushFrame: FrameRequestCallback | undefined;
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      flushFrame = callback;
      return 1;
    });
    vi.stubGlobal('cancelAnimationFrame', vi.fn());
    vi.stubGlobal('innerWidth', 240);
    vi.stubGlobal('innerHeight', 220);
    render(() => <HelpIcon inline={{ title: 'Responsive help', description: 'Description' }} />);

    const trigger = screen.getByRole('button', { name: 'Help: Responsive help' });
    vi.spyOn(trigger, 'getBoundingClientRect').mockReturnValue(new DOMRect(200, 150, 20, 20));
    fireEvent.click(trigger);
    const dialog = await screen.findByRole('dialog', { name: 'Responsive help' });
    Object.defineProperties(dialog, {
      offsetWidth: { value: 200 },
      offsetHeight: { value: 150 },
    });
    vi.spyOn(dialog, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 180, 135));
    flushFrame?.(0);
    expect(dialog.style.left).toBe('32px');
    expect(dialog.style.top).toBe('62px');

    vi.stubGlobal('innerWidth', 500);
    vi.stubGlobal('innerHeight', 600);
    fireEvent(window, new Event('resize'));
    expect(dialog.style.left).toBe('110px');
    expect(dialog.style.top).toBe('178px');

    vi.spyOn(trigger, 'getBoundingClientRect').mockReturnValue(new DOMRect(100, 350, 20, 20));
    fireEvent(window, new Event('scroll'));
    expect(dialog.style.left).toBe('10px');
    expect(dialog.style.top).toBe('192px');
    expect(screen.getByRole('button', { name: 'Close help' })).toHaveFocus();
  });
});
