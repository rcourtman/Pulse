import { createSignal, createMemo, Show, For, onMount, onCleanup, createEffect } from 'solid-js';
import { useNavigate, useLocation } from '@solidjs/router';
import Toggle from '@/components/shared/Toggle';
import { Card } from '@/components/shared/Card';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { useCollapsedSections } from './Thresholds/hooks/useCollapsedSections';
import { TagInput } from '@/components/shared/TagInput';
import Server from 'lucide-solid/icons/server';
import Monitor from 'lucide-solid/icons/monitor';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Database from 'lucide-solid/icons/database';
import Archive from 'lucide-solid/icons/archive';
import Camera from 'lucide-solid/icons/camera';
import Mail from 'lucide-solid/icons/mail';
import Users from 'lucide-solid/icons/users';
import Boxes from 'lucide-solid/icons/boxes';
import { unwrap } from 'solid-js/store';
import type { Resource } from '@/types/resource';

// Workaround for eslint false-positive when `For` is used only in JSX
const __ensureForUsage = For;
void __ensureForUsage;

import type {
  RawOverrideConfig,
  PMGThresholdDefaults,
  SnapshotAlertConfig,
  BackupAlertConfig,
} from '@/types/alerts';
import { ResourceTable } from './ResourceTable';
import { BulkEditDialog } from './BulkEditDialog';
import type { GroupHeaderMeta, Resource as TableResource } from './ResourceTable';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { logger } from '@/utils/logger';
import type {
  OverrideType,
  OfflineState,
  Override,
  ThresholdsTableProps,
} from "@/features/alerts/thresholds/types";
import {
  PMG_THRESHOLD_COLUMNS,
  PMG_NORMALIZED_TO_KEY,
  PMG_KEY_TO_NORMALIZED,
  DEFAULT_SNAPSHOT_WARNING,
  DEFAULT_SNAPSHOT_CRITICAL,
  DEFAULT_SNAPSHOT_WARNING_SIZE,
  DEFAULT_SNAPSHOT_CRITICAL_SIZE,
  DEFAULT_BACKUP_WARNING,
  DEFAULT_BACKUP_CRITICAL,
  DEFAULT_BACKUP_FRESH_HOURS,
  DEFAULT_BACKUP_STALE_HOURS,
} from "@/features/alerts/thresholds/constants";
import { normalizeDockerIgnoredInput, formatMetricValue } from "@/features/alerts/thresholds/helpers";

