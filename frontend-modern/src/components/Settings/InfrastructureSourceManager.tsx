import { For, Show, createMemo, type Accessor } from 'solid-js';
import { Plus, Search, Server } from 'lucide-solid';
import type { Connection } from '@/api/connections';
import SettingsPanel from '@/components/shared/SettingsPanel';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import type { InfrastructureSystemRow } from './connectionsTableModel';
import {
  getInfrastructureSourceManagerProducts,
  type InfrastructureOnboardingConnectionType,
} from '@/utils/infrastructureOnboardingPresentation';

interface InfrastructureSourceManagerProps {
  rows: Accessor<readonly InfrastructureSystemRow[]>;
  readOnly: boolean;
  onAddSource?: (type: InfrastructureOnboardingConnectionType) => void;
  onDetectFromAddress?: () => void;
  onOpenConnection?: (connection: Connection) => void;
}

const inlineButtonClass =
  'inline-flex items-center rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const addSectionButtonClass =
  'inline-flex items-center gap-1.5 rounded-md border border-blue-200 bg-blue-50 px-2.5 py-1 text-xs font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-900 dark:bg-blue-950/30 dark:text-blue-200 dark:hover:bg-blue-900/40';
const childIndentClass = 'pl-4 sm:pl-6';

const sortRows = (rows: readonly InfrastructureSystemRow[]): InfrastructureSystemRow[] =>
  [...rows].sort((left, right) => left.name.localeCompare(right.name));

const summarizeCoverage = (labels: readonly string[]): string => {
  if (labels.length <= 3) {
    return labels.join(', ');
  }

  return `${labels.slice(0, 3).join(', ')} +${labels.length - 3} more`;
};

