import { For, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { ALERTS_OVERVIEW_PATH } from '@/routing/resourceLinks';
import { priorityBadgeClass, type ActionItem } from './dashboardHelpers';

interface ActionQueueProps {
  items: ActionItem[];
  overflowCount: number;
}

export function ActionQueue(props: ActionQueueProps) {
  return (
    <Card>
      <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">Needs Action</h2>
      <div class="mt-2 mb-3 border-b border-gray-100 dark:border-gray-700/50" />

      <ul class="space-y-2" role="list">
        <For each={props.items}>
          {(item) => (
            <li class="flex items-start gap-2.5 hover:bg-gray-50 dark:hover:bg-gray-700/50 -mx-2 px-2 py-1 rounded transition-colors">
              <span
                class={`mt-0.5 inline-flex shrink-0 items-center rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase ${priorityBadgeClass(item.priority)}`}
              >
                {item.priority}
              </span>
              <a
                href={item.link}
                class="text-sm text-gray-700 dark:text-gray-200 hover:text-blue-600 dark:hover:text-blue-400 hover:underline truncate"
              >
                {item.label}
              </a>
            </li>
          )}
        </For>
      </ul>

      <Show when={props.overflowCount > 0}>
        <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
          <a
            href={ALERTS_OVERVIEW_PATH}
            class="text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 hover:underline"
          >
            and {props.overflowCount} more...
          </a>
        </p>
      </Show>
    </Card>
  );
}

export default ActionQueue;

