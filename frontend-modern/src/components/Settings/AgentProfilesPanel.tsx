import { Component, createSignal, createMemo, onMount, Show, For } from 'solid-js';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { AgentProfilesAPI, type AgentProfile, type AgentProfileAssignment } from '@/api/agentProfiles';
import { LicenseAPI } from '@/api/license';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { formatRelativeTime } from '@/utils/format';
import Plus from 'lucide-solid/icons/plus';
import Pencil from 'lucide-solid/icons/pencil';
import Trash2 from 'lucide-solid/icons/trash-2';
import Crown from 'lucide-solid/icons/crown';
import Users from 'lucide-solid/icons/users';
import Settings from 'lucide-solid/icons/settings';

// Known agent settings with their types and descriptions
interface BooleanSetting {
    key: string;
    type: 'boolean';
    label: string;
    description: string;
}

interface SelectSetting {
    key: string;
    type: 'select';
    label: string;
    description: string;
    options: string[];
}

interface DurationSetting {
    key: string;
    type: 'duration';
    label: string;
    description: string;
}

type KnownSetting = BooleanSetting | SelectSetting | DurationSetting;

const KNOWN_SETTINGS: KnownSetting[] = [
    { key: 'enable_docker', type: 'boolean', label: 'Enable Docker Monitoring', description: 'Monitor Docker containers on this agent' },
    { key: 'enable_kubernetes', type: 'boolean', label: 'Enable Kubernetes Monitoring', description: 'Monitor Kubernetes workloads' },
    { key: 'enable_proxmox', type: 'boolean', label: 'Enable Proxmox Mode', description: 'Auto-detect and configure Proxmox API access' },
    { key: 'log_level', type: 'select', label: 'Log Level', description: 'Agent logging verbosity', options: ['debug', 'info', 'warn', 'error'] },
    { key: 'interval', type: 'duration', label: 'Reporting Interval', description: 'How often the agent reports metrics (e.g., 30s, 1m)' },
];

