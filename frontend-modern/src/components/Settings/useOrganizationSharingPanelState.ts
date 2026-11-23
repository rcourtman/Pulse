import { batch, createEffect, createMemo, createSignal, on, onCleanup } from 'solid-js';
import { unwrap } from 'solid-js/store';
import {
  OrgsAPI,
  type IncomingOrganizationShare,
  type Organization,
  type OrganizationAccessRequestOptions,
  type OrganizationShare,
} from '@/api/orgs';
import { apiErrorStatus } from '@/api/responseUtils';
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
  getOrganizationOutgoingShareRemoveConfirmMessage,
  getOrganizationShareCreateErrorMessage,
  getOrganizationShareCreateSuccessMessage,
  getOrganizationShareDeleteErrorMessage,
  getOrganizationShareDeleteSuccessMessage,
  getOrganizationShareInvalidResourceMessage,
  getOrganizationShareResourceIdRequiredMessage,
  getOrganizationShareResourceLabel,
  getOrganizationShareTargetOrgDifferentMessage,
  getOrganizationShareTargetOrgRequiredMessage,
  getOrganizationSettingsLoadErrorMessage,
  ORGANIZATION_SHARING_UNCONFIRMED_CHANGE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import { canManageOrg, readOrganizationAggregate } from '@/utils/orgUtils';

interface OrganizationSharingPanelProps {
  currentUser?: string;
}

export type ResourceOption = {
  id: string;
  type: string;
  name: string;
};

export type ShareConfirmation =
  | {
      kind: 'remove-outgoing';
      organization: Organization;
      share: OrganizationShare;
      title: string;
      message: string;
      confirmLabel: string;
      generation: number;
    }
  | {
      kind: 'decline-incoming';
      organization: Organization;
      share: IncomingOrganizationShare;
      title: string;
      message: string;
      confirmLabel: string;
      generation: number;
    };

