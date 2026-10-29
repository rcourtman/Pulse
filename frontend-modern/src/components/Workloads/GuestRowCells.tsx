import { createMemo, createSignal, For, Show } from 'solid-js';

import { Dialog } from '@/components/shared/Dialog';
import { getShippedDocUrl } from '@/utils/docsLinks';
import {
  GUEST_DRAWER_BACKUP_PRECAUTION,
  getGuestDrawerGuestReadPrecaution,
  getGuestDrawerGuestReadPresentation,
  type GuestDrawerGuestReadEvidence,
} from './guestDrawerModel';
import {
  EXTERNAL_TEXT_LINK_BASE_CLASS,
  EXTERNAL_TEXT_LINK_VARIANT_CLASSES,
} from '@/components/shared/ExternalTextLink';

import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { useTooltip } from '@/hooks/useTooltip';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { GuestNetworkInterface } from '@/types/api';
import {
  formatBackupCompletionTime,
  formatBytes,
  formatRelativeTime,
  getBackupInfo,
  type BackupInfo,
} from '@/utils/format';
import { ALERT_BACKUP_THRESHOLDS_PATH } from '@/routing/resourceLinks';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import type { AvailabilityProbePresentation } from '@/utils/availabilityProbePresentation';
import {
  getWorkloadsGuestBackupAgeRule,
  getWorkloadsGuestBackupStatusPresentation,
  getWorkloadsGuestBackupTooltip,
  getWorkloadsGuestNetworkEmptyState,
  type WorkloadsGuestBackupDisplayStatus,
} from '@/utils/workloadGuestPresentation';

function BackupShieldIcon(props: { icon: 'check' | 'warning' | 'x' | 'running'; pulse?: boolean }) {
  return (
    <svg
      class="h-3.5 w-3.5 shrink-0"
      classList={{ 'animate-pulse': props.pulse }}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
      <Show when={props.icon === 'check'}>
        <path d="m9 12 2 2 4-4" />
      </Show>
      <Show when={props.icon === 'warning'}>
        <path d="M12 8v4M12 16h.01" />
      </Show>
      <Show when={props.icon === 'x'}>
        <path d="M10 10l4 4M14 10l-4 4" />
      </Show>
      <Show when={props.icon === 'running'}>
        <path d="M8 12h.01M12 12h.01M16 12h.01" />
      </Show>
    </svg>
  );
}

function BackupIndicator(props: {
  lastBackup: string | number | null | undefined;
  isTemplate: boolean;
  guestName?: string;
  backupRunning?: boolean;
  guestReadEvidence?: GuestDrawerGuestReadEvidence;
}) {
  if (props.isTemplate) return null;

  return (
    <BackupStatusCell
      lastBackup={props.lastBackup}
      backupRunning={props.backupRunning}
      guestName={props.guestName}
      guestReadEvidence={props.guestReadEvidence}
      indicator
    />
  );
}

function getBackupAgeBadgeLabel(
  lastBackup: string | number | null | undefined,
  info: BackupInfo,
  now: number,
): string {
  if (info.status === 'never' || info.status === 'unknown') return 'Unknown';
  const compact = formatRelativeTime(lastBackup ?? undefined, {
    compact: true,
    emptyText: 'Unknown',
    now,
  });
  if (compact === 'just now') return 'Now';
  return compact.replace(/\s+ago$/, '');
}

function getBackupAgeBadgeClass(status: WorkloadsGuestBackupDisplayStatus): string {
  const layout =
    'inline-flex h-5 min-w-13 items-center justify-center gap-1 px-1.5 text-[10px] font-semibold leading-none tabular-nums cursor-help';
  // A healthy backup only needs its shield. Existing backups that cross an age
  // threshold remain amber because age is a policy-relative caution. Missing
  // completion evidence is also cautionary: this row cannot establish that
  // no backups exist. The exact age stays visible when it is available.
  const pill = `${layout} rounded-full border`;

  switch (status) {
    case 'fresh':
      return `${layout} text-green-600 dark:text-green-400`;
    case 'stale':
      return `${pill} border-yellow-200 bg-yellow-50 text-yellow-700 dark:border-yellow-900/70 dark:bg-yellow-950/40 dark:text-yellow-300`;
    case 'overdue':
    case 'unknown':
    case 'never':
      return `${pill} border-yellow-200 bg-yellow-50 text-yellow-700 dark:border-yellow-900/70 dark:bg-yellow-950/40 dark:text-yellow-300`;
    // A running backup is work underway, not success (green would claim a
    // completed backup) and not absence (hiding it would look like nothing
    // is happening). Blue matches the Backups page's running-task tone.
    case 'running':
      return `${pill} border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900/70 dark:bg-blue-950/40 dark:text-blue-300`;
  }
}

