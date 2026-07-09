import { cleanup, render } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { For } from 'solid-js';
import { getPlatformAlertSeverityFilterOptions } from '../platformAlertSeverityFilterOptions';

afterEach(cleanup);

describe('getPlatformAlertSeverityFilterOptions', () => {
  it('returns the canonical platform alert severity filter labels and tones', () => {
    const options = getPlatformAlertSeverityFilterOptions();

    expect(options.map(({ value, label, tone }) => ({ value, label, tone }))).toEqual([
      { value: 'all', label: 'All', tone: undefined },
      { value: 'critical', label: 'Critical', tone: 'danger' },
      { value: 'warning', label: 'Warning', tone: 'warning' },
      { value: 'info', label: 'Info', tone: 'success' },
    ]);
  });

  it('renders the canonical severity filter leading dots', () => {
    const options = getPlatformAlertSeverityFilterOptions();

    render(() => (
      <div>
        <For each={options}>{(option) => option.leading}</For>
      </div>
    ));

    const dots = Array.from(document.querySelectorAll('span[aria-hidden="true"]'));
    expect(dots).toHaveLength(3);
    expect(dots[0]).toHaveClass('bg-red-500');
    expect(dots[1]).toHaveClass('bg-amber-500');
    expect(dots[2]).toHaveClass('bg-emerald-500');
  });

  it('adds the aggregate attention filter only when requested', () => {
    const options = getPlatformAlertSeverityFilterOptions({ includeAttention: true });

    expect(options.map((option) => option.value)).toEqual([
      'all',
      'attention',
      'critical',
      'warning',
      'info',
    ]);
  });
});
