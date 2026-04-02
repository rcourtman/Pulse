import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { SummaryScopeBar } from '@/components/shared/SummaryScopeBar';
import type { SummaryScopePresentation } from '@/components/shared/summaryScopePresentation';

const renderScopeBar = (active: SummaryScopePresentation, options?: {
  idleHint?: string;
  onReset?: () => void;
  pinned?: SummaryScopePresentation | null;
}) =>
  render(() => (
    <SummaryScopeBar
      testId="summary-scope"
      active={active}
      idleHint={options?.idleHint}
      onReset={options?.onReset}
      pinned={options?.pinned ?? null}
    />
  ));

describe('SummaryScopeBar', () => {
  it('renders all scope as a quiet context line without a scope badge', () => {
    renderScopeBar({
      kind: 'page',
      label: 'All workloads',
      contextLabel: null,
      mode: 'all',
    }, {
      idleHint: 'Tap a group or row to pin scope.',
    });

    const scope = screen.getByTestId('summary-scope');
    expect(scope).toHaveTextContent('Showing');
    expect(scope).toHaveTextContent('All workloads');
    expect(scope).toHaveTextContent('Tap a group or row to pin scope.');
    expect(screen.queryByText('Scope')).not.toBeInTheDocument();
    expect(scope.querySelector('.rounded-full')).toBeNull();
  });

  it('distinguishes preview from a pinned fallback without reintroducing chrome', () => {
    renderScopeBar(
      {
        kind: 'entity',
        label: 'finance-jump-01',
        contextLabel: 'Production cluster',
        mode: 'preview',
      },
      {
        pinned: {
          kind: 'group',
          label: 'Production cluster',
          contextLabel: null,
          mode: 'pinned',
        },
      },
    );

    const scope = screen.getByTestId('summary-scope');
    expect(scope).toHaveTextContent('Previewing');
    expect(scope).toHaveTextContent('finance-jump-01');
    expect(scope).toHaveTextContent('Pinned to Production cluster');
    expect(scope.querySelector('.rounded-full')).toBeNull();
  });

  it('keeps the reset affordance explicit but visually quiet', () => {
    const onReset = vi.fn();
    renderScopeBar(
      {
        kind: 'group',
        label: 'tower',
        contextLabel: null,
        mode: 'pinned',
      },
      { onReset },
    );

    const button = screen.getByRole('button', { name: 'Reset pinned scope' });
    expect(button).toHaveTextContent('Reset');

    fireEvent.click(button);
    expect(onReset).toHaveBeenCalledTimes(1);
  });
});
