import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  PlatformDetailTable,
  PlatformDetailTableBody,
  PlatformDetailTableHeader,
} from '../sharedPlatformPage';

afterEach(cleanup);

describe('PlatformDetailTable', () => {
  it('owns the same canonical structure for drawer and inline-detail tables', () => {
    render(() => (
      <PlatformDetailTable class="table-fixed text-xs">
        <PlatformDetailTableHeader>
          <TableHead>Resource</TableHead>
        </PlatformDetailTableHeader>
        <PlatformDetailTableBody>
          <TableRow>
            <TableCell>node-1</TableCell>
          </TableRow>
        </PlatformDetailTableBody>
      </PlatformDetailTable>
    ));

    const table = screen.getByRole('table');
    expect(table).toHaveClass('platform-table', 'min-w-[0px]', 'table-fixed', 'text-xs');
    expect(table.querySelector('thead tr')).toHaveClass('bg-surface-alt', 'border-border');
    expect(table.querySelector('tbody')).toHaveClass('divide-y', 'divide-border');
  });
});
