import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  DetailSectionTable,
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  formatDetailBytesValue,
  formatDetailCountValue,
  formatDetailIntegerValue,
  makeDetailRow,
} from '../DetailSectionTable';
import detailSectionTableSource from '../DetailSectionTable.tsx?raw';
import detailSectionModelSource from '../detailSectionModel.ts?raw';
import { TechnicalDetailsDisclosure, TechnicalDetailsSection } from '../TechnicalDetailsDisclosure';

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

  it('keeps detail numeric formatting in the shared model', () => {
    expect(formatDetailBytesValue(undefined)).toBeNull();
    expect(formatDetailBytesValue(0)).toBeNull();
    expect(formatDetailBytesValue(0, { allowZero: true })).toBe('0 B');
    expect(formatDetailBytesValue(8 * 1024 ** 3)).toBe('8.00 GB');
    expect(
      formatDetailBytesValue(8 * 1024 ** 3, {
        allowZero: true,
        precision: 'compact',
        trimWhole: true,
      }),
    ).toBe('8 GB');
    expect(formatDetailIntegerValue(1234.6)).toBe(new Intl.NumberFormat().format(1235));
    expect(formatDetailCountValue(1, 'disk')).toBe('1 disk');
    expect(formatDetailCountValue(2, 'vCPU', 'vCPU')).toBe('2 vCPU');
    expect(formatDetailCountValue(undefined, 'disk')).toBeNull();
  });

  it('keeps compact table rows on narrow screens and bounded section cards on desktop', () => {
    const { container } = render(() => (
      <DetailSectionTable
        sections={[
          {
            label: 'Alert',
            rows: [
              { label: 'Severity', value: 'Warning', tone: 'warning' },
              { label: 'Resource', value: 'tower', title: 'tower.example.test' },
            ],
          },
          {
            label: 'Runtime',
            rows: [{ label: 'Kernel', value: '6.8.0' }],
          },
        ]}
      />
    ));

    expect(screen.getByText('Alert')).toBeInTheDocument();
    expect(screen.getByText('Severity')).toBeInTheDocument();
    expect(screen.getByText('Warning').closest('td')).toHaveClass('text-amber-700');
    expect(screen.getByText('tower').closest('td')).toHaveAttribute('title', 'tower.example.test');

    const table = container.querySelector('table');
    expect(table).toHaveClass('table-fixed', 'lg:flex', 'lg:flex-wrap', 'lg:items-stretch');
    const sections = container.querySelectorAll('tbody');
    expect(sections).toHaveLength(2);
    expect(sections[0]).toHaveClass(
      'lg:flex',
      'lg:flex-1',
      'lg:basis-[calc(25%-0.5rem)]',
      'lg:rounded',
      'lg:border',
      'lg:p-3',
    );
    expect(screen.getByText('Severity').closest('tr')).toHaveClass(
      'lg:grid',
      'lg:grid-cols-[7rem_minmax(0,1fr)]',
      'lg:gap-3',
    );
    expect(screen.getByText('Warning').closest('td')).toHaveClass('lg:text-left');
  });

  it('preserves section test hooks and rich value content inside the shared layout', () => {
    render(() => (
      <DetailSectionTable
        dataTestId="resource-summary"
        sections={[
          {
            label: 'Container',
            testId: 'resource-container-section',
            rows: [
              {
                label: 'Release information',
                value: 'View release',
                valueClass: 'font-mono',
                valueContent: <a href="https://example.test/release">View release</a>,
              },
            ],
          },
        ]}
      />
    ));

    expect(screen.getByTestId('resource-summary')).toBeInTheDocument();
    expect(screen.getByTestId('resource-container-section').tagName).toBe('TBODY');
    expect(screen.getByRole('link', { name: 'View release' })).toHaveAttribute(
      'href',
      'https://example.test/release',
    );
    expect(screen.getByRole('link', { name: 'View release' }).closest('td')).toHaveClass(
      'font-mono',
    );
  });

  it('keeps full-width supporting content inside its responsive section card', () => {
    render(() => (
      <DetailSectionTable
        sections={[
          {
            label: 'Analysis',
            rows: [{ label: 'Health', value: 'A · 92/100' }],
            footerContent: <div data-testid="analysis-footer">Latest canonical change</div>,
          },
        ]}
      />
    ));

    const footer = screen.getByTestId('analysis-footer');
    expect(footer).toBeInTheDocument();
    expect(footer.closest('td')).toHaveAttribute('colspan', '2');
    expect(footer.closest('tr')).toHaveClass('lg:border-t', 'lg:pt-2');
  });

  it('renders optional row progress without replacing its compact text value', () => {
    const { container } = render(() => (
      <DetailSectionTable
        sections={[
          {
            label: 'Filesystems',
            rows: [
              {
                label: '/',
                value: '50% · 5.00 GB/10.0 GB · ROOTFS',
                progress: {
                  value: 50,
                  fillClass: 'bg-emerald-500',
                  ariaLabel: 'Filesystem / utilization',
                },
              },
            ],
          },
        ]}
      />
    ));

    expect(screen.getByText('50% · 5.00 GB/10.0 GB · ROOTFS')).toBeInTheDocument();
    const progress = screen.getByRole('progressbar', { name: 'Filesystem / utilization' });
    expect(progress).toHaveAttribute('aria-valuemin', '0');
    expect(progress).toHaveAttribute('aria-valuemax', '100');
    expect(progress).toHaveAttribute('aria-valuenow', '50');
    expect(progress).toHaveClass('h-1.5');
    const fill = container.querySelector('[data-progress-fill="true"]');
    expect(fill).toHaveAttribute('width', '50');
    expect(fill?.firstElementChild).toHaveClass('bg-emerald-500');
  });

  it('balances five desktop sections across three- and two-card rows', () => {
    const { container } = render(() => (
      <DetailSectionTable
        sections={Array.from({ length: 5 }, (_, index) => ({
          label: `Section ${index + 1}`,
          rows: [{ label: 'Value', value: String(index + 1) }],
        }))}
      />
    ));

    const sections = container.querySelectorAll('tbody');
    expect(sections).toHaveLength(5);
    sections.forEach((section) => expect(section).toHaveClass('lg:basis-[calc(33.333%-0.5rem)]'));
  });

  it('lazily renders technical details with the same compact section rows', () => {
    const { container } = render(() => (
      <TechnicalDetailsDisclosure
        dataTestId="technical-details"
        subtitle="Identity and runtime"
        sections={[
          {
            label: 'Runtime',
            rows: [{ label: 'Kernel', value: '6.8.0' }],
          },
        ]}
      />
    ));

    expect(screen.queryByText('Runtime')).toBeNull();
    const details = container.querySelector('details');
    expect(details).not.toBeNull();
    details!.open = true;
    fireEvent(details!, new Event('toggle'));

    const disclosure = screen.getByTestId('technical-details');
    expect(within(disclosure).getByText('Runtime')).toBeInTheDocument();
    expect(within(disclosure).getByText('Kernel')).toBeInTheDocument();
    expect(within(disclosure).getByText('6.8.0')).toBeInTheDocument();
    expect(disclosure.querySelector('table')).toHaveClass('table-fixed');
  });

  it('renders curated technical rows without an extra disclosure interaction', () => {
    render(() => (
      <TechnicalDetailsSection
        dataTestId="technical-section"
        sections={[
          {
            label: 'Hardware',
            rows: [{ label: 'CPU', value: 'Ryzen' }],
          },
        ]}
      />
    ));

    const section = screen.getByTestId('technical-section');
    expect(section.tagName).toBe('DIV');
    expect(section.querySelector('details')).toBeNull();
    expect(within(section).getByText('Hardware')).toBeInTheDocument();
    expect(within(section).getByText('Ryzen')).toBeInTheDocument();
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

    within(panel).getByRole('button', { name: 'Collapse resource-1 details' }).click();
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