export const AgentProfilesPanel: Component = () => {
    const { state } = useWebSocket();

    // License state
    const [hasFeature, setHasFeature] = createSignal(false);
    const [checkingLicense, setCheckingLicense] = createSignal(true);

    // Data state
    const [profiles, setProfiles] = createSignal<AgentProfile[]>([]);
    const [assignments, setAssignments] = createSignal<AgentProfileAssignment[]>([]);
    const [loading, setLoading] = createSignal(true);

    // Modal state
    const [showModal, setShowModal] = createSignal(false);
    const [editingProfile, setEditingProfile] = createSignal<AgentProfile | null>(null);
    const [saving, setSaving] = createSignal(false);

    // Form state
    const [formName, setFormName] = createSignal('');
    const [formSettings, setFormSettings] = createSignal<Record<string, unknown>>({});

    // Connected agents from WebSocket state
    const connectedAgents = createMemo(() => {
        const hosts = state.hosts || [];
        return hosts.map(h => ({
            id: h.id,
            hostname: h.hostname || 'Unknown',
            displayName: h.displayName,
            status: h.status || 'unknown',
            lastSeen: h.lastSeen,
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

    // Check license on mount
    onMount(async () => {
        try {
            const features = await LicenseAPI.getFeatures();
            setHasFeature(features.features?.['agent_profiles'] === true);
        } catch (err) {
            logger.error('Failed to check license', err);
            setHasFeature(false);
        } finally {
            setCheckingLicense(false);
        }

        if (hasFeature()) {
            await loadData();
        } else {
            setLoading(false);
        }
    });

    // Open modal for creating a new profile
    const handleCreate = () => {
        setEditingProfile(null);
        setFormName('');
        setFormSettings({});
        setShowModal(true);
    };

    // Open modal for editing a profile
    const handleEdit = (profile: AgentProfile) => {
        setEditingProfile(profile);
        setFormName(profile.name);
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
            const existing = editingProfile();

            if (existing) {
                await AgentProfilesAPI.updateProfile(existing.id, name, config);
                notificationStore.success(`Profile "${name}" updated`);
            } else {
                await AgentProfilesAPI.createProfile(name, config);
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

    // Render license gate
    if (checkingLicense()) {
        return (
            <Card padding="lg">
                <div class="flex items-center justify-center py-8">
                    <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500" />
                    <span class="ml-3 text-gray-600 dark:text-gray-400">Checking license...</span>
                </div>
            </Card>
        );
    }

    if (!hasFeature()) {
        return (
            <Card padding="lg" class="space-y-4">
                <div class="flex items-center gap-3">
                    <div class="flex items-center justify-center w-10 h-10 rounded-lg bg-amber-100 dark:bg-amber-900/30">
                        <Crown class="w-5 h-5 text-amber-600 dark:text-amber-400" />
                    </div>
                    <div>
                        <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Agent Profiles</h3>
                        <p class="text-sm text-gray-600 dark:text-gray-400">Pro feature</p>
                    </div>
                </div>
                <p class="text-sm text-gray-600 dark:text-gray-400">
                    Create reusable configuration profiles for your agents. Manage settings like Docker monitoring,
                    logging levels, and reporting intervals from a central location.
                </p>
                <a
                    href="https://pulserelay.pro/"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="inline-flex items-center gap-2 rounded-lg bg-gradient-to-r from-amber-500 to-orange-500 px-4 py-2 text-sm font-medium text-white transition-all hover:from-amber-600 hover:to-orange-600"
                >
                    <Crown class="w-4 h-4" />
                    Upgrade to Pro
                </a>
            </Card>
        );
    }

    return (
        <div class="space-y-6">
            {/* Profiles Section */}
            <Card padding="lg" class="space-y-4">
                <div class="flex items-center justify-between">
                    <div class="flex items-center gap-3">
                        <div class="flex items-center justify-center w-10 h-10 rounded-lg bg-blue-100 dark:bg-blue-900/30">
                            <Settings class="w-5 h-5 text-blue-600 dark:text-blue-400" />
                        </div>
                        <div>
                            <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Configuration Profiles</h3>
                            <p class="text-sm text-gray-600 dark:text-gray-400">Reusable agent configurations</p>
                        </div>
                    </div>
                    <button
                        type="button"
                        onClick={handleCreate}
                        class="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700"
                    >
                        <Plus class="w-4 h-4" />
                        New Profile
                    </button>
                </div>

                <Show when={loading()}>
                    <div class="flex items-center justify-center py-8">
                        <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500" />
                    </div>
                </Show>

                <Show when={!loading() && profiles().length === 0}>
                    <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                        <Settings class="w-12 h-12 mx-auto mb-3 opacity-50" />
                        <p class="text-sm">No profiles yet. Create one to get started.</p>
                    </div>
                </Show>

                <Show when={!loading() && profiles().length > 0}>
                    <div class="overflow-x-auto">
                        <table class="w-full text-sm">
                            <thead>
                                <tr class="border-b border-gray-200 dark:border-gray-700">
                                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Name</th>
                                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Settings</th>
                                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Agents</th>
                                    <th class="text-right py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                                <For each={profiles()}>
                                    {(profile) => (
                                        <tr class="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50">
                                            <td class="py-3 px-3">
                                                <span class="font-medium text-gray-900 dark:text-gray-100">{profile.name}</span>
                                            </td>
                                            <td class="py-3 px-3">
                                                <span class="text-gray-600 dark:text-gray-400">{getSettingsCount(profile)}</span>
                                            </td>
                                            <td class="py-3 px-3">
                                                <span class="inline-flex items-center gap-1 text-gray-600 dark:text-gray-400">
                                                    <Users class="w-4 h-4" />
                                                    {getAssignmentCount(profile.id)}
                                                </span>
                                            </td>
                                            <td class="py-3 px-3 text-right">
                                                <div class="inline-flex items-center gap-1">
                                                    <button
                                                        type="button"
                                                        onClick={() => handleEdit(profile)}
                                                        class="p-1.5 rounded-md text-gray-500 hover:text-blue-600 hover:bg-blue-50 dark:hover:text-blue-400 dark:hover:bg-blue-900/30"
                                                        title="Edit profile"
                                                    >
                                                        <Pencil class="w-4 h-4" />
                                                    </button>
                                                    <button
                                                        type="button"
                                                        onClick={() => handleDelete(profile)}
                                                        class="p-1.5 rounded-md text-gray-500 hover:text-red-600 hover:bg-red-50 dark:hover:text-red-400 dark:hover:bg-red-900/30"
                                                        title="Delete profile"
                                                    >
                                                        <Trash2 class="w-4 h-4" />
                                                    </button>
                                                </div>
                                            </td>
                                        </tr>
                                    )}
                                </For>
                            </tbody>
                        </table>
                    </div>
                </Show>
            </Card>

            {/* Assignments Section */}
            <Card padding="lg" class="space-y-4">
                <div class="flex items-center gap-3">
                    <div class="flex items-center justify-center w-10 h-10 rounded-lg bg-green-100 dark:bg-green-900/30">
                        <Users class="w-5 h-5 text-green-600 dark:text-green-400" />
                    </div>
                    <div>
                        <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Agent Assignments</h3>
                        <p class="text-sm text-gray-600 dark:text-gray-400">Assign profiles to connected agents</p>
                    </div>
                </div>

                <Show when={connectedAgents().length === 0}>
                    <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                        <Users class="w-12 h-12 mx-auto mb-3 opacity-50" />
                        <p class="text-sm">No agents connected. Install an agent to assign profiles.</p>
                    </div>
                </Show>

                <Show when={connectedAgents().length > 0}>
                    <div class="overflow-x-auto">
                        <table class="w-full text-sm">
                            <thead>
                                <tr class="border-b border-gray-200 dark:border-gray-700">
                                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Agent</th>
                                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Profile</th>
                                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Status</th>
                                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Last Seen</th>
                                </tr>
                            </thead>
                            <tbody>
                                <For each={connectedAgents()}>
                                    {(agent) => {
                                        const assignment = () => getAgentAssignment(agent.id);
                                        const isOnline = () => agent.status?.toLowerCase() === 'online' || agent.status?.toLowerCase() === 'running';

                                        return (
                                            <tr class="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50">
                                                <td class="py-3 px-3">
                                                    <div>
                                                        <span class="font-medium text-gray-900 dark:text-gray-100">
                                                            {agent.displayName || agent.hostname}
                                                        </span>
                                                        <Show when={agent.displayName && agent.hostname !== agent.displayName}>
                                                            <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">
                                                                ({agent.hostname})
                                                            </span>
                                                        </Show>
                                                    </div>
                                                </td>
                                                <td class="py-3 px-3">
                                                    <select
                                                        value={assignment()?.profile_id || ''}
                                                        onChange={(e) => handleAssign(agent.id, e.currentTarget.value)}
                                                        class="rounded-md border border-gray-300 bg-white px-2 py-1 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                                                    >
                                                        <option value="">No profile</option>
                                                        <For each={profiles()}>
                                                            {(profile) => (
                                                                <option value={profile.id}>{profile.name}</option>
                                                            )}
                                                        </For>
                                                    </select>
                                                </td>
                                                <td class="py-3 px-3">
                                                    <span class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${isOnline()
                                                        ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                                        : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
                                                        }`}>
                                                        {isOnline() ? 'Online' : 'Offline'}
                                                    </span>
                                                </td>
                                                <td class="py-3 px-3 text-gray-600 dark:text-gray-400">
                                                    {agent.lastSeen ? formatRelativeTime(agent.lastSeen) : 'Never'}
                                                </td>
                                            </tr>
                                        );
                                    }}
                                </For>
                            </tbody>
                        </table>
                    </div>
                </Show>
            </Card>

            {/* Profile Modal */}
            <Show when={showModal()}>
                <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
                    <div class="w-full max-w-lg bg-white dark:bg-gray-900 rounded-xl shadow-2xl border border-gray-200 dark:border-gray-700 mx-4">
                        <div class="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                            <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                                {editingProfile() ? 'Edit Profile' : 'New Profile'}
                            </h3>
                            <button
                                type="button"
                                onClick={() => setShowModal(false)}
                                class="p-1.5 rounded-md text-gray-500 hover:text-gray-700 hover:bg-gray-100 dark:hover:text-gray-300 dark:hover:bg-gray-800"
                            >
                                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                </svg>
                            </button>
                        </div>

                        <div class="px-6 py-4 space-y-4 max-h-[60vh] overflow-y-auto">
                            {/* Profile Name */}
                            <div class="space-y-1">
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                                    Profile Name
                                </label>
                                <input
                                    type="text"
                                    value={formName()}
                                    onInput={(e) => setFormName(e.currentTarget.value)}
                                    placeholder="e.g., Production Servers"
                                    class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-800/60"
                                />
                            </div>

                            {/* Settings */}
                            <div class="space-y-3">
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                                    Settings
                                </label>

                                <For each={KNOWN_SETTINGS}>
                                    {(setting) => (
                                        <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3 space-y-1">
                                            <div class="flex items-center justify-between">
                                                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
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
                                                        class={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${formSettings()[setting.key] === true
                                                            ? 'bg-blue-600'
                                                            : formSettings()[setting.key] === false
                                                                ? 'bg-gray-400'
                                                                : 'bg-gray-200 dark:bg-gray-700'
                                                            }`}
                                                    >
                                                        <span
                                                            class={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${formSettings()[setting.key] === true
                                                                ? 'translate-x-6'
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
                                                        class="rounded-md border border-gray-300 bg-white px-2 py-1 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
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
                                                        class="w-24 rounded-md border border-gray-300 bg-white px-2 py-1 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                                                    />
                                                </Show>
                                            </div>
                                            <p class="text-xs text-gray-500 dark:text-gray-400">{setting.description}</p>
                                        </div>
                                    )}
                                </For>
                            </div>
                        </div>

                        <div class="flex items-center justify-end gap-3 px-6 py-4 border-t border-gray-200 dark:border-gray-700">
                            <button
                                type="button"
                                onClick={() => setShowModal(false)}
                                class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
                            >
                                Cancel
                            </button>
                            <button
                                type="button"
                                onClick={handleSave}
                                disabled={saving() || !formName().trim()}
                                class="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                            >
                                {saving() ? 'Saving...' : editingProfile() ? 'Update Profile' : 'Create Profile'}
                            </button>
                        </div>
                    </div>
                </div>
            </Show>
        </div>
    );
};

export default AgentProfilesPanel;
