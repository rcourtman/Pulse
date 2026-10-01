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
            onClick={[onClick, 'row-identity']}
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
