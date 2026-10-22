import { For, Show, createMemo, createResource, type Component } from 'solid-js';
import {
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
} from '@/components/shared/DetailSectionTable';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { TechnicalDetailsSection } from '@/components/shared/TechnicalDetailsDisclosure';
import {
  PlatformDetailTable,
  PlatformDetailTableBody,
  PlatformDetailTableHeader,
  formatPlatformTableIntegerValue,
  formatPlatformTablePercentValue,
  formatPlatformTableUptimeValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformResponsiveTableLabel,
  PlatformTableNumberValue,
} from '@/features/platformPage/sharedPlatformPage';
import type { StatusIndicatorVariant } from '@/utils/status';
import { formatBytes } from '@/utils/format';
import { asTrimmedString } from '@/utils/stringUtils';
import { apiFetch } from '@/utils/apiClient';
import type {
  PMGInstance,
  PMGMailStats,
  PMGNodeStatus,
  PMGQuarantineTotals,
  PMGQueueStatus,
  PMGRelayDomain,
  PMGSpamBucket,
} from '@/types/api';
import type { Resource } from '@/types/resource';

// The Mail tab of a Mail Gateway's resource drawer: mail flow, quarantine,
// spam scores, per-node queues and the busiest domains. The drawer around it
// owns the header, status, alerts and history, so this body is headless. It
// fetches the full PMGInstance when the tab opens so the row keeps its slim
// payload.

interface PMGInstancesResponse {
  data: PMGInstance[];
  meta: { total: number };
}

async function fetchPMGInstance(id: string): Promise<PMGInstance | null> {
  const response = await apiFetch(`/api/pmg/instances?id=${encodeURIComponent(id)}`);
  if (!response.ok) {
    throw new Error(`Failed to load PMG instance (${response.status})`);
  }
  const payload = (await response.json()) as PMGInstancesResponse;
  return payload?.data?.[0] ?? null;
}

function classifyNode(node: PMGNodeStatus): { variant: StatusIndicatorVariant; label: string } {
  const raw = (node.status ?? '').toLowerCase();
  if (raw === 'online' || raw === 'ok') return { variant: 'success', label: 'Online' };
  if (raw === 'degraded' || raw === 'warning') return { variant: 'warning', label: 'Degraded' };
  if (raw === 'offline' || raw === 'down') return { variant: 'danger', label: 'Offline' };
  return { variant: 'muted', label: raw || '—' };
}

function formatAge(seconds: number): string {
  if (!seconds || seconds <= 0) return '—';
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
  if (seconds < 86_400) return `${Math.floor(seconds / 3600)}h`;
  return `${Math.floor(seconds / 86_400)}d`;
}

function queueCell(queue: PMGQueueStatus | undefined): { count: number; label: string } {
  if (!queue) return { count: 0, label: '—' };
  return {
    count: queue.total,
    label: `${queue.active}/${queue.deferred}/${queue.hold}/${queue.incoming}`,
  };
}

const inOut = (inValue: number | undefined, outValue: number | undefined): string =>
  `${formatPlatformTableIntegerValue(inValue ?? 0, '0')} / ${formatPlatformTableIntegerValue(outValue ?? 0, '0')}`;

const bytesOrDash = (value: number | undefined): string =>
  value && value > 0 ? formatBytes(value) : '—';

// StackedBar renders a horizontal stacked bar from labelled segments.
// Each segment carries a tone (color), value, and label; segments with
// 0 value collapse out. An optional legend with name + count + share
// renders below.
interface StackedSegment {
  key: string;
  label: string;
  value: number;
  tone: string;
}

