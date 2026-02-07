import { For } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { LEGACY_REDIRECTS } from '@/routing/legacyRedirects';
import { LEGACY_ROUTE_MIGRATION_METADATA } from '@/routing/legacyRouteMetadata';

type RouteMapping = {
  legacy: string;
  destination: string;
  rationale: string;
  deprecation: string;
};

const ROUTE_MAPPINGS: RouteMapping[] = [
  {
    legacy: LEGACY_REDIRECTS.proxmoxOverview.path,
    destination: LEGACY_REDIRECTS.proxmoxOverview.destination,
    rationale: LEGACY_ROUTE_MIGRATION_METADATA['proxmox-overview'].rationale,
    deprecation: LEGACY_ROUTE_MIGRATION_METADATA['proxmox-overview'].status,
  },
  {
    legacy: LEGACY_REDIRECTS.hosts.path,
    destination: LEGACY_REDIRECTS.hosts.destination,
    rationale: LEGACY_ROUTE_MIGRATION_METADATA.hosts.rationale,
    deprecation: LEGACY_ROUTE_MIGRATION_METADATA.hosts.status,
  },
  {
    legacy: LEGACY_REDIRECTS.docker.path,
    destination: LEGACY_REDIRECTS.docker.destination,
    rationale: LEGACY_ROUTE_MIGRATION_METADATA.docker.rationale,
    deprecation: LEGACY_ROUTE_MIGRATION_METADATA.docker.status,
  },
  {
    legacy: LEGACY_REDIRECTS.mail.path,
    destination: LEGACY_REDIRECTS.mail.destination,
    rationale: LEGACY_ROUTE_MIGRATION_METADATA.mail.rationale,
    deprecation: LEGACY_ROUTE_MIGRATION_METADATA.mail.status,
  },
  {
    legacy: LEGACY_REDIRECTS.services.path,
    destination: LEGACY_REDIRECTS.services.destination,
    rationale: LEGACY_ROUTE_MIGRATION_METADATA.services.rationale,
    deprecation: LEGACY_ROUTE_MIGRATION_METADATA.services.status,
  },
  {
    legacy: LEGACY_REDIRECTS.kubernetes.path,
    destination: LEGACY_REDIRECTS.kubernetes.destination,
    rationale: LEGACY_ROUTE_MIGRATION_METADATA.kubernetes.rationale,
    deprecation: LEGACY_ROUTE_MIGRATION_METADATA.kubernetes.status,
  },
];

export function MigrationGuide() {
  return (
    <div class="space-y-4">
      <Card class="p-5">
        <h1 class="text-base font-semibold text-gray-900 dark:text-gray-100">Navigation Migration Guide</h1>
        <p class="mt-2 text-sm text-gray-600 dark:text-gray-300">
          Pulse now uses a resource-first layout: Infrastructure, Workloads, Storage, and Backups.
          Legacy URLs still redirect for compatibility, but this guide shows the canonical destinations.
        </p>
        <p class="mt-2 text-xs text-amber-700 dark:text-amber-300">
          Deprecation policy: legacy redirects are transitional and are planned for removal after the migration window.
        </p>
      </Card>

      <Card padding="none" class="overflow-hidden">
        <table class="w-full border-collapse">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-700/50 border-b border-gray-200 dark:border-gray-700">
              <th class="px-4 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-300">Legacy route</th>
              <th class="px-4 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-300">New destination</th>
              <th class="px-4 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-300">Reason</th>
              <th class="px-4 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-300">Status</th>
            </tr>
          </thead>
          <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-100 dark:divide-gray-700/70">
            <For each={ROUTE_MAPPINGS}>
              {(item) => (
                <tr>
                  <td class="px-4 py-2 text-sm font-mono text-gray-700 dark:text-gray-200">{item.legacy}</td>
                  <td class="px-4 py-2 text-sm font-mono text-blue-700 dark:text-blue-300">{item.destination}</td>
                  <td class="px-4 py-2 text-sm text-gray-600 dark:text-gray-300">{item.rationale}</td>
                  <td class="px-4 py-2 text-sm text-gray-600 dark:text-gray-300">{item.deprecation}</td>
                </tr>
              )}
            </For>
          </tbody>
        </table>
      </Card>
    </div>
  );
}

export default MigrationGuide;
