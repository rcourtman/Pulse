import { batch, createEffect, createMemo, createSignal, on, onCleanup } from 'solid-js';
import { unwrap } from 'solid-js/store';
import {
  OrgsAPI,
  type IncomingOrganizationShare,
  type Organization,
  type OrganizationAccessRequestOptions,
  type OrganizationShare,
} from '@/api/orgs';
import { useResources } from '@/hooks/useResources';
import { eventBus } from '@/stores/events';
import { isMultiTenantEnabled } from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import {
  presentationPolicyHidesOrganizationSurfaces,
  presentationPolicyIsReadOnly,
} from '@/stores/sessionPresentationPolicy';
import { getOrgID } from '@/utils/apiClient';
import {
  INVALID_RESOURCE_TYPE_ERROR,
  isCanonicalResourceType,
  normalizeCanonicalResourceTypeInput,
} from '@/utils/canonicalResourceTypes';
import { logger } from '@/utils/logger';
import { normalizeOrgScope } from '@/utils/orgScope';
import { getPreferredResourceDisplayName } from '@/utils/resourceIdentity';
import type { ShareAccessRole } from '@/utils/organizationRolePresentation';
import {
  getOrganizationIncomingShareAcceptErrorMessage,
  getOrganizationIncomingShareAcceptSuccessMessage,
  getOrganizationIncomingShareDeclineConfirmMessage,
  getOrganizationIncomingShareDeclineErrorMessage,
  getOrganizationIncomingShareDeclineSuccessMessage,
  getOrganizationShareCreateErrorMessage,
  getOrganizationShareCreateSuccessMessage,
  getOrganizationShareDeleteErrorMessage,
  getOrganizationShareDeleteSuccessMessage,
  getOrganizationShareInvalidResourceMessage,
  getOrganizationShareResourceIdRequiredMessage,
  getOrganizationShareTargetOrgDifferentMessage,
  getOrganizationShareTargetOrgRequiredMessage,
  getOrganizationSettingsLoadErrorMessage,
  ORGANIZATION_SHARING_UNCONFIRMED_CHANGE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import { canManageOrg } from '@/utils/orgUtils';

interface OrganizationSharingPanelProps {
  currentUser?: string;
}

export type ResourceOption = {
  id: string;
  type: string;
  name: string;
};

export function useOrganizationSharingPanelState(props: OrganizationSharingPanelProps) {
  const [loading, setLoading] = createSignal(true);
  const [loadError, setLoadError] = createSignal<string | null>(null);
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

  const activeOrgId = () => normalizeOrgScope(getOrgID());
  const [selectedOrgID, setSelectedOrgID] = createSignal(activeOrgId());
  const [selectedUser, setSelectedUser] = createSignal(props.currentUser);
  let generation = 0;
  let disposed = false;
  let readController: AbortController | undefined;
  let mutationController: AbortController | undefined;
  const surfaceEnabled = () =>
    isMultiTenantEnabled() && !presentationPolicyHidesOrganizationSurfaces();
  const selectionIsCurrent = () =>
    !disposed &&
    surfaceEnabled() &&
    selectedOrgID() === activeOrgId() &&
    selectedUser() === props.currentUser;
  const hasCurrentSharing = () =>
    selectionIsCurrent() && !loading() && !loadError() && org()?.id === activeOrgId();
  const canManageCurrentOrg = createMemo(
    () =>
      hasCurrentSharing() &&
      !presentationPolicyIsReadOnly() &&
      canManageOrg(org(), props.currentUser),
  );
  const canChangeSharing = () => hasCurrentSharing() && canManageCurrentOrg() && !saving();
  const { resources } = useResources();

  const targetOrgOptions = createMemo(() =>
    orgs()
      .filter((candidate) => candidate.id !== activeOrgId())
      .sort((left, right) =>
        (left.displayName || left.id).localeCompare(right.displayName || right.id),
      ),
  );

  const unifiedResourceOptions = createMemo<ResourceOption[]>(() =>
    resources()
      .filter((resource) => resource.id && resource.type)
      .map((resource) => ({
        id: resource.id,
        type: resource.type,
        name: (getPreferredResourceDisplayName(resource) || resource.id).trim(),
      }))
      .sort((left, right) => left.name.localeCompare(right.name)),
  );

  const normalizedResourceType = createMemo(() =>
    normalizeCanonicalResourceTypeInput(resourceType()),
  );
  const normalizedResourceId = createMemo(() => resourceId().trim());
  const hasTargetOrg = createMemo(() => targetOrgId().trim() !== '');
  // Match the complete opaque resource ID rather than splitting on a delimiter
  // that may occur inside it. A removed quick pick is not a valid manual draft.
  const selectedResource = createMemo(() =>
    unifiedResourceOptions().find(
      (resource) => `${resource.type}::${resource.id}` === selectedQuickPick(),
    ),
  );
  const manualEntryValid = createMemo(
    () => isCanonicalResourceType(normalizedResourceType()) && normalizedResourceId() !== '',
  );
  const canCreateShare = createMemo(
    () =>
      canChangeSharing() &&
      hasTargetOrg() &&
      (selectedQuickPick() ? !!selectedResource() : manualEntryValid()),
  );

  const orgNameById = createMemo(() => {
    const map = new Map<string, string>();
    for (const organization of orgs()) {
      map.set(organization.id, organization.displayName || organization.id);
    }
    return map;
  });

  const getShareLabel = (share: {
    resourceId: string;
    resourceName?: string;
    resourceType: string;
  }): string => {
    const resourceName = share.resourceName?.trim();
    if (resourceName) return resourceName;
    if (share.resourceId.trim() !== '') return share.resourceId;
    return `${share.resourceType}:${share.resourceId}`;
  };

  createEffect(() => {
    if (unifiedResourceOptions().length === 0 && !selectedQuickPick()) setManualEntryExpanded(true);
  });

  const clearSharing = () => {
    setOrg(null);
    setOrgs([]);
    setOutgoingShares([]);
    setIncomingShares([]);
  };
  const clearDraft = () => {
    setTargetOrgId('');
    setResourceType('');
    setResourceId('');
    setResourceName('');
    setAccessRole('viewer');
    setSelectedQuickPick('');
    setManualEntryExpanded(false);
    setTargetOrgError('');
    setResourceTypeError('');
    setResourceIdError('');
  };
  const invalidateRequests = () => {
    generation += 1;
    readController?.abort();
    readController = undefined;
    mutationController?.abort();
    mutationController = undefined;
    setSaving(false);
  };

  const loadSharingData = async () => {
    if (disposed) return;
    invalidateRequests();
    const orgId = activeOrgId();
    const user = props.currentUser;
    const request = generation;
    const isCurrent = () => request === generation && selectionIsCurrent();
    batch(() => {
      if (selectedOrgID() !== orgId || selectedUser() !== user) clearDraft();
      setSelectedOrgID(orgId);
      setSelectedUser(user);
      clearSharing();
      setLoadError(null);
      setLoading(surfaceEnabled());
    });
    if (!surfaceEnabled()) return;

    const controller = new AbortController();
    readController = controller;
    const options: OrganizationAccessRequestOptions = {
      expectedOrgID: orgId,
      signal: controller.signal,
      retry: false,
    };
    try {
      // Publish one accepted snapshot, never a new org with old share rows (or
      // a partial read presented as an empty share list).
      const [orgData, memberData, allOrgs, outgoing, incoming] = await Promise.all([
        OrgsAPI.get(orgId, options),
        OrgsAPI.listMembers(orgId, options),
        OrgsAPI.list(options),
        OrgsAPI.listShares(orgId, options),
        OrgsAPI.listIncomingShares(orgId, options),
      ]);
      if (!isCurrent()) return;
      if (
        !orgData ||
        orgData.id !== orgId ||
        !Array.isArray(memberData) ||
        !Array.isArray(allOrgs) ||
        !Array.isArray(outgoing) ||
        !Array.isArray(incoming) ||
        incoming.some((share) => share.targetOrgId !== orgId)
      ) {
        throw new Error('Organization sharing returned an invalid selection');
      }
      batch(() => {
        setOrg({ ...orgData, members: memberData });
        setOrgs(allOrgs);
        setOutgoingShares(outgoing);
        setIncomingShares(incoming);
        const firstTarget =
          targetOrgId().trim() || allOrgs.find((candidate) => candidate.id !== orgId)?.id || '';
        setTargetOrgId(firstTarget);
        setTargetOrgError(firstTarget === '' ? getOrganizationShareTargetOrgRequiredMessage() : '');
      });
    } catch (error) {
      if (!isCurrent()) return;
      logger.error('Failed to load organization sharing data', error);
      const message = error instanceof Error ? error.message : '';
      setLoadError(getOrganizationSettingsLoadErrorMessage(message, 'sharing'));
    } finally {
      if (!disposed && request === generation) {
        controller.abort(); // Cancel sibling reads after a partial failure.
        readController = undefined;
        setLoading(false);
        if (!selectionIsCurrent()) void loadSharingData();
      }
    }
  };

  const changeSharing = async (
    change: (options: OrganizationAccessRequestOptions) => Promise<unknown>,
    successMessage: string,
    errorMessage: (message?: string) => string,
  ) => {
    if (!canChangeSharing()) return;
    const request = generation;
    const isCurrent = () => request === generation && selectionIsCurrent();
    const controller = new AbortController();
    mutationController = controller;
    setSaving(true);
    try {
      await change({ expectedOrgID: selectedOrgID(), signal: controller.signal, retry: false });
      if (!isCurrent()) return;
      notificationStore.success(successMessage);
      mutationController = undefined;
      setSaving(false);
      eventBus.emit('organizations_changed');
    } catch (error) {
      if (!isCurrent()) return;
      logger.error('Failed to change organization sharing', error);
      notificationStore.error(errorMessage(error instanceof Error ? error.message : undefined));
      // A failed response cannot tell us whether a change was applied. Read
      // before allowing another change; never replay a write automatically.
      batch(() => {
        clearSharing();
        setLoadError(ORGANIZATION_SHARING_UNCONFIRMED_CHANGE_MESSAGE);
      });
    } finally {
      if (mutationController === controller) {
        mutationController = undefined;
        setSaving(false);
        if (!disposed && !selectionIsCurrent()) void loadSharingData();
      }
    }
  };

  const applyResourceQuickPick = (value: string) => {
    setSelectedQuickPick(value);
    setResourceTypeError('');
    setResourceIdError('');
    if (!value) return;

    const match = selectedResource();
    if (!match) return;
    setResourceType(normalizeCanonicalResourceTypeInput(match.type));
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
    setTargetOrgError(value.trim() === '' ? getOrganizationShareTargetOrgRequiredMessage() : '');
  };

  const updateResourceType = (value: string) => {
    setSelectedQuickPick('');
    setResourceType(value);
    const normalized = normalizeCanonicalResourceTypeInput(value);
    setResourceTypeError(
      normalized === '' || isCanonicalResourceType(normalized) ? '' : INVALID_RESOURCE_TYPE_ERROR,
    );
  };

  const updateResourceId = (value: string) => {
    setSelectedQuickPick('');
    setResourceId(value);
    setResourceIdError(value.trim() === '' ? getOrganizationShareResourceIdRequiredMessage() : '');
  };

  const updateResourceName = (value: string) => {
    setSelectedQuickPick('');
    setResourceName(value);
  };

  const createShare = async () => {
    const currentOrg = org();
    if (!currentOrg || !canChangeSharing()) return;

    const nextTargetOrgId = targetOrgId().trim();
    const quickPick = selectedResource();
    const hasQuickPick = selectedQuickPick() !== '';
    if (hasQuickPick && !quickPick) {
      notificationStore.error(getOrganizationShareInvalidResourceMessage());
      return;
    }
    const nextResourceType = quickPick?.type ?? normalizedResourceType();
    const nextResourceId = quickPick?.id ?? normalizedResourceId();
    const nextResourceName = quickPick?.name ?? resourceName().trim();
    const hasValidManualType = isCanonicalResourceType(nextResourceType);
    const hasValidManualResourceId = nextResourceId !== '';

    setTargetOrgError(nextTargetOrgId === '' ? getOrganizationShareTargetOrgRequiredMessage() : '');
    if (hasQuickPick) {
      setResourceTypeError('');
      setResourceIdError('');
    } else {
      setResourceTypeError(hasValidManualType ? '' : INVALID_RESOURCE_TYPE_ERROR);
      setResourceIdError(
        hasValidManualResourceId ? '' : getOrganizationShareResourceIdRequiredMessage(),
      );
    }

    if (!nextTargetOrgId) {
      notificationStore.error(getOrganizationShareTargetOrgRequiredMessage());
      return;
    }
    if (nextTargetOrgId === currentOrg.id) {
      notificationStore.error(getOrganizationShareTargetOrgDifferentMessage());
      return;
    }
    if (!hasQuickPick && (!hasValidManualType || !hasValidManualResourceId)) {
      notificationStore.error(getOrganizationShareInvalidResourceMessage());
      return;
    }

    await changeSharing(
      (options) =>
        OrgsAPI.createShare(
          currentOrg.id,
          {
            targetOrgId: nextTargetOrgId,
            resourceType: nextResourceType,
            resourceId: nextResourceId,
            resourceName: nextResourceName,
            accessRole: accessRole(),
          },
          options,
        ),
      getOrganizationShareCreateSuccessMessage(),
      getOrganizationShareCreateErrorMessage,
    );
  };

  // Grid rows are Solid store proxies; unwrap without admitting an object from
  // an older snapshot just because it happens to have the same ID.
  const isCurrentOutgoing = (share: OrganizationShare) => outgoingShares().includes(unwrap(share));
  const isCurrentIncoming = (share: IncomingOrganizationShare) =>
    incomingShares().includes(unwrap(share));

  const deleteShare = async (share: OrganizationShare) => {
    const currentOrg = org();
    if (!currentOrg || !canChangeSharing() || !isCurrentOutgoing(share)) return;
    if (!confirm(`Remove share for ${share.resourceType}:${share.resourceId}?`)) return;
    if (org() !== currentOrg || !isCurrentOutgoing(share) || !canChangeSharing()) return;
    await changeSharing(
      (options) => OrgsAPI.deleteShare(currentOrg.id, share.id, options),
      getOrganizationShareDeleteSuccessMessage(),
      getOrganizationShareDeleteErrorMessage,
    );
  };

  const acceptIncomingShare = async (share: IncomingOrganizationShare) => {
    const currentOrg = org();
    if (
      !currentOrg ||
      !canChangeSharing() ||
      !isCurrentIncoming(share) ||
      share.targetOrgId !== currentOrg.id ||
      share.status !== 'pending'
    )
      return;
    await changeSharing(
      (options) => OrgsAPI.acceptIncomingShare(currentOrg.id, share.id, options),
      getOrganizationIncomingShareAcceptSuccessMessage(getShareLabel(share)),
      getOrganizationIncomingShareAcceptErrorMessage,
    );
  };

  const declineIncomingShare = async (share: IncomingOrganizationShare) => {
    const currentOrg = org();
    if (
      !currentOrg ||
      !canChangeSharing() ||
      !isCurrentIncoming(share) ||
      share.targetOrgId !== currentOrg.id
    )
      return;
    const shareLabel = getShareLabel(share);
    if (!confirm(getOrganizationIncomingShareDeclineConfirmMessage(shareLabel, share.status)))
      return;
    if (org() !== currentOrg || !isCurrentIncoming(share) || !canChangeSharing()) return;
    await changeSharing(
      (options) => OrgsAPI.declineIncomingShare(currentOrg.id, share.id, options),
      getOrganizationIncomingShareDeclineSuccessMessage(shareLabel, share.status),
      getOrganizationIncomingShareDeclineErrorMessage,
    );
  };

  const unsubscribeOrg = eventBus.on('org_switched', () => void loadSharingData());
  const unsubscribeChanged = eventBus.on('organizations_changed', () => void loadSharingData());
  createEffect(
    on([surfaceEnabled, () => props.currentUser, presentationPolicyIsReadOnly], () => {
      void loadSharingData();
    }),
  );
  onCleanup(() => {
    disposed = true;
    invalidateRequests();
    unsubscribeOrg();
    unsubscribeChanged();
  });

  return {
    accessRole,
    acceptIncomingShare,
    applyResourceQuickPick,
    canCreateShare,
    canManageCurrentOrg,
    createShare,
    declineIncomingShare,
    deleteShare,
    incomingShares,
    loading,
    loadError,
    loadSharingData,
    org,
    readOnly: presentationPolicyIsReadOnly,
    selectedOrgID,
    manualEntryExpanded,
    orgNameById,
    outgoingShares,
    resourceId,
    resourceIdError,
    resourceName,
    resourceType,
    resourceTypeError,
    saving,
    selectedQuickPick,
    quickPickUnavailable: () => !!selectedQuickPick() && !selectedResource(),
    setAccessRole,
    targetOrgError,
    targetOrgId,
    targetOrgOptions,
    toggleManualEntry,
    unifiedResourceOptions,
    updateResourceId,
    updateResourceName,
    updateResourceType,
    updateTargetOrg,
  };
}
