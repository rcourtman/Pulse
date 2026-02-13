import { apiFetch } from '@/utils/apiClient';
import { readAPIErrorMessage } from './responseUtils';
import type {
    ResourceType,
    ResourceDiscovery,
    DiscoveryListResponse,
    DiscoveryProgress,
    DiscoveryStatus,
    TriggerDiscoveryRequest,
    UpdateNotesRequest,
    DiscoveryInfo,
} from '../types/discovery';

const API_BASE = '/api/discovery';

/**
 * List all discoveries
 */
export async function listDiscoveries(): Promise<DiscoveryListResponse> {
    const response = await apiFetch(API_BASE);
    if (!response.ok) {
        throw new Error(await readAPIErrorMessage(response, 'Failed to list discoveries'));
    }
    return response.json();
}

/**
 * List discoveries by resource type
 */
export async function listDiscoveriesByType(
    resourceType: ResourceType
): Promise<DiscoveryListResponse> {
    const response = await apiFetch(`${API_BASE}/type/${encodeURIComponent(resourceType)}`);
    if (!response.ok) {
        throw new Error(await readAPIErrorMessage(response, `Failed to list discoveries for type ${resourceType}`));
    }
    return response.json();
}

/**
 * List discoveries by host
 */
export async function listDiscoveriesByHost(hostId: string): Promise<DiscoveryListResponse> {
    const response = await apiFetch(`${API_BASE}/host/${encodeURIComponent(hostId)}`);
    if (!response.ok) {
        throw new Error(await readAPIErrorMessage(response, `Failed to list discoveries for host ${hostId}`));
    }
    return response.json();
}

/**
 * Get a specific discovery
 */
export async function getDiscovery(
    resourceType: ResourceType,
    hostId: string,
    resourceId: string
): Promise<ResourceDiscovery | null> {
    if (resourceType === 'host') {
        // Host discovery is frequently absent before first scan. Resolve via list endpoint
        // first to avoid noisy 404s for expected "not discovered yet" states.
        const hostListResponse = await apiFetch(`${API_BASE}/host/${encodeURIComponent(hostId)}`);
        if (!hostListResponse.ok) {
            throw new Error(await readAPIErrorMessage(hostListResponse, 'Failed to list host discoveries'));
        }

        const hostList = await hostListResponse.json() as DiscoveryListResponse;
        if (!hostList.discoveries || hostList.discoveries.length === 0) {
            return null;
        }

        const resolvedHostDiscovery = hostList.discoveries.find(
            (d) =>
                d.resource_type === 'host' &&
                (d.resource_id === resourceId ||
                    d.resource_id === hostId ||
                    d.host_id === hostId)
        ) ?? hostList.discoveries.find((d) => d.resource_type === 'host');

        if (!resolvedHostDiscovery) {
            return null;
        }

        const response = await apiFetch(
            `${API_BASE}/host/${encodeURIComponent(resolvedHostDiscovery.host_id)}/${encodeURIComponent(resolvedHostDiscovery.resource_id)}`
        );
        if (response.status === 404) {
            return null;
        }
        if (!response.ok) {
            throw new Error(await readAPIErrorMessage(response, 'Failed to get discovery'));
        }
        return response.json();
    }

    const response = await apiFetch(
        `${API_BASE}/${encodeURIComponent(resourceType)}/${encodeURIComponent(hostId)}/${encodeURIComponent(resourceId)}`
    );
    if (response.status === 404) {
        return null;
    }
    if (!response.ok) {
        throw new Error(await readAPIErrorMessage(response, 'Failed to get discovery'));
    }
    return response.json();
}

/**
 * Trigger discovery for a resource
 */
export async function triggerDiscovery(
    resourceType: ResourceType,
    hostId: string,
    resourceId: string,
    options?: TriggerDiscoveryRequest
): Promise<ResourceDiscovery> {
    const response = await apiFetch(
        `${API_BASE}/${encodeURIComponent(resourceType)}/${encodeURIComponent(hostId)}/${encodeURIComponent(resourceId)}`,
        {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(options || {}),
        }
    );
    if (!response.ok) {
        throw new Error(await readAPIErrorMessage(response, 'Discovery failed'));
    }
    return response.json();
}

/**
 * Get discovery progress
 */
export async function getDiscoveryProgress(
    resourceType: ResourceType,
    hostId: string,
    resourceId: string
): Promise<DiscoveryProgress> {
    const response = await apiFetch(
        `${API_BASE}/${encodeURIComponent(resourceType)}/${encodeURIComponent(hostId)}/${encodeURIComponent(resourceId)}/progress`
    );
    if (!response.ok) {
        throw new Error(await readAPIErrorMessage(response, 'Failed to get discovery progress'));
    }
    return response.json();
}