function NetworkInfoCell(props: {
  ipAddresses: string[];
  networkInterfaces: GuestNetworkInterface[];
}) {
  const tip = useTooltip();

  const hasInterfaces = () => props.networkInterfaces.length > 0;
  const primaryIp = () =>
    props.ipAddresses[0] || props.networkInterfaces[0]?.addresses?.[0] || null;
  const totalIps = () => {
    if (props.ipAddresses.length > 0) return props.ipAddresses.length;
    return props.networkInterfaces.reduce((sum, iface) => sum + (iface.addresses?.length || 0), 0);
  };

  return (
    <>
      <span
        class="inline-flex items-center gap-1 text-xs text-muted"
        onMouseEnter={tip.onMouseEnter}
        onMouseLeave={tip.onMouseLeave}
      >
        <Show when={primaryIp()} fallback="-">
          <svg
            class="w-3.5 h-3.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="1.5"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418"
            />
          </svg>
          <span class="text-[10px] font-medium">{totalIps()}</span>
        </Show>
      </span>

      <TooltipPortal
        when={tip.show() && (hasInterfaces() || props.ipAddresses.length > 0)}
        x={tip.pos().x}
        y={tip.pos().y}
      >
        <div class="min-w-[180px] max-w-[280px]">
          <div class="font-medium mb-1 text-base-content border-b border-border pb-1">
            Network Interfaces
          </div>

          <Show when={hasInterfaces()}>
            <For each={props.networkInterfaces}>
              {(iface, idx) => (
                <div class="py-1" classList={{ 'border-t border-border': idx() > 0 }}>
                  <div class="flex items-center gap-2 text-base-content font-medium">
                    <span>{iface.name || 'eth' + idx()}</span>
                    <Show when={iface.mac}>
                      <span class="text-[9px] text-muted font-normal">{iface.mac}</span>
                    </Show>
                  </div>
                  <Show when={iface.addresses && iface.addresses.length > 0}>
                    <div class="mt-0.5 flex flex-wrap gap-1">
                      <For each={iface.addresses}>
                        {(ip) => <span class="text-base-content font-mono">{ip}</span>}
                      </For>
                    </div>
                  </Show>
                  <Show when={!iface.addresses || iface.addresses.length === 0}>
                    <span class="text-muted text-[9px]">
                      {getWorkloadsGuestNetworkEmptyState()}
                    </span>
                  </Show>
                  <Show when={(iface.rxBytes || 0) > 0 || (iface.txBytes || 0) > 0}>
                    <div class="mt-0.5 text-[9px] text-muted">
                      RX: {formatBytes(iface.rxBytes || 0)} / TX: {formatBytes(iface.txBytes || 0)}
                    </div>
                  </Show>
                </div>
              )}
            </For>
          </Show>

          <Show when={!hasInterfaces() && props.ipAddresses.length > 0}>
            <div class="py-1">
              <div class="flex items-center gap-2 text-base-content font-medium">
                <span>IP Addresses</span>
                <span class="text-[9px] text-muted font-normal">No agent data</span>
              </div>
              <div class="mt-0.5 flex flex-wrap gap-1">
                <For each={props.ipAddresses}>
                  {(ip) => <span class="text-base-content font-mono">{ip}</span>}
                </For>
              </div>
            </div>
          </Show>
        </div>
      </TooltipPortal>
    </>
  );
}

type OSType = 'windows' | 'linux' | 'unknown';

function detectOSType(osName: string): OSType {
  const lower = osName.toLowerCase();
  if (lower.includes('windows')) return 'windows';
  if (
    lower.includes('linux') ||
    lower.includes('debian') ||
    lower.includes('ubuntu') ||
    lower.includes('alpine') ||
    lower.includes('centos') ||
    lower.includes('fedora') ||
    lower.includes('arch') ||
    lower.includes('nixos') ||
    lower.includes('suse') ||
    lower.includes('gentoo') ||
    lower.includes('rhel') ||
    lower.includes('rocky') ||
    lower.includes('alma') ||
    lower.includes('devuan') ||
    lower.includes('gnu') ||
    lower.includes('freebsd') ||
    lower.includes('openbsd') ||
    lower.includes('netbsd')
  ) {
    return 'linux';
  }
  return 'unknown';
}

