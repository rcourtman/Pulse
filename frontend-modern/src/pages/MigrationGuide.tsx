import { For } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/shared/Table';
import { PageHeader } from '@/components/shared/PageHeader';
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
 <PageHeader
 title="Navigation Migration Guide"
 description={(
 <>
 Pulse now uses a resource-first layout: Infrastructure, Workloads, Storage, and Recovery.
 Legacy URLs still redirect for compatibility, but this guide shows the canonical destinations.
 </>
 )}
 />
 <div class="mt-3 text-xs text-muted space-y-1">
 <div class="font-medium text-base-content">Why change?</div>
 <div>
 Unified resources enable one inventory, one search, and consistent filters across Proxmox, agents, Docker, Kubernetes, and new sources.
 The goal is fewer duplicated pages and a navigation model that scales as integrations expand.
 </div>
 </div>
 <p class="mt-2 text-xs text-amber-700 dark:text-amber-300">
 Deprecation policy: legacy URLs exist as compatibility aliases. Update bookmarks to canonical routes.
 Tip: use the Command Palette (<span class="font-mono">Cmd+K</span>) to jump to the new destinations by typing what you remember.
 </p>
 </Card>

 <Card padding="none" class="overflow-hidden">
 <Table class="w-full border-collapse">
 <TableHeader>
 <TableRow class="bg-surface-alt border-b border-border">
 <TableHead class="px-4 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">Legacy route</TableHead>
 <TableHead class="px-4 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">New destination</TableHead>
 <TableHead class="px-4 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">Reason</TableHead>
 <TableHead class="px-4 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">Status</TableHead>
 </TableRow>
 </TableHeader>
 <TableBody class="bg-surface divide-y divide-border-subtle">
 <For each={ROUTE_MAPPINGS}>
 {(item) => (
 <TableRow>
 <TableCell class="px-4 py-2 text-sm font-mono text-base-content">{item.legacy}</TableCell>
 <TableCell class="px-4 py-2 text-sm font-mono text-blue-700 dark:text-blue-300">{item.destination}</TableCell>
 <TableCell class="px-4 py-2 text-sm text-muted">{item.rationale}</TableCell>
 <TableCell class="px-4 py-2 text-sm text-muted">{item.deprecation}</TableCell>
 </TableRow>
 )}
 </For>
 </TableBody>
 </Table>
 </Card>
 </div>
 );
}

export default MigrationGuide;
