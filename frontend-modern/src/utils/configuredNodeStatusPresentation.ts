import { unwrap } from 'solid-js/store';
import type { Resource } from '@/types/resource';
import type { NodeConfigWithStatus } from '@/types/nodes';
import {
  getSimpleStatusIndicator,
  type StatusIndicator,
} from '@/utils/status';

export function resolveConfiguredNodeStatusIndicator(options: {
  configuredStatus?: string | null;
  liveStatus?: string | null;
  connectionHealth?: string | null;
}): StatusIndicator {
  if (
    options.connectionHealth === 'unhealthy' ||
    options.connectionHealth === 'error' ||
    options.liveStatus === 'offline' ||
    options.liveStatus === 'disconnected'
  ) {
    return getSimpleStatusIndicator('offline');
  }
  if (options.connectionHealth === 'degraded') {
    return getSimpleStatusIndicator('degraded');
  }
  if (options.liveStatus === 'online' || options.connectionHealth === 'healthy') {
    return getSimpleStatusIndicator('online');
  }

  switch (options.configuredStatus) {
    case 'connected':
      return getSimpleStatusIndicator('online');
    case 'pending':
      return getSimpleStatusIndicator('pending');
    case 'disconnected':
    case 'offline':
    case 'error':
      return getSimpleStatusIndicator('offline');
    default:
      return getSimpleStatusIndicator('unknown');
  }
}

export function resolveConfiguredPveNodeStatusIndicator(
  node: NodeConfigWithStatus,
  stateNodes: Resource[],
): StatusIndicator {
  const stateNode = stateNodes.find((n) => n.platformId === node.name || n.name === node.name);
  const platformData = stateNode?.platformData
    ? (unwrap(stateNode.platformData) as Record<string, unknown>)
    : undefined;

  return resolveConfiguredNodeStatusIndicator({
    configuredStatus: node.status,
    liveStatus: stateNode?.status,
    connectionHealth: platformData?.connectionHealth as string | undefined,
  });
}

export function resolveConfiguredInstanceStatusIndicator(
  node: Pick<NodeConfigWithStatus, 'status' | 'name'>,
  instances: Array<{
    name?: string | null;
    status?: string | null;
    connectionHealth?: string | null;
  }>,
): StatusIndicator {
  const instance = instances.find((item) => item.name === node.name);
  return resolveConfiguredNodeStatusIndicator({
    configuredStatus: node.status,
    liveStatus: instance?.status,
    connectionHealth: instance?.connectionHealth,
  });
}
