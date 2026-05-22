import { For, type Component } from 'solid-js';
import { Table, TableBody, TableCell, TableHead, TableRow } from '@/components/shared/Table';

export type TrueNASInlineDetailTone = 'default' | 'success' | 'warning' | 'danger' | 'muted';

export type TrueNASInlineDetailRow = {
  label: string;
  value: string;
  title?: string;
  tone?: TrueNASInlineDetailTone;
};

export type TrueNASInlineDetailSection = {
  label: string;
  rows: TrueNASInlineDetailRow[];
};

export const makeTrueNASInlineDetailRow = (
  label: string,
  value?: string | null,
  options: Pick<TrueNASInlineDetailRow, 'title' | 'tone'> = {},
): TrueNASInlineDetailRow | null => {
  const trimmed = value?.trim();
  if (!trimmed || trimmed === '-') return null;
  return { label, value: trimmed, ...options };
};

export const compactTrueNASInlineDetailRows = (
  rows: Array<TrueNASInlineDetailRow | null>,
): TrueNASInlineDetailRow[] => rows.filter((row): row is TrueNASInlineDetailRow => Boolean(row));

export const compactTrueNASInlineDetailSections = (
  sections: Array<TrueNASInlineDetailSection | null>,
): TrueNASInlineDetailSection[] =>
  sections.filter((section): section is TrueNASInlineDetailSection =>
    Boolean(section && section.rows.length > 0),
  );

const detailValueToneClass = (tone: TrueNASInlineDetailTone | undefined): string => {
  if (tone === 'success') return 'text-emerald-700 dark:text-emerald-300';
  if (tone === 'warning') return 'text-amber-700 dark:text-amber-300';
  if (tone === 'danger') return 'text-rose-700 dark:text-rose-300';
  if (tone === 'muted') return 'text-muted';
  return 'text-base-content';
};

const truenasDetailAttribute = (
  detailKind: 'alert' | 'protection' | 'service' | undefined,
  detailFor: string,
): Record<string, string> =>
  detailKind ? { [`data-truenas-${detailKind}-detail-for`]: detailFor } : {};

export const TrueNASInlineDetailTable: Component<{
  testId: string;
  detailFor: string;
  detailKind?: 'alert' | 'protection' | 'service';
  title: string;
  summary: string;
  sections: TrueNASInlineDetailSection[];
  onClose: () => void;
}> = (props) => (
  <div
    class="space-y-3"
    data-testid={props.testId}
    data-truenas-inline-detail-for={props.detailFor}
    {...truenasDetailAttribute(props.detailKind, props.detailFor)}
  >
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <div class="text-[11px] font-medium uppercase tracking-wide text-base-content">
          {props.title}
        </div>
        <div class="mt-1 text-[10px] text-muted">{props.summary}</div>
      </div>
      <button
        type="button"
        class="inline-flex items-center rounded-md border border-border bg-surface px-2.5 py-1 text-[10px] font-medium text-base-content transition-colors hover:bg-base"
        onClick={props.onClose}
      >
        Close
      </button>
    </div>
    <div class="overflow-hidden rounded border border-border bg-surface">
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
                        class={`px-2 py-1 text-right align-top font-medium ${detailValueToneClass(row.tone)}`}
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
  </div>
);
