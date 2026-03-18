import { describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach } from 'vitest';
import { ExploreStatusBlock } from '../ExploreStatusBlock';
import type { ExploreStatus } from '../types';

afterEach(cleanup);

function makeStatus(overrides?: Partial<ExploreStatus>): ExploreStatus {
  return {
    phase: 'started',
    message: 'Exploring infrastructure...',
    ...overrides,
  };
}

describe('ExploreStatusBlock', () => {
  // --- Phase label rendering ---

  it('renders "Explore Started" for started phase', () => {
    render(() => <ExploreStatusBlock status={makeStatus({ phase: 'started' })} />);
    expect(screen.getByText('Explore Started')).toBeInTheDocument();
  });

  it('renders "Explore Completed" for completed phase', () => {
    render(() => <ExploreStatusBlock status={makeStatus({ phase: 'completed' })} />);
    expect(screen.getByText('Explore Completed')).toBeInTheDocument();
  });

  it('renders "Explore Failed" for failed phase', () => {
    render(() => <ExploreStatusBlock status={makeStatus({ phase: 'failed' })} />);
    expect(screen.getByText('Explore Failed')).toBeInTheDocument();
  });

  it('renders "Explore Skipped" for skipped phase', () => {
    render(() => <ExploreStatusBlock status={makeStatus({ phase: 'skipped' })} />);
    expect(screen.getByText('Explore Skipped')).toBeInTheDocument();
  });

  it('renders "Explore Status" for unknown phase', () => {
    render(() => <ExploreStatusBlock status={makeStatus({ phase: 'unknown_phase' })} />);
    expect(screen.getByText('Explore Status')).toBeInTheDocument();
  });

  // --- Message rendering ---

  it('renders the status message', () => {
    render(() => <ExploreStatusBlock status={makeStatus({ message: 'Scanning 12 nodes...' })} />);
    expect(screen.getByText('Scanning 12 nodes...')).toBeInTheDocument();
  });

  // --- Optional model field ---

  it('renders model name when provided', () => {
    render(() => <ExploreStatusBlock status={makeStatus({ model: 'gpt-4o' })} />);
    expect(screen.getByText('gpt-4o')).toBeInTheDocument();
  });

  it('does not render model span when model is undefined', () => {
    const { container } = render(() => (
      <ExploreStatusBlock status={makeStatus({ model: undefined })} />
    ));
    // With no model and no outcome, the header row should only contain the phase label
    const headerRow = container.querySelector('.flex');
    expect(headerRow).not.toBeNull();
    expect(headerRow!.children.length).toBe(1); // only phase label span
  });

  // --- Optional outcome field ---

  it('renders outcome when provided', () => {
    render(() => <ExploreStatusBlock status={makeStatus({ outcome: 'success' })} />);
    expect(screen.getByText('outcome=success')).toBeInTheDocument();
  });

  it('does not render outcome when undefined', () => {
    const { container } = render(() => (
      <ExploreStatusBlock status={makeStatus({ outcome: undefined })} />
    ));
    expect(container.textContent).not.toContain('outcome=');
  });

  it('renders both model and outcome together', () => {
    render(() => (
      <ExploreStatusBlock status={makeStatus({ model: 'claude-3', outcome: 'partial' })} />
    ));
    expect(screen.getByText('claude-3')).toBeInTheDocument();
    expect(screen.getByText('outcome=partial')).toBeInTheDocument();
  });

  // --- CSS class application per phase ---

  it('applies emerald classes for completed phase', () => {
    const { container } = render(() => (
      <ExploreStatusBlock status={makeStatus({ phase: 'completed' })} />
    ));
    const block = container.firstElementChild as HTMLElement;
    expect(block.classList.contains('bg-emerald-50')).toBe(true);
    expect(block.classList.contains('border-emerald-200')).toBe(true);
  });

  it('applies rose classes for failed phase', () => {
    const { container } = render(() => (
      <ExploreStatusBlock status={makeStatus({ phase: 'failed' })} />
    ));
    const block = container.firstElementChild as HTMLElement;
    expect(block.classList.contains('bg-rose-50')).toBe(true);
    expect(block.classList.contains('border-rose-200')).toBe(true);
  });

  it('applies amber classes for skipped phase', () => {
    const { container } = render(() => (
      <ExploreStatusBlock status={makeStatus({ phase: 'skipped' })} />
    ));
    const block = container.firstElementChild as HTMLElement;
    expect(block.classList.contains('bg-amber-50')).toBe(true);
    expect(block.classList.contains('border-amber-200')).toBe(true);
  });

  it('applies sky classes for started phase', () => {
    const { container } = render(() => (
      <ExploreStatusBlock status={makeStatus({ phase: 'started' })} />
    ));
    const block = container.firstElementChild as HTMLElement;
    expect(block.classList.contains('bg-sky-50')).toBe(true);
    expect(block.classList.contains('border-sky-200')).toBe(true);
  });

  it('applies sky (default) classes for unrecognized phase', () => {
    const { container } = render(() => (
      <ExploreStatusBlock status={makeStatus({ phase: 'some_new_phase' })} />
    ));
    const block = container.firstElementChild as HTMLElement;
    expect(block.classList.contains('bg-sky-50')).toBe(true);
  });

  // --- Empty string edge cases ---

  it('renders empty message without crashing', () => {
    const { container } = render(() => <ExploreStatusBlock status={makeStatus({ message: '' })} />);
    // The wrapper and message paragraph should still render
    expect(container.firstElementChild).not.toBeNull();
    const messageParagraph = container.querySelector('p');
    expect(messageParagraph).not.toBeNull();
    expect(messageParagraph!.textContent).toBe('');
  });

  it('does not render model span for empty string (falsy)', () => {
    // Empty string is falsy in JS, so model span should NOT render
    const { container } = render(() => <ExploreStatusBlock status={makeStatus({ model: '' })} />);
    const headerRow = container.querySelector('.flex');
    expect(headerRow).not.toBeNull();
    // Only phase label span should be present (no model, no outcome)
    expect(headerRow!.children.length).toBe(1);
  });

  it('renders empty outcome string (falsy, should not show outcome)', () => {
    const { container } = render(() => <ExploreStatusBlock status={makeStatus({ outcome: '' })} />);
    expect(container.textContent).not.toContain('outcome=');
  });
});
