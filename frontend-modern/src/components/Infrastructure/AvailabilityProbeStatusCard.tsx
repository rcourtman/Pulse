import { For, Show, createMemo } from 'solid-js';
import { Activity, AlertCircle, Check } from 'lucide-solid';

import type { ResourceAvailabilityMeta } from '@/types/resource';
import { InfoCardFrame, InfoCardKeyValueRow } from '@/components/shared/InfoCardFrame';
import {
  getAvailabilityProbeMethodLabel,
  getAvailabilityProbeEndpointLabel,
  getAvailabilityProbePresentation,
} from '@/utils/availabilityProbePresentation';
import { formatRelativeTime } from '@/utils/format';

export interface AvailabilityProbeStatusCardProps {
  availability: ResourceAvailabilityMeta;
}

export interface AvailabilityProbeStatusCardsProps {
  availability?: ResourceAvailabilityMeta;
  checks?: ResourceAvailabilityMeta[];
}

export function AvailabilityProbeStatusCards(props: AvailabilityProbeStatusCardsProps) {
  const checks = createMemo(() => {
    const byTarget = new Map<string, ResourceAvailabilityMeta>();
    for (const check of props.checks ?? []) {
      const key =
        check.targetId?.trim() ||
        `${check.protocol ?? ''}:${check.address ?? ''}:${check.port ?? ''}:${check.path ?? ''}`;
      byTarget.set(key, check);
    }
    if (props.availability) {
      const check = props.availability;
      const key =
        check.targetId?.trim() ||
        `${check.protocol ?? ''}:${check.address ?? ''}:${check.port ?? ''}:${check.path ?? ''}`;
      if (!byTarget.has(key)) byTarget.set(key, check);
    }
    return [...byTarget.values()];
  });

  return (
    <For each={checks()}>
      {(availability) => <AvailabilityProbeStatusCard availability={availability} />}
    </For>
  );
}

