import { For, Show, createMemo } from 'solid-js';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import type { Organization } from '@/api/orgs';
import type { OrganizationListIssue } from '@/useAppRuntimeState';
import { FormSelect } from '@/components/shared/FormSelect';
import { Button } from '@/components/shared/Button';

interface OrgSwitcherProps {
  class?: string;
  orgs: Organization[];
  selectedOrgId: string;
  loading?: boolean;
  listIssue?: OrganizationListIssue | null;
  onRetry?: () => void;
  onChange: (orgId: string) => void;
}

export function OrgSwitcher(props: OrgSwitcherProps) {
  const organizationName = (org: Organization) => org.displayName || org.id;
  const nameKey = (org: Organization) => organizationName(org).trim().replace(/\s+/g, ' ');
  const nameCounts = createMemo(() => {
    const counts = new Map<string, number>();
    for (const org of props.orgs) {
      const key = nameKey(org);
      counts.set(key, (counts.get(key) ?? 0) + 1);
    }
    return counts;
  });
  const needsIdentity = (org: Organization) => (nameCounts().get(nameKey(org)) ?? 0) > 1;
  const selectedOrg = createMemo(() => props.orgs.find((org) => org.id === props.selectedOrgId));
  const selectedNeedsIdentity = createMemo(() => {
    const selected = selectedOrg();
    return selected ? needsIdentity(selected) : false;
  });
  const selectedOrgName = createMemo(() => {
    const selected = selectedOrg();
    return selected?.displayName || props.selectedOrgId || 'Default';
  });
  let switcher: HTMLDivElement | undefined;
  const retryList = () => {
    // The retry control disappears when the real choices come back. Keep a
    // local keyboard anchor without taking focus after an asynchronous read.
    switcher?.focus({ preventScroll: true });
    props.onRetry?.();
  };

  return (
    <div
      ref={switcher}
      role="group"
      aria-label="Organization selection"
      tabIndex={-1}
      class={`flex min-w-0 items-center gap-2 focus:outline-2 focus:-outline-offset-2 focus:outline-blue-500 ${props.class ?? ''}`}
    >
      <span class="hidden lg:inline text-xs text-muted">Org</span>
      <Show
        when={props.listIssue}
        fallback={
          <Show
            when={props.orgs.length > 1}
            fallback={
              <span
                class="inline-flex min-h-7 min-w-0 max-w-44 items-center rounded-md border border-border bg-surface px-2 py-1 text-xs font-medium text-base-content [overflow-wrap:anywhere] lg:max-w-56 xl:max-w-96"
                title={selectedOrgName()}
              >
                {selectedOrgName()}
              </span>
            }
          >
            {/* The native control still sizes the field and owns input.
                Only its closed display is replaced so it can wrap. */}
            <FormSelect
              id="org-switcher-select"
              label={
                <span
                  aria-hidden="true"
                  class="flex min-w-0 flex-1 items-center justify-between gap-1.5 text-xs font-normal"
                >
                  <bdi class="min-w-0 [overflow-wrap:anywhere]">{selectedOrgName()}</bdi>
                  <ChevronDownIcon class="h-3.5 w-3.5 shrink-0 text-muted" />
                </span>
              }
              fieldBaseClass="group/org-picker grid min-w-0 max-w-44 grid-cols-[minmax(0,1fr)]"
              labelClass={`col-start-1 row-start-1 flex min-h-7 min-w-0 items-center rounded-md border border-border bg-surface px-2 py-1 shadow-xs group-focus-within/org-picker:outline-2 group-focus-within/org-picker:-outline-offset-2 group-focus-within/org-picker:outline-blue-500 ${props.loading ? 'opacity-60' : ''}`}
              selectBaseClass="z-10 col-start-1 row-start-1 h-full w-full min-w-0 cursor-pointer text-xs opacity-0 disabled:cursor-not-allowed"
              help={
                selectedNeedsIdentity() ? (
                  <>
                    Organization ID:{' '}
                    <bdi class="inline-block max-w-full align-top">{props.selectedOrgId}</bdi>
                  </>
                ) : undefined
              }
              helpClass="col-start-1 row-start-2 mt-1 leading-4 [overflow-wrap:anywhere]"
              aria-label="Organization"
              value={props.selectedOrgId}
              disabled={Boolean(props.loading)}
              onChange={(event) => props.onChange(event.currentTarget.value)}
            >
              <For each={props.orgs}>
                {(org) => (
                  <option value={org.id}>
                    {needsIdentity(org)
                      ? `${organizationName(org)} (${org.id})`
                      : organizationName(org)}
                  </option>
                )}
              </For>
            </FormSelect>
          </Show>
        }
      >
        <div class="min-w-0 max-w-44 space-y-1 text-xs leading-4">
          <div role="status" class="space-y-0.5">
            <p class="font-medium text-amber-700 dark:text-amber-300">
              {props.loading
                ? 'Checking organizations…'
                : props.listIssue === 'empty'
                  ? 'No organizations listed'
                  : 'Organization list unavailable'}
            </p>
            <Show when={props.selectedOrgId}>
              <p class="text-muted [overflow-wrap:anywhere]">
                Selected ID: <bdi>{props.selectedOrgId}</bdi>
              </p>
            </Show>
          </div>
          <Button
            size="xs"
            class="min-h-9 whitespace-normal"
            onClick={retryList}
            disabled={props.loading}
          >
            Retry organization list
          </Button>
        </div>
      </Show>
      <Show when={props.loading && !props.listIssue}>
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
