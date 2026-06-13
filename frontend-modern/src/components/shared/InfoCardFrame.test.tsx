import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { InfoCardFrame, getInfoCardFrameClass } from './InfoCardFrame';

describe('InfoCardFrame', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders the canonical compact information card frame', () => {
    render(() => (
      <InfoCardFrame data-testid="card">
        <span>Storage</span>
      </InfoCardFrame>
    ));

    const card = screen.getByTestId('card');

    expect(card).toHaveClass('rounded');
    expect(card).toHaveClass('border');
    expect(card).toHaveClass('border-border');
    expect(card).toHaveClass('bg-surface');
    expect(card).toHaveClass('p-3');
    expect(card).toHaveClass('shadow-sm');
    expect(screen.getByText('Storage')).toBeInTheDocument();
  });

  it('composes contextual classes without leaking local props', () => {
    render(() => (
      <InfoCardFrame data-testid="card" class="w-full grow">
        Context
      </InfoCardFrame>
    ));

    const card = screen.getByTestId('card');

    expect(card).toHaveClass('w-full');
    expect(card).toHaveClass('grow');
    expect(card).not.toHaveAttribute('className');
  });

  it('exposes the shared class model for presentation constants', () => {
    expect(getInfoCardFrameClass({ class: 'text-center' })).toContain('text-center');
    expect(getInfoCardFrameClass()).toContain('bg-surface');
  });
});