export function AvailabilityProbeStatusCard(props: AvailabilityProbeStatusCardProps) {
  const isUp = () => props.availability.available === true;
  const isDown = () => props.availability.available === false;
  const latency = () => {
    const ms = props.availability.latencyMillis;
    return typeof ms === 'number' && Number.isFinite(ms) && ms > 0 ? `${Math.round(ms)}ms` : null;
  };
  const lastChecked = () => formatRelativeTime(props.availability.lastChecked);
  const method = () => getAvailabilityProbeMethodLabel(props.availability);
  const presentation = () =>
    getAvailabilityProbePresentation({
      type: 'network-endpoint',
      platformType: 'availability',
      status: isUp() ? 'online' : isDown() ? 'offline' : 'unknown',
      availability: props.availability,
    });
  const isStale = () => presentation()?.freshnessLabel === 'stale';
  const isFreshUp = () => isUp() && !isStale();
  const targetAddr = () => getAvailabilityProbeEndpointLabel(props.availability);
  const failureLabel = () => {
    const err = (props.availability.lastError ?? '').trim();
    if (!err) return null;
    if (/timed?\s*out/i.test(err)) return 'Timed out';
    const httpMatch = err.match(/\b([45]\d{2})\b/);
    if (httpMatch) return `HTTP ${httpMatch[1]}`;
    if (/refused|unreachable|no route/i.test(err)) return 'Unreachable';
    return err.length > 40 ? `${err.slice(0, 40)}…` : err;
  };
  const certificate = () => props.availability.certificate;
  const certificateTrust = () => {
    switch (certificate()?.trustStatus) {
      case 'trusted':
        return { label: 'Trusted', tone: 'success' as const };
      case 'self-signed':
        return { label: 'Self-signed', tone: 'neutral' as const };
      case 'expired':
        return { label: 'Expired', tone: 'danger' as const };
      case 'not-yet-valid':
        return { label: 'Not yet valid', tone: 'danger' as const };
      case 'untrusted':
        return { label: 'Untrusted', tone: 'danger' as const };
      default:
        return { label: 'Unknown', tone: 'neutral' as const };
    }
  };
  const certificateExpiry = () => {
    const value = certificate()?.notAfter;
    if (!value) return null;
    const expiry = new Date(value);
    if (!Number.isFinite(expiry.getTime())) return null;
    const days = Math.ceil((expiry.getTime() - Date.now()) / 86_400_000);
    const date = expiry.toLocaleDateString(undefined, {
      day: 'numeric',
      month: 'short',
      year: 'numeric',
    });
    if (days < 0) return `${date} (${Math.abs(days)}d ago)`;
    if (days === 0) return `${date} (today)`;
    return `${date} (${days}d)`;
  };

  return (
    <InfoCardFrame data-testid="availability-probe-status">
      <div class="flex items-center justify-between gap-2 mb-2">
        <div class="flex min-w-0 items-center gap-1.5">
          <Activity class="h-3.5 w-3.5 text-base-content/60" aria-hidden="true" />
          <h3 class="truncate text-[11px] font-medium uppercase tracking-wide text-base-content">
            Availability
          </h3>
        </div>
        <span
          class="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold"
          classList={{
            'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300':
              isFreshUp(),
            'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300': isDown() && !isStale(),
            'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300': isStale(),
            'bg-base-200 text-muted': !isUp() && !isDown() && !isStale(),
          }}
        >
          {isStale() ? 'Stale' : isUp() ? 'Up' : isDown() ? 'Down' : 'Not checked'}
        </span>
      </div>
      <div class="space-y-1.5 text-[11px]">
        <InfoCardKeyValueRow
          label="Latency"
          value={
            <Show
              when={isUp() && latency()}
              fallback={<span class="text-red-600 dark:text-red-400">—</span>}
            >
              <span
                classList={{
                  'text-emerald-600 dark:text-emerald-400': !isStale(),
                  'text-amber-600 dark:text-amber-300': isStale(),
                }}
              >
                {latency()}
              </span>
            </Show>
          }
        />
        <InfoCardKeyValueRow label="Method" value={method()} valueTitle={targetAddr()} />
        <InfoCardKeyValueRow
          label="Target"
          value={targetAddr()}
          valueClass="truncate"
          valueTitle={targetAddr()}
        />
        <Show when={lastChecked()}>
          <InfoCardKeyValueRow
            label="Checked"
            value={lastChecked()}
            valueClass="text-base-content/70"
          />
        </Show>
        <InfoCardKeyValueRow
          label="Freshness"
          value={presentation()?.freshnessLabel ?? 'freshness unknown'}
          valueClass={
            presentation()?.freshnessLabel === 'stale' ? 'text-amber-600 dark:text-amber-300' : ''
          }
        />
        <Show when={presentation()?.correlationLabel}>
          {(label) => (
            <InfoCardKeyValueRow
              label="Resource"
              value={label()}
              valueClass="text-amber-600 dark:text-amber-300"
            />
          )}
        </Show>
        <Show when={certificate()}>
          {(cert) => (
            <div class="mt-1.5 space-y-1.5 border-t border-base-200 pt-1.5">
              <InfoCardKeyValueRow
                label="Certificate"
                value={certificateTrust().label}
                valueClass={
                  certificateTrust().tone === 'success'
                    ? 'text-emerald-600 dark:text-emerald-300'
                    : certificateTrust().tone === 'danger'
                      ? 'text-red-600 dark:text-red-300'
                      : ''
                }
                valueTitle={cert().trustError || undefined}
              />
              <Show when={cert().subject}>
                <InfoCardKeyValueRow
                  label="Subject"
                  value={cert().subject}
                  valueClass="truncate"
                  valueTitle={cert().subject}
                />
              </Show>
              <Show when={certificateExpiry()}>
                {(expiry) => (
                  <InfoCardKeyValueRow
                    label="Expires"
                    value={expiry()}
                    valueTitle={`Warning window: ${props.availability.certificateExpiryWarningDays ?? 30} days`}
                  />
                )}
              </Show>
              <InfoCardKeyValueRow
                label="Hostname"
                value={cert().hostnameValid ? 'Matches' : 'Mismatch'}
                valueClass={
                  cert().hostnameValid
                    ? 'text-emerald-600 dark:text-emerald-300'
                    : 'text-red-600 dark:text-red-300'
                }
              />
              <Show when={cert().issuer}>
                <InfoCardKeyValueRow
                  label="Issuer"
                  value={cert().issuer}
                  valueClass="truncate font-normal"
                  valueTitle={cert().issuer}
                />
              </Show>
              <Show when={cert().fingerprintSha256}>
                {(fingerprint) => (
                  <InfoCardKeyValueRow
                    label="SHA-256"
                    value={`${fingerprint().slice(0, 16)}…`}
                    valueClass="truncate font-mono text-[10px] font-normal"
                    valueTitle={fingerprint()}
                  />
                )}
              </Show>
            </div>
          )}
        </Show>
        <Show when={isDown() && failureLabel()}>
          <div class="flex items-start gap-1.5 mt-1.5 pt-1.5 border-t border-base-200">
            <AlertCircle class="h-3 w-3 text-red-500 shrink-0 mt-0.5" aria-hidden="true" />
            <span class="text-[10px] text-red-600 dark:text-red-400">{failureLabel()}</span>
          </div>
        </Show>
        <Show when={isFreshUp()}>
          <div class="flex items-center gap-1 mt-1.5 pt-1.5 border-t border-base-200">
            <Check class="h-3 w-3 text-emerald-500 shrink-0" aria-hidden="true" />
            <span class="text-[10px] text-muted">Responding normally</span>
          </div>
        </Show>
      </div>
    </InfoCardFrame>
  );
}
