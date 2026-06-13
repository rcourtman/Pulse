import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { LoadingSpinner, getLoadingSpinnerClass } from '@/components/shared/LoadingSpinner';

afterEach(cleanup);

describe('LoadingSpinner', () => {
  it('renders the canonical decorative spinner shell by default', () => {
    render(() => <LoadingSpinner />);

    const spinner = document.querySelector('span');

    expect(spinner).toHaveClass('inline-block');
    expect(spinner).toHaveClass('shrink-0');
    expect(spinner).toHaveClass('animate-spin');
    expect(spinner).toHaveClass('rounded-full');
    expect(spinner).toHaveClass('border-t-transparent');
    expect(spinner).toHaveClass('h-3');
    expect(spinner).toHaveClass('w-3');
    expect(spinner).toHaveClass('border-2');
    expect(spinner).toHaveClass('border-current');
    expect(spinner).toHaveAttribute('aria-hidden', 'true');
    expect(spinner).not.toHaveAttribute('role');
  });

  it('supports accessible status labels without leaking local props to the DOM', () => {
    render(() => (
      <LoadingSpinner
        data-testid="spinner"
        size="lg"
        tone="info"
        label="Loading findings"
        class="mx-1"
      />
    ));

    const spinner = screen.getByRole('status', { name: 'Loading findings' });

    expect(spinner).toHaveClass('h-12');
    expect(spinner).toHaveClass('w-12');
    expect(spinner).toHaveClass('border-4');
    expect(spinner).toHaveClass('border-blue-500');
    expect(spinner).toHaveClass('mx-1');
    expect(spinner).not.toHaveAttribute('aria-hidden');
    expect(spinner).not.toHaveAttribute('size');
    expect(spinner).not.toHaveAttribute('tone');
    expect(spinner).not.toHaveAttribute('label');
  });

  it('keeps size and tone class composition in the shared primitive model', () => {
    expect(getLoadingSpinnerClass({ size: 'xs', tone: 'muted', class: 'align-middle' })).toContain(
      'h-2',
    );
    expect(getLoadingSpinnerClass({ size: 'xs', tone: 'muted', class: 'align-middle' })).toContain(
      'border-slate-400',
    );
    expect(getLoadingSpinnerClass({ size: 'xs', tone: 'muted', class: 'align-middle' })).toContain(
      'align-middle',
    );
    expect(getLoadingSpinnerClass({ size: 'xl', tone: 'info' })).toContain('h-6');
    expect(getLoadingSpinnerClass({ size: 'xl', tone: 'info' })).toContain('border-blue-500');
    expect(getLoadingSpinnerClass({ size: 'button', tone: 'inverse' })).toContain('h-5');
    expect(getLoadingSpinnerClass({ size: 'button', tone: 'inverse' })).toContain('border-white');
  });
});