function OSInfoCell(props: { osName: string; osVersion: string; agentVersion: string }) {
  const tip = useTooltip();
  const osType = createMemo(() => detectOSType(props.osName));

  const OSIcon = () => {
    const type = osType();
    const iconClass = 'w-3.5 h-3.5 text-muted';

    switch (type) {
      case 'windows':
        return (
          <svg class={iconClass} viewBox="0 0 24 24" fill="currentColor">
            <path d="M3 5.5l7.038-1v6.5H3v-5.5zm0 13l7.038 1V13H3v5.5zm8.038 1.118L21 21V13h-9.962v6.618zM11.038 4.382L21 3v8h-9.962V4.382z" />
          </svg>
        );
      case 'linux':
        return (
          <svg
            class={iconClass}
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <polyline points="4 17 10 11 4 5" />
            <line x1="12" y1="19" x2="20" y2="19" />
          </svg>
        );
      default:
        return (
          <span class="text-slate-400" aria-hidden="true">
            —
          </span>
        );
    }
  };

  return (
    <>
      <span
        class="inline-flex items-center gap-1"
        onMouseEnter={tip.onMouseEnter}
        onMouseLeave={tip.onMouseLeave}
      >
        <OSIcon />
      </span>

      <TooltipPortal when={tip.show()} x={tip.pos().x} y={tip.pos().y}>
        <div class="min-w-[120px] max-w-[220px]">
          <div class="font-medium mb-1 text-slate-300 border-b border-border pb-1">
            Operating System
          </div>
          <div class="py-0.5">
            <div class="text-base-content font-medium">{props.osName}</div>
            <Show when={props.osVersion}>
              <div class="text-slate-400">Version: {props.osVersion}</div>
            </Show>
            <Show when={props.agentVersion}>
              <div class="text-slate-500 text-[9px] mt-1 pt-1 border-t border-border">
                Agent: {props.agentVersion}
              </div>
            </Show>
          </div>
        </div>
      </TooltipPortal>
    </>
  );
}

