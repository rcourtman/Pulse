import { Component, For, Show, createMemo, createUniqueId } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { FormSelect } from '@/components/shared/FormSelect';
import { CANONICAL_RESOURCE_TYPES } from '@/utils/canonicalResourceTypes';
import {
  ORGANIZATION_SHARE_ROLE_OPTIONS,
  type ShareAccessRole,
} from '@/utils/organizationRolePresentation';
import { getShareOrganizationLabel } from '@/utils/organizationSettingsPresentation';
import type {
  ResourceOption,
  useOrganizationSharingPanelState,
} from './useOrganizationSharingPanelState';

interface OrganizationSharingCreateSectionProps {
  state: ReturnType<typeof useOrganizationSharingPanelState>;
}

export const OrganizationSharingCreateSection: Component<OrganizationSharingCreateSectionProps> = (
  props,
) => {
  const fieldID = createUniqueId();
  const targetOrgID = `${fieldID}-target-org`;
  const resourceTypeID = `${fieldID}-resource-type`;
  const resourceID = `${fieldID}-resource-id`;
  // Keep names stable; expose changing validation as each control's description.
  // Describe only a current choice, not leftover draft fields after withdrawal.
  const selectedTarget = createMemo(() =>
    props.state.targetOrgOptions().find((target) => target.id === props.state.targetOrgId()),
  );
  const selectedResource = createMemo(() =>
    props.state
      .unifiedResourceOptions()
      .find((resource) => `${resource.type}::${resource.id}` === props.state.selectedQuickPick()),
  );
  const resourceOptionLabel = (resource: ResourceOption) => {
    const identity = `${resource.type}:${resource.id}`;
    return resource.name && resource.name !== resource.id
      ? `${resource.name} (${identity})`
      : identity;
  };

  return (
    <>
      <Show when={props.state.canManageCurrentOrg()}>
        <form
          aria-label="Create Share"
          onSubmit={(event) => {
            event.preventDefault();
            void props.state.createShare();
          }}
          class="p-4 sm:p-6"
        >
          <fieldset
            disabled={props.state.saving()}
            class="rounded-md border border-border p-3 space-y-2 sm:p-4 sm:space-y-3"
          >
            <h4 class="text-sm font-semibold text-base-content">Create Share</h4>

            <div class="grid gap-2 sm:gap-3 lg:grid-cols-2">
              <div class="space-y-1">
                <FormSelect
                  id={targetOrgID}
                  label="Target Organization"
                  aria-invalid={props.state.targetOrgError() ? true : undefined}
                  aria-describedby={
                    props.state.targetOrgError() ? `${targetOrgID}-error` : undefined
                  }
                  labelClass="text-xs font-medium uppercase tracking-wide text-muted"
                  value={props.state.targetOrgId()}
                  onChange={(event) => props.state.updateTargetOrg(event.currentTarget.value)}
                  help={
                    selectedTarget() && (
                      <>
                        <Show
                          when={
                            selectedTarget()?.displayName &&
                            selectedTarget()?.displayName !== selectedTarget()?.id
                          }
                          fallback="Organization "
                        >
                          <bdi>{selectedTarget()?.displayName}</bdi> ·{' '}
                        </Show>
                        ID:{' '}
                        <bdi class="font-mono whitespace-pre-wrap break-all">
                          {selectedTarget()?.id}
                        </bdi>
                      </>
                    )
                  }
                  helpClass="text-xs [overflow-wrap:anywhere]"
                  selectBaseClass={`w-full rounded-md border bg-surface px-3 py-2 text-sm text-base-content shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-500 ${props.state.targetOrgError() ? 'border-red-400 dark:border-red-500' : 'border-border'}`}
                >
                  <option value="">Select organization</option>
                  <For each={props.state.targetOrgOptions()}>
                    {(target) => (
                      <option value={target.id}>
                        {getShareOrganizationLabel(target.id, target.displayName)}
                      </option>
                    )}
                  </For>
                </FormSelect>
                <Show when={props.state.targetOrgError() !== ''}>
                  <p id={`${targetOrgID}-error`} class="text-xs text-red-600 dark:text-red-400">
                    {props.state.targetOrgError()}
                  </p>
                </Show>
              </div>

              <FormSelect
                label="Access Role"
                labelClass="text-xs font-medium uppercase tracking-wide text-muted"
                fieldBaseClass="space-y-1"
                value={props.state.accessRole()}
                onChange={(event) =>
                  props.state.setAccessRole(event.currentTarget.value as ShareAccessRole)
                }
                selectBaseClass="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-500"
              >
                <For each={ORGANIZATION_SHARE_ROLE_OPTIONS}>
                  {(option) => <option value={option.value}>{option.label}</option>}
                </For>
              </FormSelect>
            </div>

            <Show when={props.state.unifiedResourceOptions().length > 0}>
              <div class="rounded-md border border-blue-200 bg-blue-50 p-2 space-y-2 sm:p-3 dark:border-blue-900/25 dark:bg-blue-900/25">
                <FormSelect
                  label="Quick Pick Resource"
                  labelClass="text-xs font-medium uppercase tracking-wide text-blue-700 dark:text-blue-300"
                  fieldBaseClass="space-y-1 block"
                  value={props.state.selectedQuickPick()}
                  onChange={(event) =>
                    props.state.applyResourceQuickPick(event.currentTarget.value)
                  }
                  help={
                    selectedResource() && (
                      <>
                        Resource:{' '}
                        <Show
                          when={
                            selectedResource()?.name &&
                            selectedResource()?.name !== selectedResource()?.id
                          }
                        >
                          <bdi>{selectedResource()?.name}</bdi> ·{' '}
                        </Show>
                        <bdi class="font-mono whitespace-pre-wrap break-all">
                          {selectedResource()?.type}:{selectedResource()?.id}
                        </bdi>
                      </>
                    )
                  }
                  helpClass="text-xs text-blue-700 dark:text-blue-300 [overflow-wrap:anywhere]"
                  selectBaseClass="w-full rounded-md border border-blue-300 bg-surface px-3 py-2 text-sm text-base-content shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-500 dark:border-blue-700"
                >
                  <option value="">Select resource</option>
                  <For each={props.state.unifiedResourceOptions()}>
                    {(resource) => (
                      <option value={`${resource.type}::${resource.id}`}>
                        {resourceOptionLabel(resource)}
                      </option>
                    )}
                  </For>
                </FormSelect>
                <div class="flex items-center justify-between gap-2">
                  <p class="min-w-0 flex-1 text-xs text-blue-700 dark:text-blue-300">
                    Choose a discovered resource, or switch to manual entry.
                  </p>
                  <Button
                    type="button"
                    variant="ghost"
                    size="xs"
                    class="text-blue-700 hover:text-blue-800 dark:text-blue-300 dark:hover:text-blue-200"
                    onClick={props.state.toggleManualEntry}
                  >
                    {props.state.manualEntryExpanded() ? 'Hide manual entry' : 'Enter manually'}
                  </Button>
                </div>
              </div>
            </Show>

            <Show when={props.state.quickPickUnavailable()}>
              <p role="status" class="text-sm text-amber-700 dark:text-amber-300">
                The selected resource is no longer available. Choose another resource or enter it
                manually.
              </p>
            </Show>
            <Show
              when={
                props.state.unifiedResourceOptions().length === 0 ||
                props.state.manualEntryExpanded()
              }
              fallback={
                <p class="text-xs text-muted">Manual entry is hidden while quick pick is active.</p>
              }
            >
              <div class="grid gap-3 lg:grid-cols-3">
                <div class="space-y-1">
                  <label
                    for={resourceTypeID}
                    class="text-xs font-medium uppercase tracking-wide text-muted"
                  >
                    Resource Type
                  </label>
                  <input
                    id={resourceTypeID}
                    aria-invalid={props.state.resourceTypeError() ? true : undefined}
                    aria-describedby={
                      props.state.resourceTypeError() ? `${resourceTypeID}-error` : undefined
                    }
                    type="text"
                    value={props.state.resourceType()}
                    onInput={(event) => props.state.updateResourceType(event.currentTarget.value)}
                    placeholder={CANONICAL_RESOURCE_TYPES.join(' | ')}
                    class={`w-full rounded-md border bg-surface px-3 py-2 text-sm text-base-content shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-500 ${props.state.resourceTypeError() ? 'border-red-400 dark:border-red-500' : 'border-border'}`}
                  />
                  <Show when={props.state.resourceTypeError() !== ''}>
                    <p
                      id={`${resourceTypeID}-error`}
                      class="text-xs text-red-600 dark:text-red-400"
                    >
                      {props.state.resourceTypeError()}
                    </p>
                  </Show>
                </div>

                <div class="space-y-1">
                  <label
                    for={resourceID}
                    class="text-xs font-medium uppercase tracking-wide text-muted"
                  >
                    Resource ID
                  </label>
                  <input
                    id={resourceID}
                    aria-invalid={props.state.resourceIdError() ? true : undefined}
                    aria-describedby={
                      props.state.resourceIdError() ? `${resourceID}-error` : undefined
                    }
                    type="text"
                    value={props.state.resourceId()}
                    onInput={(event) => props.state.updateResourceId(event.currentTarget.value)}
                    placeholder="resource identifier"
                    class={`w-full rounded-md border bg-surface px-3 py-2 text-sm text-base-content shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-500 ${props.state.resourceIdError() ? 'border-red-400 dark:border-red-500' : 'border-border'}`}
                  />
                  <Show when={props.state.resourceIdError() !== ''}>
                    <p id={`${resourceID}-error`} class="text-xs text-red-600 dark:text-red-400">
                      {props.state.resourceIdError()}
                    </p>
                  </Show>
                </div>

                <label class="space-y-1">
                  <span class="text-xs font-medium uppercase tracking-wide text-muted">
                    Resource Name
                  </span>
                  <input
                    type="text"
                    value={props.state.resourceName()}
                    onInput={(event) => props.state.updateResourceName(event.currentTarget.value)}
                    placeholder="optional display name"
                    class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-500"
                  />
                </label>
              </div>
            </Show>

            <div class="flex justify-end">
              <Button
                type="submit"
                data-org-sharing-action="create"
                data-org-sharing-target="draft"
                variant="primary"
                size="settingsAction"
                class="w-full sm:w-auto"
                disabled={!props.state.canCreateShare()}
              >
                {props.state.saving() ? 'Saving...' : 'Create Share'}
              </Button>
            </div>
          </fieldset>
        </form>
      </Show>

      <Show when={!props.state.canManageCurrentOrg() && !props.state.readOnly()}>
        <div class="p-4 sm:p-6">
          <div class="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/25 dark:text-amber-300">
            Admin or owner role required to create or remove organization shares.
          </div>
        </div>
      </Show>
    </>
  );
};
