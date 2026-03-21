import { Component, For, Show } from 'solid-js';
import { CANONICAL_RESOURCE_TYPES } from '@/utils/canonicalResourceTypes';
import {
  ORGANIZATION_SHARE_ROLE_OPTIONS,
  type ShareAccessRole,
} from '@/utils/organizationRolePresentation';
import type { useOrganizationSharingPanelState } from './useOrganizationSharingPanelState';

interface OrganizationSharingCreateSectionProps {
  state: ReturnType<typeof useOrganizationSharingPanelState>;
}

export const OrganizationSharingCreateSection: Component<OrganizationSharingCreateSectionProps> = (
  props,
) => (
  <>
    <Show when={props.state.canManageCurrentOrg()}>
      <div class="p-4 sm:p-6">
        <div class="rounded-md border border-border p-4 space-y-3">
          <h4 class="text-sm font-semibold text-base-content">Create Share</h4>

          <div class="grid gap-3 lg:grid-cols-2">
            <label class="space-y-1">
              <span class="text-xs font-medium uppercase tracking-wide text-muted">
                Target Organization
              </span>
              <select
                value={props.state.targetOrgId()}
                onChange={(event) => props.state.updateTargetOrg(event.currentTarget.value)}
                class={`w-full rounded-md border bg-surface px-3 py-2 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 ${props.state.targetOrgError() ? 'border-red-400 dark:border-red-500' : 'border-border'}`}
              >
                <option value="">Select organization</option>
                <For each={props.state.targetOrgOptions()}>
                  {(target) => <option value={target.id}>{target.displayName || target.id}</option>}
                </For>
              </select>
              <Show when={props.state.targetOrgError() !== ''}>
                <p class="text-xs text-red-600 dark:text-red-400">
                  {props.state.targetOrgError()}
                </p>
              </Show>
            </label>

            <label class="space-y-1">
              <span class="text-xs font-medium uppercase tracking-wide text-muted">
                Access Role
              </span>
              <select
                value={props.state.accessRole()}
                onChange={(event) =>
                  props.state.setAccessRole(event.currentTarget.value as ShareAccessRole)
                }
                class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <For each={ORGANIZATION_SHARE_ROLE_OPTIONS}>
                  {(option) => <option value={option.value}>{option.label}</option>}
                </For>
              </select>
            </label>
          </div>

          <Show when={props.state.unifiedResourceOptions().length > 0}>
            <div class="rounded-md border border-blue-200 bg-blue-50 p-3 space-y-2 dark:border-blue-900 dark:bg-blue-900">
              <label class="space-y-1 block">
                <span class="text-xs font-medium uppercase tracking-wide text-blue-700 dark:text-blue-300">
                  Quick Pick Resource
                </span>
                <select
                  value={props.state.selectedQuickPick()}
                  onChange={(event) => props.state.applyResourceQuickPick(event.currentTarget.value)}
                  class="w-full rounded-md border border-blue-300 bg-surface px-3 py-2 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-blue-700"
                >
                  <option value="">Select resource</option>
                  <For each={props.state.unifiedResourceOptions()}>
                    {(resource) => (
                      <option value={`${resource.type}::${resource.id}`}>
                        {resource.name} ({resource.type})
                      </option>
                    )}
                  </For>
                </select>
              </label>
              <div class="flex flex-col items-start gap-2 sm:flex-row sm:items-center sm:justify-between">
                <p class="text-xs text-blue-700 dark:text-blue-300">
                  Choose a discovered resource, or switch to manual entry.
                </p>
                <button
                  type="button"
                  onClick={props.state.toggleManualEntry}
                  class="text-xs font-medium text-blue-700 hover:text-blue-800 dark:text-blue-300 dark:hover:text-blue-200"
                >
                  {props.state.manualEntryExpanded() ? 'Hide manual entry' : 'Enter manually'}
                </button>
              </div>
            </div>
          </Show>

          <Show
            when={
              props.state.unifiedResourceOptions().length === 0 || props.state.manualEntryExpanded()
            }
            fallback={
              <p class="text-xs text-muted">Manual entry is hidden while quick pick is active.</p>
            }
          >
            <div class="grid gap-3 lg:grid-cols-3">
              <label class="space-y-1">
                <span class="text-xs font-medium uppercase tracking-wide text-muted">
                  Resource Type
                </span>
                <input
                  type="text"
                  value={props.state.resourceType()}
                  onInput={(event) => props.state.updateResourceType(event.currentTarget.value)}
                  placeholder={CANONICAL_RESOURCE_TYPES.join(' | ')}
                  class={`w-full rounded-md border bg-surface px-3 py-2 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 ${props.state.resourceTypeError() ? 'border-red-400 dark:border-red-500' : 'border-border'}`}
                />
                <Show when={props.state.resourceTypeError() !== ''}>
                  <p class="text-xs text-red-600 dark:text-red-400">
                    {props.state.resourceTypeError()}
                  </p>
                </Show>
              </label>

              <label class="space-y-1">
                <span class="text-xs font-medium uppercase tracking-wide text-muted">
                  Resource ID
                </span>
                <input
                  type="text"
                  value={props.state.resourceId()}
                  onInput={(event) => props.state.updateResourceId(event.currentTarget.value)}
                  placeholder="resource identifier"
                  class={`w-full rounded-md border bg-surface px-3 py-2 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 ${props.state.resourceIdError() ? 'border-red-400 dark:border-red-500' : 'border-border'}`}
                />
                <Show when={props.state.resourceIdError() !== ''}>
                  <p class="text-xs text-red-600 dark:text-red-400">
                    {props.state.resourceIdError()}
                  </p>
                </Show>
              </label>

              <label class="space-y-1">
                <span class="text-xs font-medium uppercase tracking-wide text-muted">
                  Resource Name
                </span>
                <input
                  type="text"
                  value={props.state.resourceName()}
                  onInput={(event) => props.state.updateResourceName(event.currentTarget.value)}
                  placeholder="optional display name"
                  class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </label>
            </div>
          </Show>

          <div class="flex justify-end">
            <button
              type="button"
              onClick={props.state.createShare}
              disabled={!props.state.canCreateShare()}
              class="inline-flex w-full sm:w-auto items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {props.state.saving() ? 'Saving...' : 'Create Share'}
            </button>
          </div>
        </div>
      </div>
    </Show>

    <Show when={!props.state.canManageCurrentOrg()}>
      <div class="p-4 sm:p-6">
        <div class="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300">
          Admin or owner role required to create or remove organization shares.
        </div>
      </div>
    </Show>
  </>
);
