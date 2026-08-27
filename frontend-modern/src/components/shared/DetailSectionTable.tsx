import { For, type Component, type JSX } from 'solid-js';
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

const detailValueToneClass = (tone: DetailValueTone | undefined): string => {
  if (tone === 'accent') return 'text-cyan-700 dark:text-cyan-300';
  if (tone === 'success') return 'text-emerald-700 dark:text-emerald-300';
  if (tone === 'warning') return 'text-amber-700 dark:text-amber-300';
  if (tone === 'danger') return 'text-rose-700 dark:text-rose-300';
  if (tone === 'muted') return 'text-muted';
  return 'text-base-content';
};

const detailSectionDesktopBasisClass = (sectionCount: number): string =>
  sectionCount === 5 || sectionCount === 6
    ? 'lg:basis-[calc(33.333%-0.5rem)]'
    : 'lg:basis-[calc(25%-0.5rem)]';

export const DetailSectionTable: Component<{
  sections: DetailSection[];
  class?: string;
  dataTestId?: string;
}> = (props) => (
  <div
    data-testid={props.dataTestId}
    class={`${props.class ?? 'overflow-hidden rounded border border-border bg-surface'} lg:overflow-visible lg:border-0 lg:bg-transparent`}
  >
    <Table
      class="w-full table-fixed text-[11px] lg:flex lg:flex-wrap lg:items-stretch lg:gap-2"
      wrapperClass="lg:overflow-visible"
    >
      <For each={props.sections}>
        {(section) => (
          <TableBody
            data-testid={section.testId}
            class={`divide-y divide-border lg:flex lg:min-w-[16rem] lg:flex-1 lg:flex-col lg:overflow-hidden lg:rounded lg:border lg:border-border lg:bg-surface lg:p-3 lg:shadow-sm lg:divide-y-0 ${detailSectionDesktopBasisClass(props.sections.length)}`}
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
                <TableRow class="lg:grid lg:min-w-0 lg:grid-cols-[7rem_minmax(0,1fr)] lg:items-start lg:gap-3 lg:py-0.5 lg:hover:bg-transparent">
                  <TableCell class="w-[38%] px-2 py-1 align-top text-muted lg:w-auto lg:px-0 lg:py-0">
                    {row.label}
                  </TableCell>
                  <TableCell
                    class={`px-2 py-1 text-right align-top font-medium lg:min-w-0 lg:px-0 lg:py-0 lg:text-left ${detailValueToneClass(
                      row.tone,
                    )} ${row.valueClass ?? ''}`}
                    title={row.title ?? row.value}
                  >
                    {row.valueContent ?? (
                      <span
                        title={row.title ?? row.value}
                        class={
                          row.wrap
                            ? 'block whitespace-normal break-words text-left leading-snug'
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
          </TableBody>
        )}
      </For>
    </Table>
  </div>
);

export const InlineDetailPanel: Component<{
  testId: string;
  detailFor: string;
  title: JSX.Element;
  summary?: JSX.Element;
  sections: DetailSection[];
  onClose: () => void;
  class?: string;
  tableClass?: string;
  detailAttributes?: Record<string, string>;
}> = (props) => (
  <div
    class={props.class ?? 'space-y-3'}
    {...(props.detailAttributes ?? {})}
    data-testid={props.testId}
    data-inline-detail-for={props.detailFor}
  >
    <ObjectDrawerHeader
      collapseLabel={`Collapse ${props.detailFor} details`}
      onCollapse={props.onClose}
    >
      <div>
        <div class="text-[11px] font-medium uppercase tracking-wide text-base-content">
          {props.title}
        </div>
        {props.summary ? <div class="mt-1 text-[10px] text-muted">{props.summary}</div> : null}
      </div>
    </ObjectDrawerHeader>
    <DetailSectionTable sections={props.sections} class={props.tableClass} />
  </div>
);