function StackedBar(props: { segments: StackedSegment[]; ariaLabel: string; legend?: boolean }) {
  const total = createMemo(() => props.segments.reduce((sum, s) => sum + Math.max(0, s.value), 0));
  const visible = createMemo(() => props.segments.filter((s) => s.value > 0));
  return (
    <div class="space-y-2">
      <Show
        when={total() > 0}
        fallback={<div class="relative h-2.5 w-full overflow-hidden rounded-full bg-surface-alt" />}
      >
        <div
          class="relative flex h-2.5 w-full overflow-hidden rounded-full bg-surface-alt"
          role="img"
          aria-label={props.ariaLabel}
        >
          <For each={visible()}>
            {(seg) => (
              <div
                class={seg.tone}
                style={{ width: `${(seg.value / total()) * 100}%` }}
                title={`${seg.label}: ${formatPlatformTableIntegerValue(seg.value)}`}
              />
            )}
          </For>
        </div>
      </Show>
      <Show when={props.legend !== false}>
        <div class="flex flex-wrap gap-x-3 gap-y-1 text-[11px]">
          <For each={props.segments}>
            {(seg) => {
              const share = total() > 0 ? (seg.value / total()) * 100 : 0;
              return (
                <div class="flex items-center gap-1.5">
                  <span class={`inline-block h-2 w-2 rounded-xs ${seg.tone}`} aria-hidden="true" />
                  <span class="text-muted">{seg.label}</span>
                  <span class="text-base-content font-semibold tabular-nums">
                    {formatPlatformTableIntegerValue(seg.value)}
                  </span>
                  <Show when={total() > 0}>
                    <span class="text-muted tabular-nums text-[10px]">
                      {formatPlatformTablePercentValue(share)}
                    </span>
                  </Show>
                </div>
              );
            }}
          </For>
        </div>
      </Show>
    </div>
  );
}

// SpamHistogram renders a small bar chart of count per spam-score
// bucket. The bucket label (eg "0", "1-3", "10+") sits below each bar;
// the count sits above on hover via title. Tones use the canonical
// metric severity palette: 0 = clean = normal, mid-score = warning,
// high-score = critical.
function SpamHistogram(props: { buckets: PMGSpamBucket[] }) {
  const maxCount = createMemo(() => Math.max(0, ...props.buckets.map((b) => Math.max(0, b.count))));
  return (
    <div class="flex items-end gap-1 h-16" role="img" aria-label="Spam score distribution">
      <For each={props.buckets}>
        {(bucket) => {
          const heightPct = maxCount() > 0 ? (bucket.count / maxCount()) * 100 : 0;
          const tone =
            bucket.score === '0' || bucket.score.startsWith('0')
              ? 'bg-metric-normal-bg'
              : Number(bucket.score) >= 10 || bucket.score.includes('10')
                ? 'bg-metric-critical-bg'
                : Number(bucket.score) >= 5 || bucket.score.includes('5')
                  ? 'bg-metric-warning-bg'
                  : 'bg-blue-500/60';
          return (
            <div class="flex flex-1 min-w-[20px] flex-col items-center gap-1">
              <div
                class={`w-full rounded-xs ${tone}`}
                style={{ height: `${Math.max(heightPct, 2)}%` }}
                title={`Score ${bucket.score}: ${formatPlatformTableIntegerValue(bucket.count)}`}
              />
              <span class="text-[10px] text-muted font-mono tabular-nums">{bucket.score}</span>
            </div>
          );
        }}
      </For>
    </div>
  );
}

const inboundSegments = (stats: PMGMailStats): StackedSegment[] => {
  const countIn = Math.max(0, stats.countIn ?? 0);
  const spam = Math.max(0, stats.spamIn ?? 0);
  const virus = Math.max(0, stats.virusIn ?? 0);
  const bounces = Math.max(0, stats.bouncesIn ?? 0);
  const greylist = Math.max(0, stats.greylistCount ?? 0);
  const rbl = Math.max(0, stats.rblRejects ?? 0);
  const pregreet = Math.max(0, stats.pregreetRejects ?? 0);
  const clean = Math.max(0, countIn - (spam + virus + bounces));
  return [
    { key: 'clean', label: 'Clean', value: clean, tone: 'bg-metric-normal-bg' },
    { key: 'spam', label: 'Spam', value: spam, tone: 'bg-metric-warning-bg' },
    { key: 'virus', label: 'Virus', value: virus, tone: 'bg-metric-critical-bg' },
    { key: 'bounces', label: 'Bounces', value: bounces, tone: 'bg-orange-500/60' },
    { key: 'greylist', label: 'Greylist', value: greylist, tone: 'bg-blue-500/60' },
    { key: 'rbl', label: 'RBL', value: rbl, tone: 'bg-purple-500/60' },
    { key: 'pregreet', label: 'Pregreet', value: pregreet, tone: 'bg-slate-500/60' },
  ];
};

