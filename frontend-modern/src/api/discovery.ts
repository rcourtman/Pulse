import { apiFetch } from '@/utils/apiClient';
import { assertAPIResponseOK, isAPIResponseStatus, parseRequiredJSON } from './responseUtils';
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
import { toDiscoveryAPIResourceType } from '@/utils/discoveryTarget';

const API_BASE = '/api/discovery';
const resolveAPIResourceType = (resourceType: ResourceType): string =>
  toDiscoveryAPIResourceType(resourceType) || resourceType;
const resolveDiscoveryAgentId = (discovery: {
  target_id?: string;
  agent_id?: string;
  resource_id: string;
}): string => discovery.agent_id || discovery.target_id || discovery.resource_id;
const buildDiscoveryTypePath = (resourceType: ResourceType): string =>
  `${API_BASE}/type/${encodeURIComponent(resolveAPIResourceType(resourceType))}`;
const buildDiscoveryInfoPath = (resourceType: ResourceType): string =>
  `${API_BASE}/info/${encodeURIComponent(resolveAPIResourceType(resourceType))}`;
const buildTypedDiscoveryPath = (
  resourceType: ResourceType,
  targetId: string,
  resourceId: string,
): string =>
  `${API_BASE}/${encodeURIComponent(resolveAPIResourceType(resourceType))}/${encodeURIComponent(targetId)}/${encodeURIComponent(resourceId)}`;
const buildTypedDiscoverySubresourcePath = (
  resourceType: ResourceType,
  targetId: string,
  resourceId: string,
  subresource: string,
): string => `${buildTypedDiscoveryPath(resourceType, targetId, resourceId)}/${subresource}`;
const buildAgentDiscoveryCollectionPath = (agentId: string): string =>
  `${API_BASE}/agent/${encodeURIComponent(agentId)}`;
const buildAgentDiscoveryDetailPath = (agentId: string, resourceId: string): string =>
  `${buildAgentDiscoveryCollectionPath(agentId)}/${encodeURIComponent(resourceId)}`;

/**
 * List all discoveries
 */
export async function listDiscoveries(): Promise<DiscoveryListResponse> {
  const response = await apiFetch(API_BASE);
  await assertAPIResponseOK(response, 'Failed to list discoveries');
  return parseRequiredJSON(response, 'Failed to parse discoveries');
}

/**
 * List discoveries by resource type
 */
export async function listDiscoveriesByType(
  resourceType: ResourceType,
): Promise<DiscoveryListResponse> {
  const response = await apiFetch(buildDiscoveryTypePath(resourceType));
  await assertAPIResponseOK(response, `Failed to list discoveries for type ${resourceType}`);
  return parseRequiredJSON(response, `Failed to parse discoveries for type ${resourceType}`);
}

/**
 * List discoveries by agent
 */
export async function listDiscoveriesByAgent(agentId: string): Promise<DiscoveryListResponse> {
  const response = await apiFetch(buildAgentDiscoveryCollectionPath(agentId));
  await assertAPIResponseOK(response, `Failed to list discoveries for agent ${agentId}`);
  return parseRequiredJSON(response, `Failed to parse discoveries for agent ${agentId}`);
}

/**
 * Get a specific discovery
 */
