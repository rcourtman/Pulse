import { createMemo, For, Show } from 'solid-js';

import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { useTooltip } from '@/hooks/useTooltip';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { GuestNetworkInterface } from '@/types/api';
import { formatBytes, getBackupInfo } from '@/utils/format';
import {
  getDashboardGuestBackupStatusPresentation,
  getDashboardGuestBackupTooltip,
  getDashboardGuestNetworkEmptyState,
} from '@/utils/dashboardGuestPresentation';

function BackupIndicator(props: {
  lastBackup: string | number | null | undefined;
  isTemplate: boolean;
}) {
  if (props.isTemplate) return null;

  const alertsActivation = useAlertsActivation();
  const backupInfo = createMemo(() =>
    getBackupInfo(props.lastBackup, alertsActivation.getBackupThresholds()),
  );
  const config = createMemo(() => getDashboardGuestBackupStatusPresentation(backupInfo().status));

  const shouldShow = createMemo(() => {
    const status = backupInfo().status;
    return status === 'stale' || status === 'critical' || status === 'never';
  });

  const tooltipText = createMemo(() => {
    const info = backupInfo();
    return getDashboardGuestBackupTooltip(info.status, info.ageFormatted);
  });

  return (
    <Show when={shouldShow()}>
      <span class={`flex-shrink-0 ${config().color}`} title={tooltipText()}>
        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
          <path
            d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          <Show when={config().icon === 'warning'}>
            <path
              d="M12 8v4M12 16h.01"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </Show>
          <Show when={config().icon === 'x'}>
            <path
              d="M10 10l4 4M14 10l-4 4"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </Show>
        </svg>
      </span>
    </Show>
  );
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
          <div class="font-medium mb-1 text-slate-300 border-b border-border pb-1">
            Network Interfaces
          </div>

          <Show when={hasInterfaces()}>
            <For each={props.networkInterfaces}>
              {(iface, idx) => (
                <div class="py-1" classList={{ 'border-t border-border': idx() > 0 }}>
                  <div class="flex items-center gap-2 text-blue-400 font-medium">
                    <span>{iface.name || 'eth' + idx()}</span>
                    <Show when={iface.mac}>
                      <span class="text-[9px] text-slate-500 font-normal">{iface.mac}</span>
                    </Show>
                  </div>
                  <Show when={iface.addresses && iface.addresses.length > 0}>
                    <div class="mt-0.5 flex flex-wrap gap-1">
                      <For each={iface.addresses}>
                        {(ip) => <span class="text-slate-300 font-mono">{ip}</span>}
                      </For>
                    </div>
                  </Show>
                  <Show when={!iface.addresses || iface.addresses.length === 0}>
                    <span class="text-slate-500 text-[9px]">
                      {getDashboardGuestNetworkEmptyState()}
                    </span>
                  </Show>
                  <Show when={(iface.rxBytes || 0) > 0 || (iface.txBytes || 0) > 0}>
                    <div class="mt-0.5 text-[9px] text-slate-500">
                      RX: {formatBytes(iface.rxBytes || 0)} / TX: {formatBytes(iface.txBytes || 0)}
                    </div>
                  </Show>
                </div>
              )}
            </For>
          </Show>

          <Show when={!hasInterfaces() && props.ipAddresses.length > 0}>
            <div class="py-1">
              <div class="flex items-center gap-2 text-blue-400 font-medium">
                <span>IP Addresses</span>
                <span class="text-[9px] text-slate-500 font-normal">No agent data</span>
              </div>
              <div class="mt-0.5 flex flex-wrap gap-1">
                <For each={props.ipAddresses}>
                  {(ip) => <span class="text-slate-300 font-mono">{ip}</span>}
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
        return <span class="text-slate-400">—</span>;
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

function BackupStatusCell(props: { lastBackup: string | number | null | undefined }) {
  const tip = useTooltip();

  const alertsActivation = useAlertsActivation();
  const info = createMemo(() =>
    getBackupInfo(props.lastBackup, alertsActivation.getBackupThresholds()),
  );
  const config = createMemo(() => getDashboardGuestBackupStatusPresentation(info().status));

  return (
    <>
      <span
        class={`flex-shrink-0 cursor-help ${config().color}`}
        onMouseEnter={tip.onMouseEnter}
        onMouseLeave={tip.onMouseLeave}
        aria-label={`Backup status: ${info().status}`}
      >
        <svg
          class="w-4 h-4"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />

          <Show when={config().icon === 'check'}>
            <path d="m9 12 2 2 4-4" />
          </Show>
          <Show when={config().icon === 'warning'}>
            <path d="M12 8v4M12 16h.01" />
          </Show>
          <Show when={config().icon === 'x'}>
            <path d="M10 10l4 4M14 10l-4 4" />
          </Show>
        </svg>
      </span>

      <TooltipPortal when={tip.show()} x={tip.pos().x} y={tip.pos().y}>
        <div class="min-w-[140px]">
          <div class="font-medium mb-1 text-slate-300 border-b border-border pb-1">
            Backup Status
          </div>
          <Show when={info().status !== 'never'}>
            <div class="py-0.5">
              <div class="text-slate-400">Last backup</div>
              <div class="text-base-content font-medium">
                {new Date(props.lastBackup!).toLocaleDateString(undefined, {
                  weekday: 'short',
                  year: 'numeric',
                  month: 'short',
                  day: 'numeric',
                })}
              </div>
              <div class="text-slate-300">{new Date(props.lastBackup!).toLocaleTimeString()}</div>
            </div>
            <div class="pt-1 mt-1 border-t border-border">
              <span class={config().color}>{info().ageFormatted} ago</span>
            </div>
          </Show>
          <Show when={info().status === 'never'}>
            <div class="py-0.5 text-red-400">No backup has ever been recorded for this guest.</div>
          </Show>
        </div>
      </TooltipPortal>
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

export { BackupIndicator, BackupStatusCell, InfoTooltipCell, NetworkInfoCell, OSInfoCell };
