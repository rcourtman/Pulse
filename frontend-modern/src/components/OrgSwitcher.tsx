import { For, Show, createMemo } from 'solid-js';
import type { Organization } from '@/api/orgs';

interface OrgSwitcherProps {
  orgs: Organization[];
  selectedOrgId: string;
  loading?: boolean;
  onChange: (orgId: string) => void;
}

export function OrgSwitcher(props: OrgSwitcherProps) {
  const selectedOrgName = createMemo(() => {
    const selected = props.orgs.find((org) => org.id === props.selectedOrgId);
    return selected?.displayName || props.selectedOrgId || 'Default';
  });

  return (
    <div class="flex items-center gap-2">
      <span class="hidden lg:inline text-xs text-gray-600 dark:text-gray-400">Org</span>
      <Show
        when={props.orgs.length > 1}
        fallback={
          <span
            class="inline-flex h-7 items-center rounded-md border border-gray-300 bg-white px-2 text-xs font-medium text-gray-700 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
            title={selectedOrgName()}
          >
            {selectedOrgName()}
          </span>
        }
      >
        <label class="sr-only" for="org-switcher-select">
          Organization
        </label>
        <select
          id="org-switcher-select"
          aria-label="Organization"
          value={props.selectedOrgId}
          disabled={Boolean(props.loading)}
          onChange={(event) => props.onChange(event.currentTarget.value)}
          class="h-7 max-w-44 rounded-md border border-gray-300 bg-white px-2 text-xs text-gray-700 shadow-sm transition-colors focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
        >
          <For each={props.orgs}>
            {(org) => (
              <option value={org.id}>
                {org.displayName || org.id}
              </option>
            )}
          </For>
        </select>
      </Show>
    </div>
  );
}
