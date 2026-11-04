import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { DrawerAttentionSection, type DrawerAttentionItem } from '../DrawerAttentionSection';

const items = (count: number): DrawerAttentionItem[] =>
  Array.from({ length: count }, (_, index) => ({
    id: `alert-${index + 1}`,
    subject: `guest-${index + 1}`,
    metric: 'Memory',
    message: `Memory ${90 + index}%, above the 85% alert level`,
    severity: 'critical',
  }));

const renderedSubjects = () =>
  within(screen.getByRole('list', { name: 'Active alerts' }))
    .getAllByRole('listitem')
    .map((row) => row.textContent?.match(/guest-\d+/)?.[0]);

describe('DrawerAttentionSection', () => {
  afterEach(cleanup);

  it('renders nothing without alerts', () => {
    render(() => <DrawerAttentionSection items={[]} />);
    expect(screen.queryByTestId('drawer-attention-section')).not.toBeInTheDocument();
  });

  it('fills its drawer and flows alerts by the width it has', () => {
    render(() => <DrawerAttentionSection items={items(3)} />);

    const section = screen.getByTestId('drawer-attention-section');
    expect(section).toHaveClass('w-full');
    expect(section.className).not.toMatch(/max-w-/);

    // A browser pass measures the columns; the classes that produce them are what jsdom can see.
    expect(screen.getByRole('list', { name: 'Active alerts' })).toHaveClass('flex', 'flex-wrap');
    for (const row of screen.getAllByRole('listitem')) {
      expect(row).toHaveClass('flex-[1_1_min(100%,20rem)]');
    }
  });

  it('keeps long lists bounded until the operator opens them', async () => {
    render(() => <DrawerAttentionSection items={items(5)} />);

    expect(screen.getByText('5 active')).toBeInTheDocument();
    expect(renderedSubjects()).toEqual(['guest-1', 'guest-2', 'guest-3']);

    const more = screen.getByRole('button', { name: 'Show 2 more alerts' });
    expect(more).toHaveAttribute('aria-expanded', 'false');
    await fireEvent.click(more);

    expect(renderedSubjects()).toEqual(['guest-1', 'guest-2', 'guest-3', 'guest-4', 'guest-5']);
    expect(screen.getByRole('button', { name: 'Show fewer alerts' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
  });

  it('says an acknowledged alert is acknowledged rather than repeating its severity', () => {
    render(() => <DrawerAttentionSection items={[{ ...items(1)[0], acknowledged: true }]} />);

    expect(screen.getByText('Acknowledged')).toBeInTheDocument();
  });
});
