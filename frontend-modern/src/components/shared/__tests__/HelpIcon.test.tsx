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
});
