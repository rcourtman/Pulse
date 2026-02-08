import { Component, For, Show, createMemo, createSignal, onMount } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import {
  OrgsAPI,
  type IncomingOrganizationShare,
  type Organization,
  type OrganizationMember,
  type OrganizationRole,
  type OrganizationShare,
} from '@/api/orgs';
import { apiFetchJSON, getOrgID } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import Share2 from 'lucide-solid/icons/share-2';
import Trash2 from 'lucide-solid/icons/trash-2';

interface OrganizationSharingPanelProps {
  currentUser?: string;
}

type ShareAccessRole = 'viewer' | 'editor' | 'admin';

type ResourceOption = {
  id: string;
  type: string;
  name: string;
};

type ResourcesResponse = {
  resources?: Array<{
    id?: string;
    type?: string;
    name?: string;
    displayName?: string;
  }>;
};

const shareRoleClass: Record<ShareAccessRole, string> = {
  admin: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-200',
  editor: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-200',
  viewer: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
};

const accessRoleOptions: Array<{ value: ShareAccessRole; label: string }> = [
  { value: 'viewer', label: 'Viewer' },
  { value: 'editor', label: 'Editor' },
  { value: 'admin', label: 'Admin' },
];

const normalizeRole = (role: OrganizationRole): Exclude<OrganizationRole, 'member'> => {
  if (role === 'member') return 'viewer';
  return role;
};

const normalizeShareRole = (role: OrganizationRole): ShareAccessRole => {
  const normalized = normalizeRole(role);
  if (normalized === 'admin' || normalized === 'editor') return normalized;
  return 'viewer';
};

const formatDate = (value?: string) => {
  if (!value) return 'Unknown';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString();
};

const canManageOrg = (
  org: Organization | null,
  members: OrganizationMember[],
  currentUser?: string,
) => {
  if (!org || !currentUser) return false;
  if (org.ownerUserId === currentUser) return true;
  const role = normalizeRole(members.find((m) => m.userId === currentUser)?.role ?? 'viewer');
  return role === 'admin' || role === 'owner';
};

const toResourceOptions = (response: ResourcesResponse): ResourceOption[] => {
  const resources = response.resources ?? [];
  const normalized = resources
    .map((resource) => ({
      id: (resource.id ?? '').trim(),
      type: (resource.type ?? '').trim(),
      name: (resource.displayName ?? resource.name ?? resource.id ?? '').trim(),
    }))
    .filter((resource) => resource.id !== '' && resource.type !== '');

  normalized.sort((left, right) => left.name.localeCompare(right.name));
  return normalized;
};

