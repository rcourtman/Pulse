/**
 * Alert Thresholds - Shared Types
 *
 * This module defines the core types used across the thresholds components.
 * Centralizing types here ensures consistency and makes refactoring easier.
 */

import type { Alert } from '@/types/api';
// Alert types imported from shared alert types

// ============================================================================
// Resource Types
// ============================================================================

/**
 * The type of resource being configured
 */
export type ResourceType =
    | 'node'
    | 'guest'
    | 'storage'
    | 'pbs'
    | 'pmg'
    | 'hostAgent'
    | 'hostDisk'
    | 'dockerHost'
    | 'dockerContainer';

/**
 * Severity level for powered-off/offline alerts
 */
export type OfflineAlertSeverity = 'off' | 'warning' | 'critical';

/**
 * A resource that can have threshold overrides
 */
export interface ThresholdResource {
    id: string;
    name: string;
    displayName?: string;
    rawName?: string;
    type: ResourceType;
    resourceType?: string; // Human-readable: "VM", "CT", "Node", etc.

    // Hierarchy
    node?: string;
    instance?: string;
    host?: string;
    clusterName?: string;
    isClusterMember?: boolean;

    // Status
    status?: string;
    uptime?: number;
    cpu?: number;
    memory?: number;

    // Override state
    hasOverride: boolean;
    disabled?: boolean;
    disableConnectivity?: boolean;
    poweredOffSeverity?: 'warning' | 'critical';

    // Threshold values
    thresholds: ThresholdValues;
    defaults: ThresholdValues;

    // Additional metadata
    vmid?: number;
    note?: string;
    delaySeconds?: number;

    // For special resources (snapshots, backups)
    editable?: boolean;
    editScope?: 'snapshot' | 'backup';
    isEnabled?: boolean;
    toggleEnabled?: () => void;
    toggleTitleEnabled?: string;
    toggleTitleDisabled?: string;
}

/**
 * Threshold values for a resource
 */
export interface ThresholdValues {
    cpu?: number;
    memory?: number;
    disk?: number;
    diskRead?: number;
    diskWrite?: number;
    networkIn?: number;
    networkOut?: number;
    temperature?: number;
    usage?: number;
    [key: string]: number | undefined;
}

/**
 * Metadata for a group header (e.g., node grouping VMs)
 */
export interface GroupHeaderMeta {
    type?: 'node' | 'default';
    displayName?: string;
    rawName?: string;
    host?: string;
    status?: string;
    clusterName?: string;
    isClusterMember?: boolean;
}

// ============================================================================
// Section Types
// ============================================================================

/**
 * Configuration for a collapsible section
 */
export interface SectionConfig {
    id: string;
    title: string;
    resourceCount: number;
    isCollapsed: boolean;
    hasResources: boolean;
}

/**
 * Available section IDs in the Proxmox tab
 */
export type ProxmoxSectionId =
    | 'nodes'
    | 'pbs'
    | 'guests'
    | 'storage'
    | 'backups'
    | 'snapshots';

/**
 * Column definition for resource tables/grids
 */
export interface ThresholdColumn {
    key: string;
    label: string;
    unit?: string;
    tooltip?: string;
    minWidth?: number;
    hideOnMobile?: boolean;
}

// ============================================================================
// Editing State Types
// ============================================================================

/**
 * State for the currently editing resource
 */
export interface EditingState {
    resourceId: string | null;
    thresholds: Record<string, number | undefined>;
    note: string;
    isDirty: boolean;
}

/**
 * Actions for editing thresholds
 */
export interface EditingActions {
    startEditing: (
        resourceId: string,
        currentThresholds: Record<string, number | undefined>,
        defaults: Record<string, number | undefined>,
        note?: string
    ) => void;
    updateThreshold: (metric: string, value: number | undefined) => void;
    updateNote: (note: string) => void;
    saveEdit: () => void;
    cancelEdit: () => void;
}

// ============================================================================
// Global Defaults Types
// ============================================================================

/**
 * Default threshold configuration for guests
 */
export interface GuestDefaults {
    cpu?: number;
    memory?: number;
    disk?: number;
    diskRead?: number;
    diskWrite?: number;
    networkIn?: number;
    networkOut?: number;
    temperature?: number;
    usage?: number;
    disableConnectivity?: boolean;
    poweredOffSeverity?: 'warning' | 'critical';
}

/**
 * Default threshold configuration for Docker containers
 */
export interface DockerDefaults {
    cpu: number;
    memory: number;
    disk: number;
    restartCount: number;
    restartWindow: number;
    memoryWarnPct: number;
    memoryCriticalPct: number;
    serviceWarnGapPercent: number;
    serviceCriticalGapPercent: number;
}

/**
 * Time-based thresholds (delay before alerting)
 */
export interface TimeThresholds {
    guest: number;
    node: number;
    storage: number;
    pbs: number;
}

// ============================================================================
// Global Disable Flags
// ============================================================================

export interface GlobalDisableFlags {
    // Disable all alerts for resource type
    disableAllNodes: boolean;
    disableAllGuests: boolean;
    disableAllHosts: boolean;
    disableAllStorage: boolean;
    disableAllPBS: boolean;
    disableAllPMG: boolean;
    disableAllDockerHosts: boolean;
    disableAllDockerContainers: boolean;
    disableAllDockerServices: boolean;

