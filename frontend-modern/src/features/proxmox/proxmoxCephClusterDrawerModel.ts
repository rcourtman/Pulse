import {
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailRow,
  type DetailSection,
  type DetailValueTone,
} from '@/components/shared/detailSectionModel';
import {
  formatPlatformTableIntegerValue,
  formatPlatformTablePercentValue,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource, ResourceCephPoolMeta, ResourceCephServiceMeta } from '@/types/resource';
import { formatBytes } from '@/utils/format';
import { getMetricColorClass, getMetricSeverity } from '@/utils/metricThresholds';
import { asTrimmedString } from '@/utils/stringUtils';

// What a Ceph cluster drawer shows, kept out of the component so the
// "is this OK, and if not, what do I do" judgement is testable on its own:
// the problems lifted into "Worth checking", the pools and daemons that stay
// in view, a short facts line, and the long tail behind Details.

const isHealthOk = (status: string | undefined): boolean => {
  const raw = (status ?? '').trim().toUpperCase();
  return raw === '' || raw === 'HEALTH_OK' || raw === 'OK';
};

const healthTone = (value: string | undefined): DetailValueTone =>
  /ERR|CRIT/i.test(value ?? '') ? 'danger' : 'warning';

const poolLabel = (pool: ResourceCephPoolMeta): string => asTrimmedString(pool.name) || 'Pool';

const poolUsedTone = (pool: ResourceCephPoolMeta): DetailValueTone | undefined => {
  const severity = getMetricSeverity(pool.percentUsed, 'disk');
  if (severity === 'critical') return 'danger';
  if (severity === 'warning') return 'warning';
  return undefined;
};

const serviceLabel = (service: ResourceCephServiceMeta): string =>
  (asTrimmedString(service.type) || 'service').toUpperCase();

const serviceTone = (service: ResourceCephServiceMeta): DetailValueTone => {
  if (service.total === 0) return 'muted';
  if (service.running >= service.total) return 'success';
  return service.running === 0 ? 'danger' : 'warning';
};

const serviceValue = (service: ResourceCephServiceMeta): string =>
  service.total === 0 ? 'None reported' : `${service.running} of ${service.total} running`;

const osdSummary = (resource: Resource): string | null => {
  const ceph = resource.ceph;
  if (!ceph || !(ceph.numOsds > 0)) return null;
  return `${ceph.numOsdsUp} of ${ceph.numOsds} up, ${ceph.numOsdsIn} in`;
};

const capacitySummary = (resource: Resource): string | null => {
  const percent = resource.disk?.current;
  const total = resource.disk?.total ?? 0;
  if (typeof percent !== 'number' || !Number.isFinite(percent)) return null;
  if (total > 0) {
    return `${formatPlatformTablePercentValue(percent)} used · ${formatBytes(resource.disk?.used ?? 0)} of ${formatBytes(total)}`;
  }
  return `${formatPlatformTablePercentValue(percent)} used`;
};

/**
 * Problems a reader should act on, one line each. A row whose fact an open
 * alert already states (`statedMessages`) stays out, so the alert is not
 * repeated underneath itself.
 */
export const getCephClusterExceptionRows = (
  resource: Resource,
  statedMessages: readonly string[] = [],
): DetailRow[] => {
  const ceph = resource.ceph;
  if (!ceph) return [];
  const stated = statedMessages.map((message) => message.toLowerCase());
  const isStated = (value: string) =>
    stated.some((message) => message.includes(value.trim().toLowerCase()));

  const healthRows: Array<DetailRow | null> = [];
  const checks = (ceph.healthChecks ?? []).filter(
    (check) => asTrimmedString(check.summary) || asTrimmedString(check.code),
  );
  if (checks.length > 0) {
    for (const check of checks) {
      healthRows.push(
        makeDetailRow('Health', asTrimmedString(check.summary) || check.code, {
          tone: healthTone(check.severity || ceph.healthStatus),
          title: asTrimmedString(check.code),
          wrap: true,
        }),
      );
    }
  } else if (!isHealthOk(ceph.healthStatus)) {
    healthRows.push(
      makeDetailRow('Health', asTrimmedString(ceph.healthMessage) || ceph.healthStatus, {
        tone: healthTone(ceph.healthStatus),
        wrap: true,
      }),
    );
  }

  const osds = osdSummary(resource);
  const osdRow =
    osds && (ceph.numOsdsUp < ceph.numOsds || ceph.numOsdsIn < ceph.numOsds)
      ? makeDetailRow('OSDs', osds, { tone: ceph.numOsdsUp === 0 ? 'danger' : 'warning' })
      : null;

  const serviceRows = (ceph.services ?? [])
    .filter((service) => service.total > 0 && service.running < service.total)
    .map((service) =>
      makeDetailRow(`${serviceLabel(service)} daemons`, serviceValue(service), {
        tone: serviceTone(service),
      }),
    );

  const poolRows = (ceph.pools ?? []).map((pool) => {
    const tone = poolUsedTone(pool);
    return tone
      ? makeDetailRow(
          `Pool ${poolLabel(pool)}`,
          `${formatPlatformTablePercentValue(pool.percentUsed)} used`,
          { tone, wrap: true },
        )
      : null;
  });

  return compactDetailRows([...healthRows, osdRow, ...serviceRows, ...poolRows]).filter(
    (row) => !isStated(row.value),
  );
};

