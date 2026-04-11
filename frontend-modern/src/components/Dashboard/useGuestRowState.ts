import { createMemo } from 'solid-js';

import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useAnomalyForMetric } from '@/hooks/useAnomalies';
import type { Container } from '@/types/api';
import { buildMetricKey } from '@/utils/metricsKeys';
import { getGuestPowerIndicator, isGuestRunning } from '@/utils/status';
import { getShortImageName, formatBytes } from '@/utils/format';
import {
  getCanonicalWorkloadId,
  getWorkloadMetricsKind,
  isDockerManagedAppContainer,
  resolveWorkloadType,
} from '@/utils/workloads';
import { buildInfrastructureHrefForWorkload } from '@/routing/resourceLinks';
import { getWorkloadTypeBadge } from '@/components/shared/workloadTypeBadges';

import { getWorkloadDockerHostId } from './workloadTopology';
import {
  DEFAULT_FIRST_CELL_INDENT,
  EMPTY_IO_EMPHASIS,
  GROUPED_FIRST_CELL_INDENT,
  getOutlierEmphasis,
  type GuestRowProps,
} from './guestRowModel';

export function useGuestRowState(props: GuestRowProps) {
  const { isMobile } = useBreakpoint();

  const guestId = createMemo(() => getCanonicalWorkloadId(props.guest));
  const infrastructureHref = createMemo(() => buildInfrastructureHrefForWorkload(props.guest));

  const visibleColumnIdSet = createMemo(() =>
    props.visibleColumnIds ? new Set(props.visibleColumnIds) : null,
  );

  const isColVisible = (colId: string) => {
    const set = visibleColumnIdSet();
    if (!set) return true;
    return set.has(colId);
  };

  const workloadType = createMemo(() => resolveWorkloadType(props.guest));
  const metricsKey = createMemo(() =>
    buildMetricKey(getWorkloadMetricsKind(props.guest), guestId()),
  );

  const cpuAnomaly = useAnomalyForMetric(guestId, () => 'cpu');
  const memoryAnomaly = useAnomalyForMetric(guestId, () => 'memory');
  const diskAnomaly = useAnomalyForMetric(guestId, () => 'disk');

  const customUrl = createMemo(() => props.customUrl?.trim() ?? '');

  const displayId = createMemo(() => {
    const provided = props.guest.displayId?.trim();
    if (provided) return provided;
    if (typeof props.guest.vmid === 'number' && props.guest.vmid > 0) {
      return String(props.guest.vmid);
    }
    return '';
  });

  const ipAddresses = createMemo(() => props.guest.ipAddresses ?? []);
  const networkInterfaces = createMemo(() => props.guest.networkInterfaces ?? []);
  const hasNetworkInterfaces = createMemo(() => networkInterfaces().length > 0);
  const osName = createMemo(() => props.guest.osName?.trim() ?? '');
  const osVersion = createMemo(() => props.guest.osVersion?.trim() ?? '');
  const agentVersion = createMemo(() => props.guest.agentVersion?.trim() ?? '');
  const hasOsInfo = createMemo(() => osName().length > 0 || osVersion().length > 0);
  const dockerImage = createMemo(() => props.guest.image?.trim() ?? '');
  const namespace = createMemo(() => props.guest.namespace?.trim() ?? '');
  const contextLabel = createMemo(() => props.guest.contextLabel?.trim() ?? '');
  const clusterName = createMemo(() => props.guest.clusterName?.trim() ?? '');
  const isPveWorkload = createMemo(() => {
    const type = workloadType();
    return type === 'vm' || type === 'system-container';
  });

  const infoValue = createMemo(() => {
    const type = workloadType();
    if (type === 'vm' || type === 'system-container') return displayId();
    if (type === 'app-container') return dockerImage() ? getShortImageName(dockerImage()) : '';
    if (type === 'pod') return namespace();
    return '';
  });

  const infoTooltip = createMemo(() => {
    if (workloadType() === 'app-container') return dockerImage();
    return infoValue();
  });

  const supportsBackup = createMemo(() => {
    const type = workloadType();
    return type === 'vm' || type === 'system-container';
  });

  const isOCIContainer = createMemo(() => {
    if (workloadType() !== 'system-container') return false;
    const containerMeta = props.guest as Partial<Pick<Container, 'isOci'>>;
    return props.guest.type === 'oci-container' || containerMeta.isOci === true;
  });

  const ociImage = createMemo(() => {
    if (!isOCIContainer()) return null;

    const template = (props.guest as Partial<Pick<Container, 'osTemplate'>>).osTemplate;
    if (!template) return null;

    let image = template;
    if (image.startsWith('oci:')) image = image.slice(4);
    if (image.startsWith('docker:')) image = image.slice(7);
    return image;
  });

  const typeInfo = createMemo(() => {
    if (isOCIContainer()) {
      return getWorkloadTypeBadge('oci-container', {
        title: `OCI Container${ociImage() ? ` • ${ociImage()}` : ''}`,
      });
    }

    if (workloadType() === 'app-container') {
      if (!isDockerManagedAppContainer(props.guest)) {
        const platform = (props.guest.platformType || '').trim().toLowerCase();
        return getWorkloadTypeBadge('app-container', {
          title: platform === 'truenas' ? 'TrueNAS App Container' : 'Application Container',
        });
      }

      const runtime = (props.guest.containerRuntime || '').trim();
      const normalized = runtime.toLowerCase();
      const label =
        normalized === 'podman'
          ? 'Podman'
          : normalized === 'docker'
            ? 'Docker'
            : runtime
              ? runtime
              : 'Containers';
      const title =
        normalized === 'podman'
          ? 'Podman Container'
          : normalized === 'docker'
            ? 'Docker Container'
            : runtime
              ? `${runtime} Container`
              : 'Container (Docker-compatible runtime)';
      return getWorkloadTypeBadge('app-container', { label, title });
    }

    return getWorkloadTypeBadge(workloadType());
  });

  const diskRead = createMemo(() => props.guest.diskRead || 0);
  const diskWrite = createMemo(() => props.guest.diskWrite || 0);
  const networkIn = createMemo(() => props.guest.networkIn || 0);
  const networkOut = createMemo(() => props.guest.networkOut || 0);
  const ioEmphasis = createMemo(() => props.ioEmphasis ?? EMPTY_IO_EMPHASIS);
  const diskIOTotal = createMemo(() => diskRead() + diskWrite());
  const networkTotal = createMemo(() => networkIn() + networkOut());
  const diskIOEmphasis = createMemo(() => getOutlierEmphasis(diskIOTotal(), ioEmphasis().diskIO));
  const networkEmphasis = createMemo(() =>
    getOutlierEmphasis(networkTotal(), ioEmphasis().network),
  );

  const memoryExtraLines = createMemo(() => {
    if (!props.guest.memory) return undefined;

    const lines: string[] = [];
    const total = props.guest.memory.total ?? 0;
    if (
      props.guest.memory.balloon &&
      props.guest.memory.balloon > 0 &&
      props.guest.memory.balloon !== total
    ) {
      lines.push(`Balloon: ${formatBytes(props.guest.memory.balloon)}`);
    }
    if (props.guest.memory.swapTotal && props.guest.memory.swapTotal > 0) {
      const swapUsed = props.guest.memory.swapUsed ?? 0;
      lines.push(`Swap: ${formatBytes(swapUsed)} / ${formatBytes(props.guest.memory.swapTotal)}`);
    }
    return lines.length > 0 ? lines : undefined;
  });

  const memoryTooltip = createMemo(() => memoryExtraLines()?.join('\n') ?? undefined);
  const memoryPercentOnly = createMemo(() => {
    const memory = props.guest.memory;
    if (!memory) return undefined;
    if ((memory.total ?? 0) > 0) return undefined;

    const usage = memory.usage ?? 0;
    if (!Number.isFinite(usage) || usage <= 0) return undefined;
    return Math.max(0, Math.min(usage, 100));
  });

  const diskPercent = createMemo(() => {
    if (!props.guest.disk || props.guest.disk.total === 0) return 0;
    if (props.guest.disk.usage === -1) return -1;
    return (props.guest.disk.used / props.guest.disk.total) * 100;
  });

  const hasDiskUsage = createMemo(() => {
    if (!props.guest.disk) return false;
    if (props.guest.disk.total <= 0) return false;
    return diskPercent() !== -1;
  });

  const parentOnline = createMemo(() => props.parentNodeOnline !== false);
  const isRunning = createMemo(() => isGuestRunning(props.guest, parentOnline()));
  const guestStatus = createMemo(() => getGuestPowerIndicator(props.guest, parentOnline()));
  const lockLabel = createMemo(() => (props.guest.lock || '').trim());

  const hasUnacknowledgedAlert = createMemo(() => !!props.alertStyles?.hasUnacknowledgedAlert);
  const hasAcknowledgedOnlyAlert = createMemo(() => !!props.alertStyles?.hasAcknowledgedOnlyAlert);
  const showAlertHighlight = createMemo(
    () => hasUnacknowledgedAlert() || hasAcknowledgedOnlyAlert(),
  );

  const alertAccentTone = createMemo<'critical' | 'warning' | 'acknowledged' | undefined>(() => {
    if (!showAlertHighlight()) return undefined;
    if (hasUnacknowledgedAlert()) {
      return props.alertStyles?.severity === 'critical' ? 'critical' : 'warning';
    }
    return 'acknowledged';
  });

  const rowClass = createMemo(() => {
    const base = 'transition-all duration-200 relative group cursor-pointer';

    if (props.isExpanded) {
      return `${base} bg-blue-50 dark:bg-blue-900 z-10 hover:shadow-sm`;
    }

    const hover = 'hover:shadow-sm';
    const alertBg = hasUnacknowledgedAlert()
      ? props.alertStyles?.severity === 'critical'
        ? 'bg-red-50 dark:bg-red-950'
        : 'bg-yellow-50 dark:bg-yellow-950'
      : '';
    const defaultHover = hasUnacknowledgedAlert() ? '' : 'hover:bg-surface-hover';
    const stoppedDimming = !isRunning() ? 'opacity-60' : '';

    return [base, hover, defaultHover, alertBg, stoppedDimming].filter(Boolean).join(' ');
  });

  const firstCellIndent = createMemo(() =>
    props.isGroupedView ? GROUPED_FIRST_CELL_INDENT : DEFAULT_FIRST_CELL_INDENT,
  );

  const dockerHostId = createMemo(() =>
    isDockerManagedAppContainer(props.guest) ? getWorkloadDockerHostId(props.guest) : '',
  );

  return {
    cpuAnomaly,
    customUrl,
    diskAnomaly,
    diskIOEmphasis,
    diskPercent,
    diskRead,
    diskWrite,
    displayId,
    dockerHostId,
    dockerImage,
    firstCellIndent,
    guestId,
    guestStatus,
    hasDiskUsage,
    hasNetworkInterfaces,
    hasOsInfo,
    infoTooltip,
    infoValue,
    infrastructureHref,
    ipAddresses,
    isColVisible,
    isMobile,
    isOCIContainer,
    isPveWorkload,
    isRunning,
    lockLabel,
    memoryAnomaly,
    memoryPercentOnly,
    memoryTooltip,
    metricsKey,
    namespace,
    networkEmphasis,
    networkIn,
    networkInterfaces,
    networkOut,
    ociImage,
    osName,
    osVersion,
    alertAccentTone,
    supportsBackup,
    typeInfo,
    workloadType,
    contextLabel,
    clusterName,
    agentVersion,
    rowClass,
  };
}