    // Disable offline/connectivity alerts
    disableAllNodesOffline: boolean;
    disableAllGuestsOffline: boolean;
    disableAllHostsOffline: boolean;
    disableAllPBSOffline: boolean;
    disableAllPMGOffline: boolean;
    disableAllDockerHostsOffline: boolean;
}

// ============================================================================
// Props Types
// ============================================================================

/**
 * Props passed to the main ThresholdsPage component
 */
export interface ThresholdsPageProps {
    // Active alerts for visual indicators
    activeAlerts?: Record<string, Alert>;
    removeAlerts?: (predicate: (alert: Alert) => boolean) => void;

    // Callback when changes are made
    setHasUnsavedChanges: (value: boolean) => void;
}

/**
 * Props for a collapsible section component
 */
export interface CollapsibleSectionProps {
    id: string;
    title: string;
    resourceCount: number;
    defaultCollapsed?: boolean;
    onToggleCollapse?: (collapsed: boolean) => void;
    children: any; // SolidJS children
    actions?: any; // Header action buttons
    emptyMessage?: string;
}

/**
 * Props for a resource card component
 */
export interface ResourceCardProps {
    resource: ThresholdResource;
    columns: ThresholdColumn[];
    isEditing: boolean;
    editingThresholds: Record<string, number | undefined>;
    onStartEdit: () => void;
    onSaveEdit: () => void;
    onCancelEdit: () => void;
    onUpdateThreshold: (metric: string, value: number | undefined) => void;
    onUpdateNote: (note: string) => void;
    onToggleDisabled?: () => void;
    onToggleConnectivity?: () => void;
    onRemoveOverride?: () => void;
    formatValue: (metric: string, value: number | undefined) => string;
    hasActiveAlert: (resourceId: string, metric: string) => boolean;
}

/**
 * Props for the threshold badge component
 */
export interface ThresholdBadgeProps {
    metric: string;
    value: number | undefined;
    defaultValue?: number;
    isOverridden?: boolean;
    hasAlert?: boolean;
    onClick?: () => void;
    size?: 'sm' | 'md' | 'lg';
}

// ============================================================================
// Constants
// ============================================================================

/**
 * Standard columns for different resource types
 */
export const RESOURCE_COLUMNS: Record<ResourceType, ThresholdColumn[]> = {
    node: [
        { key: 'cpu', label: 'CPU %', unit: '%' },
        { key: 'memory', label: 'Memory %', unit: '%' },
        { key: 'disk', label: 'Disk %', unit: '%' },
        { key: 'temperature', label: 'Temp °C', unit: '°C' },
    ],
    guest: [
        { key: 'cpu', label: 'CPU %', unit: '%' },
        { key: 'memory', label: 'Memory %', unit: '%' },
        { key: 'disk', label: 'Disk %', unit: '%' },
        { key: 'diskRead', label: 'Disk R', unit: 'MB/s', hideOnMobile: true },
        { key: 'diskWrite', label: 'Disk W', unit: 'MB/s', hideOnMobile: true },
        { key: 'networkIn', label: 'Net In', unit: 'MB/s', hideOnMobile: true },
        { key: 'networkOut', label: 'Net Out', unit: 'MB/s', hideOnMobile: true },
    ],
    storage: [
        { key: 'usage', label: 'Usage %', unit: '%' },
    ],
    pbs: [
        { key: 'cpu', label: 'CPU %', unit: '%' },
        { key: 'memory', label: 'Memory %', unit: '%' },
    ],
    pmg: [
        // PMG has different columns - defined in PMG section
    ],
    hostAgent: [
        { key: 'cpu', label: 'CPU %', unit: '%' },
        { key: 'memory', label: 'Memory %', unit: '%' },
        { key: 'disk', label: 'Disk %', unit: '%' },
    ],
    hostDisk: [
        { key: 'disk', label: 'Disk %', unit: '%' },
    ],
    dockerHost: [],
    dockerContainer: [
        { key: 'cpu', label: 'CPU %', unit: '%' },
        { key: 'memory', label: 'Memory %', unit: '%' },
    ],
};

/**
 * Get severity color for a threshold value
 */
export const getThresholdSeverityColor = (
    value: number | undefined,
    metric: string
): 'disabled' | 'conservative' | 'moderate' | 'aggressive' | 'critical' => {
    if (value === undefined || value <= 0) return 'disabled';

    // Temperature has different scale
    if (metric === 'temperature') {
        if (value >= 90) return 'critical';
        if (value >= 80) return 'aggressive';
        if (value >= 70) return 'moderate';
        return 'conservative';
    }

    // Standard percentage metrics
    if (value >= 95) return 'conservative'; // Very permissive
    if (value >= 85) return 'moderate';
    if (value >= 70) return 'aggressive';
    return 'critical'; // Very strict
};

/**
 * CSS classes for severity colors
 */
export const SEVERITY_COLORS = {
    disabled: 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400',
    conservative: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
    moderate: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
    aggressive: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
    critical: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
} as const;