export const InfrastructureSourceManager: Component<InfrastructureSourceManagerProps> = (props) => {
  const products = createMemo(() => getInfrastructureSourceManagerProducts());
  const productRank = createMemo(() => {
    const next = new Map<InfrastructureOnboardingConnectionType, number>();
    products().forEach((product, index) => {
      next.set(product.type, index);
    });
    return next;
  });
  const groupedRows = createMemo(() => {
    const next = new Map<InfrastructureOnboardingConnectionType, InfrastructureSystemRow[]>();
    for (const product of products()) {
      next.set(product.type, []);
    }

    for (const row of props.rows()) {
      const productRows = next.get(row.connection.type as InfrastructureOnboardingConnectionType);
      if (!productRows) continue;
      productRows.push(row);
    }

    for (const [type, rows] of next.entries()) {
      next.set(type, sortRows(rows));
    }

    return next;
  });
  const sortedProducts = createMemo(() =>
    [...products()].sort((left, right) => {
      const countDifference =
        (groupedRows().get(right.type)?.length ?? 0) - (groupedRows().get(left.type)?.length ?? 0);
      if (countDifference !== 0) return countDifference;
      return (productRank().get(left.type) ?? 0) - (productRank().get(right.type) ?? 0);
    }),
  );

  const rowInteractive = (row: InfrastructureSystemRow): boolean =>
    !props.readOnly && Boolean(props.onOpenConnection) && (row.canEdit || row.isAgent);

  const actionColumnVisible = () => !props.readOnly;

  return (
    <SettingsPanel
      title="Infrastructure sources"
      description="Grouped by platform."
      noPadding
      action={
        !props.readOnly && props.onDetectFromAddress ? (
          <button
            type="button"
            onClick={props.onDetectFromAddress}
            class="inline-flex w-full items-center justify-center gap-2 rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover sm:w-auto"
          >
            <Search class="h-4 w-4" />
            Detect from address
          </button>
        ) : undefined
      }
      icon={<Server class="h-5 w-5" strokeWidth={2} />}
    >
      <Table class="w-full table-fixed text-sm">
        <TableHeader class="bg-surface-alt/60">
          <TableRow>
            <TableHead class="w-[24%] py-1.5 pl-3 pr-3 text-left text-[11px] font-medium text-muted whitespace-nowrap xl:w-[20%]">
              Source
            </TableHead>
            <TableHead class="w-[26%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap xl:w-[24%]">
              Endpoint
            </TableHead>
            <TableHead class="w-[30%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap xl:w-[28%]">
              Coverage
            </TableHead>
            <TableHead class="w-[20%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap xl:w-[16%]">
              Status
            </TableHead>
            <Show when={actionColumnVisible()}>
              <TableHead class="w-[16%] px-3 py-1.5 text-right text-[11px] font-medium text-muted whitespace-nowrap xl:w-[12%]">
                Actions
              </TableHead>
            </Show>
          </TableRow>
        </TableHeader>

        <TableBody class="divide-y divide-border-subtle bg-surface">
          <For each={sortedProducts()}>
            {(product) => {
              const rows = () => groupedRows().get(product.type) ?? [];
              const groupRowClass = () =>
                'bg-surface-alt hover:bg-surface-alt dark:bg-base dark:hover:bg-base';
              const groupLabelClass = () => 'text-[15px] font-semibold text-base-content';

              return (
                <>
                  <Show
                    when={actionColumnVisible()}
                    fallback={
                      <TableRow class={groupRowClass()}>
                        <TableCell colspan={4} class="px-3 py-1.5">
                          <div class="flex min-w-0 items-center gap-2">
                            <span class={groupLabelClass()}>{product.label}</span>
                          </div>
                        </TableCell>
                      </TableRow>
                    }
                  >
                    <TableRow class={groupRowClass()}>
                      <TableCell colspan={4} class="px-3 py-1.5">
                        <div class="flex items-center gap-2 whitespace-nowrap">
                          <div class="flex items-center gap-2">
                            <span class={groupLabelClass()}>{product.label}</span>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell class="px-3 py-1.5 text-right">
                        <Show when={!props.readOnly && props.onAddSource}>
                          <button
                            type="button"
                            onClick={() => props.onAddSource?.(product.type)}
                            class={`${addSectionButtonClass} whitespace-nowrap`}
                            aria-label={`Add ${product.label}`}
                          >
                            <Plus class="h-3.5 w-3.5" />
                            Add
                          </button>
                        </Show>
                      </TableCell>
                    </TableRow>
                  </Show>

                  <Show when={rows().length > 0}>
                    <For each={rows()}>
                      {(row, index) => {
                        const isLast = () => index() === rows().length - 1;

                        return (
                        <>
                          <TableRow>
                            <TableCell class="py-1 pl-3 pr-3 align-top">
                              <div class={`min-w-0 space-y-0.5 ${childIndentClass}`}>
                                <div class="flex min-w-0 items-center gap-2 whitespace-nowrap">
                                  <span aria-hidden="true" class="relative h-5 w-5 flex-none">
                                    <span
                                      class={`absolute left-2 top-0 w-px bg-border ${isLast() ? 'h-2.5' : 'h-5'}`}
                                    />
                                    <span class="absolute left-2 top-2.5 h-px w-3 bg-border" />
                                  </span>
                                  <div class="min-w-0">
                                    <div
                                      class="truncate text-[13px] text-base-content/80"
                                      title={row.name}
                                    >
                                      {row.name}
                                    </div>
                                  </div>
                                </div>
                              </div>
                            </TableCell>

                            <TableCell class="px-3 py-1 align-top">
                              <Show
                                when={row.host}
                                fallback={<span class="text-xs text-muted">-</span>}
                              >
                                <div
                                  class="truncate whitespace-nowrap text-[12px] text-muted"
                                  title={row.host}
                                >
                                  {row.host}
                                </div>
                              </Show>
                            </TableCell>

                            <TableCell class="px-3 py-1 align-top">
                              <Show
                                when={row.coverageLabels.length > 0}
                                fallback={<span class="text-xs text-muted">-</span>}
                              >
                                <div
                                  class="truncate whitespace-nowrap text-[12px] text-muted"
                                  title={row.coverageLabels.join(', ')}
                                >
                                  {summarizeCoverage(row.coverageLabels)}
                                </div>
                              </Show>
                            </TableCell>

                            <TableCell class="px-3 py-1 align-top">
                              <div class="flex items-center gap-1.5 whitespace-nowrap">
                                <span
                                  class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium whitespace-nowrap ${row.statusClassName}`}
                                >
                                  {row.statusLabel}
                                </span>
                                <span class="text-[12px] text-muted/90">{row.lastActivityText}</span>
                              </div>
                            </TableCell>

                            <Show when={actionColumnVisible()}>
                              <TableCell class="px-3 py-1 align-top text-right">
                                <Show
                                  when={rowInteractive(row)}
                                  fallback={<span class="text-xs text-muted">Read only</span>}
                                >
                                  <button
                                    type="button"
                                    onClick={() => props.onOpenConnection?.(row.connection)}
                                    class={inlineButtonClass}
                                  >
                                    Edit
                                  </button>
                                </Show>
                              </TableCell>
                            </Show>
                          </TableRow>

                          <Show when={row.lastErrorMessage}>
                            <TableRow class="border-b border-border/80">
                              <TableCell
                                colspan={actionColumnVisible() ? 5 : 4}
                                class="bg-surface px-3 pb-1.5 pt-0"
                              >
                                <div
                                  role="alert"
                                  class="rounded-md border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
                                >
                                  {row.lastErrorMessage}
                                </div>
                              </TableCell>
                            </TableRow>
                          </Show>
                        </>
                        );
                      }}
                    </For>
                  </Show>
                </>
              );
            }}
          </For>
        </TableBody>
      </Table>
    </SettingsPanel>
  );
};

export default InfrastructureSourceManager;
