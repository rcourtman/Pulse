import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { SummaryRowActionButton } from '@/components/shared/SummaryRowActionButton';

describe('SummaryRowActionButton', () => {
  it('owns disclosure semantics with aria-expanded and aria-controls', () => {
    const onAction = vi.fn();
    render(() => (
      <SummaryRowActionButton
        kind="disclosure"
        subjectLabel="alpha"
        expanded={true}
        controlsId="summary-row-detail-alpha"
        onAction={onAction}
      />
    ));

    const button = screen.getByRole('button', { name: 'Collapse alpha' });
    expect(button).toHaveAttribute('aria-expanded', 'true');
    expect(button).toHaveAttribute('aria-controls', 'summary-row-detail-alpha');

    fireEvent.click(button);
    expect(onAction).toHaveBeenCalledTimes(1);
  });

  it('owns pinned scope semantics with aria-pressed', () => {
    const onAction = vi.fn();
    render(() => (
      <SummaryRowActionButton
        kind="scope"
        subjectLabel="tower"
        pressed={true}
        onAction={onAction}
      />
    ));

    const button = screen.getByRole('button', {
      name: 'Unpin summary scope for tower',
    });
    expect(button).toHaveAttribute('aria-pressed', 'true');
    expect(button).toHaveTextContent('Pinned');

    fireEvent.click(button);
    expect(onAction).toHaveBeenCalledTimes(1);
  });

  it('owns global context pin semantics with aria-pressed', () => {
    const onAction = vi.fn();
    render(() => (
      <SummaryRowActionButton
        kind="context"
        subjectLabel="pve1"
        pressed={false}
        onAction={onAction}
      />
    ));

    const button = screen.getByRole('button', {
      name: 'Set global context to pve1',
    });
    expect(button).toHaveAttribute('aria-pressed', 'false');

    fireEvent.click(button);
    expect(onAction).toHaveBeenCalledTimes(1);
  });

  it('clears preview and blurs on Escape', () => {
    const onPreviewClear = vi.fn();
    render(() => (
      <SummaryRowActionButton
        kind="scope"
        subjectLabel="tower"
        pressed={false}
        onAction={vi.fn()}
        onPreviewClear={onPreviewClear}
      />
    ));

    const button = screen.getByRole('button', {
      name: 'Pin summary scope for tower',
    }) as HTMLButtonElement;
    button.blur = vi.fn();

    fireEvent.keyDown(button, { key: 'Escape' });
    expect(onPreviewClear).toHaveBeenCalledTimes(1);
    expect(button.blur).toHaveBeenCalledTimes(1);
  });

  it('activates disclosure buttons from keyboard space', () => {
    const onAction = vi.fn();
    render(() => (
      <SummaryRowActionButton
        kind="disclosure"
        subjectLabel="alpha"
        expanded={false}
        controlsId="summary-row-detail-alpha"
        onAction={onAction}
      />
    ));

    const button = screen.getByRole('button', { name: 'Expand alpha' });
    fireEvent.keyDown(button, { key: 'Space', code: 'Space' });
    expect(onAction).toHaveBeenCalledTimes(1);
  });
});
