import { fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import helpIconSource from '@/components/shared/HelpIcon.tsx?raw';
import helpIconModelSource from '@/components/shared/helpIconModel.ts?raw';
import helpIconStateSource from '@/components/shared/useHelpIconState.ts?raw';
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
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('keeps the help icon on shell, runtime, and model owners', () => {
    expect(helpIconSource).toContain('useHelpIconState');
    expect(helpIconSource).not.toContain('getHelpContent(');
    expect(helpIconSource).not.toContain('requestAnimationFrame');
    expect(helpIconSource).not.toContain('createSignal');

    expect(helpIconStateSource).toContain('requestAnimationFrame');
    expect(helpIconStateSource).toContain('document.addEventListener');
    expect(helpIconStateSource).toContain('export function useHelpIconState');
    expect(helpIconStateSource).toContain('createSignal');

    expect(helpIconModelSource).toContain('resolveHelpContent');
    expect(helpIconModelSource).toContain('calculateHelpPopoverPosition');
    expect(helpIconModelSource).toContain('helpIconSizeClasses');
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

  it('uses distinct labels when more than one help dialog is open', async () => {
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
    fireEvent.click(screen.getByRole('button', { name: 'Help: Second help' }));

    const first = await screen.findByRole('dialog', { name: 'First help' });
    const second = await screen.findByRole('dialog', { name: 'Second help' });
    expect(first.id).not.toBe(second.id);
    expect(first.getAttribute('aria-labelledby')).not.toBe(second.getAttribute('aria-labelledby'));
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
