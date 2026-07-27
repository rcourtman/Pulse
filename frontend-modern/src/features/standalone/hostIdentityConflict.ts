import type { Resource } from '@/types/resource';

// Duplicate-identity detection for host agents (the host-agent shape of
// #1584). The server flags a host when reports from more than one machine are
// being folded into a single identity, which usually means template-deployed
// machines still sharing /etc/machine-id. The hosts silently overwrite each
// other in Pulse, so the page needs to say so.

export type HostIdentityConflictHost = {
  name: string;
  hostnames: string[];
  reportIps: string[];
};

const cleanList = (values: string[] | undefined): string[] =>
  (values ?? []).map((value) => value.trim()).filter((value) => value.length > 0);

export function collectHostIdentityConflictHosts(hosts: Resource[]): HostIdentityConflictHost[] {
  const conflicts: HostIdentityConflictHost[] = [];
  for (const host of hosts) {
    const conflict = host.agent?.identityConflict;
    if (!conflict) continue;
    conflicts.push({
      name: host.name?.trim() || host.id || 'host',
      hostnames: cleanList(conflict.hostnames),
      reportIps: cleanList(conflict.reportIps),
    });
  }
  return conflicts;
}
