import { cleanup, render } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { PlatformIssueReason } from '@/features/platformPage/PlatformIssueReason';

afterEach(() => {
  cleanup();
});

describe('PlatformIssueReason', () => {
  it('leaves a healthy row empty', () => {
    const { container } = render(() => <PlatformIssueReason issue={null} />);
    expect(container.textContent).toBe('');
  });

  it('shows the first reason, counts the rest, and reads every reason out', () => {
    const { container } = render(() => (
      <PlatformIssueReason
        issue={{
          tone: 'warning',
          label: 'Attention',
          reasons: ['Datastore latency above threshold', 'vCenter has not updated this recently.'],
        }}
        data-testid="reason"
      />
    ));
    const cell = container.querySelector('[data-testid="reason"]') as HTMLElement;
    expect(cell).toHaveClass('text-amber-700');
    expect(cell).toHaveAttribute(
      'title',
      'Datastore latency above threshold\nvCenter has not updated this recently.',
    );
    const visible = [...cell.querySelectorAll('[aria-hidden="true"]')].map((el) => el.textContent);
    expect(visible).toEqual(['Datastore latency above threshold', '+1']);
    expect(cell.querySelector('.sr-only')).toHaveTextContent(
      'Attention: Datastore latency above threshold. vCenter has not updated this recently.',
    );
  });

  it('colours a down row red and omits the count for a single reason', () => {
    const { container } = render(() => (
      <PlatformIssueReason
        issue={{
          tone: 'danger',
          label: 'Inaccessible',
          reasons: ['vCenter reports it inaccessible'],
        }}
        data-testid="reason"
      />
    ));
    const cell = container.querySelector('[data-testid="reason"]') as HTMLElement;
    expect(cell).toHaveClass('text-red-600');
    expect(cell.querySelectorAll('[aria-hidden="true"]')).toHaveLength(1);
  });
});