export function useOrganizationSharingPanelState(props: OrganizationSharingPanelProps) {
  const [loading, setLoading] = createSignal(true);
  const [loadError, setLoadError] = createSignal<string | null>(null);
  const [mutationError, setMutationError] = createSignal<string | null>(null);
  const [saving, setSaving] = createSignal(false);
  const [shareConfirmation, setShareConfirmation] = createSignal<ShareConfirmation | null>(null);
  const [org, setOrg] = createSignal<Organization | null>(null);
  const [orgs, setOrgs] = createSignal<Organization[]>([]);
  const [outgoingShares, setOutgoingShares] = createSignal<OrganizationShare[]>([]);
  const [incomingShares, setIncomingShares] = createSignal<IncomingOrganizationShare[]>([]);
  const [unavailableLists, setUnavailableLists] = createSignal<{
    directory: boolean;
    outgoing: boolean;
    incoming: boolean;
  }>();
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
  // Losing a destination is a draft decision, not a transient read error.
  // Keep it invalid across refreshes until a current destination is chosen.
  let targetOrgNeedsReselection = false;
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
  const sharingChangesPaused = () => unavailableLists() !== undefined;
  const canChangeSharing = () =>
    hasCurrentSharing() &&
    canManageCurrentOrg() &&
    !sharingChangesPaused() &&
    !saving() &&
    !shareConfirmation();
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
  const hasTargetOrg = createMemo(() =>
    targetOrgOptions().some((candidate) => candidate.id === targetOrgId().trim()),
  );
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

  createEffect(() => {
    if (unifiedResourceOptions().length === 0 && !selectedQuickPick()) setManualEntryExpanded(true);
  });

  const clearSharing = () => {
    setOrg(null);
    setOrgs([]);
    setOutgoingShares([]);
    setIncomingShares([]);
    setUnavailableLists(undefined);
    setMutationError(null);
  };
  const clearDraft = () => {
    targetOrgNeedsReselection = false;
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
    setShareConfirmation(null);
    readController?.abort();
    readController = undefined;
    mutationController?.abort();
    mutationController = undefined;
    setSaving(false);
  };

  const loadSharingData = async () => {
    if (disposed) return;
    // A same-reader refresh cannot undo a dispatched write. Retain its outcome
    // and disabled controls; success reloads, uncertainty requires recovery.
    // Actual reader or authority withdrawal must still invalidate immediately.
    if (mutationController && selectionIsCurrent() && !presentationPolicyIsReadOnly()) return;
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
      // Publish only current, verified data. An unread list can leave the
      // selected scope readable, but not authorize a write or imply zero rows.
      const [orgData, memberData, allOrgs, outgoing, incoming] = await Promise.all([
        OrgsAPI.get(orgId, options),
        OrgsAPI.listMembers(orgId, options),
        readOrganizationAggregate(OrgsAPI.list(options)),
        // The outgoing endpoint loads the selected organization independently.
        readOrganizationAggregate(OrgsAPI.listShares(orgId, options), 'org_load_failed'),
        readOrganizationAggregate(OrgsAPI.listIncomingShares(orgId, options)),
      ]);
      if (!isCurrent()) return;
      if (
        !orgData ||
        orgData.id !== orgId ||
        !Array.isArray(memberData) ||
        (allOrgs !== undefined && !Array.isArray(allOrgs)) ||
        (outgoing !== undefined && !Array.isArray(outgoing)) ||
        (incoming !== undefined &&
          (!Array.isArray(incoming) || incoming.some((share) => share.targetOrgId !== orgId)))
      ) {
        throw new Error('Organization sharing returned an invalid selection');
      }
      batch(() => {
        setOrg({ ...orgData, members: memberData });
        setOrgs(allOrgs ?? []);
        setOutgoingShares(outgoing ?? []);
        setIncomingShares(incoming ?? []);
        if (!allOrgs || !outgoing || !incoming) {
          setUnavailableLists({
            directory: !allOrgs,
            outgoing: !outgoing,
            incoming: !incoming,
          });
        }
        // An unread directory neither verifies nor withdraws an explicit draft
        // recipient. Recheck it when a complete directory is read.
        if (!allOrgs) return;
        const draftTarget = targetOrgId().trim();
        // If a previously chosen destination disappeared, require a new choice
        // rather than retaining an invisible value or choosing a different org.
        if (
          draftTarget &&
          !allOrgs.some((candidate) => candidate.id === draftTarget && candidate.id !== orgId)
        ) {
          targetOrgNeedsReselection = true;
        }
        // Directory order (even a sole peer) is not an operator's choice of
        // recipient. Preserve only a current reader's explicit draft.
        setTargetOrgId(targetOrgNeedsReselection ? '' : draftTarget);
        setTargetOrgError(
          targetOrgNeedsReselection ? getOrganizationShareTargetOrgRequiredMessage() : '',
        );
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

  const restartSharingRead = () => {
    // Only replace the current reader's pending GETs, never a sharing change.
    if (!loading() || !readController || !selectionIsCurrent() || saving()) return false;
    void loadSharingData();
    return true;
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
    setMutationError(null);
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
      const status = apiErrorStatus(error);
      const resolvedError = errorMessage(error instanceof Error ? error.message : undefined);
      notificationStore.error(resolvedError);
      if (status === 400 || status === 422) {
        setMutationError(resolvedError);
        return;
      }
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
    targetOrgNeedsReselection = !targetOrgOptions().some((candidate) => candidate.id === value);
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

    if (!nextTargetOrgId || !hasTargetOrg()) {
      setTargetOrgError(getOrganizationShareTargetOrgRequiredMessage());
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

  const deleteShare = (share: OrganizationShare) => {
    const currentOrg = org();
    if (!currentOrg || !canChangeSharing() || !isCurrentOutgoing(share)) return;
    setShareConfirmation({
      kind: 'remove-outgoing',
      organization: currentOrg,
      share: unwrap(share),
      title: 'Remove outgoing share?',
      message: getOrganizationOutgoingShareRemoveConfirmMessage(share),
      confirmLabel: 'Remove share',
      generation,
    });
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
      getOrganizationIncomingShareAcceptSuccessMessage(getOrganizationShareResourceLabel(share)),
      getOrganizationIncomingShareAcceptErrorMessage,
    );
  };

  const declineIncomingShare = (share: IncomingOrganizationShare) => {
    const currentOrg = org();
    if (
      !currentOrg ||
      !canChangeSharing() ||
      !isCurrentIncoming(share) ||
      share.targetOrgId !== currentOrg.id
    )
      return;
    const shareLabel = getOrganizationShareResourceLabel(share);
    const isPending = share.status === 'pending';
    setShareConfirmation({
      kind: 'decline-incoming',
      organization: currentOrg,
      share: unwrap(share),
      title: isPending ? 'Decline incoming share?' : 'Remove incoming share?',
      message: getOrganizationIncomingShareDeclineConfirmMessage(shareLabel, share.status),
      confirmLabel: isPending ? 'Decline share' : 'Remove share',
      generation,
    });
  };

  const cancelShareConfirmation = (confirmation: ShareConfirmation) => {
    if (shareConfirmation() === confirmation) setShareConfirmation(null);
  };

  const confirmShareAction = async (confirmation: ShareConfirmation) => {
    if (shareConfirmation() !== confirmation) return;
    setShareConfirmation(null);
    if (
      confirmation.generation !== generation ||
      !canChangeSharing() ||
      org() !== confirmation.organization
    )
      return;
    if (confirmation.kind === 'remove-outgoing') {
      if (!isCurrentOutgoing(confirmation.share)) return;
      await changeSharing(
        (options) =>
          OrgsAPI.deleteShare(confirmation.organization.id, confirmation.share.id, options),
        getOrganizationShareDeleteSuccessMessage(),
        getOrganizationShareDeleteErrorMessage,
      );
      return;
    }
    if (
      !isCurrentIncoming(confirmation.share) ||
      confirmation.share.targetOrgId !== confirmation.organization.id
    )
      return;
    const shareLabel = getOrganizationShareResourceLabel(confirmation.share);
    await changeSharing(
      (options) =>
        OrgsAPI.declineIncomingShare(confirmation.organization.id, confirmation.share.id, options),
      getOrganizationIncomingShareDeclineSuccessMessage(shareLabel, confirmation.share.status),
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
    cancelShareConfirmation,
    canCreateShare,
    canManageCurrentOrg,
    confirmShareAction,
    createShare,
    declineIncomingShare,
    deleteShare,
    incomingShares,
    loading,
    loadError,
    loadSharingData,
    mutationError,
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
    restartSharingRead,
    saving,
    selectedQuickPick,
    selectedResource,
    shareConfirmation,
    quickPickUnavailable: () => !!selectedQuickPick() && !selectedResource(),
    setAccessRole,
    targetOrgError,
    targetOrgId,
    targetOrgOptions,
    toggleManualEntry,
    unifiedResourceOptions,
    unavailableLists,
    sharingChangesPaused,
    updateResourceId,
    updateResourceName,
    updateResourceType,
    updateTargetOrg,
  };
}
