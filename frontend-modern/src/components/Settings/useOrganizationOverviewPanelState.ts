import { batch, createEffect, createMemo, createSignal, on, onCleanup } from 'solid-js';
import { OrgsAPI, type Organization, type OrganizationMember } from '@/api/orgs';
import { eventBus } from '@/stores/events';
import { isMultiTenantEnabled } from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import {
  presentationPolicyHidesOrganizationSurfaces,
  presentationPolicyIsReadOnly,
} from '@/stores/sessionPresentationPolicy';
import { getOrgID } from '@/utils/apiClient';
import { logger } from '@/utils/logger';
import { normalizeOrgScope } from '@/utils/orgScope';
import {
  ORGANIZATION_OVERVIEW_UNCONFIRMED_CHANGE_MESSAGE,
  getOrganizationDisplayNameRequiredMessage,
  getOrganizationDisplayNameUpdateErrorMessage,
  getOrganizationDisplayNameUpdatedMessage,
  getOrganizationSettingsLoadErrorMessage,
} from '@/utils/organizationSettingsPresentation';
import { canManageOrg } from '@/utils/orgUtils';

interface OrganizationOverviewPanelProps {
  currentUser?: string;
}

export function useOrganizationOverviewPanelState(props: OrganizationOverviewPanelProps) {
  const [loading, setLoading] = createSignal(true);
  const [loadError, setLoadError] = createSignal<string | null>(null);
  const [saving, setSaving] = createSignal(false);
  const [org, setOrg] = createSignal<Organization | null>(null);
  const [members, setMembers] = createSignal<OrganizationMember[]>([]);
  const [displayNameDraft, setDisplayNameDraft] = createSignal('');

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
  const canManageCurrentOrg = createMemo(
    () =>
      selectionIsCurrent() &&
      !loading() &&
      !loadError() &&
      !presentationPolicyIsReadOnly() &&
      org()?.id === activeOrgId() &&
      canManageOrg(org(), props.currentUser),
  );
  const clearOrganization = () => {
    setOrg(null);
    setMembers([]);
    setDisplayNameDraft('');
  };
  const invalidateRequests = () => {
    generation += 1;
    readController?.abort();
    readController = undefined;
    mutationController?.abort();
    mutationController = undefined;
    setSaving(false);
  };

  const loadOrganization = async () => {
    if (disposed) return;
    invalidateRequests();
    const request = generation;
    const orgId = activeOrgId();
    const isCurrent = () => request === generation && selectionIsCurrent();
    batch(() => {
      setSelectedOrgID(orgId);
      setSelectedUser(props.currentUser);
      clearOrganization();
      setLoadError(null);
      setLoading(surfaceEnabled());
    });
    if (!surfaceEnabled()) return;

    const controller = new AbortController();
    readController = controller;
    const options = { expectedOrgID: orgId, signal: controller.signal, retry: false as const };
    try {
      const [orgData, memberData] = await Promise.all([
        OrgsAPI.get(orgId, options),
        OrgsAPI.listMembers(orgId, options),
      ]);
      if (!isCurrent()) return;
      if (!orgData || orgData.id !== orgId || !Array.isArray(memberData)) {
        throw new Error('Organization overview returned invalid data for the selection');
      }
      batch(() => {
        setOrg(orgData);
        setMembers(memberData);
        setDisplayNameDraft(orgData.displayName || '');
      });
    } catch (error) {
      if (!isCurrent()) return;
      logger.error('Failed to load organization overview', error);
      setLoadError(getOrganizationSettingsLoadErrorMessage('', 'overview'));
    } finally {
      if (!disposed && request === generation) {
        controller.abort();
        readController = undefined;
        setLoading(false);
        if (!selectionIsCurrent()) void loadOrganization();
      }
    }
  };

  const saveDisplayName = async () => {
    if (!canManageCurrentOrg() || saving()) return;
    const currentOrg = org()!;
    const nextName = displayNameDraft().trim();
    if (!nextName) {
      notificationStore.error(getOrganizationDisplayNameRequiredMessage());
      return;
    }
    if (nextName === currentOrg.displayName) return;

    const request = generation;
    const controller = new AbortController();
    mutationController = controller;
    const isCurrent = () => request === generation && selectionIsCurrent();
    setSaving(true);
    try {
      const updated = await OrgsAPI.update(
        currentOrg.id,
        { displayName: nextName },
        {
          expectedOrgID: selectedOrgID(),
          signal: controller.signal,
          retry: false,
        },
      );
      if (!isCurrent()) return;
      if (!updated || updated.id !== currentOrg.id) {
        throw new Error('Organization rename returned a different selection');
      }
      notificationStore.success(getOrganizationDisplayNameUpdatedMessage());
      mutationController = undefined;
      setSaving(false);
      // The same single subscription refreshes the saved snapshot, never a write.
      eventBus.emit('organizations_changed');
    } catch (error) {
      if (!isCurrent()) return;
      logger.error('Failed to update organization display name', error);
      notificationStore.error(getOrganizationDisplayNameUpdateErrorMessage());
      batch(() => {
        clearOrganization();
        setLoadError(ORGANIZATION_OVERVIEW_UNCONFIRMED_CHANGE_MESSAGE);
      });
    } finally {
      if (mutationController === controller) {
        mutationController = undefined;
        setSaving(false);
        if (!disposed && !selectionIsCurrent()) void loadOrganization();
      }
    }
  };

  const reloadOrganization = () => {
    if (!loading() && !saving()) void loadOrganization();
  };
  const unsubscribeOrg = eventBus.on('org_switched', () => void loadOrganization());
  const unsubscribeChanged = eventBus.on('organizations_changed', () => void loadOrganization());
  createEffect(
    on([surfaceEnabled, () => props.currentUser, presentationPolicyIsReadOnly], () => {
      void loadOrganization();
    }),
  );
  onCleanup(() => {
    disposed = true;
    invalidateRequests();
    unsubscribeOrg();
    unsubscribeChanged();
  });

  return {
    canManageCurrentOrg,
    displayNameDraft,
    loading,
    loadError,
    reloadOrganization,
    selectedOrgID,
    members,
    org,
    saveDisplayName,
    saving,
    setDisplayNameDraft,
  };
}
