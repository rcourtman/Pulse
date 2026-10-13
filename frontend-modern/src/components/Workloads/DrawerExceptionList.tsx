import { For, Show, type Component } from 'solid-js';

import { detailValueToneClass } from '@/components/shared/DetailSectionTable';
import type { DetailRow } from '@/components/shared/detailSectionModel';

interface DrawerExceptionListProps {
  rows: DetailRow[];
  testId: string;
}

// "Worth checking" sits directly under the alerts line: each lifted fact is one
// line (label, then its value), not a card per section. A row's own content
// (e.g. a backup's age and date) flows inline instead of stacking. The value
// keeps the shared detail-row tone, so a late backup reads as it does in Details.
export const DrawerExceptionList: Component<DrawerExceptionListProps> = (props) => (
  <Show when={props.rows.length > 0}>
    <section
      data-testid={props.testId}
      aria-label="Worth checking"
      class="rounded-md border border-amber-300 bg-amber-50/60 px-3 py-2 dark:border-amber-700/60 dark:bg-amber-900/15"
    >
      <h3 class="mb-1 text-[11px] font-semibold uppercase tracking-wide text-amber-800 dark:text-amber-200">
        Worth checking
      </h3>
      <table class="w-full border-collapse text-xs">
        <tbody>
          <For each={props.rows}>
            {(row) => (
              <tr class="align-baseline">
                <th
                  scope="row"
                  class="w-px whitespace-nowrap py-0.5 pr-3 text-left font-normal text-muted"
                >
                  {row.label}
                </th>
                <td
                  class={`py-0.5 font-medium wrap-break-word [&>*]:mt-0 [&>*]:inline [&>*+*]:ml-1.5 ${detailValueToneClass(row.tone)}`}
                  title={row.title}
                >
                  {row.valueContent ?? row.value}
                </td>
              </tr>
            )}
          </For>
        </tbody>
      </table>
    </section>
  </Show>
);
