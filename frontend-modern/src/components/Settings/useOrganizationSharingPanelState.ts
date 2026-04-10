import { createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import {
  OrgsAPI,
  type IncomingOrganizationShare,
  type Organization,
  type OrganizationShare,
} from '@/api/orgs';
import { useResources } from '@/hooks/useResources';
import { eventBus } from '@/stores/events';
import { isMultiTenantEnabled } from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import { presentationPolicyHidesOrganizationSurfaces } from '@/stores/sessionPresentationPolicy';
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
  getOrganizationShareCreateErrorMessage,
  getOrganizationShareCreateSuccessMessage,
  getOrganizationShareDeleteErrorMessage,
  getOrganizationShareDeleteSuccessMessage,
  getOrganizationShareInvalidResourceMessage,
  getOrganizationShareResourceIdRequiredMessage,
  getOrganizationShareTargetOrgDifferentMessage,
  getOrganizationShareTargetOrgRequiredMessage,
  getOrganizationSettingsLoadErrorMessage,
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
  const hasQuickPickSelection = createMemo(() => selectedQuickPick().trim() !== '');
  const manualEntryValid = createMemo(
    () => isCanonicalResourceType(normalizedResourceType()) && normalizedResourceId() !== '',
  );
  const canCreateShare = createMemo(
    () => !saving() && hasTargetOrg() && (hasQuickPickSelection() || manualEntryValid()),
  );
  const canManageCurrentOrg = createMemo(() => canManageOrg(org(), props.currentUser));

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
        targetOrgId().trim() || allOrgs.find((candidate) => candidate.id !== orgId)?.id || '';
      setTargetOrgId(firstTarget);
      setTargetOrgError(firstTarget === '' ? getOrganizationShareTargetOrgRequiredMessage() : '');
    } catch (error) {
      logger.error('Failed to load organization sharing data', error);
      const message = error instanceof Error ? error.message : '';
      notificationStore.error(getOrganizationSettingsLoadErrorMessage(message, 'sharing'));
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
    if (!currentOrg) return;

    const nextTargetOrgId = targetOrgId().trim();
    const nextResourceType = normalizedResourceType();
    const nextResourceId = normalizedResourceId();
    const nextResourceName = resourceName().trim();
    const hasQuickPick = hasQuickPickSelection();
    const hasValidManualType = isCanonicalResourceType(nextResourceType);
    const hasValidManualResourceId = nextResourceId !== '';

    setTargetOrgError(
      nextTargetOrgId === '' ? getOrganizationShareTargetOrgRequiredMessage() : '',
    );
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

    setSaving(true);
    try {
      await OrgsAPI.createShare(currentOrg.id, {
        targetOrgId: nextTargetOrgId,
        resourceType: nextResourceType,
        resourceId: nextResourceId,
        resourceName: nextResourceName,
        accessRole: accessRole(),
      });
      notificationStore.success(getOrganizationShareCreateSuccessMessage());
      await loadShares(currentOrg.id);
    } catch (error) {
      logger.error('Failed to create organization share', error);
      notificationStore.error(
        getOrganizationShareCreateErrorMessage(error instanceof Error ? error.message : undefined),
      );
    } finally {
      setSaving(false);
    }
  };

  const deleteShare = async (share: OrganizationShare) => {
    const currentOrg = org();
    if (!currentOrg) return;
    if (!confirm(`Remove share for ${share.resourceType}:${share.resourceId}?`)) return;

    setSaving(true);
    try {
      await OrgsAPI.deleteShare(currentOrg.id, share.id);
      notificationStore.success(getOrganizationShareDeleteSuccessMessage());
      await loadShares(currentOrg.id);
    } catch (error) {
      logger.error('Failed to delete organization share', error);
      notificationStore.error(
        getOrganizationShareDeleteErrorMessage(error instanceof Error ? error.message : undefined),
      );
    } finally {
      setSaving(false);
    }
  };

  onMount(() => {
    if (!isMultiTenantEnabled() || presentationPolicyHidesOrganizationSurfaces()) {
      setLoading(false);
      return;
    }
    void loadSharingData();

    const unsubscribe = eventBus.on('org_switched', () => {
      void loadSharingData();
    });
    onCleanup(unsubscribe);
  });

  return {
    accessRole,
    applyResourceQuickPick,
    canCreateShare,
    canManageCurrentOrg,
    createShare,
    deleteShare,
    incomingShares,
    loading,
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
