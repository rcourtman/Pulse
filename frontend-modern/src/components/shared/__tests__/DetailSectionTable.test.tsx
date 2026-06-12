import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  DetailSectionTable,
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
} from '../DetailSectionTable';
import detailSectionTableSource from '../DetailSectionTable.tsx?raw';
import detailSectionModelSource from '../detailSectionModel.ts?raw';

describe('DetailSectionTable', () => {
  afterEach(() => cleanup());

  it('keeps detail section row shaping in the shared model', () => {
    expect(detailSectionModelSource).toContain('export type DetailValueTone');
    expect(detailSectionModelSource).toContain('makeDetailRow');
    expect(detailSectionTableSource).toContain('DetailSectionTable');
    expect(detailSectionTableSource).toContain('InlineDetailPanel');

    expect(
      compactDetailRows([
        makeDetailRow('Host', ' tower '),
        makeDetailRow('Blank', ' '),
        makeDetailRow('Fallback', '-'),
      ]),
    ).toEqual([{ label: 'Host', value: 'tower' }]);

    expect(
      compactDetailSections([
        { label: 'Runtime', rows: [] },
        { label: 'Host', rows: [makeDetailRow('Name', 'tower')!] },
      ]),
    ).toEqual([{ label: 'Host', rows: [{ label: 'Name', value: 'tower' }] }]);
  });

  it('renders section tables with shared value tone classes', () => {
    render(() => (
      <DetailSectionTable
        sections={[
          {
            label: 'Alert',
            rows: [
              { label: 'Severity', value: 'Warning', tone: 'warning' },
              { label: 'Resource', value: 'tower', title: 'tower.example.test' },
            ],
          },
        ]}
      />
    ));

    expect(screen.getByText('Alert')).toBeInTheDocument();
    expect(screen.getByText('Severity')).toBeInTheDocument();
    expect(screen.getByText('Warning').closest('td')).toHaveClass('text-amber-700');
    expect(screen.getByText('tower').closest('td')).toHaveAttribute('title', 'tower.example.test');
  });

  it('renders inline detail panels with the canonical close action', () => {
    const onClose = vi.fn();

    render(() => (
      <InlineDetailPanel
        testId="platform-detail"
        detailFor="resource-1"
        title="Alert detail"
        summary="Warning"
        sections={[{ label: 'Alert', rows: [{ label: 'Severity', value: 'Warning' }] }]}
        detailAttributes={{ 'data-platform-alert-detail-for': 'resource-1' }}
        onClose={onClose}
      />
    ));

    const panel = screen.getByTestId('platform-detail');
    expect(panel).toHaveAttribute('data-inline-detail-for', 'resource-1');
    expect(panel).toHaveAttribute('data-platform-alert-detail-for', 'resource-1');
    expect(within(panel).getByText('Alert detail')).toBeInTheDocument();
    expect(within(panel).getAllByText('Warning')).toHaveLength(2);

    within(panel).getByRole('button', { name: 'Close' }).click();
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
