import type { ResourceType } from '../../types/discovery';

/**
 * Derives a concrete, human-runnable command for reaching a workload from its
 * resource coordinates.
 *
 * The discovery `cli_access` field is guidance written for the Pulse Assistant
 * ("Use pulse_control with target_host …") — it is not something a person types.
 * This returns what a human would actually run (e.g. `pct exec 101 -- bash`),
 * including the nested-container layer when the service runs in Docker inside an
 * LXC/VM. Returns `null` when there is no clean human command for the type (the
 * caller then falls back to showing the guidance text, e.g. k8s kubectl which is
 * already human-readable, or VMs where SSH/credentials are the real path).
 */
export function deriveCliCommand(
  resourceType: ResourceType,
  resourceId: string,
  cliAccess: string | undefined,
): string | null {
  const id = (resourceId || '').trim();
  if (!id) return null;

  // A service running in a nested Docker container changes the access path; the
  // backend records the container in cli_access as "docker exec <name> …".
  const nested = /docker exec (\S+)/.exec(cliAccess || '')?.[1];

  switch (resourceType) {
    case 'system-container':
      return nested ? `pct exec ${id} -- docker exec ${nested} bash` : `pct exec ${id} -- bash`;
    case 'app-container':
      return `docker exec ${id} bash`;
    default:
      // vm (qm guest exec is non-interactive; SSH is the real path), pod (the
      // guidance already shows a runnable kubectl exec), agent, etc.
      return null;
  }
}
