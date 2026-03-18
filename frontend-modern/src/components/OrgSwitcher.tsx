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
      <span class="hidden lg:inline text-xs text-muted">Org</span>
      <Show
        when={props.orgs.length > 1}
        fallback={
          <span
            class="inline-flex h-7 items-center rounded-md border border-border bg-surface px-2 text-xs font-medium text-base-content"
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
          class="h-7 max-w-44 rounded-md border border-border bg-surface px-2 text-xs text-base-content shadow-sm transition-colors focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60"
        >
          <For each={props.orgs}>
            {(org) => <option value={org.id}>{org.displayName || org.id}</option>}
          </For>
        </select>
      </Show>
      <Show when={props.loading}>
        <svg
          class="h-3.5 w-3.5 animate-spin text-slate-400"
          fill="none"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <circle
            class="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            stroke-width="4"
          />
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          />
        </svg>
      </Show>
    </div>
  );
}
