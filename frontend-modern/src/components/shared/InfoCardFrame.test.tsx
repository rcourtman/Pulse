import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import {
  InfoCardFrame,
  InfoCardKeyValueRow,
  getInfoCardFrameClass,
  getInfoCardKeyValueRowClass,
} from './InfoCardFrame';

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

  it('keeps values condensed on mobile and adjacent to a fixed label track on desktop', () => {
    render(() => (
      <InfoCardKeyValueRow
        data-testid="row"
        label="Hostname"
        value="apollo-114"
        valueTitle="Full hostname"
      />
    ));

    const row = screen.getByTestId('row');
    const value = screen.getByText('apollo-114');

    expect(row).toHaveClass('justify-between');
    expect(row).toHaveClass('lg:grid');
    expect(row).toHaveClass('lg:grid-cols-[7rem_minmax(0,1fr)]');
    expect(value).toHaveClass('text-right');
    expect(value).toHaveClass('lg:text-left');
    expect(value).toHaveAttribute('title', 'Full hostname');
  });

  it('can align values from the small breakpoint for compact split-card layouts', () => {
    expect(getInfoCardKeyValueRowClass({ desktopAt: 'sm' })).toContain('sm:grid');
    expect(getInfoCardKeyValueRowClass({ desktopAt: 'sm' })).toContain(
      'sm:grid-cols-[7rem_minmax(0,1fr)]',
    );
    expect(getInfoCardKeyValueRowClass({ desktopAt: 'sm' })).not.toContain('lg:grid');
  });
});
