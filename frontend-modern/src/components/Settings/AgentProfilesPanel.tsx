import { Component, createSignal, createMemo, createEffect, onMount, Show, For } from 'solid-js';
import { useResources } from '@/hooks/useResources';
import { Card } from '@/components/shared/Card';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { AgentProfilesAPI, type AgentProfile, type AgentProfileAssignment, type ProfileSuggestion } from '@/api/agentProfiles';
import { AIAPI } from '@/api/ai';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { formatRelativeTime } from '@/utils/format';
import { getUpgradeActionUrlOrFallback, hasFeature as hasEntitlement, licenseLoaded, loadLicenseStatus, licenseLoading } from '@/stores/license';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';
import { SuggestProfileModal } from './SuggestProfileModal';
import { KNOWN_SETTINGS, type SelectSetting, type StringSetting } from './agentProfileSettings';
import Plus from 'lucide-solid/icons/plus';
import Pencil from 'lucide-solid/icons/pencil';
import Trash2 from 'lucide-solid/icons/trash-2';
import Crown from 'lucide-solid/icons/crown';
import Users from 'lucide-solid/icons/users';
import Settings from 'lucide-solid/icons/settings';
import Lightbulb from 'lucide-solid/icons/lightbulb';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';


export const AgentProfilesPanel: Component = () => {
    const { byType } = useResources();

    const checkingLicense = () => !licenseLoaded() || licenseLoading();
    const hasAgentProfiles = () => hasEntitlement('agent_profiles');

    createEffect((wasPaywallVisible) => {
        const isPaywallVisible = !checkingLicense() && !hasAgentProfiles();
        if (isPaywallVisible && !wasPaywallVisible) {
            trackPaywallViewed('agent_profiles', 'settings_agent_profiles_panel');
        }
        return isPaywallVisible;
    }, false);

    // AI state - only show AI features if enabled
    const [aiAvailable, setAiAvailable] = createSignal(false);

    // Data state
    const [profiles, setProfiles] = createSignal<AgentProfile[]>([]);
    const [assignments, setAssignments] = createSignal<AgentProfileAssignment[]>([]);
    const [loading, setLoading] = createSignal(true);

    // Modal state
    const [showModal, setShowModal] = createSignal(false);
    const [showSuggestModal, setShowSuggestModal] = createSignal(false);
    const [editingProfile, setEditingProfile] = createSignal<AgentProfile | null>(null);
    const [saving, setSaving] = createSignal(false);

    // Form state
    const [formName, setFormName] = createSignal('');
    const [formDescription, setFormDescription] = createSignal('');
    const [formSettings, setFormSettings] = createSignal<Record<string, unknown>>({});

    // Connected agents from WebSocket state
    const connectedAgents = createMemo(() => {
        const hosts = byType('host');
        return hosts.map(r => ({
            id: r.id,
            hostname: r.identity?.hostname || 'Unknown',
            displayName: r.displayName,
            status: r.status || 'unknown',
            lastSeen: r.lastSeen,
        }));
    });

    // Get assignment for a specific agent
    const getAgentAssignment = (agentId: string) => {
        return assignments().find(a => a.agent_id === agentId);
    };

    // Get profile by ID
    const getProfileById = (profileId: string) => {
        return profiles().find(p => p.id === profileId);
    };

    // Count agents assigned to a profile
    const getAssignmentCount = (profileId: string) => {
        return assignments().filter(a => a.profile_id === profileId).length;
    };

    // Count settings in a profile
    const getSettingsCount = (profile: AgentProfile) => {
        return Object.keys(profile.config || {}).length;
    };

    // Get known setting keys for filtering
    const knownKeys = KNOWN_SETTINGS.map(s => s.key);

    // Get unknown keys in the form settings
    const unknownKeys = createMemo(() => {
        const settings = formSettings();
        return Object.keys(settings).filter(key => !knownKeys.includes(key));
    });

    // Load data
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
            notificationStore.error('Failed to load agent profiles');
        } finally {
            setLoading(false);
        }
    };

    // Check license and AI availability on mount
    onMount(async () => {
        await loadLicenseStatus();

        // Check if AI is available (enabled and configured) - silently fail if not
        try {
            const aiSettings = await AIAPI.getSettings();
            setAiAvailable(aiSettings.enabled && aiSettings.configured);
        } catch {
            // AI not available - that's fine, just hide the Ideas button
            setAiAvailable(false);
        }

        if (hasAgentProfiles()) {
            await loadData();
        } else {
            setLoading(false);
        }
    });

    // Open modal for creating a new profile
    const handleCreate = () => {
        setEditingProfile(null);
        setFormName('');
        setFormDescription('');
        setFormSettings({});
        setShowModal(true);
    };

    // Open suggest modal
    const handleSuggest = () => {
        setShowSuggestModal(true);
    };

    // Handle AI suggestion acceptance
    const handleSuggestionAccepted = (suggestion: ProfileSuggestion) => {
        setShowSuggestModal(false);
        setEditingProfile(null);
        setFormName(suggestion.name);
        setFormDescription(suggestion.description || '');
        setFormSettings(suggestion.config);
        setShowModal(true);
    };

    // Open modal for editing a profile
    const handleEdit = (profile: AgentProfile) => {
        setEditingProfile(profile);
        setFormName(profile.name);
        setFormDescription(profile.description || '');
        setFormSettings({ ...profile.config });
        setShowModal(true);
    };

    // Delete a profile
    const handleDelete = async (profile: AgentProfile) => {
        const assignedCount = getAssignmentCount(profile.id);
        const confirmMsg = assignedCount > 0
            ? `Delete "${profile.name}"? ${assignedCount} agent(s) will be unassigned.`
            : `Delete "${profile.name}"?`;

        if (!confirm(confirmMsg)) return;

        try {
            await AgentProfilesAPI.deleteProfile(profile.id);
            notificationStore.success(`Profile "${profile.name}" deleted`);
            await loadData();
        } catch (err) {
            logger.error('Failed to delete profile', err);
            notificationStore.error('Failed to delete profile');
        }
    };

    // Save profile (create or update)
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
            notificationStore.error('Failed to save profile');
        } finally {
            setSaving(false);
        }
    };

    // Assign profile to agent
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
            notificationStore.error('Failed to assign profile');
        }
    };

    // Update a setting in the form
    const updateSetting = (key: string, value: unknown) => {
        if (value === '' || value === undefined) {
            const updated = { ...formSettings() };
            delete updated[key];
            setFormSettings(updated);
        } else {
            setFormSettings({ ...formSettings(), [key]: value });
        }
    };

    // License gate - using Show components for proper SolidJS reactivity
    // (early returns don't re-render when signals change in SolidJS)
    return (
        <Show
            when={!checkingLicense()}
            fallback={
                <Card padding="lg">
                    <div class="flex items-center justify-center py-8">
                        <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500" />
                        <span class="ml-3 text-muted">Checking license...</span>
                    </div>
                </Card>
            }
        >
            <Show
                when={hasAgentProfiles()}
                fallback={
                    <Card padding="lg" class="space-y-4">
                        <div class="flex items-center gap-3">
                            <div class="flex items-center justify-center w-10 h-10 rounded-md bg-amber-100 dark:bg-amber-900">
                                <Crown class="w-5 h-5 text-amber-600 dark:text-amber-400" />
                            </div>
                            <div>
                                <h3 class="text-base font-semibold text-base-content">Agent Profiles</h3>
                                <p class="text-sm text-muted">Pro feature</p>
                            </div>
                        </div>
                        <p class="text-sm text-muted">
                            Create reusable configuration profiles for your agents. Manage settings like Docker monitoring,
                            logging levels, and reporting intervals from a central location.
                        </p>
                        <a
                            href={getUpgradeActionUrlOrFallback('agent_profiles')}
                            target="_blank"
                            rel="noopener noreferrer"
                            class="inline-flex items-center gap-2 rounded-md border border-amber-300 dark:border-amber-700 bg-amber-100 dark:bg-amber-900 px-4 py-2 text-sm font-medium text-amber-800 dark:text-amber-100 transition-colors hover:bg-amber-200 dark:hover:bg-amber-800"
                            onClick={() => trackUpgradeClicked('settings_agent_profiles_panel', 'agent_profiles')}
                        >
                            <Crown class="w-4 h-4" />
                            Upgrade to Pro
                        </a>
                    </Card>
                }
            >
                <div class="space-y-6">
                    {/* Profiles Section */}
                    <SettingsPanel
                        title="Configuration Profiles"
                        description="Reusable agent configurations"
                        icon={<Settings class="w-5 h-5" strokeWidth={2} />}
                        action={
                            <div class="flex items-center gap-2">
                                <button
                                    type="button"
                                    onClick={handleCreate}
                                    class="inline-flex min-h-10 sm:min-h-9 items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 sm:px-4 sm:py-2 sm:text-sm"
                                >
                                    <Plus class="w-4 h-4" />
                                    <span class="hidden sm:inline">New Profile</span>
                                    <span class="sm:hidden">New</span>
                                </button>
                                {/* Only show AI Ideas button if AI is enabled and configured */}
                                <Show when={aiAvailable()}>
                                    <button
                                        type="button"
                                        onClick={handleSuggest}
                                        title="Get AI-powered profile suggestions"
                                        class="inline-flex min-h-10 sm:min-h-9 min-w-10 items-center gap-1.5 rounded-md border border-border bg-surface px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover sm:px-3 sm:py-2 sm:text-sm"
                                    >
                                        <Lightbulb class="w-3.5 h-3.5" />
                                        <span class="hidden sm:inline">Ideas</span>
                                    </button>
                                </Show>
                            </div>
                        }
                        noPadding
                        bodyClass="divide-y divide-border"
                    >
                        <Show when={loading()}>
                            <div class="flex items-center justify-center py-8">
                                <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500" />
                            </div>
                        </Show>

                        <Show when={!loading() && profiles().length === 0}>
                            <div class="text-center py-8 text-muted">
                                <Settings class="w-12 h-12 mx-auto mb-3 opacity-50" />
                                <p class="text-sm">No profiles yet. Create one to get started.</p>
                            </div>
                        </Show>

                        <Show when={!loading() && profiles().length > 0}>
                            <div class="w-full overflow-x-auto">
                                <PulseDataGrid
                                    data={profiles()}
                                    columns={[
                                        {
                                            key: 'name',
                                            label: 'Name',
                                            render: (profile) => <span class="font-medium text-base-content">{profile.name}</span>
                                        },
                                        {
                                            key: 'settings',
                                            label: 'Settings',
                                            render: (profile) => <span class="text-muted">{getSettingsCount(profile)}</span>
                                        },
                                        {
                                            key: 'agents',
                                            label: 'Agents',
                                            render: (profile) => (
                                                <span class="inline-flex items-center gap-1 text-muted">
                                                    <Users class="w-4 h-4" />
                                                    {getAssignmentCount(profile.id)}
                                                </span>
                                            )
                                        },
                                        {
                                            key: 'actions',
                                            label: 'Actions',
                                            align: 'right',
                                            render: (profile) => (
                                                <div class="inline-flex items-center gap-1">
                                                    <button
                                                        type="button"
                                                        onClick={() => handleEdit(profile)}
                                                        class="p-1.5 rounded-md text-slate-500 hover:text-blue-600 hover:bg-blue-50 dark:hover:text-blue-400 dark:hover:bg-blue-900"
                                                        title="Edit profile"
                                                    >
                                                        <Pencil class="w-4 h-4" />
                                                    </button>
                                                    <button
                                                        type="button"
                                                        onClick={() => handleDelete(profile)}
                                                        class="p-1.5 rounded-md text-slate-500 hover:text-red-600 hover:bg-red-50 dark:hover:text-red-400 dark:hover:bg-red-900"
                                                        title="Delete profile"
                                                    >
                                                        <Trash2 class="w-4 h-4" />
                                                    </button>
                                                </div>
                                            )
                                        }
                                    ]}
                                    keyExtractor={(profile) => profile.id}
                                    emptyState="No profiles yet. Create one to get started."
                                    desktopMinWidth="600px"
                                    class="border-x-0 sm:border-x border-slate-200 dark:border-slate-800"
                                />
                            </div>
                        </Show>
                    </SettingsPanel>

                    {/* Assignments Section */}
                    <SettingsPanel
                        title="Agent Assignments"
                        description="Assign profiles to connected agents"
                        icon={<Users class="w-5 h-5" strokeWidth={2} />}
                        noPadding
                        bodyClass="divide-y divide-border"
                    >
                        <Show when={connectedAgents().length === 0}>
                            <div class="text-center py-8 text-muted">
                                <Users class="w-12 h-12 mx-auto mb-3 opacity-50" />
                                <p class="text-sm">No agents connected. Install an agent to assign profiles.</p>
                            </div>
                        </Show>

                        <Show when={connectedAgents().length > 0}>
                            <div class="w-full overflow-x-auto">
                                <PulseDataGrid
                                    data={connectedAgents()}
                                    columns={[
                                        {
                                            key: 'agent',
                                            label: 'Agent',
                                            render: (agent) => (
                                                <div>
                                                    <span class="font-medium text-base-content">
                                                        {agent.displayName || agent.hostname}
                                                    </span>
                                                    <Show when={agent.displayName && agent.hostname !== agent.displayName}>
                                                        <span class="ml-2 text-xs text-muted">
                                                            ({agent.hostname})
                                                        </span>
                                                    </Show>
                                                </div>
                                            )
                                        },
                                        {
                                            key: 'profile',
                                            label: 'Profile',
                                            render: (agent) => {
                                                const assignment = () => getAgentAssignment(agent.id);
                                                return (
                                                    <select
                                                        value={assignment()?.profile_id || ''}
                                                        onChange={(e) => handleAssign(agent.id, e.currentTarget.value)}
                                                        class="min-h-10 sm:min-h-9 w-full sm:max-w-xs rounded-md border border-slate-300 bg-white px-2.5 py-1.5 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                                                    >
                                                        <option value="">No profile</option>
                                                        <For each={profiles()}>
                                                            {(profile) => (
                                                                <option value={profile.id}>{profile.name}</option>
                                                            )}
                                                        </For>
                                                    </select>
                                                );
                                            }
                                        },
                                        {
                                            key: 'status',
                                            label: 'Status',
                                            render: (agent) => {
                                                const isOnline = () => agent.status?.toLowerCase() === 'online' || agent.status?.toLowerCase() === 'running';
                                                return (
                                                    <span class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${isOnline()
                                                        ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
                                                        : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400'
                                                        }`}>
                                                        {isOnline() ? 'Online' : 'Offline'}
                                                    </span>
                                                );
                                            }
                                        },
                                        {
                                            key: 'lastSeen',
                                            label: 'Last Seen',
                                            hiddenOnMobile: true,
                                            render: (agent) => (
                                                <span class="text-muted">
                                                    {agent.lastSeen ? formatRelativeTime(agent.lastSeen) : 'Never'}
                                                </span>
                                            )
                                        }
                                    ]}
                                    keyExtractor={(agent) => agent.id}
                                    emptyState="No agents connected. Install an agent to assign profiles."
                                    desktopMinWidth="800px"
                                    class="border-x-0 sm:border-x border-slate-200 dark:border-slate-800"
                                />
                            </div>
                        </Show>
                    </SettingsPanel>

                    {/* Profile Modal */}
                    <Show when={showModal()}>
                        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black opacity-50">
                            <div class="w-full max-w-lg bg-surface rounded-md shadow-sm border border-border mx-4">
                                <div class="flex items-center justify-between px-6 py-4 border-b border-border">
                                    <h3 class="text-lg font-semibold text-base-content">
                                        {editingProfile() ? 'Edit Profile' : 'New Profile'}
                                    </h3>
                                    <button
                                        type="button"
                                        onClick={() => setShowModal(false)}
                                        class="p-1.5 rounded-md text-slate-500 hover:text-slate-700 hover:bg-slate-100 dark:hover:text-slate-300 dark:hover:bg-slate-800"
                                    >
                                        <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                        </svg>
                                    </button>
                                </div>

                                <div class="px-6 py-4 space-y-4 max-h-[60vh] overflow-y-auto">
                                    {/* Profile Name */}
                                    <div class="space-y-1">
                                        <label class="block text-sm font-medium text-base-content">
                                            Profile Name
                                        </label>
                                        <input
                                            type="text"
                                            value={formName()}
                                            onInput={(e) => setFormName(e.currentTarget.value)}
                                            placeholder="e.g., Production Servers"
                                            class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                                        />
                                    </div>

                                    {/* Profile Description */}
                                    <div class="space-y-1">
                                        <label class="block text-sm font-medium text-base-content">
                                            Description <span class="text-slate-400 font-normal">(optional)</span>
                                        </label>
                                        <textarea
                                            value={formDescription()}
                                            onInput={(e) => setFormDescription(e.currentTarget.value)}
                                            placeholder="What is this profile for?"
                                            rows={2}
                                            class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:focus:border-blue-400 dark:focus:ring-blue-800 resize-none"
                                        />
                                    </div>

                                    {/* Settings */}
                                    <div class="space-y-3">
                                        <label class="block text-sm font-medium text-base-content">
                                            Settings
                                        </label>

                                        <For each={KNOWN_SETTINGS}>
                                            {(setting) => (
                                                <div class="rounded-md border border-border p-3 space-y-1">
                                                    <div class="flex items-center justify-between">
                                                        <label class="text-sm font-medium text-base-content">
                                                            {setting.label}
                                                        </label>
                                                        <Show when={setting.type === 'boolean'}>
                                                            <button
                                                                type="button"
                                                                onClick={() => {
                                                                    const current = formSettings()[setting.key];
                                                                    if (current === undefined) {
                                                                        updateSetting(setting.key, true);
                                                                    } else if (current === true) {
                                                                        updateSetting(setting.key, false);
                                                                    } else {
                                                                        updateSetting(setting.key, undefined);
                                                                    }
                                                                }}
                                                                class={`relative inline-flex h-8 w-12 sm:h-7 sm:w-12 items-center rounded-full transition-colors ${formSettings()[setting.key] === true
                                                                    ? 'bg-blue-600'
                                                                    : formSettings()[setting.key] === false
                                                                        ? 'bg-slate-400'
                                                                        : 'bg-surface-hover'
                                                                    }`}
                                                            >
                                                                <span
                                                                    class={`inline-block h-6 w-6 sm:h-5 sm:w-5 transform rounded-full bg-white transition-transform ${formSettings()[setting.key] === true
                                                                        ? 'translate-x-4 sm:translate-x-5'
                                                                        : formSettings()[setting.key] === false
                                                                            ? 'translate-x-1'
                                                                            : 'translate-x-3'
                                                                        }`}
                                                                />
                                                            </button>
                                                        </Show>
                                                        <Show when={setting.type === 'select'}>
                                                            <select
                                                                value={(formSettings()[setting.key] as string) || ''}
                                                                onChange={(e) => updateSetting(setting.key, e.currentTarget.value || undefined)}
                                                                class="min-h-10 sm:min-h-9 rounded-md border border-slate-300 bg-white px-2.5 py-1.5 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                                                            >
                                                                <option value="">Default</option>
                                                                <For each={(setting as SelectSetting).options}>
                                                                    {(opt) => <option value={opt}>{opt}</option>}
                                                                </For>
                                                            </select>
                                                        </Show>
                                                        <Show when={setting.type === 'duration'}>
                                                            <input
                                                                type="text"
                                                                value={(formSettings()[setting.key] as string) || ''}
                                                                onInput={(e) => updateSetting(setting.key, e.currentTarget.value || undefined)}
                                                                placeholder="30s"
                                                                class="min-h-10 sm:min-h-9 w-24 rounded-md border border-slate-300 bg-white px-2.5 py-1.5 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                                                            />
                                                        </Show>
                                                        <Show when={setting.type === 'string'}>
                                                            <input
                                                                type="text"
                                                                value={(formSettings()[setting.key] as string) || ''}
                                                                onInput={(e) => updateSetting(setting.key, e.currentTarget.value || undefined)}
                                                                placeholder={(setting as StringSetting).placeholder || ''}
                                                                class="min-h-10 sm:min-h-9 w-40 rounded-md border border-slate-300 bg-white px-2.5 py-1.5 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                                                            />
                                                        </Show>
                                                    </div>
                                                    <p class="text-xs text-muted">{setting.description}</p>
                                                </div>
                                            )}
                                        </For>

                                        {/* Unknown Keys Section */}
                                        <Show when={unknownKeys().length > 0}>
                                            <div class="pt-3 mt-3 border-t border-border">
                                                <p class="text-xs text-amber-600 dark:text-amber-400 mb-2">
                                                    Additional settings (not in standard list):
                                                </p>
                                                <For each={unknownKeys()}>
                                                    {(key) => (
                                                        <div class="rounded-md border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900 p-3 mb-2">
                                                            <div class="flex items-center justify-between">
                                                                <label class="text-sm font-medium text-base-content font-mono">
                                                                    {key}
                                                                </label>
                                                                <div class="flex items-center gap-2">
                                                                    <input
                                                                        type="text"
                                                                        value={String(formSettings()[key] ?? '')}
                                                                        onInput={(e) => {
                                                                            const val = e.currentTarget.value;
                                                                            // Try to parse as JSON for complex values
                                                                            try {
                                                                                updateSetting(key, JSON.parse(val));
                                                                            } catch {
                                                                                updateSetting(key, val || undefined);
                                                                            }
                                                                        }}
                                                                        class="min-h-10 sm:min-h-9 w-32 rounded-md border border-slate-300 bg-white px-2.5 py-1.5 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                                                                    />
                                                                    <button
                                                                        type="button"
                                                                        onClick={() => updateSetting(key, undefined)}
                                                                        class="inline-flex min-h-10 min-w-10 sm:min-h-9 sm:min-w-9 items-center justify-center rounded text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900"
                                                                        title="Remove this setting"
                                                                    >
                                                                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                                                        </svg>
                                                                    </button>
                                                                </div>
                                                            </div>
                                                        </div>
                                                    )}
                                                </For>
                                            </div>
                                        </Show>
                                    </div>
                                </div>

                                <div class="flex items-center justify-end gap-3 px-6 py-4 border-t border-border">
                                    <button
                                        type="button"
                                        onClick={() => setShowModal(false)}
                                        class="rounded-md px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
                                    >
                                        Cancel
                                    </button>
                                    <button
                                        type="button"
                                        onClick={handleSave}
                                        disabled={saving() || !formName().trim()}
                                        class="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                                    >
                                        {saving() ? 'Saving...' : editingProfile() ? 'Update Profile' : 'Create Profile'}
                                    </button>
                                </div>
                            </div>
                        </div>
                    </Show>

                    {/* Suggest Profile Modal */}
                    <Show when={showSuggestModal()}>
                        <SuggestProfileModal
                            onClose={() => setShowSuggestModal(false)}
                            onSuggestionAccepted={handleSuggestionAccepted}
                        />
                    </Show>
                </div>
            </Show>
        </Show>
    );
};

export default AgentProfilesPanel;
