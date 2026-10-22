import { For, Show, type Component, type JSX } from 'solid-js';
import type { StatusIndicator } from '@/utils/status';
import { DrawerSubjectHeading } from './DrawerSubjectHeading';
import { ObjectDrawerHeader } from './ObjectDrawerHeader';
import { ProgressBar } from './ProgressBar';
import { Table, TableBody, TableCell, TableHead, TableRow } from './Table';
import type { DetailSection, DetailValueTone } from './detailSectionModel';

export {
  compactDetailRows,
  compactDetailSections,
  formatDetailBytesValue,
  formatDetailCountValue,
  formatDetailIntegerValue,
  makeDetailRow,
  type DetailRow,
  type DetailRowProgress,
  type DetailSection,
  type DetailValueTone,
} from './detailSectionModel';

export const detailValueToneClass = (tone: DetailValueTone | undefined): string => {
  if (tone === 'accent') return 'text-cyan-700 dark:text-cyan-300';
  if (tone === 'success') return 'text-emerald-700 dark:text-emerald-300';
  if (tone === 'warning') return 'text-amber-700 dark:text-amber-300';
  if (tone === 'danger') return 'text-rose-700 dark:text-rose-300';
  if (tone === 'muted') return 'text-muted';
  return 'text-base-content';
};

const detailSectionDesktopColumnCount = (sectionCount: number): 1 | 2 | 3 | 4 => {
  if (sectionCount <= 1) return 1;
  if (sectionCount === 2) return 2;
  if (sectionCount === 3 || sectionCount === 5 || sectionCount === 6) return 3;
  return 4;
};

const detailSectionDesktopSpan = (sectionCount: number, sectionIndex: number): number => {
  const columnCount = detailSectionDesktopColumnCount(sectionCount);
  const remainder = sectionCount % columnCount;
  if (remainder === 0 || sectionIndex < sectionCount - remainder) return 1;

  const remainderIndex = sectionIndex - (sectionCount - remainder);
  const baseSpan = Math.floor(columnCount / remainder);
  const widerSectionCount = columnCount % remainder;
  return baseSpan + (remainderIndex < widerSectionCount ? 1 : 0);
};

const detailSectionDesktopBasisClass = (sectionCount: number, sectionIndex: number): string => {
  const columnCount = detailSectionDesktopColumnCount(sectionCount);
  const span = detailSectionDesktopSpan(sectionCount, sectionIndex);
  if (span >= columnCount) return 'lg:basis-full';

  if (columnCount === 4) {
    if (span === 3) return 'lg:basis-[calc(75%-0.125rem)]';
    if (span === 2) return 'lg:basis-[calc(50%-0.25rem)]';
    return 'lg:basis-[calc(25%-0.375rem)]';
  }
  if (columnCount === 3) {
    if (span === 2) return 'lg:basis-[calc(66.667%-0.167rem)]';
    return 'lg:basis-[calc(33.333%-0.333rem)]';
  }
  return 'lg:basis-[calc(50%-0.25rem)]';
};

