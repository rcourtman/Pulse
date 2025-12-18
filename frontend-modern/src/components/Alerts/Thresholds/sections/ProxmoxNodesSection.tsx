/**
 * ProxmoxNodesSection Component
 *
 * Displays Proxmox nodes in a collapsible section with the new card-based layout.
 * This is the first section to be migrated to the new architecture.
 */

import { Component, For, createMemo, createSignal } from 'solid-js';
import Server from 'lucide-solid/icons/server';
import { CollapsibleSection } from './CollapsibleSection';
import { ResourceCard } from '../components/ResourceCard';
import { GlobalDefaultsRow } from '../components/GlobalDefaultsRow';
import type { ThresholdResource, ThresholdValues, ThresholdColumn } from '../types';
import type { Alert, Node } from '@/types/api';
import type { RawOverrideConfig } from '@/types/alerts';

/**
 * Column definitions for Proxmox nodes
 */
const NODE_COLUMNS: ThresholdColumn[] = [
    { key: 'cpu', label: 'CPU %', unit: '%' },
    { key: 'memory', label: 'Memory %', unit: '%' },
    { key: 'disk', label: 'Disk %', unit: '%' },
    { key: 'temperature', label: 'Temp °C', unit: '°C' },
];

export interface ProxmoxNodesSectionProps {
    /** Raw nodes from state */
    nodes: Node[];
    /** Current overrides */
    overrides: Array<{
        id: string;
        name: string;
        type: string;
        thresholds: Record<string, number | undefined>;
        disableConnectivity?: boolean;
        note?: string;
    }>;
    /** Raw overrides config for saving */
    rawOverridesConfig: Record<string, RawOverrideConfig>;
    setRawOverridesConfig: (config: Record<string, RawOverrideConfig>) => void;
    /** Default thresholds */
    nodeDefaults: ThresholdValues;
    setNodeDefaults: (
        value: ThresholdValues | ((prev: ThresholdValues) => ThresholdValues)
    ) => void;
    factoryNodeDefaults?: ThresholdValues;
    resetNodeDefaults?: () => void;
    /** Global disable flags */
    disableAllNodes: boolean;
    setDisableAllNodes: (value: boolean) => void;
    disableAllNodesOffline: boolean;
    setDisableAllNodesOffline: (value: boolean) => void;
    /** Time thresholds */
    globalDelaySeconds?: number;
    metricDelaySeconds?: Record<string, number>;
    /** Unsaved changes tracking */
    setHasUnsavedChanges: (value: boolean) => void;
    /** Active alerts */
    activeAlerts?: Record<string, Alert>;
    removeAlerts?: (predicate: (alert: Alert) => boolean) => void;
    /** Section state */
    isCollapsed?: boolean;
    onToggleCollapse?: (collapsed: boolean) => void;
    /** Overrides management */
    setOverrides: (overrides: any[]) => void;
}

/**
 * Transform a Node to a ThresholdResource
 */
const nodeToResource = (
    node: Node,
    override?: {
        thresholds: Record<string, number | undefined>;
        disableConnectivity?: boolean;
        note?: string;
    },
    defaults?: ThresholdValues
): ThresholdResource => {
    // Build a friendly display name
    const displayName = node.displayName?.trim() || node.name;

    // Check if there are any custom thresholds
    const hasCustomThresholds = override?.thresholds &&
        Object.keys(override.thresholds).some((key) => {
            const value = override.thresholds[key];
            const defaultValue = defaults?.[key];
            return value !== undefined && value !== defaultValue;
        });

    const hasNote = Boolean(override?.note?.trim());

    return {
        id: node.id,
        name: displayName,
        displayName,
        rawName: node.name,
        type: 'node',
        resourceType: 'Node',
        status: node.status,
        uptime: node.uptime,
        cpu: node.cpu,
        memory: node.memory?.usage,
        hasOverride: hasCustomThresholds || hasNote || Boolean(override?.disableConnectivity),
        disableConnectivity: override?.disableConnectivity || false,
        thresholds: override?.thresholds || {},
        defaults: defaults || {},
        clusterName: node.isClusterMember ? node.clusterName?.trim() : undefined,
        isClusterMember: node.isClusterMember ?? false,
        instance: node.instance,
        note: override?.note,
        // Build host URL
        host: buildNodeUrl(node),
    };
};