export function ThresholdsTable(props: ThresholdsTableProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');

  const pd = (r: Resource): Record<string, unknown> | undefined =>
    r.platformData ? (unwrap(r.platformData) as Record<string, unknown>) : undefined;

  // Collapsible section state management
  const { isCollapsed, toggleSection, expandAll, collapseAll } = useCollapsedSections();

  // Help banner dismiss state (persisted to localStorage)
  const HELP_BANNER_KEY = 'pulse-thresholds-help-dismissed';
  const [helpBannerDismissed, setHelpBannerDismissed] = createSignal(
    typeof window !== 'undefined' && localStorage.getItem(HELP_BANNER_KEY) === 'true'
  );
  const dismissHelpBanner = () => {
    setHelpBannerDismissed(true);
    if (typeof window !== 'undefined') {
      localStorage.setItem(HELP_BANNER_KEY, 'true');
    }
  };

  const [searchTerm, setSearchTerm] = createSignal('');
  const [editingId, setEditingId] = createSignal<string | null>(null);
  const [editingThresholds, setEditingThresholds] = createSignal<
    Record<string, number | undefined>
  >({});
  const [editingNote, setEditingNote] = createSignal('');

  const [bulkEditIds, setBulkEditIds] = createSignal<string[]>([]);
  const [bulkEditColumns, setBulkEditColumns] = createSignal<string[]>([]);
  const [isBulkEditDialogOpen, setIsBulkEditDialogOpen] = createSignal(false);

  const [activeTab, setActiveTab] = createSignal<'proxmox' | 'pmg' | 'hosts' | 'docker'>('proxmox');
  let searchInputRef: HTMLInputElement | undefined;
  const [dockerIgnoredInput, setDockerIgnoredInput] = createSignal(
    props.dockerIgnoredPrefixes().join('\n'),
  );
  const serviceWarnInputId = 'docker-service-warn-gap';
  const serviceCriticalInputId = 'docker-service-critical-gap';

  createEffect(() => {
    const remote = props.dockerIgnoredPrefixes();
    const local = dockerIgnoredInput();
    const normalizedLocal = normalizeDockerIgnoredInput(local);

    const isSynced =
      remote.length === normalizedLocal.length &&
      remote.every((val, i) => val === normalizedLocal[i]);

    if (!isSynced) {
      setDockerIgnoredInput(remote.join('\n'));
    }
  });

  const serviceGapValidationMessage = createMemo(() => {
    const warn = Number(props.dockerDefaults.serviceWarnGapPercent ?? 0);
    const crit = Number(props.dockerDefaults.serviceCriticalGapPercent ?? 0);
    if (crit > 0 && warn > crit) {
      return 'Critical gap must be greater than or equal to the warning gap when enabled.';
    }
    return '';
  });

  // Determine active tab from URL
  const getActiveTabFromRoute = (): 'proxmox' | 'pmg' | 'hosts' | 'docker' => {
    const path = location.pathname;
    if (path.includes('/thresholds/containers')) return 'docker';
    if (path.includes('/thresholds/docker')) return 'docker'; // Legacy support
    if (path.includes('/thresholds/hosts')) return 'hosts';
    if (path.includes('/thresholds/mail-gateway')) return 'pmg';
    return 'proxmox'; // default
  };

  // Sync active tab with route on mount and route changes
  createEffect(() => {
    const tabFromRoute = getActiveTabFromRoute();
    if (activeTab() !== tabFromRoute) {
      setActiveTab(tabFromRoute);
    }
  });

  // Handle default redirect - if at /alerts/thresholds exactly, redirect to /alerts/thresholds/proxmox
  createEffect(() => {
    if (location.pathname === '/alerts/thresholds') {
      navigate('/alerts/thresholds/proxmox', { replace: true });
    }
  });

  createEffect(() => {
    if (location.pathname.startsWith('/alerts/thresholds/docker')) {
      navigate(
        location.pathname.replace('/alerts/thresholds/docker', '/alerts/thresholds/containers'),
        { replace: true, scroll: false },
      );
    }
  });

  const handleTabClick = (tab: 'proxmox' | 'pmg' | 'hosts' | 'docker') => {
    const tabRoutes = {
      proxmox: '/alerts/thresholds/proxmox',
      pmg: '/alerts/thresholds/mail-gateway',
      hosts: '/alerts/thresholds/hosts',
      docker: '/alerts/thresholds/containers',
    };
    navigate(tabRoutes[tab]);
  };

  const handleDockerIgnoredChange = (value: string) => {
    setDockerIgnoredInput(value);
    const normalized = normalizeDockerIgnoredInput(value);
    props.setDockerIgnoredPrefixes(normalized);
    props.setHasUnsavedChanges(true);
  };

  const handleResetDockerIgnored = () => {
    if (props.resetDockerIgnoredPrefixes) {
      props.resetDockerIgnoredPrefixes();
    } else {
      props.setDockerIgnoredPrefixes([]);
    }
    setDockerIgnoredInput('');
    props.setHasUnsavedChanges(true);
  };



  // Set up keyboard shortcuts
  onMount(() => {
    const isEditableElement = (el: HTMLElement | null | undefined): boolean => {
      if (!el) return false;
      const tag = el.tagName;
      return (
        tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.contentEditable === 'true'
      );
    };

    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      const activeElement = (document.activeElement as HTMLElement) ?? null;
      const inEditable = isEditableElement(target);

      // Ctrl/Cmd+F to focus search
      if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
        e.preventDefault();
        if (searchInputRef) {
          searchInputRef.focus();
          searchInputRef.select();
        }
        return;
      }

      if (e.key === 'Escape') {
        if (searchTerm()) {
          e.preventDefault();
          setSearchTerm('');
        }
        if (searchInputRef && document.activeElement === searchInputRef) {
          searchInputRef.blur();
        }
        return;
      }

      if (e.defaultPrevented || inEditable || isEditableElement(activeElement) || editingId()) {
        return;
      }

      if (e.key.length === 1 && e.key.match(/[a-z0-9]/i)) {
        e.preventDefault();
        if (searchInputRef) {
          searchInputRef.focus();
          setSearchTerm(e.key);
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);

    onCleanup(() => {
      document.removeEventListener('keydown', handleKeyDown);
    });
  });



  // Check if there's an active alert for a resource/metric
  const hasActiveAlert = (resourceId: string, metric: string): boolean => {
    if (!alertsEnabled()) return false;
    if (!props.activeAlerts) return false;
    const alertKey = `${resourceId}-${metric}`;
    return alertKey in props.activeAlerts;
  };

  // Process nodes with their overrides
  const getFriendlyNodeName = (value: string, clusterName?: string): string => {
    if (!value) return value;

    const clusterLower = clusterName?.toLowerCase().trim();

    const normalizeToken = (token?: string | null): string => {
      if (!token) return '';
      let result = token
        .replace(/\(.*?\)/g, ' ')
        .replace(/\s+/g, ' ')
        .trim();
      if (clusterLower) {
        result = result
          .split(' ')
          .filter((part) => part.toLowerCase() !== clusterLower)
          .join(' ')
          .trim();
      }
      if (!result) return '';
      const firstWord = result.split(/\s+/)[0] || result;
      const withoutDomain = firstWord.includes('.')
        ? (firstWord.split('.')[0] ?? firstWord)
        : firstWord;
      return withoutDomain.trim();
    };

    const parentheticalMatch = value.match(/\(([^)]+)\)/);
    const parentheticalRaw = parentheticalMatch?.[1]?.trim();

    let base = normalizeToken(value);
    if (!base) {
      base = value.trim();
    }

    const parenthetical = normalizeToken(parentheticalRaw);
    if (parenthetical && parenthetical.toLowerCase() !== base.toLowerCase()) {
      return parenthetical;
    }

    return base;
  };

  const buildNodeHeaderMeta = (node: Resource) => {
    const data = pd(node);
    const clusterName = (data?.clusterName as string | undefined) ?? undefined;
    const isClusterMember =
      (data?.isClusterMember as boolean | undefined) ?? Boolean(node.clusterId);

    const originalDisplayName = node.displayName?.trim() || node.name;
    const friendlyName = getFriendlyNodeName(originalDisplayName, clusterName);

    // Prioritize guestURL over host (same as NodeGroupHeader)
    const guestUrlValue =
      typeof data?.guestURL === 'string' ? (data.guestURL as string).trim() : '';
    const hostValue = typeof data?.host === 'string' ? (data.host as string).trim() : '';

    let host: string | undefined;
    if (guestUrlValue && guestUrlValue !== '') {
      host = guestUrlValue.startsWith('http') ? guestUrlValue : `https://${guestUrlValue}`;
    } else if (hostValue && hostValue !== '') {
      host = hostValue.startsWith('http')
        ? hostValue
        : `https://${hostValue.includes(':') ? hostValue : `${hostValue}:8006`}`;
    } else if (node.name) {
      host = `https://${node.name.includes(':') ? node.name : `${node.name}:8006`}`;
    }

    const headerMeta: GroupHeaderMeta = {
      type: 'node',
      displayName: friendlyName,
      rawName: originalDisplayName,
      host,
      status: node.status,
      clusterName: isClusterMember ? clusterName?.trim() || 'Cluster' : undefined,
      isClusterMember,
    };

    const keys = new Set<string>();
    [node.name, originalDisplayName, friendlyName].forEach((value) => {
      if (value && value.trim()) {
        keys.add(value.trim());
      }
    });

    return { headerMeta, keys };
  };

  const nodesWithOverrides = createMemo<TableResource[]>((prev = []) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));

    const nodes = (props.nodes ?? []).map((node) => {
      const override = overridesMap.get(node.id);
      const data = pd(node);
      const clusterName = (data?.clusterName as string | undefined) ?? undefined;
      const isClusterMember =
        (data?.isClusterMember as boolean | undefined) ?? Boolean(node.clusterId);

      // Check if any threshold values actually differ from defaults
      const hasCustomThresholds =
        override?.thresholds &&
        Object.keys(override.thresholds).some((key) => {
          const k = key as keyof typeof override.thresholds;
          return (
            override.thresholds[k] !== undefined &&
            override.thresholds[k] !== (props.nodeDefaults as any)[k]
          );
        });

      const note = typeof override?.note === 'string' ? override.note : undefined;
      const hasNote = Boolean(note && note.trim().length > 0);

      const originalDisplayName = node.displayName?.trim() || node.name;
      const friendlyName = getFriendlyNodeName(originalDisplayName, clusterName);
      const rawName = node.name;
      const sanitizedName = friendlyName || originalDisplayName || rawName.split('.')[0] || rawName;
      // Build a best-effort management URL for the node
      // Prioritize guestURL over host (same as NodeGroupHeader)
      const guestUrlValue =
        typeof data?.guestURL === 'string' ? (data.guestURL as string).trim() : '';
      const hostValue =
        (typeof data?.host === 'string' ? (data.host as string).trim() : '') || rawName;
      let normalizedHost: string;
      if (guestUrlValue && guestUrlValue !== '') {
        normalizedHost = guestUrlValue.startsWith('http') ? guestUrlValue : `https://${guestUrlValue}`;
      } else {
        normalizedHost =
          hostValue.startsWith('http://') || hostValue.startsWith('https://')
            ? hostValue
            : `https://${hostValue.includes(':') ? hostValue : `${hostValue}:8006`}`;
      }

      return {
        id: node.id,
        name: sanitizedName,
        displayName: sanitizedName,
        rawName: originalDisplayName,
        host: normalizedHost,
        type: 'node' as const,
        resourceType: 'Node',
        status: node.status,
        uptime: node.uptime,
        cpu: (node.cpu?.current ?? 0) / 100,
        memory: node.memory?.current,
        hasOverride:
          hasCustomThresholds || hasNote || Boolean(override?.disableConnectivity) || false,
        disabled: false,
        disableConnectivity: override?.disableConnectivity || false,
        thresholds: override?.thresholds || {},
        defaults: props.nodeDefaults,
        clusterName: isClusterMember ? clusterName?.trim() : undefined,
        isClusterMember,
        instance: node.platformId,
        note,
      } satisfies TableResource;
    });

    if (search) {
      return nodes.filter((n) => n.name.toLowerCase().includes(search));
    }
    return nodes;
  }, []);

  const hostAgentsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));
    const seen = new Set<string>();

    const hosts: TableResource[] = (props.hosts ?? []).map((host) => {
      const override = overridesMap.get(host.id);
      const hasCustomThresholds =
        override?.thresholds &&
        Object.keys(override.thresholds).some((key) => {
          const k = key as keyof typeof override.thresholds;
          return (
            override.thresholds[k] !== undefined &&
            override.thresholds[k] !== (props.hostDefaults as any)[k]
          );
        });

      const displayName = host.displayName?.trim() || host.identity?.hostname || host.name || host.id;
      const status = host.status;

      seen.add(host.id);

      return {
        id: host.id,
        name: displayName,
        displayName,
        rawName: host.identity?.hostname ?? host.name,
        type: 'hostAgent' as const,
        resourceType: 'Host Agent',
        node: host.identity?.hostname ?? host.name,
        instance: (pd(host)?.platform as string) || (pd(host)?.osName as string) || '',
        status,
        hasOverride:
          hasCustomThresholds ||
          Boolean(override?.disabled) ||
          Boolean(override?.disableConnectivity),
        disabled: override?.disabled || false,
        disableConnectivity: override?.disableConnectivity || false,
        thresholds: override?.thresholds || {},
        defaults: props.hostDefaults,
      } satisfies TableResource;
    });

    (props.overrides() ?? [])
      .filter((override) => override.type === 'hostAgent' && !seen.has(override.id))
      .forEach((override) => {
        const name = override.name?.trim() || override.id;
        hosts.push({
          id: override.id,
          name,
          displayName: name,
          rawName: name,
          type: 'hostAgent' as const,
          resourceType: 'Host Agent',
          node: '',
          instance: '',
          status: 'unknown',
          hasOverride: true,
          disabled: override.disabled || false,
          disableConnectivity: override.disableConnectivity || false,
          thresholds: override.thresholds || {},
          defaults: props.hostDefaults,
        } satisfies TableResource);
      });

    if (search) {
      return hosts.filter((host) => host.name.toLowerCase().includes(search));
    }

    return hosts;
  }, []);

  // Helper function to create host disk resource ID (matches backend sanitizeHostComponent)
  const hostDiskResourceID = (hostId: string, mountpoint: string, device?: string): string => {
    // Use mountpoint if available, otherwise device
    let label = (mountpoint?.trim() || device?.trim() || 'disk').toLowerCase();
    // Replicate backend sanitizeHostComponent: keep a-z 0-9, replace everything else with '-', collapse consecutive hyphens
    label = label.replace(/[^a-z0-9]/g, '-').replace(/-{2,}/g, '-').replace(/^-|-$/g, '');
    if (!label) label = 'unknown';
    return `host:${hostId}/disk:${label}`;
  };

  // Process host disks with their overrides
  const hostDisksWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));
    const seen = new Set<string>();
    const disks: TableResource[] = [];

    // Extract disks from all hosts
    (props.hosts ?? []).forEach((host) => {
      const hostDisplayName =
        host.displayName?.trim() || host.identity?.hostname || host.name || host.id;

      const disksForHost =
        (pd(host)?.disks ?? []) as Array<{
          mountpoint?: string;
          device?: string;
          used?: number;
          total?: number;
          type?: string;
        }>;

      disksForHost.forEach((disk) => {
        const diskLabel = disk.mountpoint?.trim() || disk.device?.trim() || 'disk';
        const resourceId = hostDiskResourceID(host.id, disk.mountpoint || '', disk.device);
        const override = overridesMap.get(resourceId);

        const hasCustomThresholds =
          override?.thresholds?.disk !== undefined &&
          override.thresholds.disk !== props.hostDefaults.disk;

        seen.add(resourceId);

        disks.push({
          id: resourceId,
          name: diskLabel,
          displayName: diskLabel,
          rawName: disk.device || diskLabel,
          type: 'hostDisk' as const,
          resourceType: 'Host Disk',
          host: host.id,
          node: hostDisplayName,
          instance: disk.type || '',
          status: host.status,
          hasOverride: hasCustomThresholds || Boolean(override?.disabled),
          disabled: override?.disabled || false,
          thresholds: override?.thresholds || {},
          defaults: { disk: props.hostDefaults.disk },
          subtitle: `${((disk.used || 0) / 1024 / 1024 / 1024).toFixed(1)} / ${((disk.total || 0) / 1024 / 1024 / 1024).toFixed(1)} GB`,
        } satisfies TableResource);
      });
    });

    // Include any hostDisk overrides for disks that are no longer present
    (props.overrides() ?? [])
      .filter((override) => override.type === 'hostDisk' && !seen.has(override.id))
      .forEach((override) => {
        const name = override.name || override.id;
        disks.push({
          id: override.id,
          name,
          displayName: name,
          rawName: name,
          type: 'hostDisk' as const,
          resourceType: 'Host Disk',
          host: '',
          node: 'Unknown Host',
          instance: '',
          status: 'unknown',
          hasOverride: true,
          disabled: override.disabled || false,
          thresholds: override.thresholds || {},
          defaults: { disk: props.hostDefaults.disk },
        });
      });

    if (search) {
      return disks.filter(
        (d) => d.name.toLowerCase().includes(search) || d.node?.toLowerCase().includes(search),
      );
    }

    return disks;
  }, []);

  // Group host disks by their host
  const hostDisksGroupedByHost = createMemo<Record<string, TableResource[]>>(() => {
    const grouped: Record<string, TableResource[]> = {};
    hostDisksWithOverrides().forEach((disk) => {
      const key = disk.node?.trim() || 'Unknown Host';
      if (!grouped[key]) {
        grouped[key] = [];
      }
      grouped[key].push(disk);
    });

    // Sort disks within each host by name
    Object.values(grouped).forEach((resources) => {
      resources.sort((a, b) => a.name.localeCompare(b.name));
    });

    return grouped;
  });

  // Process Docker hosts with their overrides (primarily for connectivity toggles)
  const dockerHostsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));
    const seen = new Set<string>();

    const hosts: TableResource[] = (props.dockerHosts ?? []).map((host) => {
      const originalName = host.displayName?.trim() || host.identity?.hostname || host.name || host.id;
      const friendlyName = getFriendlyNodeName(originalName);
      const override = overridesMap.get(host.id);
      const disableConnectivity = override?.disableConnectivity || false;
      const status = host.status;

      seen.add(host.id);

      return {
        id: host.id,
        name: friendlyName,
        displayName: friendlyName,
        rawName: originalName,
        type: 'dockerHost' as const,
        resourceType: 'Container Host',
        node: host.identity?.hostname ?? host.name,
        instance: (pd(host)?.platform as string) || (pd(host)?.osName as string) || '',
        status,
        hasOverride: disableConnectivity,
        disableConnectivity,
        thresholds: override?.thresholds || {},
        defaults: {},
        editable: false,
      } satisfies TableResource;
    });

    // Include any overrides referencing Docker hosts that are no longer reporting
    (props.overrides() ?? [])
      .filter((override) => override.type === 'dockerHost' && !seen.has(override.id))
      .forEach((override) => {
        const originalName = override.name || override.id;
        const friendlyName = getFriendlyNodeName(originalName);
        hosts.push({
          id: override.id,
          name: friendlyName,
          displayName: friendlyName,
          rawName: originalName,
          type: 'dockerHost',
          resourceType: 'Container Host',
          node: override.node || '',
          instance: override.instance || '',
          status: 'unknown',
          hasOverride: true,
          disableConnectivity: override.disableConnectivity || false,
          thresholds: override.thresholds || {},
          defaults: {},
          editable: false,
        });
      });

    if (search) {
      return hosts.filter((host) => host.name.toLowerCase().includes(search));
    }
    return hosts;
  }, []);

  const dockerContainersByHostId = createMemo(() => {
    const map = new Map<string, Resource[]>();
    (props.allResources ?? []).forEach((resource) => {
      if (resource.type !== 'docker-container') return;
      const parentId = resource.parentId;
      if (!parentId) return;
      const existing = map.get(parentId);
      if (existing) {
        existing.push(resource);
      } else {
        map.set(parentId, [resource]);
      }
    });
    return map;
  });

  // Process Docker containers grouped by host
  const dockerContainersGroupedByHost = createMemo<Record<string, TableResource[]>>((prev = {}) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));
    const groups: Record<string, TableResource[]> = {};
    const seen = new Set<string>();

    (props.dockerHosts ?? []).forEach((host) => {
      const hostLabel = host.displayName?.trim() || host.identity?.hostname || host.name || host.id;
      const friendlyHostName = getFriendlyNodeName(hostLabel);
      const hostLabelLower = hostLabel.toLowerCase();
      const friendlyHostNameLower = friendlyHostName.toLowerCase();

      const hostHostname = host.identity?.hostname ?? host.name;
      const containers = dockerContainersByHostId().get(host.id) ?? [];

      containers.forEach((container) => {
        const shortId = container.id.includes('/') ? (container.id.split('/').pop() ?? container.id) : container.id;
        const resourceId = `docker:${host.id}/${shortId}`;
        const override = overridesMap.get(resourceId);
        const overrideSeverity = override?.poweredOffSeverity;

        const defaults = props.dockerDefaults as Record<string, number | undefined>;
        const hasCustomThresholds =
          override?.thresholds &&
          Object.keys(override.thresholds).some((key) => {
            const k = key as keyof typeof override.thresholds;
            return (
              override.thresholds[k] !== undefined &&
              override.thresholds[k] !== defaults?.[k as keyof typeof defaults]
            );
          });

        const hasOverride =
          hasCustomThresholds ||
          override?.disabled ||
          override?.disableConnectivity ||
          overrideSeverity !== undefined ||
          false;

        const containerName = container.name?.replace(/^\/+/, '') || shortId;
        const containerNameLower = containerName.toLowerCase();
        const image = (pd(container)?.image as string) ?? '';
        const imageLower = image.toLowerCase();

        const matchesSearch =
          !search ||
          containerNameLower.includes(search) ||
          hostLabelLower.includes(search) ||
          friendlyHostNameLower.includes(search) ||
          imageLower.includes(search);
        if (!matchesSearch) {
          return;
        }

        const status = container.status;
        const groupKey = friendlyHostName || hostLabel;

        const resource: TableResource = {
          id: resourceId,
          name: containerName,
          type: 'dockerContainer',
          resourceType: 'Container',
          node: groupKey,
          instance: hostHostname,
          status,
          hasOverride,
          disabled: override?.disabled || false,
          disableConnectivity: override?.disableConnectivity || false,
          thresholds: override?.thresholds || {},
          defaults: props.dockerDefaults,
          hostId: host.id,
          image,
          poweredOffSeverity: overrideSeverity,
        };

        if (!groups[groupKey]) {
          groups[groupKey] = [];
        }
        groups[groupKey].push(resource);
        seen.add(resourceId);
      });
    });

    // Include overrides for Docker containers that aren't currently reporting
    (props.overrides() ?? [])
      .filter((override) => override.type === 'dockerContainer' && !seen.has(override.id))
      .forEach((override) => {
        const fallbackName = override.name || override.id.split('/').pop() || override.id;
        const group = 'Unassigned Containers';
        if (!groups[group]) {
          groups[group] = [];
        }
        groups[group].push({
          id: override.id,
          name: fallbackName,
          type: 'dockerContainer',
          resourceType: 'Container',
          status: 'unknown',
          hasOverride: true,
          disabled: override.disabled || false,
          disableConnectivity: override.disableConnectivity || false,
          thresholds: override.thresholds || {},
          defaults: props.dockerDefaults,
          poweredOffSeverity: override.poweredOffSeverity,
        });
      });

    Object.keys(groups).forEach((group) => {
      groups[group].sort((a, b) => a.name.localeCompare(b.name));
    });

    if (!search) {
      return groups;
    }

    // With search applied, remove empty groups (should already be filtered)
    const filteredGroups: Record<string, TableResource[]> = {};
    Object.entries(groups).forEach(([group, resources]) => {
      if (resources.length > 0) {
        filteredGroups[group] = resources;
      }
    });
    return filteredGroups;
  }, {});

  const dockerContainersFlat = createMemo<TableResource[]>(() =>
    Object.values(dockerContainersGroupedByHost() ?? {}).flat(),
  );

  const totalDockerContainers = createMemo(() =>
    (props.dockerHosts ?? []).reduce(
      (sum, host) => sum + (dockerContainersByHostId().get(host.id)?.length ?? 0),
      0,
    ),
  );

  const dockerHostGroupMeta = createMemo<Record<string, GroupHeaderMeta>>(() => {
    const meta: Record<string, GroupHeaderMeta> = {};
    (props.dockerHosts ?? []).forEach((host) => {
      const originalName = host.displayName?.trim() || host.identity?.hostname || host.name || host.id;
      const friendlyName = getFriendlyNodeName(originalName);
      const headerMeta: GroupHeaderMeta = {
        displayName: friendlyName,
        rawName: originalName,
        status: host.status,
      };

      const hostname = host.identity?.hostname ?? host.name;
      [friendlyName, originalName, hostname, host.id]
        .filter((key): key is string => Boolean(key && key.trim()))
        .forEach((key) => {
          meta[key.trim()] = headerMeta;
        });
    });

    meta['Unassigned Containers'] = {
      displayName: 'Unassigned Containers',
      status: 'unknown',
    };

    return meta;
  });

  const countOverrides = (resources: TableResource[] | undefined) =>
    resources?.filter(
      (resource) => resource.hasOverride || resource.disabled || resource.disableConnectivity,
    ).length ?? 0;

  const registerSection = (_key: string) => (_el: HTMLDivElement | null) => {
    /* no-op placeholder for future scroll restoration */
  };

  const snapshotFactoryConfig = () =>
    props.snapshotFactoryDefaults ?? {
      enabled: false,
      warningDays: DEFAULT_SNAPSHOT_WARNING,
      criticalDays: DEFAULT_SNAPSHOT_CRITICAL,
      warningSizeGiB: DEFAULT_SNAPSHOT_WARNING_SIZE,
      criticalSizeGiB: DEFAULT_SNAPSHOT_CRITICAL_SIZE,
    };

  const sanitizeSnapshotConfig = (config: SnapshotAlertConfig): SnapshotAlertConfig => {
    let warning = Math.max(0, Math.round(config.warningDays ?? 0));
    let critical = Math.max(0, Math.round(config.criticalDays ?? 0));

    if (critical > 0 && warning > critical) {
      warning = critical;
    }
    if (critical === 0 && warning > 0) {
      critical = warning;
    }

    const rawWarningSize = Number.isFinite(config.warningSizeGiB)
      ? Number(config.warningSizeGiB)
      : DEFAULT_SNAPSHOT_WARNING_SIZE;
    const rawCriticalSize = Number.isFinite(config.criticalSizeGiB)
      ? Number(config.criticalSizeGiB)
      : DEFAULT_SNAPSHOT_CRITICAL_SIZE;

    const roundSize = (value: number) => Math.round(Math.max(0, value) * 10) / 10;

    let warningSize = roundSize(rawWarningSize);
    let criticalSize = roundSize(rawCriticalSize);

    if (criticalSize > 0 && warningSize > criticalSize) {
      warningSize = criticalSize;
    }
    if (criticalSize === 0 && warningSize > 0) {
      criticalSize = warningSize;
    }

    return {
      enabled: !!config.enabled,
      warningDays: warning,
      criticalDays: critical,
      warningSizeGiB: warningSize,
      criticalSizeGiB: criticalSize,
    };
  };

  const updateSnapshotDefaults = (
    updater: SnapshotAlertConfig | ((prev: SnapshotAlertConfig) => SnapshotAlertConfig),
  ) => {
    props.setSnapshotDefaults((prev) => {
      const next =
        typeof updater === 'function'
          ? (updater as (prev: SnapshotAlertConfig) => SnapshotAlertConfig)(prev)
          : { ...prev, ...updater };
      return sanitizeSnapshotConfig(next);
    });
    props.setHasUnsavedChanges(true);
  };

  const snapshotDefaultsRecord = createMemo(() => {
    const current = props.snapshotDefaults();
    return {
      'warning days': current.warningDays ?? 0,
      'critical days': current.criticalDays ?? 0,
      'warning size (gib)': current.warningSizeGiB ?? 0,
      'critical size (gib)': current.criticalSizeGiB ?? 0,
    };
  });

  const snapshotFactoryDefaultsRecord = createMemo(() => {
    const factory = snapshotFactoryConfig();
    return {
      'warning days': factory.warningDays ?? DEFAULT_SNAPSHOT_WARNING,
      'critical days': factory.criticalDays ?? DEFAULT_SNAPSHOT_CRITICAL,
      'warning size (gib)': factory.warningSizeGiB ?? DEFAULT_SNAPSHOT_WARNING_SIZE,
      'critical size (gib)': factory.criticalSizeGiB ?? DEFAULT_SNAPSHOT_CRITICAL_SIZE,
    };
  });

  const backupFactoryConfig = () =>
    props.backupFactoryDefaults ?? {
      enabled: false,
      warningDays: DEFAULT_BACKUP_WARNING,
      criticalDays: DEFAULT_BACKUP_CRITICAL,
      freshHours: DEFAULT_BACKUP_FRESH_HOURS,
      staleHours: DEFAULT_BACKUP_STALE_HOURS,
      alertOrphaned: true,
      ignoreVMIDs: [],
    };

  const sanitizeBackupConfig = (config: BackupAlertConfig): BackupAlertConfig => {
    let warning = Math.max(0, Math.round(config.warningDays ?? 0));
    let critical = Math.max(0, Math.round(config.criticalDays ?? 0));
    let fresh = Math.max(0, Math.round(config.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS));
    let stale = Math.max(0, Math.round(config.staleHours ?? DEFAULT_BACKUP_STALE_HOURS));
    const alertOrphaned = config.alertOrphaned ?? true;
    const ignoreVMIDs = Array.from(
      new Set(
        (config.ignoreVMIDs ?? [])
          .map((value) => value.trim())
          .filter((value) => value.length > 0),
      ),
    );

    if (critical > 0 && warning > critical) {
      warning = critical;
    }
    if (critical === 0 && warning > 0) {
      critical = warning;
    }

    // Ensure stale is at least fresh
    if (stale < fresh) {
      stale = fresh;
    }

    return {
      enabled: !!config.enabled,
      warningDays: warning,
      criticalDays: critical,
      freshHours: fresh,
      staleHours: stale,
      alertOrphaned,
      ignoreVMIDs,
    };
  };

  const updateBackupDefaults = (
    updater: BackupAlertConfig | ((prev: BackupAlertConfig) => BackupAlertConfig),
  ) => {
    props.setBackupDefaults((prev) => {
      const next =
        typeof updater === 'function'
          ? (updater as (prev: BackupAlertConfig) => BackupAlertConfig)(prev)
          : { ...prev, ...updater };
      return sanitizeBackupConfig(next);
    });
    props.setHasUnsavedChanges(true);
  };

  const backupDefaultsRecord = createMemo(() => {
    const current = props.backupDefaults();
    return {
      'fresh hours': current.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS,
      'stale hours': current.staleHours ?? DEFAULT_BACKUP_STALE_HOURS,
      'warning days': current.warningDays ?? 0,
      'critical days': current.criticalDays ?? 0,
    };
  });

  const backupFactoryDefaultsRecord = createMemo(() => {
    const factory = backupFactoryConfig();
    return {
      'fresh hours': factory.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS,
      'stale hours': factory.staleHours ?? DEFAULT_BACKUP_STALE_HOURS,
      'warning days': factory.warningDays ?? DEFAULT_BACKUP_WARNING,
      'critical days': factory.criticalDays ?? DEFAULT_BACKUP_CRITICAL,
    };
  });

  const snapshotOverridesCount = createMemo(() => {
    const current = props.snapshotDefaults();
    const factory = snapshotFactoryConfig();
    const differs =
      current.enabled !== factory.enabled ||
      (current.warningDays ?? DEFAULT_SNAPSHOT_WARNING) !==
      (factory.warningDays ?? DEFAULT_SNAPSHOT_WARNING) ||
      (current.criticalDays ?? DEFAULT_SNAPSHOT_CRITICAL) !==
      (factory.criticalDays ?? DEFAULT_SNAPSHOT_CRITICAL) ||
      (current.warningSizeGiB ?? DEFAULT_SNAPSHOT_WARNING_SIZE) !==
      (factory.warningSizeGiB ?? DEFAULT_SNAPSHOT_WARNING_SIZE) ||
      (current.criticalSizeGiB ?? DEFAULT_SNAPSHOT_CRITICAL_SIZE) !==
      (factory.criticalSizeGiB ?? DEFAULT_SNAPSHOT_CRITICAL_SIZE);
    return differs ? 1 : 0;
  });

  const backupOverridesCount = createMemo(() => {
    const backupCurrent = props.backupDefaults();
    const backupFactory = backupFactoryConfig();
    const currentIgnore = backupCurrent.ignoreVMIDs ?? [];
    const factoryIgnore = backupFactory.ignoreVMIDs ?? [];
    const ignoreDiff =
      currentIgnore.length !== factoryIgnore.length ||
      currentIgnore.some((value, index) => value !== factoryIgnore[index]);
    return backupCurrent.enabled !== backupFactory.enabled ||
      (backupCurrent.warningDays ?? DEFAULT_BACKUP_WARNING) !==
      (backupFactory.warningDays ?? DEFAULT_BACKUP_WARNING) ||
      (backupCurrent.criticalDays ?? DEFAULT_BACKUP_CRITICAL) !==
      (backupFactory.criticalDays ?? DEFAULT_BACKUP_CRITICAL) ||
      (backupCurrent.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS) !==
      (backupFactory.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS) ||
      (backupCurrent.staleHours ?? DEFAULT_BACKUP_STALE_HOURS) !==
      (backupFactory.staleHours ?? DEFAULT_BACKUP_STALE_HOURS) ||
      (backupCurrent.alertOrphaned ?? true) !== (backupFactory.alertOrphaned ?? true) ||
      ignoreDiff
      ? 1
      : 0;
  });

  // Process guests with their overrides and group by node
  const guestsGroupedByNode = createMemo<Record<string, TableResource[]>>((prev = {}) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));

    const guests = (props.allGuests() ?? []).map((guest) => {
      const gpd = guest.platformData ? (unwrap(guest.platformData) as Record<string, unknown>) : undefined;
      const vmid = (gpd?.vmid as number | undefined) ?? undefined;
      const node = (gpd?.node as string | undefined) ?? '';
      const instance = (gpd?.instance as string | undefined) ?? guest.platformId ?? '';
      const guestId = guest.id;
      const override = overridesMap.get(guestId);
      const overrideSeverity = override?.poweredOffSeverity;

      // Check if any threshold values actually differ from defaults
      const hasCustomThresholds =
        override?.thresholds &&
        Object.keys(override.thresholds).some((key) => {
          const k = key as keyof typeof override.thresholds;
          return (
            override.thresholds[k] !== undefined &&
            override.thresholds[k] !== (props.guestDefaults as any)[k]
          );
        });

      // A guest has an override if it has custom thresholds OR is disabled OR has connectivity disabled
      const hasOverride =
        hasCustomThresholds ||
        override?.disabled ||
        override?.disableConnectivity ||
        overrideSeverity !== undefined ||
        false;

      return {
        id: guestId,
        name: guest.name,
        type: 'guest' as const,
        resourceType: guest.type === 'vm' ? 'VM' : 'CT',
        vmid,
        node,
        instance,
        status: guest.status,
        hasOverride: hasOverride,
        disabled: override?.disabled || false,
        disableConnectivity: override?.disableConnectivity || false,
        thresholds: override?.thresholds || {},
        defaults: props.guestDefaults,
        backup: override?.backup || props.backupDefaults(),
        snapshot: override?.snapshot || props.snapshotDefaults(),
        poweredOffSeverity: overrideSeverity,
      };
    });

    const filteredGuests = search
      ? guests.filter(
        (g) =>
          g.name.toLowerCase().includes(search) ||
          g.vmid?.toString().includes(search) ||
          g.node?.toLowerCase().includes(search),
      )
      : guests;

    // Group by instance (not node - node is just the hostname which may be duplicated)
    // Instance is the disambiguated name like "px1" or "px1 (10.0.2.224)"
    const grouped: Record<string, TableResource[]> = {};
    filteredGuests.forEach((guest) => {
      const groupKey = guest.instance || guest.node || 'Unknown';
      if (!grouped[groupKey]) {
        grouped[groupKey] = [];
      }
      grouped[groupKey].push(guest);
    });

    // Sort guests within each group by vmid
    Object.keys(grouped).forEach((node) => {
      grouped[node].sort((a, b) => {
        if (a.vmid && b.vmid) return a.vmid - b.vmid;
        return a.name.localeCompare(b.name);
      });
    });

    return grouped;
  }, {});

  const guestsFlat = createMemo<TableResource[]>(() =>
    Object.values(guestsGroupedByNode() ?? {}).flat(),
  );

  const guestGroupHeaderMeta = createMemo<Record<string, GroupHeaderMeta>>(() => {
    const meta: Record<string, GroupHeaderMeta> = {};
    (props.nodes ?? []).forEach((node) => {
      const { headerMeta, keys } = buildNodeHeaderMeta(node);
      keys.forEach((key) => {
        meta[key] = headerMeta;
      });
    });
    return meta;
  });

  // Process PBS servers with their overrides
  const pbsServersWithOverrides = createMemo<TableResource[]>((prev = []) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));

    // Get PBS instances from props
    const pbsInstances = props.pbsInstances || [];

    const pbsServers = pbsInstances.map((pbs) => {
      // Offline PBS instances report zero metrics; keep them visible so connectivity toggles stay usable
      // PBS IDs already have "pbs-" prefix from backend, don't double it
      const pbsId = pbs.id;
      const override = overridesMap.get(pbsId);

      // Check if any threshold values actually differ from defaults
      const hasCustomThresholds =
        override?.thresholds &&
        Object.keys(override.thresholds).some((key) => {
          const k = key as keyof typeof override.thresholds;
          // PBS uses pbsDefaults for CPU/Memory (not nodeDefaults)
          return (
            override.thresholds[k] !== undefined &&
            override.thresholds[k] !== (props.pbsDefaults?.[k as keyof typeof props.pbsDefaults] ?? (k === 'cpu' ? 80 : 85))
          );
        });

      const disableConnectivity = override?.disableConnectivity || false;
      const hasOverride = hasCustomThresholds || disableConnectivity;

      return {
        id: pbsId,
        name: pbs.name,
        type: 'pbs' as const,
        resourceType: 'PBS',
        host: pbs.host,
        status: pbs.status,
        cpu: pbs.cpu,
        memory: pbs.memory,
        memoryUsed: pbs.memoryUsed,
        memoryTotal: pbs.memoryTotal,
        uptime: pbs.uptime,
        hasOverride,
        disabled: false,
        disableConnectivity,
        thresholds: override?.thresholds || {},
        defaults: {
          cpu: props.pbsDefaults?.cpu ?? 80,
          memory: props.pbsDefaults?.memory ?? 85,
        },
      };
    });

    if (search) {
      return pbsServers.filter(
        (p) => p.name.toLowerCase().includes(search) || p.host?.toLowerCase().includes(search),
      );
    }
    return pbsServers;
  }, []);

  const pmgGlobalDefaults = createMemo<Record<string, number>>(() => {
    const defaults = props.pmgThresholds();
    const record: Record<string, number> = {};
    PMG_THRESHOLD_COLUMNS.forEach(({ key, normalized }) => {
      const value = defaults[key];
      record[normalized] = typeof value === 'number' && Number.isFinite(value) ? value : 0;
    });
    return record;
  });

  const setPMGGlobalDefaults = (
    value:
      | Record<string, number | undefined>
      | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
  ) => {
    const current = pmgGlobalDefaults();
    const nextRecord =
      typeof value === 'function' ? value({ ...current }) : { ...current, ...value };

    let changed = false;
    props.setPMGThresholds((prev: PMGThresholdDefaults) => {
      const updated: PMGThresholdDefaults = { ...prev };
      PMG_THRESHOLD_COLUMNS.forEach(({ key, normalized }) => {
        const raw = nextRecord[normalized];
        if (typeof raw === 'number' && !Number.isNaN(raw)) {
          const sanitized = Math.max(0, Math.round(raw));
          if (updated[key] !== sanitized) {
            updated[key] = sanitized;
            changed = true;
          }
        }
      });
      return updated;
    });

    if (changed) {
      props.setHasUnsavedChanges(true);
    }
  };

  // Process PMG servers with their overrides
  const pmgServersWithOverrides = createMemo<TableResource[]>((prev = []) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));

    // Get PMG instances from props
    const pmgInstances = props.pmgInstances || [];
    const defaultThresholds = pmgGlobalDefaults();

    const pmgServers = pmgInstances.map((pmg) => {
      // PMG IDs should already have appropriate prefix from backend
      const pmgId = pmg.id;
      const override = overridesMap.get(pmgId);

      const thresholdOverrides: Record<string, number> = {};
      const overrideThresholds = (override?.thresholds ?? {}) as Record<string, unknown>;
      Object.entries(overrideThresholds).forEach(([rawKey, rawValue]) => {
        if (typeof rawValue !== 'number' || Number.isNaN(rawValue)) return;
        const normalizedKey =
          PMG_KEY_TO_NORMALIZED.get(rawKey as keyof PMGThresholdDefaults) ||
          (PMG_NORMALIZED_TO_KEY.has(rawKey) ? rawKey : undefined);
        if (!normalizedKey) return;
        thresholdOverrides[normalizedKey] = rawValue;
      });

      const hasOverride =
        override?.disableConnectivity ||
        override?.disabled ||
        Object.keys(thresholdOverrides).length > 0 ||
        false;

      return {
        id: pmgId,
        name: pmg.name,
        type: 'pmg' as const,
        resourceType: 'PMG',
        host: pmg.host,
        status: pmg.status,
        hasOverride,
        disabled: override?.disabled || false,
        disableConnectivity: override?.disableConnectivity || false,
        thresholds: thresholdOverrides,
        defaults: { ...defaultThresholds },
      };
    });

    if (search) {
      return pmgServers.filter(
        (p) => p.name.toLowerCase().includes(search) || p.host?.toLowerCase().includes(search),
      );
    }
    return pmgServers;
  }, []);

  const storageCoords = (r: Resource): { node: string; instance: string } => {
    const data = pd(r);
    if (r.type === 'datastore') {
      const instance =
        (data?.pbsInstanceId as string | undefined) || r.parentId || r.platformId || 'pbs';
      const node = (data?.pbsInstanceName as string | undefined) || instance;
      return { node, instance };
    }
    return {
      node: (data?.node as string | undefined) || '',
      instance: (data?.instance as string | undefined) || r.platformId || '',
    };
  };

  const normalizeStorageStatus = (status: string | undefined): string => {
    switch ((status ?? '').toLowerCase()) {
      case 'online':
      case 'running':
      case 'available':
        return 'available';
      default:
        return 'offline';
    }
  };

  // Process storage with their overrides
  const storageWithOverrides = createMemo<TableResource[]>((prev = []) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));

    const storageDevices = (props.storage ?? []).map((storage) => {
      const override = overridesMap.get(storage.id);
      const coords = storageCoords(storage);

      // Storage only has usage threshold
      const hasCustomThresholds =
        override?.thresholds?.usage !== undefined &&
        override.thresholds.usage !== props.storageDefault();

      // A storage device has an override if it has custom thresholds OR is disabled
      const hasOverride = hasCustomThresholds || override?.disabled || false;

      return {
        id: storage.id,
        name: storage.name,
        type: 'storage' as const,
        resourceType: 'Storage',
        node: coords.node,
        instance: coords.instance,
        status: normalizeStorageStatus(storage.status),
        hasOverride: hasOverride,
        disabled: override?.disabled || false,
        thresholds: override?.thresholds || {},
        defaults: { usage: props.storageDefault() },
      };
    });

    if (search) {
      return storageDevices.filter(
        (s) => s.name.toLowerCase().includes(search) || s.node?.toLowerCase().includes(search),
      );
    }
    return storageDevices;
  }, []);

  const storageGroupedByNode = createMemo<Record<string, TableResource[]>>(() => {
    const grouped: Record<string, TableResource[]> = {};
    storageWithOverrides().forEach((storage) => {
      const key = storage.node?.trim() || 'Unassigned';
      if (!grouped[key]) {
        grouped[key] = [];
      }
      grouped[key].push(storage);
    });

    Object.values(grouped).forEach((resources) => {
      resources.sort((a, b) => a.name.localeCompare(b.name));
    });

    return grouped;
  });

  const summaryItems = createMemo(() => {
    try {
      const items = [
        {
          key: 'nodes' as const,
          label: 'Nodes',
          total: props.nodes?.length ?? 0,
          overrides: countOverrides(nodesWithOverrides()),
          tab: 'proxmox' as const,
        },
        {
          key: 'dockerHosts' as const,
          label: 'Container Hosts',
          total: props.dockerHosts?.length ?? 0,
          overrides: countOverrides(dockerHostsWithOverrides()),
          tab: 'docker' as const,
        },
        {
          key: 'hostAgents' as const,
          label: 'Host Agents',
          total: props.hosts?.length ?? 0,
          overrides: countOverrides(hostAgentsWithOverrides()),
          tab: 'hosts' as const,
        },
        {
          key: 'hostDisks' as const,
          label: 'Host Disks',
          total: hostDisksWithOverrides().length,
          overrides: countOverrides(hostDisksWithOverrides()),
          tab: 'hosts' as const,
        },
        {
          key: 'storage' as const,
          label: 'Storage',
          total: props.storage?.length ?? 0,
          overrides: countOverrides(storageWithOverrides()),
          tab: 'proxmox' as const,
        },
        {
          key: 'backups' as const,
          label: 'Recovery',
          total: 1,
          overrides: backupOverridesCount(),
          tab: 'proxmox' as const,
        },
        {
          key: 'snapshots' as const,
          label: 'Snapshot Age',
          total: 1,
          overrides: snapshotOverridesCount(),
          tab: 'proxmox' as const,
        },
        {
          key: 'pbs' as const,
          label: 'PBS Servers',
          total: props.pbsInstances?.length ?? 0,
          overrides: countOverrides(pbsServersWithOverrides()),
          tab: 'proxmox' as const,
        },
        {
          key: 'pmg' as const,
          label: 'Mail Gateways',
          total: props.pmgInstances?.length ?? 0,
          overrides: countOverrides(pmgServersWithOverrides()),
          tab: 'pmg' as const,
        },
        {
          key: 'dockerContainers' as const,
          label: 'Containers',
          total: totalDockerContainers() ?? 0,
          overrides: countOverrides(dockerContainersFlat()),
          tab: 'docker' as const,
        },
        {
          key: 'guests' as const,
          label: 'VMs & Containers',
          total: props.allGuests?.()?.length ?? 0,
          overrides: countOverrides(guestsFlat()),
          tab: 'proxmox' as const,
        },
      ];

      const filtered = items.filter((item) => item.total > 0 || item.overrides > 0);
      return filtered.filter((item) => item.tab === activeTab());
    } catch (err) {
      logger.error('Error in summaryItems memo:', err);
      return [];
    }
  });

  const hasSection = (key: string) => summaryItems()?.some((item) => item.key === key) ?? false;

  const startEditing = (
    resourceId: string,
    currentThresholds: Record<string, number | undefined>,
    defaults: Record<string, number | undefined>,
    note?: string,
  ) => {
    setEditingId(resourceId);
    // Merge defaults with overrides for editing
    const mergedThresholds = { ...defaults, ...currentThresholds };
    setEditingThresholds(mergedThresholds);
    setEditingNote(note ?? '');
  };

  const saveEdit = (resourceId: string) => {
    // Flatten grouped guests to find the resource
    const allGuests = guestsFlat();
    const allDockerContainers = dockerContainersFlat();
    const allResources = [
      ...nodesWithOverrides(),
      ...hostAgentsWithOverrides(),
      ...hostDisksWithOverrides(),
      ...dockerHostsWithOverrides(),
      ...allGuests,
      ...allDockerContainers,
      ...storageWithOverrides(),
      ...pbsServersWithOverrides(),
    ];
    const resource = allResources.find((r) => r.id === resourceId);
    if (!resource) return;

    const editedThresholds = editingThresholds();
    const trimmedNote = editingNote().trim();
    const noteForOverride = trimmedNote.length > 0 ? trimmedNote : undefined;

    if (resource.editScope === 'backup') {
      const currentBackupDefaults = props.backupDefaults();
      const nextWarning =
        editedThresholds['warning days'] ??
        currentBackupDefaults.warningDays ??
        DEFAULT_BACKUP_WARNING;
      const nextCritical =
        editedThresholds['critical days'] ??
        currentBackupDefaults.criticalDays ??
        DEFAULT_BACKUP_CRITICAL;

      updateBackupDefaults({
        enabled: currentBackupDefaults.enabled,
        warningDays: nextWarning,
        criticalDays: nextCritical,
      });

      cancelEdit();
      return;
    }

    if (resource.editScope === 'snapshot') {
      const currentSnapshotDefaults = props.snapshotDefaults();
      const nextWarning =
        editedThresholds['warning days'] ??
        currentSnapshotDefaults.warningDays ??
        DEFAULT_SNAPSHOT_WARNING;
      const nextCritical =
        editedThresholds['critical days'] ??
        currentSnapshotDefaults.criticalDays ??
        DEFAULT_SNAPSHOT_CRITICAL;
      const nextWarningSize =
        editedThresholds['warning size (gib)'] ??
        currentSnapshotDefaults.warningSizeGiB ??
        DEFAULT_SNAPSHOT_WARNING_SIZE;
      const nextCriticalSize =
        editedThresholds['critical size (gib)'] ??
        currentSnapshotDefaults.criticalSizeGiB ??
        DEFAULT_SNAPSHOT_CRITICAL_SIZE;

      updateSnapshotDefaults({
        enabled: currentSnapshotDefaults.enabled,
        warningDays: nextWarning,
        criticalDays: nextCritical,
        warningSizeGiB: nextWarningSize,
        criticalSizeGiB: nextCriticalSize,
      });

      cancelEdit();
      return;
    }

    const defaultThresholds = (resource.defaults ?? {}) as Record<string, number | undefined>;

    // Only include values that differ from defaults
    const overrideThresholds: Record<string, number> = {};
    Object.keys(editedThresholds).forEach((key) => {
      const editedValue = editedThresholds[key];
      const defaultValue = defaultThresholds[key as keyof typeof defaultThresholds];
      if (editedValue !== undefined && editedValue !== defaultValue) {
        overrideThresholds[key] = editedValue;
      }
    });

    // Find existing override to check for backup/snapshot fields
    const existingOverrideCheck = props.overrides().find((o) => o.id === resourceId);

    const hasStateOnlyOverride = Boolean(
      resource.disabled ||
      resource.disableConnectivity ||
      resource.poweredOffSeverity !== undefined ||
      noteForOverride !== undefined ||
      existingOverrideCheck?.backup ||
      existingOverrideCheck?.snapshot,
    );

    // If no threshold overrides or state flags remain, remove the override entirely
    if (Object.keys(overrideThresholds).length === 0 && !hasStateOnlyOverride) {
      // If there was an existing override, remove it
      if (resource.hasOverride) {
        const newOverrides = props.overrides().filter((o) => o.id !== resourceId);
        props.setOverrides(newOverrides);

        // Also remove from raw config
        const newRawConfig = { ...props.rawOverridesConfig() };
        delete newRawConfig[resourceId];
        props.setRawOverridesConfig(newRawConfig);
        props.setHasUnsavedChanges(true);
      }
      cancelEdit();
      return;
    }

    // Create or update override
    const override: Override = {
      id: resourceId,
      name: resource.name,
      type: resource.type as OverrideType,
      resourceType: resource.resourceType,
      vmid: 'vmid' in resource ? resource.vmid : undefined,
      node: 'node' in resource ? resource.node : undefined,
      instance: 'instance' in resource ? resource.instance : undefined,
      disabled: resource.disabled,
      disableConnectivity: resource.disableConnectivity,
      poweredOffSeverity: resource.poweredOffSeverity,
      note: noteForOverride,
      backup: existingOverrideCheck?.backup,
      snapshot: existingOverrideCheck?.snapshot,
      thresholds: overrideThresholds,
    };

    // Update overrides list
    const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
    if (existingIndex >= 0) {
      const newOverrides = [...props.overrides()];
      newOverrides[existingIndex] = override;
      props.setOverrides(newOverrides);
    } else {
      props.setOverrides([...props.overrides(), override]);
    }

    // Update raw config
    const newRawConfig: Record<string, RawOverrideConfig> = { ...props.rawOverridesConfig() };
    const previousRaw = props.rawOverridesConfig()[resourceId];
    const hysteresisThresholds: RawOverrideConfig = {};
    if (previousRaw) {
      if (previousRaw.disabled !== undefined) {
        hysteresisThresholds.disabled = previousRaw.disabled;
      }
      if (previousRaw.disableConnectivity !== undefined) {
        hysteresisThresholds.disableConnectivity = previousRaw.disableConnectivity;
      }
      if (previousRaw.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = previousRaw.poweredOffSeverity;
      }
    }
    Object.entries(overrideThresholds).forEach(([metric, value]) => {
      if (value !== undefined && value !== null) {
        hysteresisThresholds[metric] = {
          trigger: value,
          clear: Math.max(0, value - 5),
        };
      }
    });
    if (resource.disabled) {
      hysteresisThresholds.disabled = true;
    } else {
      delete hysteresisThresholds.disabled;
    }
    if (resource.disableConnectivity) {
      hysteresisThresholds.disableConnectivity = true;
      delete hysteresisThresholds.poweredOffSeverity;
    } else {
      if (
        (resource.type === 'guest' || resource.type === 'dockerContainer') &&
        props.guestDisableConnectivity()
      ) {
        hysteresisThresholds.disableConnectivity = false;
      } else {
        delete hysteresisThresholds.disableConnectivity;
      }
      if (resource.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = resource.poweredOffSeverity;
      } else {
        delete hysteresisThresholds.poweredOffSeverity;
      }
    }
    if (noteForOverride) {
      hysteresisThresholds.note = noteForOverride;
    } else {
      delete hysteresisThresholds.note;
    }
    if (previousRaw?.backup) {
      hysteresisThresholds.backup = previousRaw.backup;
    }
    if (previousRaw?.snapshot) {
      hysteresisThresholds.snapshot = previousRaw.snapshot;
    }
    newRawConfig[resourceId] = hysteresisThresholds;
    props.setRawOverridesConfig(newRawConfig);

    props.setHasUnsavedChanges(true);
    setEditingId(null);
    setEditingThresholds({});
    setEditingNote('');
  };

  const handleBulkEdit = (ids: string[], columns: string[]) => {
    setBulkEditIds(ids);
    setBulkEditColumns(columns);
    setIsBulkEditDialogOpen(true);
  };

  const handleSaveBulkEdit = (thresholds: Record<string, number | undefined>) => {
    setIsBulkEditDialogOpen(false);

    const newOverrides = [...props.overrides()];
    const newRawConfig = { ...props.rawOverridesConfig() };
    const allResources = [
      ...nodesWithOverrides(),
      ...hostAgentsWithOverrides(),
      ...hostDisksWithOverrides(),
      ...dockerHostsWithOverrides(),
      ...pbsServersWithOverrides(),
      ...pmgServersWithOverrides(),
      ...storageWithOverrides(),
    ];

    for (const id of bulkEditIds()) {
      const resource = allResources.find(r => r.id === id);
      if (!resource) continue;

      const defaultThresholds = (resource.defaults ?? {}) as Record<string, number | undefined>;
      const existingOverrideCheck = newOverrides.find((o) => o.id === id);
      const previousRaw = newRawConfig[id];

      // Merge current thresholds explicitly checking what differs from defaults
      const currentOverrides = existingOverrideCheck?.thresholds ?? {};
      const newThresholds: Record<string, number | undefined> = { ...currentOverrides };

      // Update with new bulk thresholds
      Object.keys(thresholds).forEach(key => {
        if (thresholds[key] !== undefined) {
          const val = thresholds[key];
          if (val === defaultThresholds[key as keyof typeof defaultThresholds]) {
            delete newThresholds[key];
          } else {
            newThresholds[key] = val as number;
          }
        }
      });

      const hasStateOnlyOverride = Boolean(
        resource.disabled ||
        resource.disableConnectivity ||
        resource.poweredOffSeverity !== undefined ||
        existingOverrideCheck?.note !== undefined ||
        existingOverrideCheck?.backup ||
        existingOverrideCheck?.snapshot,
      );

      // If no override fields remain, remove entirely
      if (Object.keys(newThresholds).length === 0 && !hasStateOnlyOverride) {
        if (resource.hasOverride) {
          const idx = newOverrides.findIndex(o => o.id === id);
          if (idx !== -1) newOverrides.splice(idx, 1);
          delete newRawConfig[id];
        }
        continue;
      }

      // Create new override
      const override: Override = {
        id: id,
        name: resource.name,
        type: resource.type as OverrideType,
        resourceType: resource.resourceType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
        node: 'node' in resource ? resource.node : undefined,
        instance: 'instance' in resource ? resource.instance : undefined,
        disabled: resource.disabled,
        disableConnectivity: resource.disableConnectivity,
        poweredOffSeverity: resource.poweredOffSeverity,
        note: existingOverrideCheck?.note,
        backup: existingOverrideCheck?.backup,
        snapshot: existingOverrideCheck?.snapshot,
        thresholds: newThresholds,
      };

      // Update overrides
      const existingIndex = newOverrides.findIndex((o) => o.id === id);
      if (existingIndex >= 0) {
        newOverrides[existingIndex] = override;
      } else {
        newOverrides.push(override);
      }

      // Update raw config
      const hysteresisThresholds: RawOverrideConfig = {};
      if (previousRaw) {
        if (previousRaw.disabled !== undefined) hysteresisThresholds.disabled = previousRaw.disabled;
        if (previousRaw.disableConnectivity !== undefined) hysteresisThresholds.disableConnectivity = previousRaw.disableConnectivity;
        if (previousRaw.poweredOffSeverity !== undefined) hysteresisThresholds.poweredOffSeverity = previousRaw.poweredOffSeverity;
        if (previousRaw.note !== undefined) hysteresisThresholds.note = previousRaw.note;
        if (previousRaw.backup !== undefined) hysteresisThresholds.backup = previousRaw.backup;
        if (previousRaw.snapshot !== undefined) hysteresisThresholds.snapshot = previousRaw.snapshot;
      }

      Object.entries(newThresholds).forEach(([metric, value]) => {
        if (value !== undefined && value !== null) {
          hysteresisThresholds[metric] = {
            trigger: value,
            clear: Math.max(0, value - 5),
          };
        }
      });

      newRawConfig[id] = hysteresisThresholds;
    }

    props.setOverrides(newOverrides);
    props.setRawOverridesConfig(newRawConfig);
    props.setHasUnsavedChanges(true);

    // Clear bulk edit state
    setBulkEditIds([]);
    setBulkEditColumns([]);
  };

  const cancelEdit = () => {
    setEditingId(null);
    setEditingThresholds({});
    setEditingNote('');
  };

  const updateMetricDelay = (
    typeKey: 'guest' | 'node' | 'storage' | 'pbs' | 'host',
    metricKey: string,
    value: number | null,
  ) => {
    const normalizedMetric = metricKey.trim().toLowerCase();
    if (!normalizedMetric) return;

    let changed = false;
    props.setMetricTimeThresholds((prev) => {
      const current = prev ? { ...prev } : {};
      const existing = prev?.[typeKey];
      const typeOverrides = existing ? { ...existing } : {};

      if (value === null) {
        if (typeOverrides[normalizedMetric] === undefined) {
          return prev;
        }
        delete typeOverrides[normalizedMetric];
        changed = true;
      } else {
        const sanitized = Math.max(0, Math.round(value));
        if (typeOverrides[normalizedMetric] === sanitized) {
          return prev;
        }
        typeOverrides[normalizedMetric] = sanitized;
        changed = true;
      }

      if (!changed) {
        return prev;
      }

      if (Object.keys(typeOverrides).length === 0) {
        delete current[typeKey];
      } else {
        current[typeKey] = typeOverrides;
      }

      return current;
    });

    if (changed) {
      props.setHasUnsavedChanges(true);
    }
  };

  const removeOverride = (resourceId: string) => {
    props.setOverrides(props.overrides().filter((o) => o.id !== resourceId));

    const newRawConfig = { ...props.rawOverridesConfig() };
    delete newRawConfig[resourceId];
    props.setRawOverridesConfig(newRawConfig);

    props.setHasUnsavedChanges(true);
  };

  const toggleBackup = (resourceId: string, forceState?: boolean) => {
    const allGuests = guestsFlat();
    const allDockerContainers = dockerContainersFlat();
    const resource = [...allGuests, ...allDockerContainers].find((r) => r.id === resourceId);
    if (!resource || (resource.type !== 'guest' && resource.type !== 'dockerContainer')) return;

    const existingOverride = props.overrides().find((o) => o.id === resourceId);
    const baseConfig = existingOverride?.backup || props.backupDefaults();
    const newEnabled = forceState !== undefined ? forceState : !baseConfig.enabled;
    const newBackup = { ...baseConfig, enabled: newEnabled };

    const override: Override = {
      ...(existingOverride || {
        id: resourceId,
        name: resource.name,
        type: resource.type as any,
        vmid: 'vmid' in resource ? (resource as any).vmid : undefined,
        node: 'node' in resource ? (resource as any).node : undefined,
        instance: 'instance' in resource ? (resource as any).instance : undefined,
        thresholds: {},
      }),
      backup: newBackup,
    };

    const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
    if (existingIndex >= 0) {
      const newOverrides = [...props.overrides()];
      newOverrides[existingIndex] = override;
      props.setOverrides(newOverrides);
    } else {
      props.setOverrides([...props.overrides(), override]);
    }

    const newRawConfig = { ...props.rawOverridesConfig() };
    newRawConfig[resourceId] = {
      ...(newRawConfig[resourceId] || {}),
      backup: newBackup,
    };
    props.setRawOverridesConfig(newRawConfig);
    props.setHasUnsavedChanges(true);
  };

  const toggleSnapshot = (resourceId: string, forceState?: boolean) => {
    const allGuests = guestsFlat();
    const allDockerContainers = dockerContainersFlat();
    const resource = [...allGuests, ...allDockerContainers].find((r) => r.id === resourceId);
    if (!resource || (resource.type !== 'guest' && resource.type !== 'dockerContainer')) return;

    const existingOverride = props.overrides().find((o) => o.id === resourceId);
    const baseConfig = existingOverride?.snapshot || props.snapshotDefaults();
    const newEnabled = forceState !== undefined ? forceState : !baseConfig.enabled;
    const newSnapshot = { ...baseConfig, enabled: newEnabled };

    const override: Override = {
      ...(existingOverride || {
        id: resourceId,
        name: resource.name,
        type: resource.type as any,
        vmid: 'vmid' in resource ? (resource as any).vmid : undefined,
        node: 'node' in resource ? (resource as any).node : undefined,
        instance: 'instance' in resource ? (resource as any).instance : undefined,
        thresholds: {},
      }),
      snapshot: newSnapshot,
    };

    const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
    if (existingIndex >= 0) {
      const newOverrides = [...props.overrides()];
      newOverrides[existingIndex] = override;
      props.setOverrides(newOverrides);
    } else {
      props.setOverrides([...props.overrides(), override]);
    }

    const newRawConfig = { ...props.rawOverridesConfig() };
    newRawConfig[resourceId] = {
      ...(newRawConfig[resourceId] || {}),
      snapshot: newSnapshot,
    };
    props.setRawOverridesConfig(newRawConfig);
    props.setHasUnsavedChanges(true);
  };

  const toggleDisabled = (resourceId: string, forceState?: boolean) => {
    // Flatten grouped guests to find the resource
    const allGuests = guestsFlat();
    const allDockerContainers = dockerContainersFlat();
    const allResources = [
      ...allGuests,
      ...allDockerContainers,
      ...storageWithOverrides(),
      ...pbsServersWithOverrides(),
      ...hostAgentsWithOverrides(),
      ...hostDisksWithOverrides(),
    ];
    const resource = allResources.find((r) => r.id === resourceId);
    if (
      !resource ||
      (resource.type !== 'guest' &&
        resource.type !== 'storage' &&
        resource.type !== 'pbs' &&
        resource.type !== 'dockerContainer' &&
        resource.type !== 'hostAgent' &&
        resource.type !== 'hostDisk')
    )
      return;

    // Get existing override if it exists
    const existingOverride = props.overrides().find((o) => o.id === resourceId);

    // Determine the current disabled state - check the resource's current state, not the override
    const currentDisabledState = resource.disabled;
    const newDisabledState = forceState !== undefined ? forceState : !currentDisabledState;

    // Clean the thresholds to exclude 'disabled' if it got in there
    const cleanThresholds: Record<string, number> = { ...(existingOverride?.thresholds || {}) };
    delete (cleanThresholds as Record<string, unknown>).disabled;

    // If enabling (disabled = false) and no custom thresholds exist, remove the override entirely
    if (!newDisabledState && (!existingOverride || Object.keys(cleanThresholds).length === 0)) {
      // Remove the override completely
      props.setOverrides(props.overrides().filter((o) => o.id !== resourceId));

      // Remove from raw config
      const newRawConfig = { ...props.rawOverridesConfig() };
      delete newRawConfig[resourceId];
      props.setRawOverridesConfig(newRawConfig);
    } else {
      const override: Override = {
        id: resourceId,
        name: resource.name,
        type: resource.type,
        resourceType: resource.resourceType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
        node: 'node' in resource ? resource.node : undefined,
        instance: 'instance' in resource ? (resource as any).instance : undefined,
        disabled: newDisabledState,
        disableConnectivity: existingOverride?.disableConnectivity,
        poweredOffSeverity: existingOverride?.poweredOffSeverity,
        backup: existingOverride?.backup,
        snapshot: existingOverride?.snapshot,
        thresholds: cleanThresholds, // Only keep actual threshold overrides
      };

      const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
      if (existingIndex >= 0) {
        const newOverrides = [...props.overrides()];
        newOverrides[existingIndex] = override;
        props.setOverrides(newOverrides);
      } else {
        props.setOverrides([...props.overrides(), override]);
      }

      // Update raw config
      const newRawConfig: Record<string, RawOverrideConfig> = { ...props.rawOverridesConfig() };
      const hysteresisThresholds: RawOverrideConfig = {};

      // Only add threshold overrides that differ from defaults
      Object.entries(override.thresholds).forEach(([metric, value]) => {
        if (typeof value === 'number') {
          hysteresisThresholds[metric] = {
            trigger: value,
            clear: Math.max(0, value - 5),
          };
        }
      });

      if (newDisabledState) {
        hysteresisThresholds.disabled = true;
      } else {
        delete hysteresisThresholds.disabled;
      }

      if (override.backup) {
        hysteresisThresholds.backup = override.backup;
      }
      if (override.snapshot) {
        hysteresisThresholds.snapshot = override.snapshot;
      }
      if (override.disableConnectivity) {
        hysteresisThresholds.disableConnectivity = true;
      }
      if (override.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = override.poweredOffSeverity;
      }

      if (Object.keys(hysteresisThresholds).length === 0) {
        delete newRawConfig[resourceId];
      } else {
        newRawConfig[resourceId] = hysteresisThresholds;
      }
      props.setRawOverridesConfig(newRawConfig);
    }

    if (newDisabledState && props.removeAlerts) {
      if (resource.type === 'guest') {
        props.removeAlerts(
          (alert) => alert.resourceId === resourceId && alert.type === 'powered-off',
        );
      } else if (resource.type === 'pbs') {
        const offlineId = `pbs-offline-${resourceId}`;
        props.removeAlerts(
          (alert) =>
            alert.resourceId === resourceId && (alert.id === offlineId || alert.type === 'offline'),
        );
      } else if (resource.type === 'dockerContainer') {
        props.removeAlerts(
          (alert) =>
            alert.resourceId === resourceId &&
            (alert.type === 'docker-container-state' || alert.type === 'docker-container-health'),
        );
      }
    }

    props.setHasUnsavedChanges(true);
  };

  const toggleNodeConnectivity = (resourceId: string, forceState?: boolean) => {
    // Find the resource - could be a node, PBS server, or guest
    const nodes = nodesWithOverrides();
    const pbsServers = pbsServersWithOverrides();
    const guests = guestsFlat();
    const hostAgents = hostAgentsWithOverrides();
    const dockerHosts = dockerHostsWithOverrides();
    const resource = [...nodes, ...pbsServers, ...guests, ...hostAgents, ...dockerHosts].find(
      (r) => r.id === resourceId,
    );
    if (
      !resource ||
      (resource.type !== 'node' &&
        resource.type !== 'pbs' &&
        resource.type !== 'guest' &&
        resource.type !== 'hostAgent' &&
        resource.type !== 'dockerHost')
    )
      return;

    // Get existing override if it exists
    const existingOverride = props.overrides().find((o) => o.id === resourceId);

    // Determine the current state - use the resource's computed state, not just the override
    const currentDisableConnectivity = resource.disableConnectivity;
    const newDisableConnectivity =
      forceState !== undefined ? forceState : !currentDisableConnectivity;

    // Clean the thresholds to exclude any unwanted fields
    const cleanThresholds: Record<string, number> = { ...(existingOverride?.thresholds || {}) };
    delete (cleanThresholds as Record<string, unknown>).disabled;
    delete (cleanThresholds as Record<string, unknown>).disableConnectivity;

    // If enabling connectivity alerts (disableConnectivity = false) and no custom thresholds exist, remove the override entirely
    if (!newDisableConnectivity && Object.keys(cleanThresholds).length === 0) {
      // Remove the override completely
      props.setOverrides(props.overrides().filter((o) => o.id !== resourceId));

      // Remove from raw config
      const newRawConfig = { ...props.rawOverridesConfig() };
      delete newRawConfig[resourceId];
      props.setRawOverridesConfig(newRawConfig);
    } else {
      // Update or create the override
      const override: Override = {
        id: resourceId,
        name: resource.name,
        type: resource.type as OverrideType,
        resourceType: resource.resourceType,
        disableConnectivity: newDisableConnectivity,
        disabled: existingOverride?.disabled,
        poweredOffSeverity: existingOverride?.poweredOffSeverity,
        backup: existingOverride?.backup,
        snapshot: existingOverride?.snapshot,
        thresholds: cleanThresholds,
      };

      // Update overrides list
      const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
      if (existingIndex >= 0) {
        const newOverrides = [...props.overrides()];
        newOverrides[existingIndex] = override;
        props.setOverrides(newOverrides);
      } else {
        props.setOverrides([...props.overrides(), override]);
      }

      // Update raw config
      const newRawConfig = { ...props.rawOverridesConfig() };
      const hysteresisThresholds: Record<string, any> = {};

      // Add threshold configs
      Object.entries(cleanThresholds).forEach(([metric, value]) => {
        if (value !== undefined && value !== null) {
          hysteresisThresholds[metric] = {
            trigger: value,
            clear: Math.max(0, (value as number) - 5),
          };
        }
      });

      if (newDisableConnectivity) {
        hysteresisThresholds.disableConnectivity = true;
      } else {
        delete hysteresisThresholds.disableConnectivity;
      }

      if (override.backup) {
        hysteresisThresholds.backup = override.backup;
      }
      if (override.snapshot) {
        hysteresisThresholds.snapshot = override.snapshot;
      }
      if (override.disabled) {
        hysteresisThresholds.disabled = true;
      }
      if (override.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = override.poweredOffSeverity;
      }

      if (Object.keys(hysteresisThresholds).length === 0) {
        delete newRawConfig[resourceId];
      } else {
        newRawConfig[resourceId] = hysteresisThresholds;
      }
      props.setRawOverridesConfig(newRawConfig);
    }

    props.setHasUnsavedChanges(true);

    if (props.removeAlerts && resource.type === 'dockerHost') {
      const offlineId = `docker-host-offline-${resourceId}`;
      const resourceKey = `docker:${resourceId}`;
      props.removeAlerts((alert) => alert.id === offlineId || alert.resourceId === resourceKey);
    }
  };

  const setOfflineState = (resourceId: string, state: OfflineState) => {
    const guests = guestsFlat();
    const dockerContainers = dockerContainersFlat();
    const resource = [...guests, ...dockerContainers].find((r) => r.id === resourceId);
    if (!resource) return;

    const isDockerContainer = resource.type === 'dockerContainer';
    const defaultDisabled = isDockerContainer
      ? props.dockerDisableConnectivity()
      : props.guestDisableConnectivity();
    const defaultSeverity = isDockerContainer
      ? props.dockerPoweredOffSeverity()
      : props.guestPoweredOffSeverity();

    const existingOverride = props.overrides().find((o) => o.id === resourceId);
    const cleanThresholds: Record<string, number> = { ...(existingOverride?.thresholds || {}) };
    delete (cleanThresholds as Record<string, unknown>).disabled;
    delete (cleanThresholds as Record<string, unknown>).disableConnectivity;
    delete (cleanThresholds as Record<string, unknown>).poweredOffSeverity;

    const newDisableConnectivity = state === 'off';
    const newSeverity: 'warning' | 'critical' | undefined =
      state === 'off' ? undefined : state === 'critical' ? 'critical' : 'warning';

    const overrideDisabled = existingOverride?.disabled || false;
    const hasThresholds = Object.keys(cleanThresholds).length > 0;

    const differsFromDefaults =
      newDisableConnectivity !== defaultDisabled ||
      (!newDisableConnectivity && newSeverity !== defaultSeverity);

    if (
      !differsFromDefaults &&
      !hasThresholds &&
      !overrideDisabled &&
      !existingOverride?.disableConnectivity
    ) {
      // Remove override entirely
      if (existingOverride) {
        props.setOverrides(props.overrides().filter((o) => o.id !== resourceId));
        const newRawConfig = { ...props.rawOverridesConfig() };
        delete newRawConfig[resourceId];
        props.setRawOverridesConfig(newRawConfig);
        props.setHasUnsavedChanges(true);
      }
      return;
    }

    const override: Override = {
      id: resourceId,
      name: resource.name,
      type: resource.type as OverrideType,
      resourceType: resource.resourceType,
      vmid: 'vmid' in resource ? resource.vmid : undefined,
      node: 'node' in resource ? resource.node : undefined,
      instance: 'instance' in resource ? resource.instance : undefined,
      disabled: overrideDisabled,
      disableConnectivity: newDisableConnectivity,
      poweredOffSeverity: newDisableConnectivity ? undefined : newSeverity,
      backup: existingOverride?.backup,
      snapshot: existingOverride?.snapshot,
      thresholds: cleanThresholds,
    };

    const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
    if (existingIndex >= 0) {
      const newOverrides = [...props.overrides()];
      newOverrides[existingIndex] = override;
      props.setOverrides(newOverrides);
    } else {
      props.setOverrides([...props.overrides(), override]);
    }

    const newRawConfig: Record<string, RawOverrideConfig> = { ...props.rawOverridesConfig() };
    const hysteresisThresholds: RawOverrideConfig = {};

    Object.entries(cleanThresholds).forEach(([metric, value]) => {
      if (value !== undefined && value !== null) {
        hysteresisThresholds[metric] = {
          trigger: value,
          clear: Math.max(0, value - 5),
        };
      }
    });

    if (overrideDisabled) {
      hysteresisThresholds.disabled = true;
    }

    if (newDisableConnectivity) {
      hysteresisThresholds.disableConnectivity = true;
    } else {
      if (defaultDisabled) {
        hysteresisThresholds.disableConnectivity = false;
      }
      if (newSeverity) {
        hysteresisThresholds.poweredOffSeverity = newSeverity;
      }
    }

    if (override.backup) {
      hysteresisThresholds.backup = override.backup;
    }
    if (override.snapshot) {
      hysteresisThresholds.snapshot = override.snapshot;
    }

    if (Object.keys(hysteresisThresholds).length > 0) {
      newRawConfig[resourceId] = hysteresisThresholds;
    } else {
      delete newRawConfig[resourceId];
    }

    props.setRawOverridesConfig(newRawConfig);
    props.setHasUnsavedChanges(true);

    if (props.removeAlerts && newDisableConnectivity) {
      if (resource.type === 'guest') {
        props.removeAlerts(
          (alert) => alert.resourceId === resourceId && alert.type === 'powered-off',
        );
      } else if (resource.type === 'dockerContainer') {
        props.removeAlerts(
          (alert) =>
            alert.resourceId === resourceId &&
            (alert.type === 'docker-container-state' || alert.type === 'docker-container-health'),
        );
      }
    }
  };

  return (
    <div class="space-y-4">
      {/* Search Bar */}
      <div class="relative">
        <input
          ref={searchInputRef}
          type="text"
          placeholder="Search resources... (Ctrl+F)"
          value={searchTerm()}
          onInput={(e) => setSearchTerm(e.currentTarget.value)}
          class="w-full pl-10 pr-20 py-2 text-sm border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
        />
        <kbd class="absolute right-10 top-2 hidden sm:inline-flex items-center gap-0.5 px-1.5 py-0.5 text-[10px] font-medium text-slate-400 dark:text-slate-500 bg-slate-100 dark:bg-slate-700 rounded border border-slate-200 dark:border-slate-600">
          ⌘F
        </kbd>
        <svg
          class="absolute left-3 top-2.5 w-5 h-5 text-slate-400"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
          />
        </svg>
        <Show when={searchTerm()}>
          <button
            type="button"
            onClick={() => setSearchTerm('')}
            class="absolute right-3 top-2.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </Show>
      </div>

      {/* Help Banner - Dismissible */}
      <Show when={!helpBannerDismissed()}>
        <div class="rounded-md border border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-900 p-3 relative group">
          <button
            type="button"
            onClick={dismissHelpBanner}
            class="absolute top-2 right-2 p-1 rounded-md text-blue-400 hover:text-blue-600 dark:text-blue-500 dark:hover:text-blue-300 hover:bg-blue-100 dark:hover:bg-blue-900 opacity-0 group-hover:opacity-100 transition-opacity"
            title="Dismiss tips"
            aria-label="Dismiss tips"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
          <div class="flex items-start gap-2 pr-6">
            <svg
              class="w-5 h-5 text-blue-600 dark:text-blue-400 flex-shrink-0 mt-0.5"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <div class="text-sm text-blue-900 dark:text-blue-100">
              <span class="font-medium">Quick tips:</span> Set any threshold to{' '}
              <code class="px-1 py-0.5 bg-blue-100 dark:bg-blue-900 rounded text-xs font-mono">
                0
              </code>{' '}
              to disable alerts for that metric. Click on disabled thresholds showing{' '}
              <span class="italic">Off</span> to re-enable them. Resources with custom settings show a{' '}
              <span class="inline-flex items-center px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded text-xs">
                Custom
              </span>{' '}
              badge. <span class="text-blue-600 dark:text-blue-400">Click sections to collapse/expand.</span>
            </div>
          </div>
        </div>
      </Show>

      {/* Tab Navigation */}
      <div class="border-b border-slate-200 dark:border-slate-700">
        <nav class="-mb-px flex gap-4 sm:gap-6" aria-label="Tabs">
          <button
            type="button"
            onClick={() => handleTabClick('proxmox')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${activeTab() === 'proxmox'
              ? 'border-blue-500 text-blue-600 dark:text-blue-400'
              : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300 dark:text-slate-400 dark:hover:text-slate-300'
              }`}
          >
            <Server class="w-4 h-4" />
            <span class="hidden sm:inline">Proxmox / PBS</span>
            <span class="sm:hidden">Proxmox</span>
          </button>
          <button
            type="button"
            onClick={() => handleTabClick('pmg')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${activeTab() === 'pmg'
              ? 'border-blue-500 text-blue-600 dark:text-blue-400'
              : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300 dark:text-slate-400 dark:hover:text-slate-300'
              }`}
          >
            <Mail class="w-4 h-4" />
            <span class="hidden sm:inline">Mail Gateway</span>
            <span class="sm:hidden">Mail</span>
          </button>
          <button
            type="button"
            onClick={() => handleTabClick('hosts')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${activeTab() === 'hosts'
              ? 'border-blue-500 text-blue-600 dark:text-blue-400'
              : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300 dark:text-slate-400 dark:hover:text-slate-300'
              }`}
          >
            <Users class="w-4 h-4" />
            <span class="hidden sm:inline">Host Agents</span>
            <span class="sm:hidden">Hosts</span>
          </button>
          <button
            type="button"
            onClick={() => handleTabClick('docker')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${activeTab() === 'docker'
              ? 'border-blue-500 text-blue-600 dark:text-blue-400'
              : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300 dark:text-slate-400 dark:hover:text-slate-300'
              }`}
          >
            <Boxes class="w-4 h-4" />
            <span>Containers</span>
          </button>
        </nav>
      </div>

      {/* Section Controls - Only show on Proxmox tab which has multiple sections */}
      <Show when={activeTab() === 'proxmox'}>
        <div class="flex justify-end gap-2">
          <button
            type="button"
            onClick={expandAll}
            class="text-xs px-2 py-1 text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 rounded transition-colors"
          >
            Expand all
          </button>
          <span class="text-slate-300 dark:text-slate-600">|</span>
          <button
            type="button"
            onClick={collapseAll}
            class="text-xs px-2 py-1 text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 rounded transition-colors"
          >
            Collapse all
          </button>
        </div>
      </Show>

      <div class="space-y-6">
        <Show when={activeTab() === 'proxmox'}>
          <Show when={hasSection('nodes')}>
            <CollapsibleSection
              id="nodes"
              title="Proxmox Nodes"
              resourceCount={nodesWithOverrides().length}
              collapsed={isCollapsed('nodes')}
              onToggle={() => toggleSection('nodes')}
              icon={<Server class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllNodes()}
              emptyMessage="No nodes match the current filters."
            >
              <div ref={registerSection('nodes')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  resources={nodesWithOverrides()}
                  columns={['CPU %', 'Memory %', 'Disk %', 'Temp °C']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage="No nodes match the current filters."
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  onToggleNodeConnectivity={toggleNodeConnectivity}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) => handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %', 'Temp °C'])}
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={props.nodeDefaults}
                  setGlobalDefaults={props.setNodeDefaults}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={props.disableAllNodes}
                  onToggleGlobalDisable={() => props.setDisableAllNodes(!props.disableAllNodes())}
                  globalDisableOfflineFlag={props.disableAllNodesOffline}
                  onToggleGlobalDisableOffline={() =>
                    props.setDisableAllNodesOffline(!props.disableAllNodesOffline())
                  }
                  showDelayColumn={true}
                  globalDelaySeconds={props.timeThresholds().node}
                  metricDelaySeconds={props.metricTimeThresholds().node ?? {}}
                  onMetricDelayChange={(metric, value) => updateMetricDelay('node', metric, value)}
                  factoryDefaults={props.factoryNodeDefaults}
                  onResetDefaults={props.resetNodeDefaults}
                />
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('pbs')}>
            <CollapsibleSection
              id="pbs"
              title="PBS Servers"
              resourceCount={pbsServersWithOverrides().length}
              collapsed={isCollapsed('pbs')}
              onToggle={() => toggleSection('pbs')}
              icon={<Database class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllPBS()}
              emptyMessage="No PBS servers configured."
            >
              <div ref={registerSection('pbs')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  resources={pbsServersWithOverrides()}
                  columns={['CPU %', 'Memory %']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage="No PBS servers match the current filters."
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  onToggleNodeConnectivity={toggleNodeConnectivity}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) => handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk R MB/s', 'Disk W MB/s', 'Net In MB/s', 'Net Out MB/s'])}
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={props.pbsDefaults ?? { cpu: 80, memory: 85 }}
                  setGlobalDefaults={props.setPBSDefaults}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={props.disableAllPBS}
                  onToggleGlobalDisable={() => props.setDisableAllPBS(!props.disableAllPBS())}
                  globalDisableOfflineFlag={props.disableAllPBSOffline}
                  onToggleGlobalDisableOffline={() =>
                    props.setDisableAllPBSOffline(!props.disableAllPBSOffline())
                  }
                  showDelayColumn={true}
                  globalDelaySeconds={props.timeThresholds().pbs}
                  metricDelaySeconds={props.metricTimeThresholds().pbs ?? {}}
                  onMetricDelayChange={(metric, value) => updateMetricDelay('pbs', metric, value)}
                  factoryDefaults={props.factoryPBSDefaults}
                  onResetDefaults={props.resetPBSDefaults}
                />
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('guests')}>
            <CollapsibleSection
              id="guests"
              title="VMs & Containers"
              resourceCount={props.allGuests().length}
              collapsed={isCollapsed('guests')}
              onToggle={() => toggleSection('guests')}
              icon={<Monitor class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllGuests()}
              emptyMessage="No VMs or containers found."
            >
              <div ref={registerSection('guests')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  groupedResources={guestsGroupedByNode()}
                  groupHeaderMeta={guestGroupHeaderMeta()}
                  columns={[
                    'CPU %',
                    'Memory %',
                    'Disk %',
                    'Backup',
                    'Snapshot',
                    'Disk R MB/s',
                    'Disk W MB/s',
                    'Net In MB/s',
                    'Net Out MB/s',
                  ]}
                  activeAlerts={props.activeAlerts}
                  emptyMessage="No VMs or containers match the current filters."
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  onToggleNodeConnectivity={toggleNodeConnectivity}
                  onToggleBackup={toggleBackup}
                  onToggleSnapshot={toggleSnapshot}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) => handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %', 'Disk R MB/s', 'Disk W MB/s', 'Net In MB/s', 'Net Out MB/s'])}
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={props.guestDefaults}
                  setGlobalDefaults={props.setGuestDefaults}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={props.disableAllGuests}
                  onToggleGlobalDisable={() => props.setDisableAllGuests(!props.disableAllGuests())}
                  globalDisableOfflineFlag={() => props.guestDisableConnectivity()}
                  onToggleGlobalDisableOffline={() =>
                    props.setGuestDisableConnectivity(!props.guestDisableConnectivity())
                  }
                  globalOfflineSeverity={props.guestPoweredOffSeverity()}
                  onSetGlobalOfflineState={(state) => {
                    if (state === 'off') {
                      props.setGuestDisableConnectivity(true);
                    } else {
                      props.setGuestDisableConnectivity(false);
                      props.setGuestPoweredOffSeverity(state === 'critical' ? 'critical' : 'warning');
                    }
                    props.setHasUnsavedChanges(true);
                  }}
                  onSetOfflineState={setOfflineState}
                  showDelayColumn={true}
                  globalDelaySeconds={props.timeThresholds().guest}
                  metricDelaySeconds={props.metricTimeThresholds().guest ?? {}}
                  onMetricDelayChange={(metric, value) => updateMetricDelay('guest', metric, value)}
                  factoryDefaults={props.factoryGuestDefaults}
                  onResetDefaults={props.resetGuestDefaults}
                />
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={activeTab() === 'proxmox'}>
            <CollapsibleSection
              id="guest-filtering"
              title="Guest Filtering"
              collapsed={isCollapsed('guest-filtering')}
              onToggle={() => toggleSection('guest-filtering')}
              icon={<Monitor class="w-5 h-5" />}
              emptyMessage="Configure guest filtering rules."
            >
              <div class="grid grid-cols-1 gap-6 p-4 xl:grid-cols-3">
                <Card padding="md" tone="card">
                  <div class="mb-2">
                    <h3 class="text-sm font-semibold text-slate-900 dark:text-slate-100">Ignored Prefixes</h3>
                    <p class="text-xs text-slate-600 dark:text-slate-400">Skip metrics for guests starting with:</p>
                  </div>
                  <TagInput
                    tags={props.ignoredGuestPrefixes()}
                    onChange={(tags) => {
                      props.setIgnoredGuestPrefixes(tags);
                      props.setHasUnsavedChanges(true);
                    }}
                    placeholder="dev-"
                  />
                </Card>
                <Card padding="md" tone="card">
                  <div class="mb-2">
                    <h3 class="text-sm font-semibold text-slate-900 dark:text-slate-100">Tag Whitelist</h3>
                    <p class="text-xs text-slate-600 dark:text-slate-400">Only monitor guests with at least one of these tags (leave empty to disable whitelist):</p>
                  </div>
                  <TagInput
                    tags={props.guestTagWhitelist()}
                    onChange={(tags) => {
                      props.setGuestTagWhitelist(tags);
                      props.setHasUnsavedChanges(true);
                    }}
                    placeholder="production"
                  />
                </Card>
                <Card padding="md" tone="card">
                  <div class="mb-2">
                    <h3 class="text-sm font-semibold text-slate-900 dark:text-slate-100">Tag Blacklist</h3>
                    <p class="text-xs text-slate-600 dark:text-slate-400">Ignore guests with any of these tags:</p>
                  </div>
                  <TagInput
                    tags={props.guestTagBlacklist()}
                    onChange={(tags) => {
                      props.setGuestTagBlacklist(tags);
                      props.setHasUnsavedChanges(true);
                    }}
                    placeholder="maintenance"
                  />
                </Card>
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('backups')}>
            <CollapsibleSection
              id="backups"
              title="Recovery"
              collapsed={isCollapsed('backups')}
              onToggle={() => toggleSection('backups')}
              icon={<Archive class="w-5 h-5" />}
              isGloballyDisabled={!props.backupDefaults().enabled}
              emptyMessage="Configure recovery alert thresholds."
            >
              <div ref={registerSection('backups')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  resources={[
                    {
                      id: 'backups-defaults',
                      name: 'Global Defaults',
                      thresholds: backupDefaultsRecord(),
                      defaults: backupDefaultsRecord(),
                      editable: true,
                      editScope: 'backup',
                    },
                  ]}
                  columns={[
                    'Fresh Hours',
                    'Stale Hours',
                    'Warning Days',
                    'Critical Days',
                    'Warning Size (GiB)',
                    'Critical Size (GiB)',
                  ]}
                  activeAlerts={props.activeAlerts}
                  emptyMessage=""
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) => handleBulkEdit(ids, ['Usage %'])}
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={backupDefaultsRecord()}
                  setGlobalDefaults={(value) => {
                    updateBackupDefaults((prev) => {
                      const currentRecord = {
                        'fresh hours': prev.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS,
                        'stale hours': prev.staleHours ?? DEFAULT_BACKUP_STALE_HOURS,
                        'warning days': prev.warningDays ?? 0,
                        'critical days': prev.criticalDays ?? 0,
                      };
                      const nextRecord =
                        typeof value === 'function'
                          ? value(currentRecord)
                          : { ...currentRecord, ...value };
                      return {
                        ...prev,
                        freshHours:
                          typeof nextRecord['fresh hours'] === 'number'
                            ? nextRecord['fresh hours']
                            : prev.freshHours,
                        staleHours:
                          typeof nextRecord['stale hours'] === 'number'
                            ? nextRecord['stale hours']
                            : prev.staleHours,
                        warningDays:
                          typeof nextRecord['warning days'] === 'number'
                            ? nextRecord['warning days']
                            : prev.warningDays,
                        criticalDays:
                          typeof nextRecord['critical days'] === 'number'
                            ? nextRecord['critical days']
                            : prev.criticalDays,
                      };
                    });
                  }}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={() => !props.backupDefaults().enabled}
                  onToggleGlobalDisable={() =>
                    updateBackupDefaults((prev) => ({
                      ...prev,
                      enabled: !prev.enabled,
                    }))
                  }
                  factoryDefaults={backupFactoryDefaultsRecord()}
                  onResetDefaults={() => {
                    if (props.resetBackupDefaults) {
                      props.resetBackupDefaults();
                      props.setHasUnsavedChanges(true);
                    } else {
                      updateBackupDefaults(backupFactoryConfig());
                    }
                  }}
                />
                <Card padding="md" tone="card" class="mt-6">
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <h3 class="text-sm font-semibold text-slate-900 dark:text-slate-100">Orphaned backups</h3>
                      <p class="mt-1 text-xs text-slate-600 dark:text-slate-400">
                        Alert when backups exist for VMIDs that are no longer in inventory.
                      </p>
                    </div>
                    <Toggle
                      checked={props.backupDefaults().alertOrphaned ?? true}
                      onToggle={() =>
                        updateBackupDefaults((prev) => ({
                          ...prev,
                          alertOrphaned: !(prev.alertOrphaned ?? true),
                        }))
                      }
                      label={<span class="text-sm font-medium text-slate-900 dark:text-slate-100">Alerts</span>}
                      description={
                        <span class="text-xs text-slate-500 dark:text-slate-400">
                          Toggle orphaned VM/CT backup alerts
                        </span>
                      }
                      size="sm"
                    />
                  </div>
                  <div class="mt-4">
                    <label class="text-xs font-medium uppercase tracking-wide text-slate-600 dark:text-slate-400">
                      Ignore VMIDs
                    </label>
                    <p class="mt-1 text-xs text-slate-600 dark:text-slate-400">
                      One per line. Use a trailing * to match a prefix (example: 10*).
                    </p>
                    <TagInput
                      tags={props.backupDefaults().ignoreVMIDs ?? []}
                      onChange={(tags) => {
                        updateBackupDefaults((prev) => ({ ...prev, ignoreVMIDs: tags }));
                        props.setHasUnsavedChanges(true);
                      }}
                      placeholder="100, 200, 10*"
                    />
                  </div>
                </Card>
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('snapshots')}>
            <CollapsibleSection
              id="snapshots"
              title="Snapshot Age"
              collapsed={isCollapsed('snapshots')}
              onToggle={() => toggleSection('snapshots')}
              icon={<Camera class="w-5 h-5" />}
              isGloballyDisabled={!props.snapshotDefaults().enabled}
              emptyMessage="Configure snapshot age thresholds."
            >
              <div ref={registerSection('snapshots')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  resources={[
                    {
                      id: 'snapshots-defaults',
                      name: 'Global Defaults',
                      thresholds: snapshotDefaultsRecord(),
                      defaults: snapshotDefaultsRecord(),
                      editable: true,
                      editScope: 'snapshot',
                    },
                  ]}
                  columns={['Warning Days', 'Critical Days']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage=""
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) => handleBulkEdit(ids, ['Usage %', 'Temperature °C'])}
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={snapshotDefaultsRecord()}
                  setGlobalDefaults={(value) => {
                    updateSnapshotDefaults((prev) => {
                      const currentRecord = {
                        'warning days': prev.warningDays ?? 0,
                        'critical days': prev.criticalDays ?? 0,
                        'warning size (gib)': prev.warningSizeGiB ?? 0,
                        'critical size (gib)': prev.criticalSizeGiB ?? 0,
                      };
                      const nextRecord =
                        typeof value === 'function'
                          ? value(currentRecord)
                          : { ...currentRecord, ...value };
                      return {
                        ...prev,
                        warningDays:
                          typeof nextRecord['warning days'] === 'number'
                            ? nextRecord['warning days']
                            : prev.warningDays,
                        criticalDays:
                          typeof nextRecord['critical days'] === 'number'
                            ? nextRecord['critical days']
                            : prev.criticalDays,
                        warningSizeGiB:
                          typeof nextRecord['warning size (gib)'] === 'number'
                            ? nextRecord['warning size (gib)']
                            : prev.warningSizeGiB,
                        criticalSizeGiB:
                          typeof nextRecord['critical size (gib)'] === 'number'
                            ? nextRecord['critical size (gib)']
                            : prev.criticalSizeGiB,
                      };
                    });
                  }}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={() => !props.snapshotDefaults().enabled}
                  onToggleGlobalDisable={() =>
                    updateSnapshotDefaults((prev) => ({
                      ...prev,
                      enabled: !prev.enabled,
                    }))
                  }
                  factoryDefaults={snapshotFactoryDefaultsRecord()}
                  onResetDefaults={() => {
                    if (props.resetSnapshotDefaults) {
                      props.resetSnapshotDefaults();
                      props.setHasUnsavedChanges(true);
                    } else {
                      updateSnapshotDefaults(snapshotFactoryConfig());
                    }
                  }}
                />
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('storage')}>
            <CollapsibleSection
              id="storage"
              title="Storage Devices"
              resourceCount={props.storage.length}
              collapsed={isCollapsed('storage')}
              onToggle={() => toggleSection('storage')}
              icon={<HardDrive class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllStorage()}
              emptyMessage="No storage devices found."
            >
              <div ref={registerSection('storage')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  groupedResources={storageGroupedByNode()}
                  groupHeaderMeta={guestGroupHeaderMeta()}
                  columns={['Usage %']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage="No storage devices match the current filters."
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  showOfflineAlertsColumn={false}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) => handleBulkEdit(ids, ['Usage %'])}
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={{ usage: props.storageDefault() }}
                  setGlobalDefaults={(value) => {
                    if (typeof value === 'function') {
                      const newValue = value({ usage: props.storageDefault() });
                      props.setStorageDefault(newValue.usage ?? 85);
                    } else {
                      props.setStorageDefault(value.usage ?? 85);
                    }
                  }}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={props.disableAllStorage}
                  onToggleGlobalDisable={() => props.setDisableAllStorage(!props.disableAllStorage())}
                  showDelayColumn={true}
                  globalDelaySeconds={props.timeThresholds().storage}
                  metricDelaySeconds={props.metricTimeThresholds().storage ?? {}}
                  onMetricDelayChange={(metric, value) => updateMetricDelay('storage', metric, value)}
                  factoryDefaults={
                    props.factoryStorageDefault !== undefined
                      ? { usage: props.factoryStorageDefault }
                      : undefined
                  }
                  onResetDefaults={props.resetStorageDefault}
                />
              </div>
            </CollapsibleSection>
          </Show>
        </Show>

        <Show when={activeTab() === 'pmg'}>
          <Show
            when={pmgServersWithOverrides().length > 0}
            fallback={
              <div class="rounded-md border border-slate-200 bg-white p-6 text-sm text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300">
                No mail gateways configured yet. Add a PMG instance in Settings to manage
                thresholds.
              </div>
            }
          >
            <div ref={registerSection('pmg')} class="scroll-mt-24">
              <ResourceTable
                title="Mail Gateway Thresholds"
                resources={pmgServersWithOverrides()}
                columns={[
                  'Queue Warn',
                  'Queue Crit',
                  'Deferred Warn',
                  'Deferred Crit',
                  'Hold Warn',
                  'Hold Crit',
                  'Oldest Warn (min)',
                  'Oldest Crit (min)',
                  'Spam Warn',
                  'Spam Crit',
                  'Virus Warn',
                  'Virus Crit',
                  'Growth Warn %',
                  'Growth Warn Min',
                  'Growth Crit %',
                  'Growth Crit Min',
                ]}
                activeAlerts={props.activeAlerts}
                emptyMessage="No mail gateways match the current filters."
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                onToggleNodeConnectivity={toggleNodeConnectivity}
                showOfflineAlertsColumn={true}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                editingNote={editingNote}
                setEditingNote={setEditingNote}
                onBulkEdit={(ids) => handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %'])}
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={pmgGlobalDefaults()}
                setGlobalDefaults={setPMGGlobalDefaults}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllPMG}
                onToggleGlobalDisable={() => props.setDisableAllPMG(!props.disableAllPMG())}
                globalDisableOfflineFlag={props.disableAllPMGOffline}
                onToggleGlobalDisableOffline={() =>
                  props.setDisableAllPMGOffline(!props.disableAllPMGOffline())
                }
              />
            </div>
          </Show>
        </Show>

        <Show when={activeTab() === 'hosts'}>
          <Show when={hasSection('hostAgents')}>
            <div ref={registerSection('hostAgents')} class="scroll-mt-24">
              <ResourceTable
                title="Host Agents"
                resources={hostAgentsWithOverrides()}
                columns={['CPU %', 'Memory %', 'Disk %', 'Disk Temp °C']}
                activeAlerts={props.activeAlerts}
                emptyMessage="No host agents match the current filters."
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                onToggleNodeConnectivity={toggleNodeConnectivity}
                showOfflineAlertsColumn={true}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                editingNote={editingNote}
                setEditingNote={setEditingNote}
                onBulkEdit={(ids) => handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %', 'Temp °C'])}
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={props.hostDefaults}
                setGlobalDefaults={props.setHostDefaults}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllHosts}
                onToggleGlobalDisable={() => props.setDisableAllHosts(!props.disableAllHosts())}
                globalDisableOfflineFlag={props.disableAllHostsOffline}
                onToggleGlobalDisableOffline={() =>
                  props.setDisableAllHostsOffline(!props.disableAllHostsOffline())
                }
                showDelayColumn={true}
                globalDelaySeconds={props.timeThresholds().host}
                metricDelaySeconds={props.metricTimeThresholds().host ?? {}}
                onMetricDelayChange={(metric, value) => updateMetricDelay('host', metric, value)}
                factoryDefaults={props.factoryHostDefaults}
                onResetDefaults={props.resetHostDefaults}
              />
            </div>
          </Show>

          <Show when={hasSection('hostDisks')}>
            <CollapsibleSection
              id="hostDisks"
              title="Host Disks"
              resourceCount={hostDisksWithOverrides().length}
              collapsed={isCollapsed('hostDisks')}
              onToggle={() => toggleSection('hostDisks')}
              icon={<HardDrive class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllHosts()}
              emptyMessage="No host disks found. Host agents with mounted filesystems will appear here."
            >
              <div ref={registerSection('hostDisks')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  groupedResources={hostDisksGroupedByHost()}
                  groupHeaderMeta={guestGroupHeaderMeta()}
                  columns={['Disk %']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage="No host disks match the current filters."
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  showOfflineAlertsColumn={false}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) => handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk R MB/s', 'Disk W MB/s', 'Net In MB/s', 'Net Out MB/s'])}
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={{ disk: props.hostDefaults.disk }}
                  setGlobalDefaults={(value) => {
                    if (typeof value === 'function') {
                      const newValue = value({ disk: props.hostDefaults.disk });
                      props.setHostDefaults((prev) => ({ ...prev, disk: newValue.disk }));
                    } else {
                      props.setHostDefaults((prev) => ({ ...prev, disk: value.disk }));
                    }
                  }}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                />
              </div>
            </CollapsibleSection>
          </Show>
        </Show>

        <Show when={activeTab() === 'docker'}>
          <Card padding="md" tone="card" class="mb-6">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h3 class="text-sm font-semibold text-slate-900 dark:text-slate-100">
                  Ignored container prefixes
                </h3>
                <p class="mt-1 text-xs text-slate-600 dark:text-slate-400">
                  Containers whose name or ID starts with any prefix below are skipped for container
                  alerts. Enter one prefix per line; matching is case-insensitive.
                </p>
              </div>
              <Show when={(props.dockerIgnoredPrefixes().length ?? 0) > 0}>
                <button
                  type="button"
                  class="inline-flex items-center justify-center rounded-md border border-transparent bg-slate-100 px-3 py-1 text-xs font-medium text-slate-700 transition hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
                  onClick={handleResetDockerIgnored}
                >
                  Reset
                </button>
              </Show>
            </div>
            <textarea
              value={dockerIgnoredInput()}
              onInput={(event) => handleDockerIgnoredChange(event.currentTarget.value)}
              onKeyDown={(event) => {
                // Ensure Enter key works in textarea for creating new lines
                if (event.key === 'Enter') {
                  // Don't prevent default - allow the newline to be inserted
                  event.stopPropagation();
                }
              }}
              placeholder="runner-"
              rows={4}
              class="mt-4 w-full rounded-md border border-slate-300 bg-white p-3 text-sm text-slate-900 focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:focus:border-sky-400 dark:focus:ring-sky-600"
            />
          </Card>

          <Card padding="md" tone="card" class="mb-6">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h3 class="text-sm font-semibold text-slate-900 dark:text-slate-100">Swarm service alerts</h3>
                <p class="mt-1 text-xs text-slate-600 dark:text-slate-400">
                  Pulse raises alerts when running replicas fall behind the desired count or a rollout gets stuck. Adjust the gap thresholds below or disable service alerts entirely.
                </p>
              </div>
              <Toggle
                checked={!props.disableAllDockerServices()}
                onToggle={() => {
                  props.setDisableAllDockerServices(!props.disableAllDockerServices());
                  props.setHasUnsavedChanges(true);
                }}
                label={<span class="text-sm font-medium text-slate-900 dark:text-slate-100">Alerts</span>}
                description={<span class="text-xs text-slate-500 dark:text-slate-400">Toggle Swarm service replica monitoring</span>}
                size="sm"
              />
            </div>

            <div class="mt-4 grid gap-4 sm:grid-cols-2">
              <div>
                <label
                  for={serviceWarnInputId}
                  class="text-xs font-medium uppercase tracking-wide text-slate-600 dark:text-slate-400"
                >
                  Warning gap %
                </label>
                <input
                  type="number"
                  min="0"
                  max="100"
                  id={serviceWarnInputId}
                  value={props.dockerDefaults.serviceWarnGapPercent}
                  onInput={(event) => {
                    const value = Number(event.currentTarget.value);
                    const normalized = Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : 0;
                    props.setDockerDefaults((prev) => ({
                      ...prev,
                      serviceWarnGapPercent: normalized,
                    }));
                    props.setHasUnsavedChanges(true);
                  }}
                  class="mt-1 w-full rounded-md border border-slate-300 bg-white p-2 text-sm text-slate-900 focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:focus:border-sky-400 dark:focus:ring-sky-600"
                />
                <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                  Convert to warning when at least this percentage of replicas are missing.
                </p>
              </div>
              <div>
                <label
                  for={serviceCriticalInputId}
                  class="text-xs font-medium uppercase tracking-wide text-slate-600 dark:text-slate-400"
                >
                  Critical gap %
                </label>
                <input
                  type="number"
                  min="0"
                  max="100"
                  id={serviceCriticalInputId}
                  value={props.dockerDefaults.serviceCriticalGapPercent}
                  onInput={(event) => {
                    const value = Number(event.currentTarget.value);
                    const normalized = Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : 0;
                    props.setDockerDefaults((prev) => ({
                      ...prev,
                      serviceCriticalGapPercent: normalized,
                    }));
                    props.setHasUnsavedChanges(true);
                  }}
                  class="mt-1 w-full rounded-md border border-slate-300 bg-white p-2 text-sm text-slate-900 focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:focus:border-sky-400 dark:focus:ring-sky-600"
                />
                <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                  Raise a critical alert when the missing replica gap meets or exceeds this value.
                </p>
              </div>
            </div>
            {serviceGapValidationMessage() && (
              <p class="mt-1.5 text-xs font-medium text-red-600 dark:text-red-400">
                {serviceGapValidationMessage()}
              </p>
            )}
          </Card>

          <Show when={hasSection('dockerHosts')}>
            <div ref={registerSection('dockerHosts')} class="scroll-mt-24">
              <ResourceTable
                title="Container Hosts"
                resources={dockerHostsWithOverrides()}
                columns={[]}
                activeAlerts={props.activeAlerts}
                emptyMessage="No container hosts match the current filters."
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                onToggleNodeConnectivity={toggleNodeConnectivity}
                showOfflineAlertsColumn={true}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                editingNote={editingNote}
                setEditingNote={setEditingNote}
                onBulkEdit={(ids) => handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %', 'Disk R MB/s', 'Disk W MB/s', 'Net In MB/s', 'Net Out MB/s', 'Restart Count', 'Restart Window (s)'])}
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDisableFlag={props.disableAllDockerHosts}
                onToggleGlobalDisable={() =>
                  props.setDisableAllDockerHosts(!props.disableAllDockerHosts())
                }
                globalDisableOfflineFlag={props.disableAllDockerHostsOffline}
                onToggleGlobalDisableOffline={() =>
                  props.setDisableAllDockerHostsOffline(!props.disableAllDockerHostsOffline())
                }
              />
            </div>
          </Show>

          <Show when={hasSection('dockerContainers')}>
            <div ref={registerSection('dockerContainers')} class="scroll-mt-24">
              <ResourceTable
                title="Containers"
                groupedResources={dockerContainersGroupedByHost()}
                groupHeaderMeta={dockerHostGroupMeta()}
                columns={[
                  'CPU %',
                  'Memory %',
                  'Disk %',
                  'Restart Count',
                  'Restart Window (s)',
                  'Memory Warn %',
                  'Memory Critical %',
                ]}
                activeAlerts={props.activeAlerts}
                emptyMessage="No containers match the current filters."
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                showOfflineAlertsColumn={false}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                editingNote={editingNote}
                setEditingNote={setEditingNote}
                onBulkEdit={(ids) => handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %', 'Temp °C'])}
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={{
                  cpu: props.dockerDefaults.cpu,
                  memory: props.dockerDefaults.memory,
                  disk: props.dockerDefaults.disk,
                  restartCount: props.dockerDefaults.restartCount,
                  restartWindow: props.dockerDefaults.restartWindow,
                  memoryWarnPct: props.dockerDefaults.memoryWarnPct,
                  memoryCriticalPct: props.dockerDefaults.memoryCriticalPct,
                }}
                setGlobalDefaults={(value) => {
                  const current = {
                    cpu: props.dockerDefaults.cpu,
                    memory: props.dockerDefaults.memory,
                    disk: props.dockerDefaults.disk,
                    restartCount: props.dockerDefaults.restartCount,
                    restartWindow: props.dockerDefaults.restartWindow,
                    memoryWarnPct: props.dockerDefaults.memoryWarnPct,
                    memoryCriticalPct: props.dockerDefaults.memoryCriticalPct,
                  };
                  const next =
                    typeof value === 'function' ? value(current) : { ...current, ...value };

                  props.setDockerDefaults((prev) => ({
                    ...prev,
                    cpu: next.cpu ?? prev.cpu,
                    memory: next.memory ?? prev.memory,
                    disk: next.disk ?? prev.disk,
                    restartCount: next.restartCount ?? prev.restartCount,
                    restartWindow: next.restartWindow ?? prev.restartWindow,
                    memoryWarnPct: next.memoryWarnPct ?? prev.memoryWarnPct,
                    memoryCriticalPct: next.memoryCriticalPct ?? prev.memoryCriticalPct,
                  }));
                }}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllDockerContainers}
                onToggleGlobalDisable={() =>
                  props.setDisableAllDockerContainers(!props.disableAllDockerContainers())
                }
                globalDisableOfflineFlag={() => props.dockerDisableConnectivity()}
                onToggleGlobalDisableOffline={() =>
                  props.setDockerDisableConnectivity(!props.dockerDisableConnectivity())
                }
                showDelayColumn={true}
                globalDelaySeconds={props.timeThresholds().guest}
                metricDelaySeconds={props.metricTimeThresholds().guest ?? {}}
                onMetricDelayChange={(metric, value) => updateMetricDelay('guest', metric, value)}
                globalOfflineSeverity={props.dockerPoweredOffSeverity()}
                onSetGlobalOfflineState={(state) => {
                  if (state === 'off') {
                    props.setDockerDisableConnectivity(true);
                  } else {
                    props.setDockerDisableConnectivity(false);
                    props.setDockerPoweredOffSeverity(state === 'critical' ? 'critical' : 'warning');
                  }
                  props.setHasUnsavedChanges(true);
                }}
                onSetOfflineState={setOfflineState}
                factoryDefaults={props.factoryDockerDefaults}
                onResetDefaults={props.resetDockerDefaults}
              />
            </div>
          </Show>
        </Show>
      </div>

      <BulkEditDialog
        isOpen={isBulkEditDialogOpen()}
        onClose={() => setIsBulkEditDialogOpen(false)}
        selectedIds={bulkEditIds()}
        columns={bulkEditColumns()}
        onSave={handleSaveBulkEdit}
      />
    </div>
  );
}