export const DetailSectionTable: Component<{
  sections: DetailSection[];
  class?: string;
  dataTestId?: string;
  /** Wrap every value instead of truncating it (record drawers: no hover on phones). */
  wrapValues?: boolean;
}> = (props) => (
  <div
    data-testid={props.dataTestId}
    class={`${props.class ?? 'overflow-hidden rounded-sm border border-border bg-surface'} lg:overflow-visible lg:border-0 lg:bg-transparent`}
  >
    <Table
      class="w-full table-fixed text-[11px] lg:flex lg:flex-wrap lg:items-stretch lg:gap-2"
      wrapperClass="lg:overflow-visible"
    >
      <For each={props.sections}>
        {(section, sectionIndex) => (
          <TableBody
            data-testid={section.testId}
            class={`divide-y divide-border lg:flex lg:min-w-0 lg:flex-none lg:flex-col lg:overflow-hidden lg:rounded-sm lg:border lg:border-border lg:bg-surface lg:p-3 lg:shadow-xs lg:divide-y-0 ${detailSectionDesktopBasisClass(props.sections.length, sectionIndex())}`}
          >
            <TableRow class="bg-surface-alt lg:mb-1 lg:block lg:bg-transparent lg:hover:bg-transparent">
              <TableHead
                colspan={2}
                class="px-2 py-1 text-left text-[10px] font-semibold uppercase tracking-wide text-muted lg:block lg:px-0 lg:pb-1 lg:pt-0 lg:text-base-content"
              >
                {section.label}
              </TableHead>
            </TableRow>
            <For each={section.rows}>
              {(row) => (
                <TableRow
                  class={`lg:grid lg:min-w-0 lg:items-start lg:gap-3 lg:py-0.5 lg:hover:bg-transparent ${row.layout === 'stacked' ? 'lg:grid-cols-[minmax(0,1fr)]' : 'lg:grid-cols-[7rem_minmax(0,1fr)]'}`}
                >
                  {row.layout !== 'stacked' ? (
                    <TableCell class="w-[38%] px-2 py-1 text-muted lg:w-auto lg:px-0 lg:py-0">
                      {row.label}
                    </TableCell>
                  ) : null}
                  <TableCell
                    colspan={row.layout === 'stacked' ? 2 : undefined}
                    class={`px-2 py-1 text-right font-medium lg:min-w-0 lg:px-0 lg:py-0 lg:text-left ${detailValueToneClass(
                      row.tone,
                    )} ${row.valueClass ?? ''}`}
                    title={row.title ?? row.value}
                  >
                    {row.layout === 'stacked' ? (
                      <span class="mb-1 block whitespace-normal text-left font-normal leading-snug text-muted wrap-anywhere">
                        {row.label}
                      </span>
                    ) : null}
                    {row.valueContent ?? (
                      <span
                        title={row.title ?? row.value}
                        class={
                          row.wrap || props.wrapValues
                            ? 'block whitespace-normal wrap-break-word text-left leading-snug'
                            : 'block truncate'
                        }
                      >
                        {row.value}
                      </span>
                    )}
                    {row.progress ? (
                      <ProgressBar
                        value={row.progress.value}
                        fillClass={row.progress.fillClass}
                        ariaLabel={row.progress.ariaLabel}
                        class="mt-1 h-1.5"
                      />
                    ) : null}
                  </TableCell>
                </TableRow>
              )}
            </For>
            {section.footerContent ? (
              <TableRow class="lg:mt-2 lg:block lg:min-w-0 lg:border-t lg:border-border lg:pt-2 lg:hover:bg-transparent">
                <TableCell colspan={2} class="px-2 py-2 lg:block lg:px-0 lg:py-0">
                  {section.footerContent}
                </TableCell>
              </TableRow>
            ) : null}
          </TableBody>
        )}
      </For>
    </Table>
  </div>
);

// A record drawer (an alert, an event, a job run) is headed like an object
// drawer: the subject's name and status dot, a one-line summary, and actions
// such as a link to the object the record is about. Its values wrap because a
// record is mostly sentences and identifiers a phone cannot hover to read.
export const InlineDetailPanel: Component<{
  testId: string;
  detailFor: string;
  /** The subject: what the record is about, not the kind of panel. */
  title: string;
  status?: StatusIndicator;
  summary?: JSX.Element;
  actions?: JSX.Element;
  sections: DetailSection[];
  onClose: () => void;
  /** Content cards that matter more than the sections (attachments, restore evidence). */
  children?: JSX.Element;
  class?: string;
  tableClass?: string;
  detailAttributes?: Record<string, string>;
}> = (props) => (
  <section
    class={props.class ?? 'space-y-3'}
    {...(props.detailAttributes ?? {})}
    aria-labelledby={`${props.testId}-heading-${props.detailFor}`}
    data-testid={props.testId}
    data-inline-detail-for={props.detailFor}
  >
    <ObjectDrawerHeader
      collapseLabel={`Collapse ${props.title} details`}
      onCollapse={props.onClose}
      actions={props.actions}
    >
      <div class="min-w-0 space-y-1">
        <DrawerSubjectHeading
          headingId={`${props.testId}-heading-${props.detailFor}`}
          title={props.title}
          statusVariant={props.status?.variant}
          statusLabel={props.status?.label}
        />
        <Show when={props.summary}>
          <div class="text-[11px] leading-snug text-muted wrap-anywhere">{props.summary}</div>
        </Show>
      </div>
    </ObjectDrawerHeader>
    {props.children}
    <Show when={props.sections.length > 0}>
      <DetailSectionTable sections={props.sections} class={props.tableClass} wrapValues />
    </Show>
  </section>
);