const quarantineSegments = (quarantine: PMGQuarantineTotals): StackedSegment[] => [
  { key: 'spam', label: 'Spam', value: quarantine.spam ?? 0, tone: 'bg-metric-warning-bg' },
  { key: 'virus', label: 'Virus', value: quarantine.virus ?? 0, tone: 'bg-metric-critical-bg' },
  {
    key: 'attachment',
    label: 'Attachment',
    value: quarantine.attachment ?? 0,
    tone: 'bg-orange-500/60',
  },
  {
    key: 'blacklisted',
    label: 'Blacklisted',
    value: quarantine.blacklisted ?? 0,
    tone: 'bg-slate-500/60',
  },
];

const relayDomainLabel = (domain: PMGRelayDomain): string => {
  const comment = asTrimmedString(domain.comment);
  return comment ? `${domain.domain} (${comment})` : domain.domain;
};

// The mail-flow content cards: each figure is a detail row, and the two
// pictures (inbound disposition, spam scores) sit under their card's rows.
export const buildMailGatewayMailFlowSections = (instance: PMGInstance): DetailSection[] => {
  const stats = instance.mailStats;
  const quarantine = instance.quarantine;
  const spamBuckets = instance.spamDistribution ?? [];
  const relayDomains = (instance.relayDomains ?? []).filter((domain) =>
    asTrimmedString(domain.domain),
  );
  const timeframe = asTrimmedString(stats?.timeframe);
  return compactDetailSections([
    stats
      ? {
          label: timeframe ? `Mail flow (${timeframe})` : 'Mail flow',
          testId: 'pmg-mail-flow-section',
          rows: compactDetailRows([
            makeDetailRow('Mail in / out', inOut(stats.countIn, stats.countOut)),
            makeDetailRow('Spam in / out', inOut(stats.spamIn, stats.spamOut)),
            makeDetailRow('Virus in / out', inOut(stats.virusIn, stats.virusOut)),
            makeDetailRow('Bounces in / out', inOut(stats.bouncesIn, stats.bouncesOut)),
            makeDetailRow(
              'Bytes in / out',
              `${bytesOrDash(stats.bytesIn)} / ${bytesOrDash(stats.bytesOut)}`,
            ),
            makeDetailRow('Greylisted', formatPlatformTableIntegerValue(stats.greylistCount, '0')),
            makeDetailRow('Junk in', formatPlatformTableIntegerValue(stats.junkIn, '0')),
            makeDetailRow('RBL rejects', formatPlatformTableIntegerValue(stats.rblRejects, '0')),
            makeDetailRow(
              'Pregreet rejects',
              formatPlatformTableIntegerValue(stats.pregreetRejects, '0'),
            ),
            makeDetailRow(
              'Avg process',
              stats.averageProcessTimeMs ? `${Math.round(stats.averageProcessTimeMs)}ms` : '—',
            ),
          ]),
          footerContent: (
            <StackedBar
              segments={inboundSegments(stats)}
              ariaLabel="Inbound mail disposition breakdown"
            />
          ),
        }
      : null,
    quarantine
      ? {
          label: 'Quarantine',
          testId: 'pmg-quarantine-section',
          rows: compactDetailRows(
            quarantineSegments(quarantine).map((segment) =>
              makeDetailRow(segment.label, formatPlatformTableIntegerValue(segment.value, '0')),
            ),
          ),
          footerContent: (
            <StackedBar
              segments={quarantineSegments(quarantine)}
              ariaLabel="Quarantine breakdown by category"
              legend={false}
            />
          ),
        }
      : null,
    spamBuckets.length > 0
      ? {
          label: 'Spam scores',
          testId: 'pmg-spam-scores-section',
          rows: compactDetailRows([
            makeDetailRow(
              'Buckets',
              `${spamBuckets.length} bucket${spamBuckets.length === 1 ? '' : 's'}`,
            ),
          ]),
          footerContent: <SpamHistogram buckets={spamBuckets} />,
        }
      : null,
    relayDomains.length > 0
      ? {
          label: 'Relay domains',
          testId: 'pmg-relay-domains-section',
          rows: compactDetailRows([
            makeDetailRow('Domains', relayDomains.map(relayDomainLabel).join(', '), {
              wrap: true,
            }),
          ]),
        }
      : null,
  ]);
};