function BackupStatusCell(props: {
  lastBackup: string | number | null | undefined;
  backupRunning?: boolean;
  indicator?: boolean;
  guestName?: string;
  guestReadEvidence?: GuestDrawerGuestReadEvidence;
}) {
  const now = useRelativeTimeNow();
  const tip = useTooltip();
  const [detailsOpen, setDetailsOpen] = createSignal(false);
  let trigger: HTMLButtonElement | undefined;

  const alertsActivation = useAlertsActivation();
  const info = createMemo(() =>
    getBackupInfo(props.lastBackup, alertsActivation.getBackupThresholds(), now()),
  );
  // The badge shows "running" while a backup is underway: green would claim
  // a completed backup that does not exist yet, and the age states would
  // hide that the situation is being remediated. The age itself still
  // reflects the latest COMPLETED backup and is kept in the tooltip.
  const displayStatus = createMemo<WorkloadsGuestBackupDisplayStatus>(() =>
    props.backupRunning ? 'running' : info().status,
  );
  const config = createMemo(() => getWorkloadsGuestBackupStatusPresentation(displayStatus()));
  const badgeLabel = createMemo(() =>
    displayStatus() === 'running'
      ? 'Running'
      : getBackupAgeBadgeLabel(props.lastBackup, info(), now()),
  );
  const hasCompletedBackup = createMemo(() => info().ageMs !== null);
  // Parse/format only when the evidence is disclosed, not for every closed
  // table row on each shared-clock tick. Unknown/future evidence has no time.
  const completionTime = () =>
    hasCompletedBackup() ? formatBackupCompletionTime(props.lastBackup!) : null;
  const ageRule = createMemo(() =>
    getWorkloadsGuestBackupAgeRule(info().status, alertsActivation.getBackupThresholds()),
  );
  const ariaLabel = createMemo(() => {
    const currentInfo = info();
    if (props.indicator) {
      return getWorkloadsGuestBackupTooltip(
        currentInfo.status,
        currentInfo.ageFormatted,
        props.backupRunning,
      );
    }
    if (currentInfo.status === 'unknown') {
      return `Backup status: ${props.backupRunning ? 'backup running now, ' : ''}${currentInfo.ageFormatted}`;
    }
    if (props.backupRunning) {
      return hasCompletedBackup()
        ? `Backup status: backup running now, last completed backup ${currentInfo.ageFormatted}`
        : 'Backup status: backup running now, no completed backup reported';
    }
    if (currentInfo.status === 'never') return 'Backup status: no completed backup reported';
    return `Backup status: ${currentInfo.status}, last backup ${currentInfo.ageFormatted}`;
  });

  // Both entry points reveal the same retained completion evidence. Hover
  // stays a shortcut on a fine pointer; the native button opens a read-only
  // dialog for keyboard/touch without stealing the enclosing row action.
  // Backup completion and current guest-read state are separate evidence.
  // Reuse the drawer's allowlisted interpretation; never infer thaw from a
  // fresh timestamp, retained metadata or a reporting Pulse Agent.
  const guestReadPresentation = () =>
    props.guestReadEvidence ? getGuestDrawerGuestReadPresentation(props.guestReadEvidence) : null;
  const guestReadPrecaution = () =>
    props.guestReadEvidence ? getGuestDrawerGuestReadPrecaution(props.guestReadEvidence) : null;
  const Evidence = (evidenceProps: { dialog?: boolean; ruleLink?: boolean }) => (
    <div class="space-y-2 text-xs">
      <Show when={props.guestName}>
        <p class="font-medium text-base-content wrap-break-word">{props.guestName}</p>
      </Show>
      <Show when={props.backupRunning}>
        <p class="text-blue-600 dark:text-blue-400">Backup running now…</p>
      </Show>
      <Show when={completionTime()}>
        {(completion) => (
          <div class="space-y-1">
            <div class="text-muted">Last completed backup</div>
            <time
              class="block whitespace-normal wrap-break-word font-medium text-base-content"
              dateTime={completion().completedAt}
            >
              {completion().dateLabel}
            </time>
            <div class="text-base-content">{info().ageFormatted}</div>
            <Show when={ageRule()}>
              {(rule) => (
                <p class="text-muted" data-backup-age-rule>
                  {rule()}
                </p>
              )}
            </Show>
            <Show when={evidenceProps.ruleLink && ageRule()}>
              <a
                href={ALERT_BACKUP_THRESHOLDS_PATH}
                class={`${EXTERNAL_TEXT_LINK_BASE_CLASS} ${EXTERNAL_TEXT_LINK_VARIANT_CLASSES.compactAction} -mx-1`}
                onClick={() => setDetailsOpen(false)}
              >
                Change when backups count as old
              </a>
            </Show>
          </div>
        )}
      </Show>
      <Show when={info().status === 'unknown'}>
        <p class="text-amber-700 dark:text-amber-300">{info().ageFormatted}</p>
      </Show>
      <Show when={info().status === 'never'}>
        <p class="text-amber-700 dark:text-amber-300">
          No completed backup is reported in the current inventory. Missing data does not mean no
          backups exist.
        </p>
      </Show>
      <Show when={guestReadPresentation()}>
        {(reading) => (
          <div class="space-y-2 border-t border-border pt-2" data-backup-guest-read-evidence>
            <p
              class={
                reading().tone === 'warning'
                  ? 'font-medium text-amber-700 dark:text-amber-300'
                  : 'font-medium text-base-content'
              }
            >
              Guest-agent reads: {reading().label}
            </p>
            <p class="whitespace-normal wrap-break-word leading-relaxed text-base-content">
              {guestReadPrecaution() || reading().detail}
            </p>
            <Show when={reading().precaution}>
              <p class="whitespace-normal wrap-break-word leading-relaxed text-base-content">
                {GUEST_DRAWER_BACKUP_PRECAUTION}
              </p>
              {/* A hover tooltip is not an interactive surface. The same
                  read-only guidance is reachable by keyboard/touch here. */}
              <Show when={evidenceProps.dialog}>
                <a
                  href={getShippedDocUrl('VM_DISK_MONITORING.md')}
                  class="inline-flex min-h-6 items-center font-medium text-blue-600 dark:text-blue-400 underline underline-offset-2 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500"
                >
                  Backup safety guidance
                </a>
              </Show>
            </Show>
          </div>
        )}
      </Show>
    </div>
  );
  const focusClass =
    'focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 focus-visible:ring-offset-surface';

  // The indicator sits in the guest name cell, where on phones the whole row
  // is the disclosure target (the expand chevron is screen-reader only). Keep
  // its touch target at the 24px minimum: a 44px box covered most of the
  // name cell's row-tap area and doubled every phone row's height. The
  // negative margins let that 24px target overhang the 14px shield, so it
  // neither makes the row taller nor takes width from the truncating name
  // beside it. The backup-column badge owns its
  // own cell, so it keeps the 44px target.
  return (
    <>
      <button
        ref={trigger}
        type="button"
        class={
          props.indicator
            ? `-mx-[5px] -my-1 inline-flex min-h-6 min-w-6 shrink-0 items-center justify-center rounded-sm sm:m-0 sm:min-h-0 sm:min-w-0 ${config().color} ${focusClass}`
            : `${getBackupAgeBadgeClass(displayStatus())} min-h-11 min-w-11 sm:min-h-0 ${focusClass}`
        }
        onMouseEnter={tip.onMouseEnter}
        onMouseLeave={tip.onMouseLeave}
        onClick={(event) => {
          event.stopPropagation();
          tip.onMouseLeave();
          setDetailsOpen(true);
        }}
        aria-label={ariaLabel()}
        title={props.indicator ? ariaLabel() : undefined}
        aria-haspopup="dialog"
        aria-expanded={detailsOpen()}
      >
        <BackupShieldIcon icon={config().icon} pulse={displayStatus() === 'running'} />
        <Show when={!props.indicator && displayStatus() !== 'fresh'}>
          <span>{badgeLabel()}</span>
        </Show>
      </button>

      <TooltipPortal when={tip.show()} x={tip.pos().x} y={tip.pos().y}>
        <div class="min-w-[140px] max-w-[280px]">
          <div class="mb-1 border-b border-border pb-1 font-medium text-base-content">
            Backup status
          </div>
          <Evidence />
        </div>
      </TooltipPortal>
      <Show when={detailsOpen()}>
        <Dialog
          isOpen
          ariaLabel={`Backup status details${props.guestName ? ` for ${props.guestName}` : ''}`}
          onClose={() => setDetailsOpen(false)}
          returnFocus={() => trigger}
          panelClass="max-w-sm"
        >
          <div class="flex items-center justify-between gap-3 border-b border-border px-4 py-2">
            <h2 class="text-sm font-semibold text-base-content">Backup status</h2>
            <button
              type="button"
              aria-label="Close backup details"
              class="min-h-11 rounded-sm px-3 text-xs text-muted hover:bg-surface-hover focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500"
              onClick={() => setDetailsOpen(false)}
            >
              Close
            </button>
          </div>
          <div class="overflow-y-auto p-4">
            <Evidence dialog ruleLink />
          </div>
        </Dialog>
      </Show>
    </>
  );
}

