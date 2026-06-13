import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import {
  SettingsLoadingSkeleton,
  SettingsSkeletonBlock,
  SettingsSkeletonMetricGrid,
  SettingsSkeletonProgressCard,
  SettingsSkeletonTable,
} from '@/components/shared/SettingsLoadingSkeleton';

afterEach(cleanup);

describe('SettingsLoadingSkeleton', () => {
  it('renders a labelled settings loading status shell', () => {
    render(() => (
      <SettingsLoadingSkeleton label="Loading organization access" padding="panel">
        <SettingsSkeletonBlock class="h-4 w-24" />
      </SettingsLoadingSkeleton>
    ));

    const shell = screen.getByRole('status', { name: 'Loading organization access' });
    const block = shell.querySelector('.animate-pulse');

    expect(shell).toHaveClass('space-y-5');
    expect(shell).toHaveClass('p-4');
    expect(shell).toHaveClass('sm:p-6');
    expect(block).toHaveClass('bg-surface-hover');
    expect(block).toHaveAttribute('aria-hidden', 'true');
  });

  it('renders canonical metric, progress, and table skeleton sections', () => {
    render(() => (
      <SettingsLoadingSkeleton>
        <SettingsSkeletonMetricGrid count={2} columns="two" valueWidth="w-40" />
        <SettingsSkeletonProgressCard rows={1} />
        <SettingsSkeletonTable
          rows={2}
          titleWidth="w-28"
          cells={[{ class: 'h-4 w-40' }, { class: 'h-4 w-14', radius: 'full' }]}
        />
      </SettingsLoadingSkeleton>
    ));

    expect(document.querySelectorAll('.rounded-md.border.border-border')).toHaveLength(4);
    expect(document.querySelectorAll('.border-t.border-border-subtle')).toHaveLength(2);
    expect(document.querySelector('.rounded-full')).toBeTruthy();
    expect(document.querySelector('.bg-surface-alt')).toBeTruthy();
  });
});