export const OrganizationSharingPanel: Component<OrganizationSharingPanelProps> = (props) => {
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [org, setOrg] = createSignal<Organization | null>(null);
  const [members, setMembers] = createSignal<OrganizationMember[]>([]);
  const [orgs, setOrgs] = createSignal<Organization[]>([]);
  const [resourceOptions, setResourceOptions] = createSignal<ResourceOption[]>([]);
  const [outgoingShares, setOutgoingShares] = createSignal<OrganizationShare[]>([]);
  const [incomingShares, setIncomingShares] = createSignal<IncomingOrganizationShare[]>([]);

  const [targetOrgId, setTargetOrgId] = createSignal('');
  const [resourceType, setResourceType] = createSignal('');
  const [resourceId, setResourceId] = createSignal('');
  const [resourceName, setResourceName] = createSignal('');
  const [accessRole, setAccessRole] = createSignal<ShareAccessRole>('viewer');

  const activeOrgId = () => getOrgID() || 'default';

  const targetOrgOptions = createMemo(() =>
    orgs()
      .filter((candidate) => candidate.id !== activeOrgId())
      .sort((left, right) => (left.displayName || left.id).localeCompare(right.displayName || right.id)),
  );

  const orgNameById = createMemo(() => {
    const map = new Map<string, string>();
    for (const organization of orgs()) {
      map.set(organization.id, organization.displayName || organization.id);
    }
    return map;
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
      const resourcesPromise = apiFetchJSON<ResourcesResponse>('/api/resources').catch((error) => {
        logger.warn('Failed to load resource options for sharing', error);
        return { resources: [] };
      });

      const [orgData, memberData, allOrgs, resources] = await Promise.all([
        OrgsAPI.get(orgId),
        OrgsAPI.listMembers(orgId),
        OrgsAPI.list(),
        resourcesPromise,
      ]);

      setOrg(orgData);
      setMembers(memberData);
      setOrgs(allOrgs);
      setResourceOptions(toResourceOptions(resources));

      await loadShares(orgId);

      const firstTarget =
        targetOrgId().trim() ||
        allOrgs.find((candidate) => candidate.id !== orgId)?.id ||
        '';
      setTargetOrgId(firstTarget);
    } catch (error) {
      logger.error('Failed to load organization sharing data', error);
      notificationStore.error('Failed to load organization sharing details');
    } finally {
      setLoading(false);
    }
  };

  const applyResourceQuickPick = (value: string) => {
    const [nextType, nextID] = value.split('::');
    const match = resourceOptions().find(
      (resource) => resource.id === (nextID ?? '') && resource.type === (nextType ?? ''),
    );
    if (!match) return;
    setResourceType(match.type);
    setResourceId(match.id);
    setResourceName(match.name);
  };

  const createShare = async () => {
    const currentOrg = org();
    if (!currentOrg) return;

    const nextTargetOrgId = targetOrgId().trim();
    const nextResourceType = resourceType().trim().toLowerCase();
    const nextResourceId = resourceId().trim();
    const nextResourceName = resourceName().trim();

    if (!nextTargetOrgId) {
      notificationStore.error('Target organization is required');
      return;
    }
    if (nextTargetOrgId === currentOrg.id) {
      notificationStore.error('Target organization must differ from the current organization');
      return;
    }
    if (!nextResourceType || !nextResourceId) {
      notificationStore.error('Resource type and ID are required');
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
    void loadSharingData();
  });

  return (
    <div class="space-y-6">
      <SettingsPanel
        title="Organization Sharing"
        description="Share views and resources across organizations with explicit role-based access."
        icon={<Share2 class="w-5 h-5" />}
        bodyClass="space-y-5"
      >
        <Show when={!loading()} fallback={<p class="text-sm text-gray-500 dark:text-gray-400">Loading sharing settings...</p>}>
          <Show when={canManageOrg(org(), members(), props.currentUser)}>
            <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-4 space-y-3">
              <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Create Share</h4>

              <div class="grid gap-3 lg:grid-cols-2">
                <label class="space-y-1">
                  <span class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    Target Organization
                  </span>
                  <select
                    value={targetOrgId()}
                    onChange={(event) => setTargetOrgId(event.currentTarget.value)}
                    class="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                  >
                    <option value="">Select organization</option>
                    <For each={targetOrgOptions()}>
                      {(target) => <option value={target.id}>{target.displayName || target.id}</option>}
                    </For>
                  </select>
                </label>

                <label class="space-y-1">
                  <span class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    Access Role
                  </span>
                  <select
                    value={accessRole()}
                    onChange={(event) => setAccessRole(event.currentTarget.value as ShareAccessRole)}
                    class="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                  >
                    <For each={accessRoleOptions}>
                      {(option) => <option value={option.value}>{option.label}</option>}
                    </For>
                  </select>
                </label>
              </div>

              <Show when={resourceOptions().length > 0}>
                <label class="space-y-1 block">
                  <span class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    Quick Pick Resource
                  </span>
                  <select
                    value={resourceType() && resourceId() ? `${resourceType()}::${resourceId()}` : ''}
                    onChange={(event) => applyResourceQuickPick(event.currentTarget.value)}
                    class="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                  >
                    <option value="">Select resource</option>
                    <For each={resourceOptions()}>
                      {(resource) => (
                        <option value={`${resource.type}::${resource.id}`}>
                          {resource.name} ({resource.type})
                        </option>
                      )}
                    </For>
                  </select>
                </label>
              </Show>

              <div class="grid gap-3 lg:grid-cols-3">
                <label class="space-y-1">
                  <span class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    Resource Type
                  </span>
                  <input
                    type="text"
                    value={resourceType()}
                    onInput={(event) => setResourceType(event.currentTarget.value)}
                    placeholder="vm | container | view"
                    class="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                  />
                </label>

                <label class="space-y-1">
                  <span class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    Resource ID
                  </span>
                  <input
                    type="text"
                    value={resourceId()}
                    onInput={(event) => setResourceId(event.currentTarget.value)}
                    placeholder="resource identifier"
                    class="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                  />
                </label>

                <label class="space-y-1">
                  <span class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    Resource Name
                  </span>
                  <input
                    type="text"
                    value={resourceName()}
                    onInput={(event) => setResourceName(event.currentTarget.value)}
                    placeholder="optional display name"
                    class="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                  />
                </label>
              </div>

              <div class="flex justify-end">
                <button
                  type="button"
                  onClick={createShare}
                  disabled={saving()}
                  class="inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {saving() ? 'Saving...' : 'Create Share'}
                </button>
              </div>
            </div>
          </Show>

          <Show when={!canManageOrg(org(), members(), props.currentUser)}>
            <div class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
              Admin or owner role required to create or remove organization shares.
            </div>
          </Show>

          <div class="space-y-2">
            <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Outgoing Shares</h4>
            <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
              <table class="w-full text-sm">
                <thead class="bg-gray-50 dark:bg-gray-800/70">
                  <tr>
                    <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-300">Resource</th>
                    <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-300">Target Org</th>
                    <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-300">Access</th>
                    <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-300">Created</th>
                    <th class="px-3 py-2 text-right font-medium text-gray-600 dark:text-gray-300">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  <Show
                    when={outgoingShares().length > 0}
                    fallback={
                      <tr>
                        <td colSpan={5} class="px-3 py-4 text-center text-sm text-gray-500 dark:text-gray-400">
                          No outgoing shares configured.
                        </td>
                      </tr>
                    }
                  >
                    <For each={outgoingShares()}>
                      {(share) => {
                        const role = normalizeShareRole(share.accessRole);
                        return (
                          <tr class="border-t border-gray-100 dark:border-gray-800">
                            <td class="px-3 py-2 text-gray-900 dark:text-gray-100">
                              <div class="flex flex-col">
                                <span>{share.resourceName || share.resourceId}</span>
                                <span class="text-xs text-gray-500 dark:text-gray-400">
                                  {share.resourceType}:{share.resourceId}
                                </span>
                              </div>
                            </td>
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-300">
                              {orgNameById().get(share.targetOrgId) || share.targetOrgId}
                            </td>
                            <td class="px-3 py-2">
                              <span class={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${shareRoleClass[role]}`}>
                                {role}
                              </span>
                            </td>
                            <td class="px-3 py-2 text-gray-600 dark:text-gray-400">{formatDate(share.createdAt)}</td>
                            <td class="px-3 py-2 text-right">
                              <Show when={canManageOrg(org(), members(), props.currentUser)}>
                                <button
                                  type="button"
                                  onClick={() => {
                                    void deleteShare(share);
                                  }}
                                  disabled={saving()}
                                  class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900/20 disabled:cursor-not-allowed disabled:opacity-60"
                                >
                                  <Trash2 class="w-3.5 h-3.5" />
                                  Remove
                                </button>
                              </Show>
                            </td>
                          </tr>
                        );
                      }}
                    </For>
                  </Show>
                </tbody>
              </table>
            </div>
          </div>

          <div class="space-y-2">
            <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Incoming Shares</h4>
            <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
              <table class="w-full text-sm">
                <thead class="bg-gray-50 dark:bg-gray-800/70">
                  <tr>
                    <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-300">Source Org</th>
                    <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-300">Resource</th>
                    <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-300">Access</th>
                    <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-300">Shared</th>
                  </tr>
                </thead>
                <tbody>
                  <Show
                    when={incomingShares().length > 0}
                    fallback={
                      <tr>
                        <td colSpan={4} class="px-3 py-4 text-center text-sm text-gray-500 dark:text-gray-400">
                          No incoming shares from other organizations.
                        </td>
                      </tr>
                    }
                  >
                    <For each={incomingShares()}>
                      {(share) => {
                        const role = normalizeShareRole(share.accessRole);
                        return (
                          <tr class="border-t border-gray-100 dark:border-gray-800">
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{share.sourceOrgName || share.sourceOrgId}</td>
                            <td class="px-3 py-2 text-gray-900 dark:text-gray-100">
                              <div class="flex flex-col">
                                <span>{share.resourceName || share.resourceId}</span>
                                <span class="text-xs text-gray-500 dark:text-gray-400">
                                  {share.resourceType}:{share.resourceId}
                                </span>
                              </div>
                            </td>
                            <td class="px-3 py-2">
                              <span class={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${shareRoleClass[role]}`}>
                                {role}
                              </span>
                            </td>
                            <td class="px-3 py-2 text-gray-600 dark:text-gray-400">{formatDate(share.createdAt)}</td>
                          </tr>
                        );
                      }}
                    </For>
                  </Show>
                </tbody>
              </table>
            </div>
          </div>
        </Show>
      </SettingsPanel>
    </div>
  );
};

export default OrganizationSharingPanel;
