import { createEffect, createMemo, createSignal, onMount } from 'solid-js';
import { useWebSocket } from '@/contexts/appRuntime';
import { AIAPI } from '@/api/ai';
import {
  AgentProfilesAPI,
  MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE,
  type AgentProfile,
  type AgentProfileAssignment,
  type ProfileSuggestion,
} from '@/api/agentProfiles';
import { useResources } from '@/hooks/useResources';
import { notificationStore } from '@/stores/notifications';
import {
  hasFeature as hasEntitlement,
  licenseLoaded,
  licenseLoading,
} from '@/stores/license';
import {
  commercialPosture,
  getUpgradeActionDestination,
  loadCommercialPosture,
} from '@/stores/licenseCommercial';
import { loadLicenseStatus } from '@/stores/license';
import type { ConnectedInfrastructureItem } from '@/types/api';
import type { Resource } from '@/types/resource';
import { formatRelativeTime } from '@/utils/format';
import { logger } from '@/utils/logger';
import {
  getUpgradeActionButtonClass,
  UPGRADE_ACTION_LABEL,
  UPGRADE_TRIAL_LABEL,
  UPGRADE_TRIAL_LINK_CLASS,
} from '@/utils/upgradePresentation';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';
import { runStartProTrialAction } from '@/utils/trialStartAction';
import { KNOWN_SETTINGS } from './agentProfileSettings';
import {
  getActionableAgentIdFromResource,
  getActionableDockerRuntimeIdFromResource,
  getActionableKubernetesClusterIdFromResource,
  isAgentProfileAssignableResource,
} from '@/utils/agentResources';
import {
  getPreferredInfrastructureDisplayName,
  getPreferredNamedEntityLabel,
  getPreferredResourceDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';
import {
  getAgentStatusIndicator,
  getStatusIndicatorBadgeToneClasses,
  isConnectedHealthStatus,
} from '@/utils/status';

const resourcePriority = (resource: Resource): number => {
  switch (resource.type) {
    case 'agent':
      return 0;
    case 'pbs':
      return 1;
    case 'pmg':
      return 2;
    case 'truenas':
      return 3;
    case 'k8s-cluster':
      return 4;
    case 'docker-host':
      return 5;
    default:
      return 99;
  }
};

export const useAgentProfilesPanelState = () => {
  const { resources } = useResources();
  const { state } = useWebSocket();

  const checkingLicense = () => !licenseLoaded() || licenseLoading();
  const hasAgentProfiles = () => hasEntitlement('agent_profiles');
  const [startingTrial, setStartingTrial] = createSignal(false);
  const canStartTrial = () => commercialPosture()?.trial_eligible !== false;

  const [aiAvailable, setAiAvailable] = createSignal(false);
  const [profiles, setProfiles] = createSignal<AgentProfile[]>([]);
  const [assignments, setAssignments] = createSignal<AgentProfileAssignment[]>([]);
  const [loading, setLoading] = createSignal(true);

  const [showModal, setShowModal] = createSignal(false);
  const [showSuggestModal, setShowSuggestModal] = createSignal(false);
  const [editingProfile, setEditingProfile] = createSignal<AgentProfile | null>(null);
  const [saving, setSaving] = createSignal(false);

  const [formName, setFormName] = createSignal('');
  const [formDescription, setFormDescription] = createSignal('');
  const [formSettings, setFormSettings] = createSignal<Record<string, unknown>>({});

  const connectedInfrastructureItems = createMemo<ConnectedInfrastructureItem[]>(
    () => state.connectedInfrastructure,
  );

  const activeSurfaceControlIds = createMemo(() => {
    const agent = new Set<string>();
    const docker = new Set<string>();
    const kubernetes = new Set<string>();

    connectedInfrastructureItems()
      .filter((item) => item.status === 'active')
      .forEach((item) => {
        item.surfaces.forEach((surface) => {
          const controlId = surface.controlId?.trim();
          if (!controlId) return;
          if (surface.kind === 'agent') agent.add(controlId);
          if (surface.kind === 'docker') docker.add(controlId);
          if (surface.kind === 'kubernetes') kubernetes.add(controlId);
        });
      });

    return { agent, docker, kubernetes };
  });

  const connectedAgents = createMemo(() => {
    const sorted = resources()
      .filter(isAgentProfileAssignableResource)
      .filter((resource) => isConnectedHealthStatus(resource.status))
      .filter((resource) => {
        if (resource.type === 'agent') {
          const agentId = getActionableAgentIdFromResource(resource);
          return !agentId || activeSurfaceControlIds().agent.has(agentId);
        }
        if (resource.type === 'docker-host') {
          const runtimeId = getActionableDockerRuntimeIdFromResource(resource);
          return !runtimeId || activeSurfaceControlIds().docker.has(runtimeId);
        }
        if (resource.type === 'k8s-cluster') {
          const clusterId = getActionableKubernetesClusterIdFromResource(resource);
          return !clusterId || activeSurfaceControlIds().kubernetes.has(clusterId);
        }
        return true;
      })
      .map((resource) => ({ resource, assignmentId: getActionableAgentIdFromResource(resource) }))
      .filter((entry): entry is { resource: Resource; assignmentId: string } =>
        Boolean(entry.assignmentId),
      )
      .sort((a, b) => {
        const byPriority = resourcePriority(a.resource) - resourcePriority(b.resource);
        if (byPriority !== 0) return byPriority;
        const aName = getPreferredInfrastructureDisplayName(a.resource);
        const bName = getPreferredInfrastructureDisplayName(b.resource);
        return aName.localeCompare(bName);
      });

    const byAssignmentId = new Map<string, Resource>();
    for (const entry of sorted) {
      if (byAssignmentId.has(entry.assignmentId)) continue;
      byAssignmentId.set(entry.assignmentId, entry.resource);
    }

    return Array.from(byAssignmentId.entries())
      .map(([assignmentId, resource]) => ({
        id: assignmentId,
        assignmentId,
        hostname: getPreferredResourceHostname(resource) || 'Unknown',
        displayName: getPreferredInfrastructureDisplayName(resource),
        status: resource.status || 'unknown',
        lastSeen: resource.lastSeen,
      }))
      .sort((a, b) =>
        getPreferredNamedEntityLabel(a).localeCompare(getPreferredNamedEntityLabel(b)),
      );
  });

  const getAgentAssignment = (agentId: string) => {
    return assignments().find((assignment) => assignment.agent_id === agentId);
  };

  const getProfileById = (profileId: string) => {
    return profiles().find((profile) => profile.id === profileId);
  };

  const getProfileOptionLabel = (profileId: string) => {
    const profile = getProfileById(profileId);
    if (profile) return profile.name || profile.id;
    return `Missing profile (${profileId})`;
  };

  const getAssignmentCount = (profileId: string) => {
    return assignments().filter((assignment) => assignment.profile_id === profileId).length;
  };

  const getSettingsCount = (profile: AgentProfile) => {
    return Object.keys(profile.config || {}).length;
  };

  const knownKeys = KNOWN_SETTINGS.map((setting) => setting.key);

  const unknownKeys = createMemo(() => {
    const settings = formSettings();
    return Object.keys(settings).filter((key) => !knownKeys.includes(key));
  });

  const loadData = async () => {
    setLoading(true);
    try {
      const [profilesData, assignmentsData] = await Promise.all([
        AgentProfilesAPI.listProfiles(),
        AgentProfilesAPI.listAssignments(),
      ]);
      setProfiles(profilesData);
      setAssignments(assignmentsData);
    } catch (err) {
      logger.error('Failed to load agent profiles', err);
      notificationStore.error(err instanceof Error ? err.message : 'Failed to load agent profiles');
    } finally {
      setLoading(false);
    }
  };

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      await runStartProTrialAction({
        showSuccess: notificationStore.success,
        showError: notificationStore.error,
      });
    } finally {
      setStartingTrial(false);
    }
  };

  createEffect((wasPaywallVisible) => {
    const isPaywallVisible = !checkingLicense() && !hasAgentProfiles();
    if (isPaywallVisible && !wasPaywallVisible) {
      trackPaywallViewed('agent_profiles', 'settings_agent_profiles_panel');
    }
    return isPaywallVisible;
  }, false);

  onMount(async () => {
    await loadLicenseStatus();
    await loadCommercialPosture();

    try {
      const aiSettings = await AIAPI.getSettings();
      setAiAvailable(aiSettings.enabled && aiSettings.configured);
    } catch {
      setAiAvailable(false);
    }

    if (hasAgentProfiles()) {
      await loadData();
    } else {
      setLoading(false);
    }
  });

  const handleCreate = () => {
    setEditingProfile(null);
    setFormName('');
    setFormDescription('');
    setFormSettings({});
    setShowModal(true);
  };

  const handleSuggest = () => {
    setShowSuggestModal(true);
  };

  const handleSuggestionAccepted = (suggestion: ProfileSuggestion) => {
    setShowSuggestModal(false);
    setEditingProfile(null);
    setFormName(suggestion.name);
    setFormDescription(suggestion.description || '');
    setFormSettings(suggestion.config);
    setShowModal(true);
  };

  const handleEdit = (profile: AgentProfile) => {
    setEditingProfile(profile);
    setFormName(profile.name);
    setFormDescription(profile.description || '');
    setFormSettings({ ...profile.config });
    setShowModal(true);
  };

  const handleDelete = async (profile: AgentProfile) => {
    const assignedCount = getAssignmentCount(profile.id);
    const confirmMessage =
      assignedCount > 0
        ? `Delete "${profile.name}"? ${assignedCount} agent(s) will be unassigned.`
        : `Delete "${profile.name}"?`;

    if (!confirm(confirmMessage)) return;

    try {
      await AgentProfilesAPI.deleteProfile(profile.id);
      notificationStore.success(`Profile "${profile.name}" deleted`);
      await loadData();
    } catch (err) {
      logger.error('Failed to delete profile', err);
      notificationStore.error(
        err instanceof Error && err.message ? err.message : 'Failed to delete profile',
      );
    }
  };

  const handleSave = async () => {
    const name = formName().trim();
    if (!name) {
      notificationStore.error('Profile name is required');
      return;
    }

    setSaving(true);
    try {
      const config = formSettings();
      const description = formDescription().trim() || undefined;
      const existing = editingProfile();

      if (existing) {
        await AgentProfilesAPI.updateProfile(existing.id, name, config, description);
        notificationStore.success(`Profile "${name}" updated`);
      } else {
        await AgentProfilesAPI.createProfile(name, config, description);
        notificationStore.success(`Profile "${name}" created`);
      }

      setShowModal(false);
      await loadData();
    } catch (err) {
      logger.error('Failed to save profile', err);
      notificationStore.error(
        err instanceof Error && err.message ? err.message : 'Failed to save profile',
      );
    } finally {
      setSaving(false);
    }
  };

  const handleAssign = async (agentId: string, profileId: string) => {
    try {
      if (profileId === '') {
        await AgentProfilesAPI.unassignProfile(agentId);
        notificationStore.success('Profile unassigned');
      } else {
        await AgentProfilesAPI.assignProfile(agentId, profileId);
        const profile = getProfileById(profileId);
        notificationStore.success(`Assigned "${profile?.name || profileId}"`);
      }
      await loadData();
    } catch (err) {
      logger.error('Failed to assign profile', err);
      if (
        err instanceof Error &&
        err.message === MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE
      ) {
        await loadData();
      }
      notificationStore.error(
        err instanceof Error && err.message ? err.message : 'Failed to assign profile',
      );
    }
  };

  const updateSetting = (key: string, value: unknown) => {
    if (value === '' || value === undefined) {
      const updated = { ...formSettings() };
      delete updated[key];
      setFormSettings(updated);
      return;
    }

    setFormSettings({ ...formSettings(), [key]: value });
  };

  return {
    aiAvailable,
    canStartTrial,
    checkingLicense,
    connectedAgents,
    formDescription,
    formName,
    formSettings,
    getAgentAssignment,
    getAssignmentCount,
    getProfileById,
    getProfileOptionLabel,
    getSettingsCount,
    getStatusIndicatorBadgeToneClasses,
    getUpgradeActionDestination,
    getUpgradeActionButtonClass,
    handleAssign,
    handleCreate,
    handleDelete,
    handleEdit,
    handleSave,
    handleStartTrial,
    handleSuggest,
    handleSuggestionAccepted,
    hasAgentProfiles,
    loading,
    profiles,
    saving,
    setFormDescription,
    setFormName,
    setShowModal,
    setShowSuggestModal,
    showModal,
    showSuggestModal,
    startingTrial,
    trackUpgradeClicked,
    unknownKeys,
    updateSetting,
    editingProfile,
    assignments,
    setFormSettings,
    formatRelativeTime,
    getAgentStatusIndicator,
    UPGRADE_ACTION_LABEL,
    UPGRADE_TRIAL_LABEL,
    UPGRADE_TRIAL_LINK_CLASS,
  };
};
