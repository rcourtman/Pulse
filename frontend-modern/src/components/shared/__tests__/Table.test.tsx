import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import tableSource from '@/components/shared/Table.tsx?raw';

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('TableRow touch activation', () => {
  it('marks clickable rows as native click targets without changing row semantics', () => {
    const nativeListener = vi.spyOn(HTMLTableRowElement.prototype, 'addEventListener');
    const onClick = vi.fn();
    render(() => (
      <Table>
        <TableBody>
          <TableRow onClick={onClick}>
            <TableCell>Tap resource</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ));
    expect(nativeListener.mock.calls.some(([type]) => type === 'click')).toBe(true);
    const row = screen.getByRole('row');
    expect(row).not.toHaveAttribute('tabindex');
    expect(row).not.toHaveAttribute('aria-expanded');
    fireEvent.click(screen.getByText('Tap resource'));
    expect(onClick).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(row, { key: 'Enter' });
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('does not make static rows native click targets', () => {
    const nativeListener = vi.spyOn(HTMLTableRowElement.prototype, 'addEventListener');
    render(() => (
      <Table>
        <TableBody>
          <TableRow>
            <TableCell>Static resource</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ));
    expect(nativeListener.mock.calls.some(([type]) => type === 'click')).toBe(false);
  });

  it('lets delegated child actions stop the row before its action runs', () => {
    const onClick = vi.fn();
    const onChild = vi.fn();
    render(() => (
      <Table>
        <TableBody>
          <TableRow onClick={onClick}>
            <TableCell>
              <button
                type="button"
                onClick={(event) => {
                  event.stopPropagation();
                  onChild();
                }}
              >
                Child action
              </button>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ));
    fireEvent.click(screen.getByRole('button', { name: 'Child action' }));
    expect(onChild).toHaveBeenCalledTimes(1);
    expect(onClick).not.toHaveBeenCalled();
  });

  it('removes the native target when the row action is withdrawn', () => {
    const removeListener = vi.spyOn(HTMLTableRowElement.prototype, 'removeEventListener');
    const onClick = vi.fn();
    const [enabled, setEnabled] = createSignal(true);
    render(() => (
      <Table>
        <TableBody>
          <TableRow onClick={enabled() ? onClick : undefined}>
            <TableCell>Changing resource</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ));
    fireEvent.click(screen.getByText('Changing resource'));
    setEnabled(false);
    expect(removeListener.mock.calls.some(([type]) => type === 'click')).toBe(true);
    fireEvent.click(screen.getByText('Changing resource'));
    expect(onClick).toHaveBeenCalledTimes(1);
    setEnabled(true);
    fireEvent.click(screen.getByText('Changing resource'));
    expect(onClick).toHaveBeenCalledTimes(2);
  });

  it('preserves bound delegated handlers and caller-owned native handlers', () => {
    const onClick = vi.fn();
    const onNative = vi.fn();
    render(() => (
      <Table>
        <TableBody>
          <TableRow
            on:click={(event) => {
              event.stopPropagation();
              onNative();
            }}
          >
            <TableCell>Native action</TableCell>
          </TableRow>
          <TableRow onClick={[onClick, 'second-identity']}>
            <TableCell>Bound action</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ));
    fireEvent.click(screen.getByText('Native action'));
    expect(onNative).toHaveBeenCalledTimes(1);
    expect(onClick).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText('Bound action'));
    expect(onClick).toHaveBeenCalledTimes(1);
    expect(onClick.mock.calls[0][0]).toBe('second-identity');
  });
});

describe('TableBody', () => {
  it('keeps the shared table wrapper CSP-safe', () => {
    expect(tableSource).toContain('touch-scroll');
    expect(tableSource).toContain('table-scroll-shell');
    expect(tableSource).toContain('min-w-0 max-w-full');
    expect(tableSource).not.toContain('style={{');
    expect(tableSource).not.toContain('style={');
  });

  it('exposes an explicit phone page-scroll owner variant', () => {
    render(() => (
      <Table phoneVerticalScrollOwner="page">
        <tbody>
          <tr>
            <td>phone row</td>
          </tr>
        </tbody>
      </Table>
    ));

    expect(screen.getByText('phone row').closest('.table-scroll-shell')).toHaveClass(
      'table-scroll-shell-phone-page',
    );
  });

  it('keeps default dividers when no custom divider classes are provided', () => {
    render(() => (
      <Table>
        <TableBody>
          <TableRow>
            <TableCell>default</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ));

    const tbody = screen.getByText('default').closest('tbody');
    expect(tbody).not.toBeNull();
    expect(tbody!.className).toContain('divide-y');
    expect(tbody!.className).toContain('divide-border');
  });

  it('lets callers fully own divider classes when custom divider classes are provided', () => {
    render(() => (
      <Table>
        <TableBody class="divide-y divide-border-subtle/60">
          <TableRow>
            <TableCell>custom</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ));

    const tbody = screen.getByText('custom').closest('tbody');
    expect(tbody).not.toBeNull();
    expect(tbody!.className).toContain('divide-y');
    expect(tbody!.className).toContain('divide-border-subtle/60');
    expect(tbody!.className).not.toContain('divide-border ');
  });
});

describe('TableHeader', () => {
  it('keeps default header borders when no custom border classes are provided', () => {
    render(() => (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>default header</TableHead>
          </TableRow>
        </TableHeader>
      </Table>
    ));

    const thead = screen.getByText('default header').closest('thead');
    expect(thead).not.toBeNull();
    expect(thead!.className).toContain('border-b');
    expect(thead!.className).toContain('border-border');
  });

  it('lets callers own header border classes when custom border classes are provided', () => {
    render(() => (
      <Table>
        <TableHeader class="border-b border-border-subtle/60">
          <TableRow>
            <TableHead>custom header</TableHead>
          </TableRow>
        </TableHeader>
      </Table>
    ));

    const thead = screen.getByText('custom header').closest('thead');
    expect(thead).not.toBeNull();
    expect(thead!.className).toContain('border-b');
    expect(thead!.className).toContain('border-border-subtle/60');
    expect(thead!.className).not.toContain('border-border ');
  });
});

describe('TableCell and TableHead padding ownership', () => {
  const cellClass = (text: string) => screen.getByText(text).closest('td')!.className;
  const headClass = (text: string) => screen.getByText(text).closest('th')!.className;

  it('keeps the base padding when the caller names none', () => {
    render(() => (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>default head</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>default cell</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ));

    expect(headClass('default head')).toContain('px-2 sm:px-3 py-1.5');
    expect(cellClass('default cell')).toContain('px-2 sm:px-3 py-0.5');
  });

  it('drops the base horizontal padding when the caller passes px-*, including important and larger values', () => {
    render(() => (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead class="px-1 sm:px-1.5 lg:px-2 py-0.5">tight head</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell class="px-1 sm:px-1.5 lg:px-2 py-1">tight cell</TableCell>
            <TableCell class="px-1!">important cell</TableCell>
            <TableCell class="px-3 py-1.5">wide cell</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ));

    const tightHead = headClass('tight head');
    expect(tightHead).not.toMatch(/(?:^|\s)(?:px|pl|pr)-2\b/);
    expect(tightHead).not.toContain('sm:px-3');
    expect(tightHead).not.toContain('py-1.5');
    expect(tightHead).toContain('px-1 sm:px-1.5 lg:px-2 py-0.5');

    const tightCell = cellClass('tight cell');
    expect(tightCell).not.toMatch(/(?:^|\s)(?:px|pl|pr)-2\b/);
    expect(tightCell).not.toContain('sm:px-3');
    expect(tightCell).not.toContain('py-0.5');
    expect(tightCell).toContain('px-1 sm:px-1.5 lg:px-2 py-1');

    const importantCell = cellClass('important cell');
    expect(importantCell).not.toContain('px-2');
    expect(importantCell).toContain('py-0.5');

    const wideCell = cellClass('wide cell');
    expect(wideCell).not.toContain('px-2');
    expect(wideCell).not.toContain('py-0.5');
  });

  it('keeps the base padding on the sides the caller leaves alone', () => {
    render(() => (
      <Table>
        <TableBody>
          <TableRow>
            <TableCell class="py-2 pr-2">right only</TableCell>
            <TableCell class="pl-3">left only</TableCell>
            <TableCell class="pt-0 pb-1.5">vertical only</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ));

    const rightOnly = cellClass('right only');
    expect(rightOnly).toContain('pl-2 sm:pl-3');
    expect(rightOnly).not.toContain('px-2');
    expect(rightOnly).not.toContain('pr-2 sm:pr-3');
    expect(rightOnly).not.toContain('py-0.5');

    const leftOnly = cellClass('left only');
    expect(leftOnly).toContain('pr-2 sm:pr-3');
    expect(leftOnly).not.toContain('px-2');
    expect(leftOnly).toContain('py-0.5');

    const verticalOnly = cellClass('vertical only');
    expect(verticalOnly).toContain('px-2 sm:px-3');
    expect(verticalOnly).not.toContain('py-0.5');
  });

  it('treats p-* as owning every side and ignores prefixed-only padding', () => {
    render(() => (
      <Table>
        <TableBody>
          <TableRow>
            <TableCell class="border-0 p-0">spacer</TableCell>
            <TableCell class="lg:px-0 lg:py-0">prefixed</TableCell>
            <TableCell class="space-x-2 group-hover:pl-4">unrelated</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ));

    const spacer = cellClass('spacer');
    expect(spacer).not.toMatch(/(?:^|\s)(?:px|pl|pr|py|pt|pb)-/);
    expect(spacer).toContain('p-0');

    expect(cellClass('prefixed')).toContain('px-2 sm:px-3 py-0.5');
    expect(cellClass('unrelated')).toContain('px-2 sm:px-3 py-0.5');
  });

  it('re-resolves the base padding when the caller class changes', () => {
    const [compact, setCompact] = createSignal(false);
    render(() => (
      <Table>
        <TableBody>
          <TableRow>
            <TableCell class={compact() ? 'px-1' : 'text-right'}>reactive</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ));

    expect(cellClass('reactive')).toContain('px-2 sm:px-3');
    setCompact(true);
    expect(cellClass('reactive')).not.toContain('px-2');
    expect(cellClass('reactive')).toContain('px-1');
    setCompact(false);
    expect(cellClass('reactive')).toContain('px-2 sm:px-3');
  });
});