/**
 * Update user notes for a discovery
 */
export async function updateDiscoveryNotes(
    resourceType: ResourceType,
    hostId: string,
    resourceId: string,
    notes: UpdateNotesRequest
): Promise<ResourceDiscovery> {
    const response = await apiFetch(
        `${API_BASE}/${encodeURIComponent(resourceType)}/${encodeURIComponent(hostId)}/${encodeURIComponent(resourceId)}/notes`,
        {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(notes),
        }
    );
    if (!response.ok) {
        throw new Error(await readAPIErrorMessage(response, 'Failed to update notes'));
    }
    return response.json();
}

/**
 * Delete a discovery
 */
export async function deleteDiscovery(
    resourceType: ResourceType,
    hostId: string,
    resourceId: string
): Promise<void> {
    const response = await apiFetch(
        `${API_BASE}/${encodeURIComponent(resourceType)}/${encodeURIComponent(hostId)}/${encodeURIComponent(resourceId)}`,
        {
            method: 'DELETE',
        }
    );
    if (!response.ok) {
        throw new Error(await readAPIErrorMessage(response, 'Failed to delete discovery'));
    }
}

/**
 * Get discovery service status
 */
export async function getDiscoveryStatus(): Promise<DiscoveryStatus> {
    const response = await apiFetch(`${API_BASE}/status`);
    if (!response.ok) {
        throw new Error(await readAPIErrorMessage(response, 'Failed to get discovery status'));
    }
    return response.json();
}

/**
 * Get discovery info for a resource type (AI provider info, commands that will run)
 */
export async function getDiscoveryInfo(resourceType: ResourceType): Promise<DiscoveryInfo> {
    const response = await apiFetch(`${API_BASE}/info/${encodeURIComponent(resourceType)}`);
    if (!response.ok) {
        throw new Error(await readAPIErrorMessage(response, 'Failed to get discovery info'));
    }
    return response.json();
}

/**
 * Helper to format the last updated time
 */
export function formatDiscoveryAge(updatedAt: string): string {
    if (!updatedAt) return 'Unknown';

    const updated = new Date(updatedAt);
    const now = new Date();
    const diffMs = now.getTime() - updated.getTime();
    const diffMins = Math.floor(diffMs / (1000 * 60));
    const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

    if (diffMins < 1) return 'Just now';
    if (diffMins === 1) return '1 minute ago';
    if (diffMins < 60) return `${diffMins} minutes ago`;
    if (diffHours === 1) return '1 hour ago';
    if (diffHours < 24) return `${diffHours} hours ago`;
    if (diffDays === 1) return '1 day ago';
    return `${diffDays} days ago`;
}

/**
 * Helper to get a human-readable category name
 */
export function getCategoryDisplayName(category: string): string {
    const names: Record<string, string> = {
        database: 'Database',
        web_server: 'Web Server',
        cache: 'Cache',
        message_queue: 'Message Queue',
        monitoring: 'Monitoring',
        backup: 'Backup',
        nvr: 'NVR',
        storage: 'Storage',
        container: 'Container',
        virtualizer: 'Virtualizer',
        network: 'Network',
        security: 'Security',
        media: 'Media',
        home_automation: 'Home Automation',
        unknown: 'Unknown',
    };
    return names[category] || category;
}

/**
 * Helper to get confidence level description
 */
export function getConfidenceLevel(confidence: number): {
    label: string;
    color: string;
} {
    if (confidence >= 0.9) {
        return { label: 'High confidence', color: 'text-green-600 dark:text-green-400' };
    }
    if (confidence >= 0.7) {
        return { label: 'Medium confidence', color: 'text-amber-600 dark:text-amber-400' };
    }
    return { label: 'Low confidence', color: 'text-gray-500 dark:text-gray-400' };
}

/**
 * Connected agent info from WebSocket
 */
export interface ConnectedAgent {
    agent_id: string;
    hostname: string;
    version: string;
    platform: string;
    connected_at: string;
}

/**
 * Get list of agents connected via WebSocket (for command execution)
 */
export async function getConnectedAgents(): Promise<{ count: number; agents: ConnectedAgent[] }> {
    const response = await apiFetch('/api/ai/agents');
    if (!response.ok) {
        throw new Error(await readAPIErrorMessage(response, 'Failed to get connected agents'));
    }
    return response.json();
}
