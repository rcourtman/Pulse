import { Component, For, Show, createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import {
  OrgsAPI,
  type IncomingOrganizationShare,
  type Organization,
  type OrganizationRole,
  type OrganizationShare,
} from '@/api/orgs';
import { getOrgID } from '@/utils/apiClient';
import { canManageOrg, formatOrgDate, normalizeRole, roleBadgeClass } from '@/utils/orgUtils';
import { isMultiTenantEnabled } from '@/stores/license';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { useResources, getDisplayName } from '@/hooks/useResources';
import Share2 from 'lucide-solid/icons/share-2';
import Trash2 from 'lucide-solid/icons/trash-2';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';

interface OrganizationSharingPanelProps {
  currentUser?: string;
}

type ShareAccessRole = 'viewer' | 'editor' | 'admin';

type ResourceOption = {
  id: string;
  type: string;
  name: string;
};

const accessRoleOptions: Array<{ value: ShareAccessRole; label: string }> = [
  { value: 'viewer', label: 'Viewer' },
  { value: 'editor', label: 'Editor' },
  { value: 'admin', label: 'Admin' },
];

const VALID_RESOURCE_TYPES = ['vm', 'container', 'host', 'storage', 'pbs', 'pmg'] as const;
const INVALID_RESOURCE_TYPE_ERROR = `Invalid resource type. Valid types: ${VALID_RESOURCE_TYPES.join(', ')}`;

const isValidResourceType = (value: string): value is (typeof VALID_RESOURCE_TYPES)[number] =>
  (VALID_RESOURCE_TYPES as readonly string[]).includes(value);

const normalizeShareRole = (role: OrganizationRole): ShareAccessRole => {
  const normalized = normalizeRole(role);
  if (normalized === 'admin' || normalized === 'editor') return normalized;
  return 'viewer';
};

export const OrganizationSharingPanel: Component<OrganizationSharingPanelProps> = (props) => {
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [org, setOrg] = createSignal<Organization | null>(null);
  const [orgs, setOrgs] = createSignal<Organization[]>([]);
  const [outgoingShares, setOutgoingShares] = createSignal<OrganizationShare[]>([]);
  const [incomingShares, setIncomingShares] = createSignal<IncomingOrganizationShare[]>([]);

  const [targetOrgId, setTargetOrgId] = createSignal('');
  const [resourceType, setResourceType] = createSignal('');
  const [resourceId, setResourceId] = createSignal('');
  const [resourceName, setResourceName] = createSignal('');
  const [accessRole, setAccessRole] = createSignal<ShareAccessRole>('viewer');
  const [selectedQuickPick, setSelectedQuickPick] = createSignal('');
  const [manualEntryExpanded, setManualEntryExpanded] = createSignal(false);

  const [targetOrgError, setTargetOrgError] = createSignal('');
  const [resourceTypeError, setResourceTypeError] = createSignal('');
  const [resourceIdError, setResourceIdError] = createSignal('');

  const activeOrgId = () => getOrgID() || 'default';
  const { resources } = useResources();

  const targetOrgOptions = createMemo(() =>
    orgs()
      .filter((candidate) => candidate.id !== activeOrgId())
      .sort((left, right) => (left.displayName || left.id).localeCompare(right.displayName || right.id)),
  );
  const unifiedResourceOptions = createMemo<ResourceOption[]>(() => {
    return resources()
      .filter((resource) => resource.id && resource.type)
      .map((resource) => ({
        id: resource.id,
        type: resource.type,
        name: (getDisplayName(resource) || resource.id).trim(),
      }))
      .sort((left, right) => left.name.localeCompare(right.name));
  });

  const normalizedResourceType = createMemo(() => resourceType().trim().toLowerCase());
  const normalizedResourceId = createMemo(() => resourceId().trim());
  const hasTargetOrg = createMemo(() => targetOrgId().trim() !== '');
  const hasQuickPickSelection = createMemo(() => selectedQuickPick().trim() !== '');
  const manualEntryValid = createMemo(
    () => isValidResourceType(normalizedResourceType()) && normalizedResourceId() !== '',
  );
  const canCreateShare = createMemo(
    () => !saving() && hasTargetOrg() && (hasQuickPickSelection() || manualEntryValid()),
  );

  const orgNameById = createMemo(() => {
    const map = new Map<string, string>();
    for (const organization of orgs()) {
      map.set(organization.id, organization.displayName || organization.id);
    }
    return map;
  });

  createEffect(() => {
    if (unifiedResourceOptions().length > 0) return;
    setManualEntryExpanded(true);
    setSelectedQuickPick('');
  });

  const loadShares = async (orgId: string) => {
    const [outgoing, incoming] = await Promise.all([
      OrgsAPI.listShares(orgId),
      OrgsAPI.listIncomingShares(orgId),
    ]);
    setOutgoingShares(outgoing);
    setIncomingShares(incoming);
  };

  const loadSharingData = async () => {
    setLoading(true);
    try {
      const orgId = activeOrgId();

      const [orgData, memberData, allOrgs] = await Promise.all([
        OrgsAPI.get(orgId),
        OrgsAPI.listMembers(orgId),
        OrgsAPI.list(),
      ]);

      setOrg({ ...orgData, members: memberData });
      setOrgs(allOrgs);
      if (unifiedResourceOptions().length === 0) {
        setManualEntryExpanded(true);
        setSelectedQuickPick('');
      }

      await loadShares(orgId);

      const firstTarget =
        targetOrgId().trim() ||
        allOrgs.find((candidate) => candidate.id !== orgId)?.id ||
        '';
      setTargetOrgId(firstTarget);
      setTargetOrgError(firstTarget === '' ? 'Target organization is required' : '');
    } catch (error) {
      logger.error('Failed to load organization sharing data', error);
      const msg = error instanceof Error ? error.message : '';
      if (msg.includes('402')) {
        notificationStore.error('Multi-tenant requires an Enterprise license');
      } else if (msg.includes('501')) {
        notificationStore.error('Multi-tenant is not enabled on this server');
      } else {
        notificationStore.error('Failed to load organization sharing details');
      }
    } finally {
      setLoading(false);
    }
  };

  const applyResourceQuickPick = (value: string) => {
    setSelectedQuickPick(value);
    setResourceTypeError('');
    setResourceIdError('');
    if (!value) return;

    const [nextType, nextID] = value.split('::');
    const match = unifiedResourceOptions().find(
      (resource) => resource.id === (nextID ?? '') && resource.type === (nextType ?? ''),
    );
    if (!match) return;
    setResourceType(match.type);
    setResourceId(match.id);
    setResourceName(match.name);
  };

  const toggleManualEntry = () => {
    const expanding = !manualEntryExpanded();
    setManualEntryExpanded(expanding);
    setResourceTypeError('');
    setResourceIdError('');
    if (expanding) {
      setSelectedQuickPick('');
    }
  };

  const updateTargetOrg = (value: string) => {
    setTargetOrgId(value);
    setTargetOrgError(value.trim() === '' ? 'Target organization is required' : '');
  };

  const updateResourceType = (value: string) => {
    setSelectedQuickPick('');
    setResourceType(value);
    const normalized = value.trim().toLowerCase();
    setResourceTypeError(normalized === '' || isValidResourceType(normalized) ? '' : INVALID_RESOURCE_TYPE_ERROR);
  };

  const updateResourceId = (value: string) => {
    setSelectedQuickPick('');
    setResourceId(value);
    setResourceIdError(value.trim() === '' ? 'Resource ID is required' : '');
  };

  const updateResourceName = (value: string) => {
    setSelectedQuickPick('');
    setResourceName(value);
  };

  const createShare = async () => {
    const currentOrg = org();
    if (!currentOrg) return;

    const nextTargetOrgId = targetOrgId().trim();
    const nextResourceType = normalizedResourceType();
    const nextResourceId = normalizedResourceId();
    const nextResourceName = resourceName().trim();
    const hasQuickPick = hasQuickPickSelection();
    const hasValidManualType = isValidResourceType(nextResourceType);
    const hasValidManualResourceId = nextResourceId !== '';

    setTargetOrgError(nextTargetOrgId === '' ? 'Target organization is required' : '');
    if (hasQuickPick) {
      setResourceTypeError('');
      setResourceIdError('');
    } else {
      setResourceTypeError(hasValidManualType ? '' : INVALID_RESOURCE_TYPE_ERROR);
      setResourceIdError(hasValidManualResourceId ? '' : 'Resource ID is required');
    }

    if (!nextTargetOrgId) {
      notificationStore.error('Target organization is required');
      return;
    }
    if (nextTargetOrgId === currentOrg.id) {
      notificationStore.error('Target organization must differ from the current organization');
      return;
    }
    if (!hasQuickPick && (!hasValidManualType || !hasValidManualResourceId)) {
      notificationStore.error('Valid resource type and resource ID are required');
      return;
    }

    setSaving(true);
    try {
      await OrgsAPI.createShare(currentOrg.id, {
        targetOrgId: nextTargetOrgId,
        resourceType: nextResourceType,
        resourceId: nextResourceId,
        resourceName: nextResourceName,
        accessRole: accessRole(),
      });
      notificationStore.success('Resource shared successfully');
      await loadShares(currentOrg.id);
    } catch (error) {
      logger.error('Failed to create organization share', error);
      notificationStore.error(error instanceof Error ? error.message : 'Failed to create share');
    } finally {
      setSaving(false);
    }
  };

  const deleteShare = async (share: OrganizationShare) => {
    const currentOrg = org();
    if (!currentOrg) return;
    if (!confirm(`Remove share for ${share.resourceType}:${share.resourceId}?`)) {
      return;
    }

    setSaving(true);
    try {
      await OrgsAPI.deleteShare(currentOrg.id, share.id);
      notificationStore.success('Share removed');
      await loadShares(currentOrg.id);
    } catch (error) {
      logger.error('Failed to delete organization share', error);
      notificationStore.error(error instanceof Error ? error.message : 'Failed to delete share');
    } finally {
      setSaving(false);
    }
  };

  onMount(() => {
    if (!isMultiTenantEnabled()) return;
    void loadSharingData();

    const unsubscribe = eventBus.on('org_switched', () => {
      void loadSharingData();
    });
    onCleanup(unsubscribe);
  });

  return (
    <Show when={isMultiTenantEnabled()} fallback={<div class="p-4 text-sm text-slate-500">This feature is not available.</div>}>
      <div class="space-y-6">
        <SettingsPanel
          title="Organization Sharing"
          description="Share views and resources across organizations with explicit role-based access."
          icon={<Share2 class="w-5 h-5" />}
          noPadding
          bodyClass="divide-y divide-border"
        >
          <Show
            when={!loading()}
            fallback={
              <div class="space-y-5 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
                <div class="rounded-md border border-border p-4 space-y-3">
                  <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />

                  <div class="grid gap-3 lg:grid-cols-2">
                    <div class="space-y-2">
                      <div class="h-3 w-28 animate-pulse rounded bg-surface-hover" />
                      <div class="h-10 w-full animate-pulse rounded bg-surface-hover" />
                    </div>
                    <div class="space-y-2">
                      <div class="h-3 w-20 animate-pulse rounded bg-surface-hover" />
                      <div class="h-10 w-full animate-pulse rounded bg-surface-hover" />
                    </div>
                  </div>

                  <div class="rounded-md border border-border p-3 space-y-2">
                    <div class="h-3 w-32 animate-pulse rounded bg-surface-hover" />
                    <div class="h-10 w-full animate-pulse rounded bg-surface-hover" />
                  </div>

                  <div class="grid gap-3 lg:grid-cols-3">
                    {Array.from({ length: 3 }).map(() => (
                      <div class="space-y-2">
                        <div class="h-3 w-24 animate-pulse rounded bg-surface-hover" />
                        <div class="h-10 w-full animate-pulse rounded bg-surface-hover" />
                      </div>
                    ))}
                  </div>

                  <div class="flex justify-end">
                    <div class="h-10 w-28 animate-pulse rounded bg-surface-hover" />
                  </div>
                </div>

                <div class="space-y-2">
                  <div class="h-4 w-28 animate-pulse rounded bg-surface-hover" />
                  <div class="overflow-hidden rounded-md border border-border">
                    <div class="h-10 w-full animate-pulse bg-surface-alt" />
                    {Array.from({ length: 3 }).map(() => (
                      <div class="border-t border-border-subtle px-3 py-3">
                        <div class="flex items-center gap-3">
                          <div class="h-4 w-40 animate-pulse rounded bg-surface-hover" />
                          <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
                          <div class="h-4 w-14 animate-pulse rounded-full bg-surface-hover" />
                          <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                <div class="space-y-2">
                  <div class="h-4 w-28 animate-pulse rounded bg-surface-hover" />
                  <div class="overflow-hidden rounded-md border border-border">
                    <div class="h-10 w-full animate-pulse bg-surface-alt" />
                    {Array.from({ length: 3 }).map(() => (
                      <div class="border-t border-border-subtle px-3 py-3">
                        <div class="flex items-center gap-3">
                          <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
                          <div class="h-4 w-40 animate-pulse rounded bg-surface-hover" />
                          <div class="h-4 w-14 animate-pulse rounded-full bg-surface-hover" />
                          <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            }
          >
            <Show when={canManageOrg(org(), props.currentUser)}>
              <div class="p-4 sm:p-6 hover:bg-surface-hover transition-colors">
                <div class="rounded-md border border-border p-4 space-y-3">
                  <h4 class="text-sm font-semibold text-base-content">Create Share</h4>

                  <div class="grid gap-3 lg:grid-cols-2">
                    <label class="space-y-1">
                      <span class="text-xs font-medium uppercase tracking-wide text-muted">
                        Target Organization
                      </span>
                      <select
                        value={targetOrgId()}
                        onChange={(event) => updateTargetOrg(event.currentTarget.value)}
                        class={`w-full rounded-md border bg-white px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-800 dark:text-slate-100 ${targetOrgError() ? 'border-red-400 dark:border-red-500' : 'border-border'
 }`}
                      >
                        <option value="">Select organization</option>
                        <For each={targetOrgOptions()}>
                          {(target) => <option value={target.id}>{target.displayName || target.id}</option>}
                        </For>
                      </select>
                      <Show when={targetOrgError() !== ''}>
 <p class="text-xs text-red-600 dark:text-red-400">{targetOrgError()}</p>
 </Show>
 </label>

 <label class="space-y-1">
 <span class="text-xs font-medium uppercase tracking-wide text-muted">
 Access Role
 </span>
 <select
 value={accessRole()}
 onChange={(event) => setAccessRole(event.currentTarget.value as ShareAccessRole)}
 class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 "
 >
 <For each={accessRoleOptions}>
 {(option) => <option value={option.value}>{option.label}</option>}
 </For>
 </select>
 </label>
 </div>

 <Show when={unifiedResourceOptions().length > 0}>
 <div class="rounded-md border border-blue-200 bg-blue-50 p-3 space-y-2 dark:border-blue-900 dark:bg-blue-900">
 <label class="space-y-1 block">
 <span class="text-xs font-medium uppercase tracking-wide text-blue-700 dark:text-blue-300">
 Quick Pick Resource
 </span>
 <select
 value={selectedQuickPick()}
 onChange={(event) => applyResourceQuickPick(event.currentTarget.value)}
 class="w-full rounded-md border border-blue-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-blue-700 dark:bg-slate-800 "
 >
 <option value="">Select resource</option>
 <For each={unifiedResourceOptions()}>
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
 onClick={toggleManualEntry}
 class="text-xs font-medium text-blue-700 hover:text-blue-800 dark:text-blue-300 dark:hover:text-blue-200"
 >
 {manualEntryExpanded() ?'Hide manual entry' : 'Enter manually'}
                        </button>
                      </div>
                    </div>
                  </Show>

                  <Show
                    when={unifiedResourceOptions().length === 0 || manualEntryExpanded()}
                    fallback={
                      <p class="text-xs text-muted">
                        Manual entry is hidden while quick pick is active.
                      </p>
                    }
                  >
                    <div class="grid gap-3 lg:grid-cols-3">
                      <label class="space-y-1">
                        <span class="text-xs font-medium uppercase tracking-wide text-muted">
                          Resource Type
                        </span>
                        <input
                          type="text"
                          value={resourceType()}
                          onInput={(event) => updateResourceType(event.currentTarget.value)}
                          placeholder={VALID_RESOURCE_TYPES.join(' | ')}
                          class={`w-full rounded-md border bg-white px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-800 dark:text-slate-100 ${resourceTypeError() ? 'border-red-400 dark:border-red-500' : 'border-border'
 }`}
                        />
                        <Show when={resourceTypeError() !== ''}>
 <p class="text-xs text-red-600 dark:text-red-400">{resourceTypeError()}</p>
 </Show>
 </label>

 <label class="space-y-1">
 <span class="text-xs font-medium uppercase tracking-wide text-muted">
 Resource ID
 </span>
 <input
 type="text"
 value={resourceId()}
 onInput={(event) => updateResourceId(event.currentTarget.value)}
 placeholder="resource identifier"
 class={`w-full rounded-md border bg-white px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-800 ${resourceIdError() ?'border-red-400 dark:border-red-500' : 'border-border'
 }`}
                        />
                        <Show when={resourceIdError() !== ''}>
 <p class="text-xs text-red-600 dark:text-red-400">{resourceIdError()}</p>
 </Show>
 </label>

 <label class="space-y-1">
 <span class="text-xs font-medium uppercase tracking-wide text-muted">
 Resource Name
 </span>
 <input
 type="text"
 value={resourceName()}
 onInput={(event) => updateResourceName(event.currentTarget.value)}
 placeholder="optional display name"
 class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 "
 />
 </label>
 </div>
 </Show>

 <div class="flex justify-end">
 <button
 type="button"
 onClick={createShare}
 disabled={!canCreateShare()}
 class="inline-flex w-full sm:w-auto items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
 >
 {saving() ?'Saving...' : 'Create Share'}
                    </button>
                  </div>
                </div>
              </div>
            </Show>

            <Show when={!canManageOrg(org(), props.currentUser)}>
              <div class="p-4 sm:p-6 hover:bg-surface-hover transition-colors">
                <div class="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300">
                  Admin or owner role required to create or remove organization shares.
                </div>
              </div>
            </Show>

            <div class="space-y-2 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
              <h4 class="text-sm font-semibold text-base-content">Outgoing Shares</h4>
              <div class="mt-4 -mx-4 sm:mx-0 overflow-x-auto w-full">
                <PulseDataGrid
                  data={outgoingShares()}
                  columns={[
                    {
                      key: 'resourceName',
                      label: 'Resource',
                      render: (share) => (
                        <div class="flex flex-col">
                          <span class="text-base-content">{share.resourceName || share.resourceId}</span>
                          <span class="text-xs text-muted">
                            {share.resourceType}:{share.resourceId}
                          </span>
                        </div>
                      )
                    },
                    {
                      key: 'targetOrgId',
                      label: 'Target Org',
                      render: (share) => <span class="text-base-content">{orgNameById().get(share.targetOrgId) || share.targetOrgId}</span>
                    },
                    {
                      key: 'accessRole',
                      label: 'Access',
                      render: (share) => {
                        const role = normalizeShareRole(share.accessRole);
                        return (
                          <span class={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${roleBadgeClass(role)}`}>
                            {role}
                          </span>
                        );
                      }
                    },
                    {
                      key: 'createdAt',
                      label: 'Created',
                      render: (share) => <span class="text-muted">{formatOrgDate(share.createdAt)}</span>
                    },
                    {
                      key: 'actions',
                      label: 'Actions',
                      align: 'right',
                      render: (share) => (
                        <Show when={canManageOrg(org(), props.currentUser)}>
                          <button
                            type="button"
                            onClick={() => {
                              void deleteShare(share);
                            }}
                            disabled={saving()}
                            class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900 disabled:cursor-not-allowed disabled:opacity-60"
                          >
                            <Trash2 class="w-3.5 h-3.5" />
                            Remove
                          </button>
                        </Show>
                      )
                    }
                  ]}
                  keyExtractor={(share) => share.id}
                  emptyState="No outgoing shares configured."
                  desktopMinWidth="760px"
                  class="border-x-0 sm:border-x sm:border-t sm:border-b sm:rounded-md border-y border-border"
                />
              </div>
            </div>

            <div class="space-y-2 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
              <h4 class="text-sm font-semibold text-base-content">Incoming Shares</h4>
              <div class="mt-4 -mx-4 sm:mx-0 overflow-x-auto w-full">
                <PulseDataGrid
                  data={incomingShares()}
                  columns={[
                    {
                      key: 'sourceOrg',
                      label: 'Source Org',
                      render: (share) => <span class="text-base-content">{share.sourceOrgName || share.sourceOrgId}</span>
                    },
                    {
                      key: 'resource',
                      label: 'Resource',
                      render: (share) => (
                        <div class="flex flex-col">
                          <span class="text-base-content">{share.resourceName || share.resourceId}</span>
                          <span class="text-xs text-muted">
                            {share.resourceType}:{share.resourceId}
                          </span>
                        </div>
                      )
                    },
                    {
                      key: 'accessRole',
                      label: 'Access',
                      render: (share) => {
                        const role = normalizeShareRole(share.accessRole);
                        return (
                          <span class={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${roleBadgeClass(role)}`}>
                            {role}
                          </span>
                        );
                      }
                    },
                    {
                      key: 'createdAt',
                      label: 'Shared',
                      render: (share) => <span class="text-muted">{formatOrgDate(share.createdAt)}</span>
                    }
                  ]}
                  keyExtractor={(share) => share.id}
                  emptyState="No incoming shares from other organizations."
                  desktopMinWidth="620px"
                  class="border-x-0 sm:border-x sm:border-t sm:border-b sm:rounded-md border-y border-border"
                />
              </div>
            </div>
          </Show>
        </SettingsPanel>
      </div>
    </Show>
  );
};

export default OrganizationSharingPanel;