/** The pools and daemons that stay in view: the cluster's own content. */
export const buildCephClusterContentSections = (resource: Resource): DetailSection[] => {
  const pools = resource.ceph?.pools ?? [];
  const services = resource.ceph?.services ?? [];
  return compactDetailSections([
    {
      label: `Pools (${pools.length})`,
      testId: 'ceph-cluster-pools',
      rows: pools.map((pool) => {
        const percent = Math.max(0, Math.min(100, pool.percentUsed));
        return {
          // Pool names are compared and copied, so they sit above their
          // figures and wrap rather than truncate.
          label: poolLabel(pool),
          layout: 'stacked' as const,
          wrap: true,
          tone: poolUsedTone(pool),
          value: [
            `${formatPlatformTablePercentValue(pool.percentUsed)} used`,
            `${formatBytes(pool.storedBytes)} stored`,
            `${formatBytes(pool.availableBytes)} available`,
            `${formatPlatformTableIntegerValue(pool.objects, '0')} objects`,
          ].join(' · '),
          progress: {
            value: percent,
            fillClass: getMetricColorClass(percent, 'disk'),
            ariaLabel: `${poolLabel(pool)} used`,
          },
        };
      }),
    },
    {
      label: 'Services',
      testId: 'ceph-cluster-services',
      rows: services.map((service) => ({
        label: serviceLabel(service),
        value: serviceValue(service),
        tone: serviceTone(service),
      })),
    },
  ]);
};

/** One short line of what the row does not say in full. */
export const getCephClusterFacts = (resource: Resource): string[] => {
  const ceph = resource.ceph;
  const osds = osdSummary(resource);
  return [
    capacitySummary(resource) ?? '',
    osds ? `OSDs ${osds}` : '',
    ceph ? `MON ${ceph.numMons} · MGR ${ceph.numMgrs}` : '',
    ceph && ceph.numPGs > 0
      ? `${formatPlatformTableIntegerValue(ceph.numPGs)} placement groups`
      : '',
  ].filter((fact) => fact.length > 0);
};

/** The long tail: identifiers and raw health text, behind a closed Details. */
export const buildCephClusterDetailSections = (resource: Resource): DetailSection[] => {
  const ceph = resource.ceph;
  const healthChecks = (ceph?.healthChecks ?? [])
    .map((check) =>
      [asTrimmedString(check.code), asTrimmedString(check.summary)].filter(Boolean).join(': '),
    )
    .filter(Boolean);
  const poolHealth = ceph?.poolHealth;
  const tags = (resource.tags ?? []).map((tag) => asTrimmedString(tag)).filter(Boolean);
  return compactDetailSections([
    {
      label: 'Cluster',
      testId: 'ceph-cluster-identity',
      rows: compactDetailRows([
        makeDetailRow('Name', asTrimmedString(resource.name) || resource.id, { wrap: true }),
        makeDetailRow('FSID', asTrimmedString(ceph?.fsid), { wrap: true }),
        makeDetailRow('Platform', asTrimmedString(resource.platformId), { wrap: true }),
        makeDetailRow('Capacity', capacitySummary(resource)),
        makeDetailRow('Tags', tags.join(', '), { wrap: true }),
      ]),
    },
    {
      label: 'Health',
      testId: 'ceph-cluster-health',
      rows: compactDetailRows([
        makeDetailRow('Status', asTrimmedString(ceph?.healthStatus)),
        makeDetailRow('Message', asTrimmedString(ceph?.healthMessage), { wrap: true }),
        makeDetailRow('Checks', healthChecks.join(' · '), { wrap: true }),
        makeDetailRow('Pool health', asTrimmedString(poolHealth?.summary), { wrap: true }),
        makeDetailRow('Recommendation', asTrimmedString(poolHealth?.recommendation), {
          wrap: true,
        }),
      ]),
    },
    {
      label: 'Topology',
      testId: 'ceph-cluster-topology',
      rows: compactDetailRows([
        makeDetailRow('Monitors', ceph ? String(ceph.numMons) : null),
        makeDetailRow('Managers', ceph ? String(ceph.numMgrs) : null),
        makeDetailRow('OSDs', osdSummary(resource)),
        makeDetailRow(
          'Placement groups',
          ceph && ceph.numPGs > 0 ? formatPlatformTableIntegerValue(ceph.numPGs) : null,
        ),
      ]),
    },
  ]);
};
