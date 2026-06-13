import { For, type Component, type JSX } from 'solid-js';
import { Button } from './Button';
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

export const DetailSectionTable: Component<{
  sections: DetailSection[];
  class?: string;
}> = (props) => (
  <div class={props.class ?? 'overflow-hidden rounded border border-border bg-surface'}>
    <Table class="w-full table-fixed text-[11px]">
      <TableBody class="divide-y divide-border">
        <For each={props.sections}>
          {(section) => (
            <>
              <TableRow class="bg-surface-alt">
                <TableHead
                  colspan={2}
                  class="px-2 py-1 text-left text-[10px] font-semibold uppercase tracking-wide text-muted"
                >
                  {section.label}
                </TableHead>
              </TableRow>
              <For each={section.rows}>
                {(row) => (
                  <TableRow>
                    <TableCell class="w-[38%] px-2 py-1 align-top text-muted">
                      {row.label}
                    </TableCell>
                    <TableCell
                      class={`px-2 py-1 text-right align-top font-medium ${detailValueToneClass(
                        row.tone,
                      )}`}
                      title={row.title ?? row.value}
                    >
                      <span class="block truncate">{row.value}</span>
                    </TableCell>
                  </TableRow>
                )}
              </For>
            </>
          )}
        </For>
      </TableBody>
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
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <div class="text-[11px] font-medium uppercase tracking-wide text-base-content">
          {props.title}
        </div>
        {props.summary ? <div class="mt-1 text-[10px] text-muted">{props.summary}</div> : null}
      </div>
      <Button
        type="button"
        variant="outline"
        size="xs"
        class="bg-surface text-[10px] hover:bg-base"
        onClick={props.onClose}
      >
        Close
      </Button>
    </div>
    <DetailSectionTable sections={props.sections} class={props.tableClass} />
  </div>
);
