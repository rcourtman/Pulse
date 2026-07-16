import type { Resource } from '@/types/resource';

// Duplicate-identity detection for Docker hosts (#1584). The server flags a
// host when reports from more than one machine are being folded into a single
// identity, which usually means cloned VMs still sharing /etc/machine-id. The
// hosts silently overwrite each other in Pulse, so the page needs to say so.

export type IdentityConflictHost = {
  name: string;
  hostnames: string[];
};

export function collectIdentityConflictHosts(hosts: Resource[]): IdentityConflictHost[] {
  const conflicts: IdentityConflictHost[] = [];
  for (const host of hosts) {
    const conflict = host.docker?.identityConflict;
    if (!conflict) continue;
    const hostnames = (conflict.hostnames ?? [])
      .map((hostname) => hostname.trim())
      .filter((hostname) => hostname.length > 0);
    conflicts.push({
      name: host.name?.trim() || host.id || 'host',
      hostnames,
    });
  }
  return conflicts;
}
