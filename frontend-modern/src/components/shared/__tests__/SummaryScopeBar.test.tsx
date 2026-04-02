import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { SummaryScopeBar } from '@/components/shared/SummaryScopeBar';
import type { SummaryScopePresentation } from '@/components/shared/summaryScopePresentation';

const renderScopeBar = (scope: SummaryScopePresentation, options?: {
  onReset?: () => void;
}) =>
  render(() => (
    <SummaryScopeBar
      testId="summary-scope"
      scope={scope}
      onReset={options?.onReset}
    />
  ));

describe('SummaryScopeBar', () => {
  it('renders pinned scope as a compact inline context control', () => {
    renderScopeBar({
      kind: 'group',
      label: 'Production cluster',
      contextLabel: null,
      mode: 'pinned',
    });

    const scope = screen.getByTestId('summary-scope');
    expect(scope).toHaveTextContent('Pinned to');
    expect(scope).toHaveTextContent('Production cluster');
    expect(scope).not.toHaveTextContent('Previewing');
    expect(scope).not.toHaveTextContent('Showing');
  });

  it('keeps contextual group labels as secondary helper text', () => {
    renderScopeBar({
      kind: 'entity',
      label: 'finance-jump-01',
      contextLabel: 'Production cluster',
      mode: 'pinned',
    });

    const scope = screen.getByTestId('summary-scope');
    expect(scope).toHaveTextContent('Pinned to');
    expect(scope).toHaveTextContent('finance-jump-01');
    expect(scope).toHaveTextContent('Within Production cluster');
  });

  it('keeps the clear affordance explicit but quiet', () => {
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
