import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';

import { PageHeader } from '@/components/shared/PageHeader';

describe('PageHeader', () => {
  it('keeps a compact phone heading and action on one responsive row', () => {
    const { container } = render(() => (
      <PageHeader
        title="Alerts Overview"
        description="Current alert delivery and incident state."
        actions={<button type="button">Pause</button>}
      />
    ));

    const header = container.querySelector('[data-page-header]');
    expect(header).toHaveClass('flex-row', 'items-start', 'justify-between', 'gap-2', 'sm:gap-4');

    const heading = screen.getByRole('heading', { name: 'Alerts Overview' });
    expect(heading).toHaveClass('text-xl', 'sm:text-2xl');

    const action = screen.getByRole('button', { name: 'Pause' }).parentElement;
    expect(action).toHaveClass('w-auto', 'shrink-0');
  });
});
