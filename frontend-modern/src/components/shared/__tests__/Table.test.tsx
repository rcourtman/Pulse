import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';
import tableSource from '@/components/shared/Table.tsx?raw';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/shared/Table';

describe('TableBody', () => {
  it('keeps the shared table wrapper CSP-safe', () => {
    expect(tableSource).toContain('touch-scroll');
    expect(tableSource).not.toContain('style={{');
    expect(tableSource).not.toContain('style={');
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
