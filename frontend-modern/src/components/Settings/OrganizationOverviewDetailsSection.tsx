import { Component, Show } from 'solid-js';
import { formatOrgDate } from '@/utils/orgUtils';
import { getOrganizationOverviewManageRequiredMessage } from '@/utils/organizationSettingsPresentation';
import type { useOrganizationOverviewPanelState } from './useOrganizationOverviewPanelState';

interface OrganizationOverviewDetailsSectionProps {
  state: ReturnType<typeof useOrganizationOverviewPanelState>;
}

export const OrganizationOverviewDetailsSection: Component<
  OrganizationOverviewDetailsSectionProps
> = (props) => (
  <Show when={props.state.org()}>
    {(currentOrg) => (
      <div class="space-y-6 p-4 sm:p-6">
        <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div class="rounded-md border border-border p-3">
            <p class="text-xs uppercase tracking-wide text-muted">Organization</p>
            <p class="mt-1 text-sm font-medium text-base-content">
              {currentOrg().displayName || currentOrg().id}
            </p>
          </div>
          <div class="rounded-md border border-border p-3">
            <p class="text-xs uppercase tracking-wide text-muted">Org ID</p>
            <p class="mt-1 text-sm font-mono break-all text-base-content">{currentOrg().id}</p>
          </div>
          <div class="rounded-md border border-border p-3">
            <p class="text-xs uppercase tracking-wide text-muted">Created</p>
            <p class="mt-1 text-sm font-medium text-base-content">
              {formatOrgDate(currentOrg().createdAt)}
            </p>
          </div>
          <div class="rounded-md border border-border p-3">
            <p class="text-xs uppercase tracking-wide text-muted">Members</p>
            <p class="mt-1 text-sm font-medium text-base-content">{props.state.members().length}</p>
          </div>
        </div>

        <div class="space-y-2">
          <label
            class="block text-sm font-medium text-base-content"
            for="org-display-name-input"
          >
            Display Name
          </label>
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
            <input
              id="org-display-name-input"
              type="text"
              value={props.state.displayNameDraft()}
              onInput={(event) => props.state.setDisplayNameDraft(event.currentTarget.value)}
              disabled={!props.state.canManageCurrentOrg() || props.state.saving()}
              class="w-full rounded-md border px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 "
            />
            <button
              type="button"
              onClick={props.state.saveDisplayName}
              disabled={!props.state.canManageCurrentOrg() || props.state.saving()}
              class="inline-flex w-full sm:w-auto items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {props.state.saving() ? 'Saving...' : 'Save'}
            </button>
          </div>
          <Show when={!props.state.canManageCurrentOrg()}>
            <p class="text-xs text-muted">{getOrganizationOverviewManageRequiredMessage()}</p>
          </Show>
        </div>
      </div>
    )}
  </Show>
);