function InfoTooltipCell(props: { value: string; tooltip: string; type: string }) {
  const tip = useTooltip();

  const label = createMemo(() => {
    if (props.type === 'app-container') return 'Image';
    if (props.type === 'pod') return 'Namespace';
    return 'ID';
  });

  return (
    <>
      <span
        class="truncate max-w-[100px] cursor-help"
        onMouseEnter={tip.onMouseEnter}
        onMouseLeave={tip.onMouseLeave}
      >
        {props.value}
      </span>

      <TooltipPortal when={tip.show()} x={tip.pos().x} y={tip.pos().y}>
        <div class="max-w-[280px]">
          <div class="font-medium mb-1 text-slate-300 border-b border-border pb-1">{label()}</div>
          <div class="py-0.5 text-base-content break-all">{props.tooltip}</div>
        </div>
      </TooltipPortal>
    </>
  );
}

function AvailabilityProbeCell(props: { presentation: AvailabilityProbePresentation }) {
  const p = createMemo(() => props.presentation);

  const badgeText = createMemo(() => {
    const result = p().resultLabel;
    const compactResult = /^\d+\s*ms$/.test(result)
      ? result.replace(/\s/, '')
      : result === 'reachable'
        ? 'up'
        : result === 'not checked'
          ? 'pending'
          : result;
    // 'fresh' is Pulse vouching for its own probe pipeline, not a fact about the
    // guest. It is true on nearly every row, so printing it turns the column into
    // repeated noise. Surface freshness only when it is a reason to distrust the
    // result.
    const freshness = p().freshnessLabel === 'freshness unknown' ? 'unknown' : p().freshnessLabel;
    return freshness === 'fresh' ? compactResult : `${compactResult} · ${freshness}`;
  });

  return (
    <Show when={badgeText()}>
      <span
        class={`inline-flex items-center rounded-sm px-1 py-0.5 text-[9px] font-semibold leading-none whitespace-nowrap ${p().toneClassName}`}
        title={p().rowLabel}
      >
        {badgeText()}
      </span>
    </Show>
  );
}

export {
  AvailabilityProbeCell,
  BackupIndicator,
  BackupStatusCell,
  InfoTooltipCell,
  NetworkInfoCell,
  OSInfoCell,
};