const CARD_HEADING_CLASS = 'text-[10px] font-semibold uppercase tracking-wide text-base-content';

export const ProxmoxMailGatewayMailFlow: Component<{ resource: Resource }> = (props) => {
  const id = () => asTrimmedString(props.resource.pmg?.instanceId) || props.resource.id;
  const [instance, { refetch }] = createResource<PMGInstance | null, string>(id, fetchPMGInstance);

  // Solid rethrows a resource's error when it is read, so every read goes
  // through this guard and the failure renders as a notice below.
  const failed = () => instance.state === 'errored';
  const loaded = createMemo(() => (failed() ? undefined : instance()));
  const nodes = createMemo<PMGNodeStatus[]>(() => loaded()?.nodes ?? []);
  const topDomains = createMemo(() =>
    (loaded()?.domainStats ?? [])
      .slice()
      .sort((a, b) => (b.mailCount ?? 0) - (a.mailCount ?? 0))
      .slice(0, 8),
  );
  const flowSections = createMemo(() => {
    const value = loaded();
    return value ? buildMailGatewayMailFlowSections(value) : [];
  });
  const name = () =>
    asTrimmedString(props.resource.displayName) ||
    asTrimmedString(props.resource.name) ||
    props.resource.id;
  const facts = () =>
    [
      asTrimmedString(loaded()?.version) || asTrimmedString(props.resource.pmg?.version),
      asTrimmedString(loaded()?.host),
    ].filter((fact): fact is string => Boolean(fact));

  return (
    <div class="space-y-3" data-testid="pmg-mail-flow">
      <Show
        when={!failed()}
        fallback={
          <InlineNotice
            tone="danger"
            role="alert"
            actionLabel="Retry"
            actionAriaLabel={`Retry loading mail flow for ${name()}`}
            actionOnClick={() => void refetch()}
          >
            Could not load mail flow for {name()}.
          </InlineNotice>
        }
      >
        <Show
          when={loaded() !== undefined}
          fallback={
            <p role="status" class="flex items-center gap-2 text-xs text-muted">
              <LoadingSpinner size="sm" tone="muted" />
              Loading mail flow
            </p>
          }
        >
          <Show
            when={loaded()}
            fallback={
              <InlineNotice tone="info" role="status">
                This gateway has not reported mail flow yet.
              </InlineNotice>
            }
          >
            <Show when={facts().length > 0}>
              <div
                data-testid="pmg-mail-flow-facts"
                class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted"
              >
                <For each={facts()}>{(fact) => <span class="wrap-anywhere">{fact}</span>}</For>
              </div>
            </Show>

            <Show when={flowSections().length > 0}>
              <TechnicalDetailsSection dataTestId="pmg-mail-flow-cards" sections={flowSections()} />
            </Show>

            <InfoCardFrame data-testid="pmg-cluster-nodes">
              <div class="mb-2 flex items-baseline justify-between gap-2">
                <h3 class={CARD_HEADING_CLASS}>Cluster nodes</h3>
                <span class="text-[10px] text-muted tabular-nums">
                  {nodes().length} node{nodes().length === 1 ? '' : 's'}
                </span>
              </div>
              <Show
                when={nodes().length > 0}
                fallback={<p class="text-xs text-muted">No cluster nodes reported.</p>}
              >
                <PlatformDetailTable class="min-w-0 table-fixed text-xs">
                  <PlatformDetailTableHeader>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30 md:w-[24%]`}
                    >
                      Node
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-15 md:w-[14%]`}
                    >
                      Role
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[16%]`}
                    >
                      <PlatformResponsiveTableLabel compact="Up" full="Uptime" />
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[16%]`}
                    >
                      <PlatformResponsiveTableLabel compact="Ld" full="Load" />
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[15%]`}
                    >
                      <PlatformResponsiveTableLabel compact="Q" full="Queue" />
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-10 platform-table-narrow-hidden md:w-[15%]`}
                    >
                      <PlatformResponsiveTableLabel compact="Old" full="Oldest" />
                    </TableHead>
                  </PlatformDetailTableHeader>
                  <PlatformDetailTableBody>
                    <For each={nodes()}>
                      {(node) => {
                        const cls = classifyNode(node);
                        const queue = queueCell(node.queueStatus);
                        return (
                          <TableRow>
                            <TableCell class={getPlatformTableCellClassForKind('name')}>
                              <div class="flex min-w-0 items-start gap-2">
                                <StatusDot
                                  size="sm"
                                  variant={cls.variant}
                                  title={cls.label}
                                  ariaHidden
                                  class="mt-1"
                                />
                                <span class="min-w-0 whitespace-normal font-mono text-[11px] font-semibold text-base-content wrap-anywhere">
                                  {node.name || '—'}
                                </span>
                              </div>
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-[11px] text-base-content`}
                            >
                              {asTrimmedString(node.role) || '—'}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              {formatPlatformTableUptimeValue(node.uptime)}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-[11px] text-base-content tabular-nums`}
                            >
                              {asTrimmedString(node.loadAvg) || '—'}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              <span class="font-semibold">
                                <PlatformTableNumberValue
                                  value={queue.count}
                                  format={formatPlatformTableIntegerValue}
                                />
                              </span>
                              <span
                                class="ml-1 text-muted text-[10px] font-mono"
                                title="active/deferred/hold/incoming"
                              >
                                {queue.label}
                              </span>
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} platform-table-narrow-hidden text-base-content`}
                            >
                              {node.queueStatus?.oldestAge
                                ? formatAge(node.queueStatus.oldestAge)
                                : '—'}
                            </TableCell>
                          </TableRow>
                        );
                      }}
                    </For>
                  </PlatformDetailTableBody>
                </PlatformDetailTable>
              </Show>
            </InfoCardFrame>

            <InfoCardFrame data-testid="pmg-top-domains">
              <div class="mb-2 flex items-baseline justify-between gap-2">
                <h3 class={CARD_HEADING_CLASS}>Top domains</h3>
                <span class="text-[10px] text-muted tabular-nums">
                  top {formatPlatformTableIntegerValue(topDomains().length)}
                </span>
              </div>
              <Show
                when={topDomains().length > 0}
                fallback={<p class="text-xs text-muted">No domain stats reported.</p>}
              >
                <PlatformDetailTable class="min-w-0 table-fixed text-xs">
                  <PlatformDetailTableHeader>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30 md:w-[28%]`}
                    >
                      Domain
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[14%]`}
                    >
                      Mail
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[14%]`}
                    >
                      Spam
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[14%]`}
                    >
                      Virus
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[30%]`}
                    >
                      Bytes
                    </TableHead>
                  </PlatformDetailTableHeader>
                  <PlatformDetailTableBody>
                    <For each={topDomains()}>
                      {(domain) => (
                        <TableRow>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('name')} font-mono text-[11px] text-base-content`}
                          >
                            {/* Domains are compared and copied, so they wrap
                                rather than truncate. */}
                            <span
                              class="block whitespace-normal wrap-anywhere"
                              data-testid="pmg-top-domain-name"
                            >
                              {domain.domain || '—'}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={domain.mailCount}
                              format={formatPlatformTableIntegerValue}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={domain.spamCount}
                              format={formatPlatformTableIntegerValue}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={domain.virusCount}
                              format={formatPlatformTableIntegerValue}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-muted tabular-nums`}
                          >
                            {bytesOrDash(domain.bytes)}
                          </TableCell>
                        </TableRow>
                      )}
                    </For>
                  </PlatformDetailTableBody>
                </PlatformDetailTable>
              </Show>
            </InfoCardFrame>
          </Show>
        </Show>
      </Show>
    </div>
  );
};

export default ProxmoxMailGatewayMailFlow;
