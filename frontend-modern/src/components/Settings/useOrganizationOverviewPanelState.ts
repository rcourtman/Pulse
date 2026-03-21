import { createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { OrgsAPI, type Organization, type OrganizationMember } from '@/api/orgs';
import { eventBus } from '@/stores/events';
import { isMultiTenantEnabled } from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import { getOrgID } from '@/utils/apiClient';
import { logger } from '@/utils/logger';
import { normalizeOrgScope } from '@/utils/orgScope';
import {
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
  const [saving, setSaving] = createSignal(false);
  const [org, setOrg] = createSignal<Organization | null>(null);
  const [members, setMembers] = createSignal<OrganizationMember[]>([]);
  const [displayNameDraft, setDisplayNameDraft] = createSignal('');

  const activeOrgId = () => normalizeOrgScope(getOrgID());
  const canManageCurrentOrg = createMemo(() => canManageOrg(org(), props.currentUser));

  const loadOrganization = async () => {
    setLoading(true);
    try {
      const orgId = activeOrgId();
      const [orgData, memberData] = await Promise.all([
        OrgsAPI.get(orgId),
        OrgsAPI.listMembers(orgId),
      ]);
      setOrg(orgData);
      setMembers(memberData);
      setDisplayNameDraft(orgData.displayName || '');
    } catch (error) {
      logger.error('Failed to load organization overview', error);
      const message = error instanceof Error ? error.message : '';
      notificationStore.error(getOrganizationSettingsLoadErrorMessage(message, 'overview'));
    } finally {
      setLoading(false);
    }
  };

  const saveDisplayName = async () => {
    const currentOrg = org();
    if (!currentOrg) return;
    const nextName = displayNameDraft().trim();
    if (!nextName) {
      notificationStore.error(getOrganizationDisplayNameRequiredMessage());
      return;
    }
    if (nextName === currentOrg.displayName) return;

    setSaving(true);
    try {
      const updated = await OrgsAPI.update(currentOrg.id, { displayName: nextName });
      setOrg(updated);
      setDisplayNameDraft(updated.displayName || '');
      notificationStore.success(getOrganizationDisplayNameUpdatedMessage());
    } catch (error) {
      logger.error('Failed to update organization display name', error);
      notificationStore.error(
        getOrganizationDisplayNameUpdateErrorMessage(
          error instanceof Error ? error.message : undefined,
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  onMount(() => {
    if (!isMultiTenantEnabled()) return;
    void loadOrganization();

    const unsubscribe = eventBus.on('org_switched', () => {
      void loadOrganization();
    });
    onCleanup(unsubscribe);
  });

  return {
    canManageCurrentOrg,
    displayNameDraft,
    loading,
    members,
    org,
    saveDisplayName,
    saving,
    setDisplayNameDraft,
  };
}
