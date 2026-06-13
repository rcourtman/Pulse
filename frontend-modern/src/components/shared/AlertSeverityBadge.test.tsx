import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { AlertSeverityBadge, AlertSeverityDot } from '@/components/shared/AlertSeverityBadge';

afterEach(cleanup);

describe('AlertSeverityBadge', () => {
  it('renders critical alert severity through the shared status badge primitive', () => {
    render(() => <AlertSeverityBadge severity="CRITICAL" bucket="critical" />);

    const badge = screen.getByText('Critical');
    expect(badge).toHaveClass('inline-flex');
    expect(badge).toHaveClass('rounded-full');
    expect(badge).toHaveClass('text-[10px]');
    expect(badge).toHaveClass('bg-red-100');
    expect(badge).toHaveClass('text-red-700');
  });

  it('uses the severity bucket for tone while preserving provider severity text', () => {
    render(() => <AlertSeverityBadge severity="restart_loop" bucket="warning" />);

    const badge = screen.getByText('Restart Loop');
    expect(badge).toHaveClass('bg-amber-100');
    expect(badge).toHaveClass('text-amber-700');
  });

  it('renders alert severity dots through the shared status dot primitive', () => {
    render(() => <AlertSeverityDot severity="Image update" bucket="info" />);

    const dot = screen.getByTitle('Image Update');
    expect(dot).toHaveClass('inline-block');
    expect(dot).toHaveClass('h-2');
    expect(dot).toHaveClass('bg-slate-400');
  });
});