/**
 * Build management URL for a node
 */
const buildNodeUrl = (node: Node): string | undefined => {
    const hostValue = node.host?.trim();
    if (hostValue) {
        return hostValue.startsWith('http')
            ? hostValue
            : `https://${hostValue.includes(':') ? hostValue : `${hostValue}:8006`}`;
    }
    if (node.name) {
        return `https://${node.name.includes(':') ? node.name : `${node.name}:8006`}`;
    }
    return undefined;
};

export const ProxmoxNodesSection: Component<ProxmoxNodesSectionProps> = (props) => {
    // Editing state
    const [editingId, setEditingId] = createSignal<string | null>(null);
    const [editingThresholds, setEditingThresholds] = createSignal<Record<string, number | undefined>>({});
    const [editingNote, setEditingNote] = createSignal('');

    // Transform nodes to resources
    const resources = createMemo(() => {
        const overridesMap = new Map(
            props.overrides
                .filter((o) => o.type === 'node')
                .map((o) => [o.id, o])
        );

        return props.nodes.map((node) => {
            const override = overridesMap.get(node.id);
            return nodeToResource(node, override, props.nodeDefaults);
        });
    });

    // Check if there's an active alert for a resource/metric
    const hasActiveAlert = (resourceId: string, metric: string): boolean => {
        if (!props.activeAlerts) return false;
        const alertKey = `${resourceId}-${metric}`;
        return alertKey in props.activeAlerts;
    };

    // Format threshold value for display
    const formatValue = (metric: string, value: number | undefined): string => {
        if (value === undefined || value === null) return '—';
        if (value <= 0) return 'Off';

        if (metric === 'temperature') return `${value}°C`;
        if (['cpu', 'memory', 'disk'].includes(metric)) return `${value}%`;
        return String(value);
    };

    // Start editing a resource
    const startEditing = (resource: ThresholdResource) => {
        const mergedThresholds: Record<string, number | undefined> = {};
        NODE_COLUMNS.forEach((col) => {
            mergedThresholds[col.key] = resource.thresholds[col.key] ?? resource.defaults[col.key];
        });
        setEditingThresholds(mergedThresholds);
        setEditingNote(resource.note || '');
        setEditingId(resource.id);
    };

    // Save edit
    const saveEdit = (resource: ThresholdResource) => {
        const newThresholds: Record<string, number | undefined> = {};
        const thresholds = editingThresholds();

        // Only save values that differ from defaults
        NODE_COLUMNS.forEach((col) => {
            const value = thresholds[col.key];
            const defaultValue = props.nodeDefaults[col.key];
            if (value !== undefined && value !== defaultValue) {
                newThresholds[col.key] = value;
            }
        });

        const note = editingNote().trim();
        const hasChanges = Object.keys(newThresholds).length > 0 || note;

        // Update overrides
        const existingIndex = props.overrides.findIndex((o) => o.id === resource.id);
        const newOverride = {
            id: resource.id,
            name: resource.name,
            type: 'node',
            thresholds: newThresholds,
            disableConnectivity: resource.disableConnectivity,
            note: note || undefined,
        };

        const newOverrides = [...props.overrides];
        if (hasChanges) {
            if (existingIndex >= 0) {
                newOverrides[existingIndex] = newOverride;
            } else {
                newOverrides.push(newOverride);
            }
        } else if (existingIndex >= 0) {
            // Remove override if no changes
            newOverrides.splice(existingIndex, 1);
        }
        props.setOverrides(newOverrides);

        // Update raw config
        const newRawConfig = { ...props.rawOverridesConfig };
        if (hasChanges) {
            const rawOverride: RawOverrideConfig = {};
            Object.entries(newThresholds).forEach(([key, value]) => {
                if (value !== undefined) {
                    rawOverride[key] = { trigger: value, clear: Math.max(0, value - 5) };
                }
            });
            if (resource.disableConnectivity) {
                rawOverride.disableConnectivity = true;
            }
            if (note) {
                rawOverride.note = note;
            }
            newRawConfig[resource.id] = rawOverride;
        } else {
            delete newRawConfig[resource.id];
        }
        props.setRawOverridesConfig(newRawConfig);

        props.setHasUnsavedChanges(true);
        setEditingId(null);
        setEditingThresholds({});
        setEditingNote('');
    };

    // Cancel edit
    const cancelEdit = () => {
        setEditingId(null);
        setEditingThresholds({});
        setEditingNote('');
    };

    // Toggle connectivity alerts for a node
    const toggleConnectivity = (resource: ThresholdResource) => {
        const newDisableConnectivity = !resource.disableConnectivity;

        const existingIndex = props.overrides.findIndex((o) => o.id === resource.id);
        const existingOverride = existingIndex >= 0 ? props.overrides[existingIndex] : null;

        const newOverride = {
            id: resource.id,
            name: resource.name,
            type: 'node',
            thresholds: existingOverride?.thresholds || {},
            disableConnectivity: newDisableConnectivity,
            note: existingOverride?.note,
        };

        const newOverrides = [...props.overrides];
        if (existingIndex >= 0) {
            newOverrides[existingIndex] = newOverride;
        } else {
            newOverrides.push(newOverride);
        }
        props.setOverrides(newOverrides);

        // Update raw config
        const newRawConfig = { ...props.rawOverridesConfig };
        const existing = newRawConfig[resource.id] || {};
        if (newDisableConnectivity) {
            existing.disableConnectivity = true;
        } else {
            delete existing.disableConnectivity;
        }
        if (Object.keys(existing).length > 0) {
            newRawConfig[resource.id] = existing;
        } else {
            delete newRawConfig[resource.id];
        }
        props.setRawOverridesConfig(newRawConfig);

        props.setHasUnsavedChanges(true);
    };

    // Remove override for a resource
    const removeOverride = (resourceId: string) => {
        props.setOverrides(props.overrides.filter((o) => o.id !== resourceId));
        const newRawConfig = { ...props.rawOverridesConfig };
        delete newRawConfig[resourceId];
        props.setRawOverridesConfig(newRawConfig);
        props.setHasUnsavedChanges(true);
    };

    return (
        <CollapsibleSection
            id="nodes"
            title="Proxmox Nodes"
            resourceCount={props.nodes.length}
            collapsed={props.isCollapsed}
            onToggle={props.onToggleCollapse}
            icon={<Server class="w-5 h-5" />}
            isGloballyDisabled={props.disableAllNodes}
            emptyMessage="No Proxmox nodes found."
        >
            <div class="space-y-4">
                {/* Global Defaults */}
                <GlobalDefaultsRow
                    defaults={props.nodeDefaults}
                    factoryDefaults={props.factoryNodeDefaults}
                    columns={NODE_COLUMNS}
                    onUpdateDefaults={props.setNodeDefaults}
                    setHasUnsavedChanges={props.setHasUnsavedChanges}
                    onResetDefaults={props.resetNodeDefaults}
                    globalDisabled={props.disableAllNodes}
                    onToggleGlobalDisabled={() => props.setDisableAllNodes(!props.disableAllNodes)}
                    showOfflineSettings={true}
                    disableConnectivity={props.disableAllNodesOffline}
                    onSetOfflineState={(state) => {
                        if (state === 'off') {
                            props.setDisableAllNodesOffline(true);
                        } else {
                            props.setDisableAllNodesOffline(false);
                        }
                        props.setHasUnsavedChanges(true);
                    }}
                />

                {/* Resource Cards */}
                <div class="space-y-2">
                    <For each={resources()}>
                        {(resource) => (
                            <ResourceCard
                                resource={resource}
                                columns={NODE_COLUMNS}
                                isEditing={editingId() === resource.id}
                                editingThresholds={editingThresholds()}
                                editingNote={editingNote()}
                                onStartEdit={() => startEditing(resource)}
                                onSaveEdit={() => saveEdit(resource)}
                                onCancelEdit={cancelEdit}
                                onUpdateThreshold={(metric, value) => {
                                    setEditingThresholds((prev) => ({ ...prev, [metric]: value }));
                                }}
                                onUpdateNote={setEditingNote}
                                onToggleConnectivity={() => toggleConnectivity(resource)}
                                onRemoveOverride={() => removeOverride(resource.id)}
                                formatValue={formatValue}
                                hasActiveAlert={hasActiveAlert}
                                showOfflineAlerts={true}
                                globalDefaults={props.nodeDefaults}
                            />
                        )}
                    </For>
                </div>
            </div>
        </CollapsibleSection>
    );
};

export default ProxmoxNodesSection;