export async function getDiscovery(
  resourceType: ResourceType,
  targetId: string,
  resourceId: string,
): Promise<ResourceDiscovery | null> {
  if (resourceType === 'agent') {
    // Agent discovery is frequently absent before first scan. Resolve via list endpoint
    // first to avoid noisy 404s for expected "not discovered yet" states.
    const agentListResponse = await apiFetch(buildAgentDiscoveryCollectionPath(targetId));
    await assertAPIResponseOK(agentListResponse, 'Failed to list agent discoveries');

    const agentList = await parseRequiredJSON<DiscoveryListResponse>(
      agentListResponse,
      'Failed to parse agent discoveries',
    );
    if (!agentList.discoveries || agentList.discoveries.length === 0) {
      return null;
    }

    const resolvedAgentDiscovery =
      agentList.discoveries.find(
        (d) =>
          d.resource_type === 'agent' &&
          (d.resource_id === resourceId ||
            d.resource_id === targetId ||
            resolveDiscoveryAgentId(d) === targetId),
      ) ?? agentList.discoveries.find((d) => d.resource_type === 'agent');

    if (!resolvedAgentDiscovery) {
      return null;
    }

    const resolvedAgentId = resolveDiscoveryAgentId(resolvedAgentDiscovery);
    if (!resolvedAgentId) {
      return null;
    }

    const response = await apiFetch(
      buildAgentDiscoveryDetailPath(resolvedAgentId, resolvedAgentDiscovery.resource_id),
    );
    if (isAPIResponseStatus(response, 404)) {
      return null;
    }
    await assertAPIResponseOK(response, 'Failed to get discovery');
    return parseRequiredJSON(response, 'Failed to parse discovery');
  }

  const response = await apiFetch(buildTypedDiscoveryPath(resourceType, targetId, resourceId));
  if (isAPIResponseStatus(response, 404)) {
    return null;
  }
  await assertAPIResponseOK(response, 'Failed to get discovery');
  return parseRequiredJSON(response, 'Failed to parse discovery');
}

/**
 * Trigger discovery for a resource
 */
export async function triggerDiscovery(
  resourceType: ResourceType,
  targetId: string,
  resourceId: string,
  options?: TriggerDiscoveryRequest,
): Promise<ResourceDiscovery> {
  const response = await apiFetch(buildTypedDiscoveryPath(resourceType, targetId, resourceId), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(options || {}),
  });
  await assertAPIResponseOK(response, 'Discovery failed');
  return parseRequiredJSON(response, 'Failed to parse discovery trigger response');
}

/**
 * Get discovery progress
 */
export async function getDiscoveryProgress(
  resourceType: ResourceType,
  targetId: string,
  resourceId: string,
): Promise<DiscoveryProgress> {
  const response = await apiFetch(
    buildTypedDiscoverySubresourcePath(resourceType, targetId, resourceId, 'progress'),
  );
  await assertAPIResponseOK(response, 'Failed to get discovery progress');
  return parseRequiredJSON(response, 'Failed to parse discovery progress');
}

/**
 * Update user notes for a discovery
 */
export async function updateDiscoveryNotes(
  resourceType: ResourceType,
  targetId: string,
  resourceId: string,
  notes: UpdateNotesRequest,
): Promise<ResourceDiscovery> {
  const response = await apiFetch(
    buildTypedDiscoverySubresourcePath(resourceType, targetId, resourceId, 'notes'),
    {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(notes),
    },
  );
  await assertAPIResponseOK(response, 'Failed to update notes');
  return parseRequiredJSON(response, 'Failed to parse updated discovery notes');
}

/**
 * Delete a discovery
 */
export async function deleteDiscovery(
  resourceType: ResourceType,
  targetId: string,
  resourceId: string,
): Promise<void> {
  const response = await apiFetch(buildTypedDiscoveryPath(resourceType, targetId, resourceId), {
    method: 'DELETE',
  });
  await assertAPIResponseOK(response, 'Failed to delete discovery');
}

/**
 * Get discovery service status
 */
export async function getDiscoveryStatus(): Promise<DiscoveryStatus> {
  const response = await apiFetch(`${API_BASE}/status`);
  await assertAPIResponseOK(response, 'Failed to get discovery status');
  return parseRequiredJSON(response, 'Failed to parse discovery status');
}

/**
 * Get discovery info for a resource type (AI provider info, commands that will run)
 */
export async function getDiscoveryInfo(resourceType: ResourceType): Promise<DiscoveryInfo> {
  const response = await apiFetch(buildDiscoveryInfoPath(resourceType));
  await assertAPIResponseOK(response, 'Failed to get discovery info');
  return parseRequiredJSON(response, 'Failed to parse discovery info');
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
  return { label: 'Low confidence', color: 'text-muted' };
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
  await assertAPIResponseOK(response, 'Failed to get connected agents');
  return parseRequiredJSON(response, 'Failed to parse connected agents');
}
