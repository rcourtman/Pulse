import { createSignal, onCleanup, batch } from 'solid-js';
import { createStore, produce, reconcile, unwrap } from 'solid-js/store';
import type {
  State,
  WSMessage,
  Alert,
  ResolvedAlert,
  ConnectedInfrastructureItem,
} from '@/types/api';
import type { Resource, ResourceCapability } from '@/types/resource';
import { createDefaultResourcePolicy } from '@/types/resource';
import { logger } from '@/utils/logger';
import { POLLING_INTERVALS, WEBSOCKET } from '@/constants';
import { notificationStore } from './notifications';
import { eventBus } from './events';
import { ALERTS_DETECTION_EVENT, isAlertsDetectionEnabled } from '@/utils/alertsActivation';
import { syncWithAgentCommand } from './containerUpdates';
import {
  getAgentDiscoveryResourceId,
  isAppContainerDiscoveryResourceType,
} from '@/utils/discoveryTarget';
import {
  buildFastResourceStorePatchOps,
  getFastResourceMergePatchKeys,
  mergeCanonicalResourceDeltaSnapshot,
  mergeCanonicalResourceSnapshot,
  unionResourceChangedKeys,
  type ResourceChangedKeys,
} from '@/utils/resourceStateAdapters';
import { apiFetchJSON } from '@/utils/apiClient';

// Advertised to the server as max_message_bytes on the upgrade request; the
// server withholds any state frame larger than this and the store recovers
// over REST. 32 MiB keeps estates several times past the old 8 MiB ceiling on
// the cheap socket delta path instead of the REST full-state fallback loop.
const MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES = 32 * 1024 * 1024; // 32 MiB
const AUTO_REGISTER_NOTIFICATION_FRESH_MS = 2 * 60 * 1000;
const AUTO_REGISTER_NOTIFICATION_FUTURE_SKEW_MS = 30 * 1000;
const AUTO_REGISTER_NOTIFICATION_DEDUPE_MS = 10 * 60 * 1000;
const AUTO_REGISTER_NOTIFICATION_SESSION_GRACE_MS = 5 * 1000;
const ACTIVE_ALERTS_REST_RECOVERY_MIN_INTERVAL_MS = 30 * 1000;
const ACTIVE_ALERTS_COLD_HYDRATION_DELAY_MS = 5 * 1000;

export type ActiveAlertsHydrationStatus = 'pending' | 'ready' | 'unavailable';

type TimestampedWSMessage = WSMessage & { timestamp?: number | string };
type AutoRegisterNotificationPayload = {
  type?: string;
  source?: string;
  host?: string;
  name?: string;
  nodeId?: string;
  nodeName?: string;
  timestamp?: number | string;
};

const shownAutoRegisterNotifications = new Map<string, number>();

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;
const asMergePatchRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined;
const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

// The frame ceiling has to be negotiated on the upgrade request. A message
// sent after open can race the server's initial-state build.
const buildSocketUrl = (rawUrl: string): string => {
  try {
    const parsed = new URL(rawUrl, window.location.href);
    if (parsed.protocol === 'http:') parsed.protocol = 'ws:';
    if (parsed.protocol === 'https:') parsed.protocol = 'wss:';
    parsed.searchParams.set('max_message_bytes', String(MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES));
    return parsed.toString();
  } catch {
    return rawUrl;
  }
};

const isInboundPayloadWithinLimit = (payload: string): boolean => {
  // JSON state is overwhelmingly ASCII. Avoid allocating a second multi-megabyte
  // Blob for normal messages, while retaining an exact UTF-8 check for strings
  // large enough that non-ASCII code points could cross the byte limit.
  if (payload.length <= Math.floor(MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES / 3)) return true;
  if (payload.length > MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES) return false;
  return new TextEncoder().encode(payload).byteLength <= MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES;
};

const applyJSONMergePatch = (current: unknown, patch: unknown): unknown => {
  const patchRecord = asMergePatchRecord(patch);
  if (!patchRecord) return patch;

  const result: Record<string, unknown> = { ...(asMergePatchRecord(current) ?? {}) };
  Object.entries(patchRecord).forEach(([key, value]) => {
    if (value === null) {
      delete result[key];
      return;
    }
    result[key] = asMergePatchRecord(value) ? applyJSONMergePatch(result[key], value) : value;
  });
  return result;
};

// Top-level keys a merge patch touches, with platformData expanded one level
// so the fast merge path can reason about the metric mirror leaves. Returns
// null when the patch replaces or deletes platformData wholesale — the change
// shape is then too coarse for a per-key fast path.
const collectResourcePatchKeys = (patch: Record<string, unknown>): readonly string[] | null => {
  const keys: string[] = [];
  for (const key of Object.keys(patch)) {
    if (key === 'id') continue;
    if (key === 'platformData') {
      const platformData = asRecord(patch.platformData);
      if (!platformData) return null;
      for (const leaf of Object.keys(platformData)) keys.push(`platformData.${leaf}`);
      continue;
    }
    keys.push(key);
  }
  return keys;
};

type KeyedStateDelta = { upserts?: unknown; removed?: unknown; order?: unknown };

const applyKeyedStateDelta = <T extends { id: string }>(
  current: readonly T[],
  delta: KeyedStateDelta,
): {
  entries: T[];
  changedIds: Set<string>;
  changedKeys: Map<string, readonly string[] | null>;
} => {
  const resourcesById = new Map(current.map((resource) => [resource.id, resource] as const));
  const removed = Array.isArray(delta.removed)
    ? new Set(delta.removed.filter((id): id is string => typeof id === 'string'))
    : new Set<string>();
  removed.forEach((id) => resourcesById.delete(id));
  const changedIds = new Set(removed);
  const changedKeys = new Map<string, readonly string[] | null>();
  removed.forEach((id) => changedKeys.set(id, null));

  const addedIds: string[] = [];
  if (Array.isArray(delta.upserts)) {
    delta.upserts.forEach((patch) => {
      const patchRecord = asRecord(patch);
      const id = asString(patchRecord?.id);
      if (!id || !patchRecord) return;
      changedIds.add(id);
      const isNewRow = !resourcesById.has(id);
      if (isNewRow) addedIds.push(id);
      if (isNewRow || changedKeys.has(id)) {
        // Added rows and repeated patches for one id in a single frame have no
        // usable per-key shape; force the full merge path.
        changedKeys.set(id, null);
      } else {
        changedKeys.set(id, collectResourcePatchKeys(patchRecord));
      }
      const next = applyJSONMergePatch(resourcesById.get(id), patch) as T;
      if (next.id === id) resourcesById.set(id, next);
    });
  }

  const requestedOrder = Array.isArray(delta.order)
    ? delta.order.filter((id): id is string => typeof id === 'string')
    : undefined;
  const order = requestedOrder ?? [
    ...current.map((resource) => resource.id).filter((id) => !removed.has(id)),
    ...addedIds,
  ];
  const ordered = order
    .map((id) => resourcesById.get(id))
    .filter((resource): resource is T => Boolean(resource));
  const included = new Set(ordered.map((resource) => resource.id));
  resourcesById.forEach((resource, id) => {
    if (!included.has(id)) ordered.push(resource);
  });
  return { entries: ordered, changedIds, changedKeys };
};

const applyResourceStateDelta = (
  current: readonly Resource[],
  delta: KeyedStateDelta,
): {
  resources: Resource[];
  changedIds: Set<string>;
  changedKeys: Map<string, readonly string[] | null>;
} => {
  const applied = applyKeyedStateDelta(current, delta);
  return {
    resources: applied.entries,
    changedIds: applied.changedIds,
    changedKeys: applied.changedKeys,
  };
};

const parseTimestampMs = (value: unknown): number | null => {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value < 1_000_000_000_000 ? value * 1000 : value;
  }
  if (typeof value !== 'string' || value.trim().length === 0) {
    return null;
  }
  const numeric = Number(value);
  if (Number.isFinite(numeric)) {
    return numeric < 1_000_000_000_000 ? numeric * 1000 : numeric;
  }
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? null : parsed;
};

const resolveMessageTimestampMs = (message: TimestampedWSMessage): number | null => {
  const envelopeTimestamp = parseTimestampMs(message.timestamp);
  if (envelopeTimestamp !== null) {
    return envelopeTimestamp;
  }
  return parseTimestampMs(asRecord((message as { data?: unknown }).data)?.timestamp);
};

const buildAutoRegisterNotificationKey = (node: AutoRegisterNotificationPayload): string => {
  const nodeIdentity = node.nodeId || node.nodeName || node.name || node.host || 'unknown';
  return [node.type || 'unknown', nodeIdentity, node.host || ''].join('|');
};

const shouldShowAutoRegisterNotification = (
  message: TimestampedWSMessage,
  node: AutoRegisterNotificationPayload,
  now = Date.now(),
  connectionOpenedAtMs = 0,
): boolean => {
  const source = node.source?.trim().toLowerCase();
  if (source && source !== 'script') {
    return false;
  }

  const eventTimestampMs = resolveMessageTimestampMs(message);
  if (eventTimestampMs === null) {
    return false;
  }
  if (
    connectionOpenedAtMs > 0 &&
    eventTimestampMs < connectionOpenedAtMs - AUTO_REGISTER_NOTIFICATION_SESSION_GRACE_MS
  ) {
    return false;
  }
  if (
    eventTimestampMs < now - AUTO_REGISTER_NOTIFICATION_FRESH_MS ||
    eventTimestampMs > now + AUTO_REGISTER_NOTIFICATION_FUTURE_SKEW_MS
  ) {
    return false;
  }

  for (const [key, shownAt] of shownAutoRegisterNotifications) {
    if (now - shownAt > AUTO_REGISTER_NOTIFICATION_DEDUPE_MS) {
      shownAutoRegisterNotifications.delete(key);
    }
  }

  const key = buildAutoRegisterNotificationKey(node);
  const previousShownAt = shownAutoRegisterNotifications.get(key);
  if (
    typeof previousShownAt === 'number' &&
    now - previousShownAt <= AUTO_REGISTER_NOTIFICATION_DEDUPE_MS
  ) {
    return false;
  }
  shownAutoRegisterNotifications.set(key, now);
  return true;
};

// Type-safe WebSocket store
export function createWebSocketStore(url: string) {
  let wsUrl = url;
  const [connected, setConnected] = createSignal(false);
  const [reconnecting, setReconnecting] = createSignal(false);
  const [initialDataReceived, setInitialDataReceived] = createSignal(false);
  const createInitialState = (): State => ({
    // Canonical v6 state comes from unified resources.
    connectedInfrastructure: [],
    metrics: [],
    performance: {
      apiCallDuration: {},
      lastPollDuration: 0,
      pollingStartTime: '',
      totalApiCalls: 0,
      failedApiCalls: 0,
      cacheHits: 0,
      cacheMisses: 0,
    },
    connectionHealth: {},
    stats: {
      startTime: new Date().toISOString(),
      uptime: 0,
      pollingCycles: 0,
      webSocketClients: 0,
      version: '2.0.0',
    },
    activeAlerts: [],
    recentlyResolved: [],
    lastUpdate: 0,
    pveTagColors: {},
    pveTagStyles: {},
    // Unified resources for cross-platform monitoring
    resources: [],
  });
  const [state, setState] = createStore<State>(createInitialState());
  const [activeAlerts, setActiveAlerts] = createStore<Record<string, Alert>>({});
  const [activeAlertsHydrationStatus, setActiveAlertsHydrationStatus] =
    createSignal<ActiveAlertsHydrationStatus>('pending');
  const [recentlyResolved, setRecentlyResolved] = createStore<Record<string, ResolvedAlert>>({});
  const [updateProgress, setUpdateProgress] = createSignal<unknown>(null);
  let resourceChangeVersion = 0;
  const [resourceChange, setResourceChange] = createSignal<{
    version: number;
    changedIds: ReadonlySet<string> | null;
    changedKeys: ResourceChangedKeys | null;
  }>({ version: 0, changedIds: null, changedKeys: null });
  // Bounded per-tick changed-id history so a consumer that mounted or resumed
  // a few ticks behind the live version can still catch up with a delta merge
  // instead of a full-estate remerge. A full-snapshot commit (changedIds null)
  // invalidates the whole span, so the history resets there.
  const RESOURCE_CHANGE_HISTORY_LIMIT = 30;
  let resourceChangeHistory: {
    version: number;
    changedIds: ReadonlySet<string>;
    changedKeys: ResourceChangedKeys | null;
  }[] = [];

  const changedResourceIdsSince = (sinceVersion: number): ReadonlySet<string> | null =>
    changedResourceMetaSince(sinceVersion)?.changedIds ?? null;

  // Union of changed ids and their per-key change shapes across the history
  // window (sinceVersion, current]. changedKeys entries degrade to null for a
  // row whenever any covered tick could not describe its change shape.
  const changedResourceMetaSince = (
    sinceVersion: number,
  ): { changedIds: ReadonlySet<string>; changedKeys: ResourceChangedKeys } | null => {
    if (sinceVersion >= resourceChangeVersion) return null;
    if (resourceChangeHistory.length === 0) return null;
    if (resourceChangeHistory[0].version > sinceVersion + 1) return null;
    if (resourceChangeHistory[resourceChangeHistory.length - 1].version !== resourceChangeVersion) {
      return null;
    }
    const union = new Set<string>();
    const keysUnion = new Map<string, readonly string[] | null>();
    for (const entry of resourceChangeHistory) {
      if (entry.version <= sinceVersion) continue;
      entry.changedIds.forEach((id) => {
        union.add(id);
        const tickKeys = entry.changedKeys ? (entry.changedKeys.get(id) ?? null) : null;
        keysUnion.set(
          id,
          keysUnion.has(id) ? unionResourceChangedKeys(keysUnion.get(id), tickKeys) : tickKeys,
        );
      });
    }
    return { changedIds: union, changedKeys: keysUnion };
  };

  // Track alerts with pending acknowledgment changes to prevent race conditions
  const pendingAckChanges = new Map<string, { ack: boolean; previousAckTime?: string }>();
  const pendingAckTimeouts = new Map<string, number>();

  let alertsEnabled = isAlertsDetectionEnabled();
  let lastActiveAlertsPayload: Record<string, Alert> = {};
  let hasActiveAlertsSnapshot = false;
  let activeAlertsRevision = 0;

  const clearPendingAckTimeout = (alertIdentifier: string) => {
    const timeout = pendingAckTimeouts.get(alertIdentifier);
    if (timeout) {
      window.clearTimeout(timeout);
      pendingAckTimeouts.delete(alertIdentifier);
    }
  };

  const clearPendingAck = (alertIdentifier: string) => {
    pendingAckChanges.delete(alertIdentifier);
    clearPendingAckTimeout(alertIdentifier);
  };

  const applyActiveAlerts = (alertsMap: Record<string, Alert>) => {
    // Remove alerts that no longer exist
    const currentAlertIds = Object.keys(activeAlerts);
    currentAlertIds.forEach((id) => {
      if (!alertsMap[id]) {
        setActiveAlerts(id, undefined as unknown as Alert);
      }
    });

    // Add or update alerts with pending acknowledgment safeguards
    Object.entries(alertsMap).forEach(([id, alert]) => {
      if (pendingAckChanges.has(id)) {
        const pending = pendingAckChanges.get(id)!;

        if (pending.ack) {
          if (!alert.acknowledged) {
            logger.debug(
              `Skipping update for alert ${id} - awaiting server acknowledgment confirmation`,
            );
            return;
          }

          const serverAckTime = alert.ackTime || '';
          const previousAckTime = pending.previousAckTime || '';
          if (serverAckTime === previousAckTime) {
            logger.debug(
              `Server ack time for alert ${id} unchanged (${serverAckTime}). Treating as confirmed`,
            );
          }
        } else if (alert.acknowledged) {
          logger.debug(
            `Skipping update for alert ${id} - awaiting server unacknowledge confirmation`,
          );
          return;
        }

        clearPendingAck(id);
      }

      setActiveAlerts(id, alert);
    });

    setState('activeAlerts', Object.values(alertsMap));
  };

  const handleAlertsDetectionEvent = (event: Event) => {
    const detail = (event as CustomEvent<boolean | null>).detail;
    alertsEnabled = detail ?? true;
    applyActiveAlerts(alertsEnabled ? lastActiveAlertsPayload : {});
  };

  if (typeof window !== 'undefined') {
    window.addEventListener(ALERTS_DETECTION_EVENT, handleAlertsDetectionEvent, {
      passive: true,
    });
  }

  let ws: WebSocket | null = null;
  // Pristine copy of the server's resource array. Resource deltas are diffed
  // against the server's per-client snapshot, so they must be applied to this
  // raw payload — never to the canonically merged `state.resources`, whose
  // coalesced host IDs and enriched fields no longer match the server
  // baseline. Kept isolated from the store: entries handed to the canonical
  // merge are cloned first, because the merge output shares nested references
  // and Solid's reconcile mutates adopted objects in place.
  let rawServerResources: Resource[] | null = null;
  const deferredResourceIds = new Set<string>();
  // Per-key change shapes for deferred ids, unioned across the hidden ticks so
  // the resume merge can still take the fast path for metrics-only rows.
  const deferredResourceKeys = new Map<string, readonly string[] | null>();
  // Resource ticks defer while operator input is active (scrolling, wheel,
  // pointer presses, typing), not just while the tab is hidden: on a large
  // estate even the baseline patch walks the whole resource array, so a tick
  // landing mid-interaction blocks the main thread for hundreds of
  // milliseconds and delays the response to that input. Input-gated ticks
  // queue unapplied in arrival order and land at input idle, when nothing is
  // reacting to the operator. Bare pointer movement deliberately does not
  // gate: a resting hand on the mouse must not starve data freshness. Hidden
  // tabs keep the in-place baseline advance instead: a tab can stay hidden
  // for hours, and the queue must stay bounded by one interaction burst.
  const OPERATOR_INPUT_ACTIVE_WINDOW_MS = 250;
  const DEFERRED_FLUSH_RECHECK_MS = 300;
  const deferredResourceDeltaQueue: { upserts?: unknown; removed?: unknown; order?: unknown }[] =
    [];
  // The reporting projection arrives as per-item merge patches keyed like
  // resources. The raw baseline mirrors the server's per-client snapshot and
  // stays isolated from the store; the pending set records which items the
  // store has not reconciled yet, so a sync touches only changed items and a
  // tick never deep-walks the whole projection again.
  let rawConnectedInfrastructure: ConnectedInfrastructureItem[] | null = null;
  const pendingInfrastructureIds = new Set<string>();
  let pendingInfrastructureFull = false;
  // Active alerts ride the same keyed delta transport, but their application
  // is never gated: alert lifecycle updates stay immediate during operator
  // input (alerts subsystem contract), so this baseline only slims the wire.
  let rawActiveAlerts: Alert[] | null = null;
  // lastUpdate is the realtime tick token every downstream consumer keys on
  // (unified resource projections, workload remaps). It defers with the rest
  // so a mid-interaction tick triggers no derived recomputation at all.
  let deferredLastUpdate: number | null = null;
  let lastOperatorInputAt = 0;
  let deferredFlushTimer = 0;
  // Latest capabilityCatalog from the state payload. Broadcast resources carry
  // capabilitiesRef instead of inline capability blobs; ingestion expands the
  // ref back into `capabilities` so consumers keep the inline shape.
  let capabilityCatalog: Record<string, ResourceCapability[]> = {};
  // Same dedupe contract for non-default policies and AI-safe summaries.
  let policyCatalog: Record<string, Resource['policy']> = {};
  let aiSafeSummaryCatalog: Record<string, string> = {};
  // Records which catalog ref an expanded capabilities array or policy was
  // cloned from, so a patched row re-expands only when its ref changed.
  const capabilitiesExpandedFromRef = new WeakMap<object, string>();
  const policyExpandedFromRef = new WeakMap<object, string>();

  const hydrateSlimResource = (resource: Resource): void => {
    if (!resource || typeof resource !== 'object') return;
    const ref = resource.capabilitiesRef;
    if (ref) {
      const current = resource.capabilities as unknown as object | undefined;
      if (!current || capabilitiesExpandedFromRef.get(current) !== ref) {
        const entry = capabilityCatalog[ref];
        if (entry) {
          // Per-row clone: reconcile mutates adopted objects in place, so rows
          // must not share one materialized capabilities array.
          const expanded = structuredClone(entry);
          capabilitiesExpandedFromRef.set(expanded as unknown as object, ref);
          resource.capabilities = expanded;
        }
      }
    }
    const policyRef = resource.policyRef;
    if (policyRef) {
      const currentPolicy = resource.policy as unknown as object | undefined;
      if (!currentPolicy || policyExpandedFromRef.get(currentPolicy) !== policyRef) {
        const policyEntry = policyCatalog[policyRef];
        if (policyEntry) {
          // Per-row clone: reconcile mutates adopted objects in place.
          const expandedPolicy = structuredClone(policyEntry);
          policyExpandedFromRef.set(expandedPolicy as unknown as object, policyRef);
          resource.policy = expandedPolicy;
        }
      }
    }
    if (resource.aiSafeSummaryRef) {
      const summary = aiSafeSummaryCatalog[resource.aiSafeSummaryRef];
      // Strings are immutable; no clone needed.
      if (summary) resource.aiSafeSummary = summary;
    }
    if (!resource.policy) {
      resource.policy = createDefaultResourcePolicy();
    }
  };
  let lastFullStateRecoveryAt = 0;
  // Set once the server has sent a full snapshot too large for the inbound
  // guard. Asking for another one over the socket would only reproduce the same
  // oversized frame, so recovery has to leave the WebSocket entirely.
  let oversizedSnapshotObserved = false;
  let restHydrationConnectionId: number | null = null;
  let pendingRestHydrationConnectionId: number | null = null;
  let restHydrationTimeout = 0;
  let nextConnectionId = 0;
  let activeConnectionId: number | null = null;
  let reconnectTimeout = 0;
  let reconnectDelayTimeout = 0;
  let lastServerActivityAt = Date.now();
  let heartbeatInterval = 0;
  let reconnectAttempt = 0;
  let isReconnecting = false;
  let isDisposed = false;
  let activeAlertsRecoveryRequestId = 0;
  let activeAlertsRecoveryInFlight: Promise<boolean> | null = null;
  let lastActiveAlertsRecoveryAt = 0;
  let activeAlertsColdHydrationTimeout = 0;
  let currentConnectionOpenedAtMs = 0;
  const maxReconnectDelay = POLLING_INTERVALS.RECONNECT_MAX;
  const initialReconnectDelay = POLLING_INTERVALS.RECONNECT_BASE;
  const heartbeatIntervalMs = 30000; // Send heartbeat every 30 seconds
  const heartbeatTimeoutMs = heartbeatIntervalMs * 3;
  const reconnectJitterRatio = 0.2;

  const commitActiveAlertsTruth = (alertsMap: Record<string, Alert>) => {
    if (activeAlertsColdHydrationTimeout) {
      window.clearTimeout(activeAlertsColdHydrationTimeout);
      activeAlertsColdHydrationTimeout = 0;
    }
    activeAlertsRevision += 1;
    hasActiveAlertsSnapshot = true;
    lastActiveAlertsPayload = alertsMap;
    setActiveAlertsHydrationStatus('ready');
    applyActiveAlerts(alertsEnabled ? alertsMap : {});
  };

  // Cold-start recovery is owned by the canonical store rather than the page.
  // HTTP may refresh the displayed alert projection, but it never establishes
  // the socket-owned keyed-delta baseline. A revision fence prevents a late
  // HTTP response from replacing newer WebSocket truth.
  const recoverActiveAlertsFromREST = (force = false): Promise<boolean> => {
    if (isDisposed) return Promise.resolve(false);
    if (activeAlertsRecoveryInFlight) return activeAlertsRecoveryInFlight;

    const now = Date.now();
    if (
      !force &&
      lastActiveAlertsRecoveryAt > 0 &&
      now - lastActiveAlertsRecoveryAt < ACTIVE_ALERTS_REST_RECOVERY_MIN_INTERVAL_MS
    ) {
      return Promise.resolve(false);
    }

    lastActiveAlertsRecoveryAt = now;
    const requestId = ++activeAlertsRecoveryRequestId;
    const startingRevision = activeAlertsRevision;
    if (!hasActiveAlertsSnapshot) {
      setActiveAlertsHydrationStatus('pending');
    }

    let request: Promise<boolean>;
    request = (async () => {
      try {
        const alerts = await apiFetchJSON<Alert[]>('/api/alerts/active');
        if (
          isDisposed ||
          requestId !== activeAlertsRecoveryRequestId ||
          startingRevision !== activeAlertsRevision
        ) {
          return false;
        }
        if (!Array.isArray(alerts)) {
          throw new Error('Active alert recovery returned an unusable payload');
        }

        const recoveredAlerts: Record<string, Alert> = {};
        for (const alert of alerts) {
          if (alert && typeof alert.id === 'string' && alert.id.length > 0) {
            recoveredAlerts[alert.id] = alert;
          }
        }
        // Intentionally leave rawActiveAlerts null. Only a connection-native
        // socket snapshot may become the baseline for activeAlertsDelta.
        commitActiveAlertsTruth(recoveredAlerts);
        logger.info('Recovered active alert truth over REST while live updates reconnect');
        return true;
      } catch (error) {
        if (
          !isDisposed &&
          requestId === activeAlertsRecoveryRequestId &&
          startingRevision === activeAlertsRevision &&
          !hasActiveAlertsSnapshot
        ) {
          setActiveAlertsHydrationStatus('unavailable');
          logger.error('Active alert REST recovery failed', error);
        }
        return false;
      } finally {
        if (requestId === activeAlertsRecoveryRequestId) {
          activeAlertsRecoveryInFlight = null;
        }
      }
    })();
    activeAlertsRecoveryInFlight = request;
    return request;
  };

  const scheduleColdActiveAlertsRecovery = (connectionId: number) => {
    if (hasActiveAlertsSnapshot || activeAlertsColdHydrationTimeout) return;
    activeAlertsColdHydrationTimeout = window.setTimeout(() => {
      activeAlertsColdHydrationTimeout = 0;
      if (!isCurrentConnection(connectionId) || hasActiveAlertsSnapshot) return;
      void recoverActiveAlertsFromREST();
    }, ACTIVE_ALERTS_COLD_HYDRATION_DELAY_MS);
  };

  const clearHeartbeatTimer = () => {
    if (heartbeatInterval) {
      window.clearInterval(heartbeatInterval);
      heartbeatInterval = 0;
    }
  };

  const computeReconnectDelay = (attempt: number) => {
    const baseDelay = Math.min(initialReconnectDelay * Math.pow(2, attempt), maxReconnectDelay);
    const jitterWindow = Math.floor(baseDelay * reconnectJitterRatio);
    if (jitterWindow <= 0) {
      return baseDelay;
    }

    const jitter = Math.floor(Math.random() * (jitterWindow * 2 + 1)) - jitterWindow;
    return Math.min(maxReconnectDelay, Math.max(0, baseDelay + jitter));
  };

  const clearReconnectTimeout = () => {
    if (reconnectTimeout) {
      window.clearTimeout(reconnectTimeout);
      reconnectTimeout = 0;
    }
  };

  const clearReconnectDelayTimeout = () => {
    if (reconnectDelayTimeout) {
      window.clearTimeout(reconnectDelayTimeout);
      reconnectDelayTimeout = 0;
    }
  };

  const clearHeartbeatInterval = () => {
    if (heartbeatInterval) {
      window.clearInterval(heartbeatInterval);
      heartbeatInterval = 0;
    }
  };

  const resetConnectionBaseline = () => {
    rawServerResources = null;
    deferredResourceIds.clear();
    deferredResourceKeys.clear();
    deferredResourceDeltaQueue.length = 0;
    rawConnectedInfrastructure = null;
    pendingInfrastructureIds.clear();
    pendingInfrastructureFull = false;
    rawActiveAlerts = null;
    deferredLastUpdate = null;
    lastFullStateRecoveryAt = 0;
    oversizedSnapshotObserved = false;
    // A recovery request from a retired connection may still settle, but its
    // connection id prevents it from mutating this connection. Clearing the
    // owner here lets the replacement connection start its own recovery now.
    restHydrationConnectionId = null;
    pendingRestHydrationConnectionId = null;
    if (restHydrationTimeout) {
      window.clearTimeout(restHydrationTimeout);
      restHydrationTimeout = 0;
    }
  };

  const syncContainerCommands = (
    resources: readonly Resource[],
    changedResourceIds?: ReadonlySet<string>,
  ) => {
    const commandSyncResources = changedResourceIds
      ? resources.filter((resource) => changedResourceIds.has(resource.id))
      : resources;
    commandSyncResources.forEach((resource: any) => {
      if (resource?.type !== 'docker-host') return;
      const platformData = asRecord(resource.platformData);
      const dockerData = asRecord(platformData?.docker);
      const command = dockerData?.command || platformData?.command;
      if (!command || typeof command !== 'object') return;

      const agentIds = new Set<string>([
        resource.id,
        asString(dockerData?.hostSourceId) || '',
        asString(platformData?.hostSourceId) || '',
        asString(resource?.discoveryTarget?.agentId) || '',
        isAppContainerDiscoveryResourceType(resource?.discoveryTarget?.resourceType)
          ? asString(resource?.discoveryTarget?.resourceId) || ''
          : '',
      ]);
      agentIds.forEach((agentId) => {
        if (agentId) {
          syncWithAgentCommand(agentId, command as any);
        }
      });
    });
  };

  const commitResources = (
    nextResources: Resource[],
    changedResourceIds?: ReadonlySet<string>,
    changedResourceKeys?: ResourceChangedKeys,
  ) => {
    logger.debug('[WebSocket] Updating resources', {
      count: nextResources.length,
      changedCount: changedResourceIds?.size ?? nextResources.length,
    });
    // A metrics tick leaves row count and order untouched, so it can commit as
    // per-index writes instead of a whole-array reconcile (whose unwrap pass
    // deep-walks every merged row). Delta-merge pass-through rows keep their
    // store proxy identity, so the alignment scan skips them by equality.
    const currentResources = state.resources;
    let alignedPatchIndices: number[] | null = null;
    if (
      changedResourceIds &&
      Array.isArray(currentResources) &&
      currentResources.length === nextResources.length
    ) {
      alignedPatchIndices = [];
      for (let index = 0; index < nextResources.length; index += 1) {
        const nextRow = nextResources[index];
        const currentRow = currentResources[index];
        if (nextRow === currentRow) continue;
        if (!nextRow || !currentRow || nextRow.id !== currentRow.id) {
          alignedPatchIndices = null;
          break;
        }
        alignedPatchIndices.push(index);
      }
    }
    if (alignedPatchIndices) {
      for (const index of alignedPatchIndices) {
        const row = nextResources[index];
        const fastKeys = getFastResourceMergePatchKeys(
          changedResourceKeys,
          row.id,
          currentResources[index],
        );
        if (fastKeys) {
          for (const op of buildFastResourceStorePatchOps(row, fastKeys)) {
            if (op.leaf !== undefined) {
              setState(
                'resources',
                index,
                op.key as keyof Resource,
                op.leaf as never,
                (op.mode === 'reconcile' ? reconcile(op.value) : op.value) as never,
              );
            } else {
              setState(
                'resources',
                index,
                op.key as keyof Resource,
                (op.mode === 'reconcile' ? reconcile(op.value) : op.value) as never,
              );
            }
          }
        } else {
          setState('resources', index, reconcile(row, { key: 'id' }));
        }
      }
    } else {
      setState('resources', reconcile(nextResources, { key: 'id' }));
    }
    const committedChangedIds = changedResourceIds ? new Set(changedResourceIds) : null;
    const committedChangedKeys = changedResourceKeys ?? null;
    setResourceChange({
      version: ++resourceChangeVersion,
      changedIds: committedChangedIds,
      changedKeys: committedChangedKeys,
    });
    if (committedChangedIds) {
      resourceChangeHistory.push({
        version: resourceChangeVersion,
        changedIds: committedChangedIds,
        changedKeys: committedChangedKeys,
      });
      if (resourceChangeHistory.length > RESOURCE_CHANGE_HISTORY_LIMIT) {
        resourceChangeHistory.splice(
          0,
          resourceChangeHistory.length - RESOURCE_CHANGE_HISTORY_LIMIT,
        );
      }
    } else {
      resourceChangeHistory = [];
    }
    syncContainerCommands(nextResources, changedResourceIds);
  };

  const isOperatorInputActive = () =>
    Date.now() - lastOperatorInputAt < OPERATOR_INPUT_ACTIVE_WINDOW_MS;

  // Deltas queued during an active scroll apply strictly in arrival order.
  // Draining advances the raw baseline and unions the change shapes into the
  // deferral set without reconciling, so one merge materializes everything.
  const applyQueuedResourceDeltas = () => {
    if (deferredResourceDeltaQueue.length === 0) return;
    const queued = deferredResourceDeltaQueue.splice(0);
    if (!rawServerResources) return;
    for (const delta of queued) {
      const appliedDelta = applyResourceStateDelta(rawServerResources, delta);
      appliedDelta.resources.forEach(hydrateSlimResource);
      rawServerResources = appliedDelta.resources;
      appliedDelta.changedIds.forEach((id) => {
        const tickKeys = appliedDelta.changedKeys.get(id) ?? null;
        deferredResourceKeys.set(
          id,
          deferredResourceIds.has(id)
            ? unionResourceChangedKeys(deferredResourceKeys.get(id), tickKeys)
            : tickKeys,
        );
        deferredResourceIds.add(id);
      });
    }
  };

  // Reconcile the projection store from the raw baseline, touching only items
  // whose ids are pending. Untouched items keep their current store references
  // so reconcile's per-item identity check skips them outright; changed items
  // are cloned so the store never adopts baseline-owned objects.
  const syncInfrastructureStore = () => {
    const baseline = rawConnectedInfrastructure;
    if (baseline === null) return;
    if (pendingInfrastructureFull) {
      pendingInfrastructureFull = false;
      pendingInfrastructureIds.clear();
      setState('connectedInfrastructure', reconcile(structuredClone(baseline), { key: 'id' }));
      return;
    }
    if (pendingInfrastructureIds.size === 0) return;
    const changed = new Set(pendingInfrastructureIds);
    pendingInfrastructureIds.clear();
    const currentItems = unwrap(state.connectedInfrastructure) as ConnectedInfrastructureItem[];
    const currentById = new Map(currentItems.map((item) => [item.id, item] as const));
    const next = baseline.map((item) => {
      if (!changed.has(item.id)) {
        const existing = currentById.get(item.id);
        if (existing) return existing;
      }
      return structuredClone(item);
    });
    setState('connectedInfrastructure', reconcile(next, { key: 'id' }));
  };

  const flushDeferredResources = () => {
    const pendingLastUpdate = deferredLastUpdate;
    deferredLastUpdate = null;
    if (rawServerResources !== null) {
      applyQueuedResourceDeltas();
    }
    const baseline = rawServerResources;
    let commitDeferredMerge: (() => void) | undefined;
    if (baseline !== null && deferredResourceIds.size > 0) {
      const changedResourceIds = new Set(deferredResourceIds);
      const changedResourceKeys = new Map(deferredResourceKeys);
      deferredResourceIds.clear();
      deferredResourceKeys.clear();
      const nextResources = mergeCanonicalResourceDeltaSnapshot(
        baseline,
        state.resources,
        changedResourceIds,
        changedResourceKeys,
      );
      commitDeferredMerge = () =>
        commitResources(nextResources, changedResourceIds, changedResourceKeys);
    }
    const hasInfrastructureFlush =
      rawConnectedInfrastructure !== null &&
      (pendingInfrastructureFull || pendingInfrastructureIds.size > 0);
    if (!commitDeferredMerge && !hasInfrastructureFlush && pendingLastUpdate === null) {
      return;
    }
    batch(() => {
      syncInfrastructureStore();
      commitDeferredMerge?.();
      if (pendingLastUpdate !== null) {
        setState('lastUpdate', pendingLastUpdate);
      }
    });
  };

  // Flush deferred ticks once nothing gates them: a hidden document flushes
  // through the visibilitychange listener, and active scrolling re-arms a
  // recheck until the gesture goes idle.
  const scheduleDeferredResourceFlush = () => {
    if (
      deferredResourceIds.size === 0 &&
      deferredResourceDeltaQueue.length === 0 &&
      !pendingInfrastructureFull &&
      pendingInfrastructureIds.size === 0 &&
      deferredLastUpdate === null
    ) {
      return;
    }
    if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return;
    if (!isOperatorInputActive()) {
      flushDeferredResources();
      return;
    }
    if (deferredFlushTimer) return;
    deferredFlushTimer = window.setTimeout(() => {
      deferredFlushTimer = 0;
      scheduleDeferredResourceFlush();
    }, DEFERRED_FLUSH_RECHECK_MS);
  };

  const handleResourceVisibilityChange = () => {
    if (document.visibilityState !== 'visible') return;
    scheduleDeferredResourceFlush();
  };

  const handleOperatorInput = () => {
    lastOperatorInputAt = Date.now();
  };
  const OPERATOR_INPUT_EVENTS = ['scroll', 'wheel', 'pointerdown', 'keydown'] as const;

  if (typeof document !== 'undefined') {
    document.addEventListener('visibilitychange', handleResourceVisibilityChange, {
      passive: true,
    });
  }
  if (typeof window !== 'undefined') {
    // Capture phase: scroll events do not bubble, so a window-level capture
    // listener is the only way to observe every scroller (app shell, drawers)
    // without adding per-surface listeners; the other input events ride the
    // same registration.
    for (const eventName of OPERATOR_INPUT_EVENTS) {
      window.addEventListener(eventName, handleOperatorInput, { capture: true, passive: true });
    }
  }

  const beginConnection = () => {
    if (activeAlertsColdHydrationTimeout) {
      window.clearTimeout(activeAlertsColdHydrationTimeout);
      activeAlertsColdHydrationTimeout = 0;
    }
    const connectionId = ++nextConnectionId;
    activeConnectionId = connectionId;
    resetConnectionBaseline();
    setInitialDataReceived(false);
    return connectionId;
  };

  const isCurrentConnection = (connectionId: number) =>
    !isDisposed && activeConnectionId === connectionId;

  const retireConnection = (connectionId: number) => {
    if (activeConnectionId !== connectionId) return false;
    if (activeAlertsColdHydrationTimeout) {
      window.clearTimeout(activeAlertsColdHydrationTimeout);
      activeAlertsColdHydrationTimeout = 0;
    }
    activeConnectionId = null;
    resetConnectionBaseline();
    setInitialDataReceived(false);
    return true;
  };

  const shutdown = () => {
    if (isDisposed) return;
    isDisposed = true;
    isReconnecting = false;
    setReconnecting(false);
    clearReconnectTimeout();
    clearReconnectDelayTimeout();
    clearHeartbeatInterval();
    pendingAckTimeouts.forEach((timeout) => window.clearTimeout(timeout));
    pendingAckTimeouts.clear();
    pendingAckChanges.clear();
    activeAlertsRecoveryRequestId += 1;
    activeAlertsRecoveryInFlight = null;
    if (activeAlertsColdHydrationTimeout) {
      window.clearTimeout(activeAlertsColdHydrationTimeout);
      activeAlertsColdHydrationTimeout = 0;
    }
    activeConnectionId = null;
    resetConnectionBaseline();

    if (typeof window !== 'undefined') {
      window.removeEventListener(ALERTS_DETECTION_EVENT, handleAlertsDetectionEvent);
    }

    if (ws) {
      ws.onopen = null;
      ws.onmessage = null;
      ws.onclose = null;
      ws.onerror = null;
      ws.close(1000, 'Component unmounting');
      ws = null;
    }
  };

  const connect = () => {
    if (isDisposed) return;
    clearReconnectDelayTimeout();
    const connectionId = beginConnection();

    try {
      // Close existing connection if any
      if (ws) {
        if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
          ws.close(1000, 'Reconnecting');
        }
        ws = null;
      }

      const socket = new WebSocket(buildSocketUrl(wsUrl));
      ws = socket;
      setupWebSocket(socket, connectionId);
    } catch (err) {
      retireConnection(connectionId);
      logger.error('Failed to create WebSocket', err);
      if (!hasActiveAlertsSnapshot) {
        void recoverActiveAlertsFromREST();
      }
      handleReconnect();
    }
  };

  const handleReconnect = () => {
    if (isDisposed || isReconnecting) return;

    isReconnecting = true;
    setReconnecting(true);

    // Clear any existing timeout
    clearReconnectTimeout();

    // Calculate exponential backoff delay with jitter
    const delay = computeReconnectDelay(reconnectAttempt);

    logger.info(`Reconnecting in ${delay}ms (attempt ${reconnectAttempt + 1})`);
    reconnectAttempt++;

    reconnectTimeout = window.setTimeout(() => {
      if (isDisposed) {
        isReconnecting = false;
        return;
      }
      isReconnecting = false;
      if (isDisposed) return;
      connect();
    }, delay);
  };

  // Applies one decoded server message to the store. Socket snapshots may
  // establish a delta baseline; REST snapshots update display state only.
  const processServerMessage = (
    data: unknown,
    connectionId: number,
    source: 'websocket' | 'rest' = 'websocket',
  ) => {
    if (!isCurrentConnection(connectionId)) return;
    try {
      const message = data as TimestampedWSMessage;

      if (
        message.type === WEBSOCKET.MESSAGE_TYPES.INITIAL_STATE ||
        message.type === WEBSOCKET.MESSAGE_TYPES.RAW_DATA
      ) {
        const resourceDeltaWithoutBaseline =
          message.data !== undefined &&
          message.data.resources === undefined &&
          'resourceDelta' in message.data &&
          message.data.resourceDelta !== undefined &&
          rawServerResources === null;
        // Update state properties individually, but batch the whole payload to
        // reduce reactive recomputations and UI thrash on large updates.
        if (message.data)
          batch(() => {
            // Mark that we've received usable data (initial payload or raw update)
            if (!initialDataReceived() && !resourceDeltaWithoutBaseline) {
              setInitialDataReceived(true);
            }

            // Adopt the capability catalog before any resource handling below:
            // the delta engine ships catalog changes in the same frame as the
            // first resource that references a new entry.
            if (message.data.capabilityCatalog !== undefined) {
              capabilityCatalog = message.data.capabilityCatalog ?? {};
            }
            if (message.data.policyCatalog !== undefined) {
              policyCatalog = message.data.policyCatalog ?? {};
            }
            if (message.data.aiSafeSummaryCatalog !== undefined) {
              aiSafeSummaryCatalog = message.data.aiSafeSummaryCatalog ?? {};
            }

            // Canonical resource contract:
            // `state.resources` is the authoritative frontend model.
            // `state.connectedInfrastructure` is the authoritative reporting projection.
            if (message.data.connectedInfrastructure !== undefined) {
              const connectedInfrastructure = Array.isArray(message.data.connectedInfrastructure)
                ? (message.data.connectedInfrastructure as ConnectedInfrastructureItem[])
                : [];
              pendingInfrastructureIds.clear();
              if (source === 'websocket') {
                // Only a socket payload shares the server's per-client delta
                // lineage. A REST recovery is an independently built display
                // snapshot and must never become the baseline for later socket
                // patches.
                rawConnectedInfrastructure = structuredClone(connectedInfrastructure);
                pendingInfrastructureFull = true;
                if (
                  typeof document !== 'undefined' &&
                  document.visibilityState !== 'hidden' &&
                  isOperatorInputActive()
                ) {
                  scheduleDeferredResourceFlush();
                } else {
                  syncInfrastructureStore();
                }
              } else {
                rawConnectedInfrastructure = null;
                pendingInfrastructureFull = false;
                setState(
                  'connectedInfrastructure',
                  reconcile(structuredClone(connectedInfrastructure), { key: 'id' }),
                );
              }
            } else if (
              'connectedInfrastructureDelta' in message.data &&
              message.data.connectedInfrastructureDelta !== undefined
            ) {
              if (rawConnectedInfrastructure !== null) {
                const appliedInfrastructure = applyKeyedStateDelta(
                  rawConnectedInfrastructure,
                  message.data.connectedInfrastructureDelta as KeyedStateDelta,
                );
                rawConnectedInfrastructure = appliedInfrastructure.entries;
                appliedInfrastructure.changedIds.forEach((id) => {
                  pendingInfrastructureIds.add(id);
                });
                if (
                  typeof document !== 'undefined' &&
                  document.visibilityState !== 'hidden' &&
                  isOperatorInputActive()
                ) {
                  scheduleDeferredResourceFlush();
                } else {
                  syncInfrastructureStore();
                }
              } else {
                requestFullStateRecovery(connectionId);
              }
            }
            if (message.data.metrics !== undefined) setState('metrics', message.data.metrics);
            if (message.data.performance !== undefined)
              setState('performance', message.data.performance);
            if (message.data.connectionHealth !== undefined)
              setState('connectionHealth', message.data.connectionHealth);
            if (message.data.stats !== undefined) setState('stats', message.data.stats);
            if (message.data.pveTagColors !== undefined) {
              setState('pveTagColors', message.data.pveTagColors ?? {});
            }
            if (message.data.pveTagStyles !== undefined) {
              setState('pveTagStyles', message.data.pveTagStyles ?? {});
            }
            // Handle unified resources
            let nextResources: Resource[] | undefined;
            let changedResourceIds: ReadonlySet<string> | undefined;
            let changedResourceKeys: ResourceChangedKeys | undefined;
            if (message.data.resources !== undefined) {
              // A full snapshot supersedes every pending deferral, and queued
              // deltas reference the baseline this snapshot replaces.
              deferredResourceIds.clear();
              deferredResourceKeys.clear();
              deferredResourceDeltaQueue.length = 0;
              if (Array.isArray(message.data.resources)) {
                // Expand capabilitiesRef and synthesize omitted default
                // policies before the rows feed the baseline clone and the
                // canonical merge.
                message.data.resources.forEach(hydrateSlimResource);
                // Only a full payload actually delivered on this socket can
                // establish its server-owned delta baseline. REST recovery is
                // an independently built display snapshot and stays baseline-
                // free until the server later delivers a socket snapshot.
                rawServerResources =
                  source === 'websocket'
                    ? (structuredClone(message.data.resources) as Resource[])
                    : null;
                if (source === 'websocket') {
                  oversizedSnapshotObserved = false;
                  pendingRestHydrationConnectionId = null;
                  if (restHydrationTimeout) {
                    window.clearTimeout(restHydrationTimeout);
                    restHydrationTimeout = 0;
                  }
                }
                nextResources = mergeCanonicalResourceSnapshot(
                  message.data.resources,
                  state.resources,
                );
              } else {
                rawServerResources = source === 'websocket' ? [] : null;
                nextResources = [];
              }
            } else if (
              'resourceDelta' in message.data &&
              message.data.resourceDelta !== undefined
            ) {
              if (rawServerResources) {
                if (
                  typeof document !== 'undefined' &&
                  document.visibilityState !== 'hidden' &&
                  (isOperatorInputActive() || deferredResourceDeltaQueue.length > 0)
                ) {
                  // Mid-gesture ticks queue unapplied so they cost nothing
                  // until scroll idle. A non-empty queue keeps queueing even
                  // after the gesture ends so deltas never apply out of order.
                  deferredResourceDeltaQueue.push(message.data.resourceDelta);
                  scheduleDeferredResourceFlush();
                } else {
                  // Preserve arrival order before applying this tick in place.
                  applyQueuedResourceDeltas();
                  const appliedDelta = applyResourceStateDelta(
                    rawServerResources,
                    message.data.resourceDelta,
                  );
                  // Patched rows are fresh objects: re-synthesize a nulled-out
                  // default policy and re-expand capabilities when a patch
                  // moved the ref. Unpatched rows no-op.
                  appliedDelta.resources.forEach(hydrateSlimResource);
                  rawServerResources = appliedDelta.resources;
                  changedResourceIds = appliedDelta.changedIds;
                  if (typeof document !== 'undefined' && document.visibilityState === 'hidden') {
                    changedResourceIds.forEach((id) => {
                      const tickKeys = appliedDelta.changedKeys.get(id) ?? null;
                      deferredResourceKeys.set(
                        id,
                        deferredResourceIds.has(id)
                          ? unionResourceChangedKeys(deferredResourceKeys.get(id), tickKeys)
                          : tickKeys,
                      );
                      deferredResourceIds.add(id);
                    });
                  } else {
                    const combinedKeys = appliedDelta.changedKeys;
                    deferredResourceIds.forEach((id) => {
                      combinedKeys.set(
                        id,
                        combinedKeys.has(id)
                          ? unionResourceChangedKeys(
                              combinedKeys.get(id),
                              deferredResourceKeys.get(id) ?? null,
                            )
                          : (deferredResourceKeys.get(id) ?? null),
                      );
                    });
                    changedResourceIds = new Set([...deferredResourceIds, ...changedResourceIds]);
                    deferredResourceIds.clear();
                    deferredResourceKeys.clear();
                    changedResourceKeys = combinedKeys;
                    nextResources = mergeCanonicalResourceDeltaSnapshot(
                      rawServerResources,
                      state.resources,
                      changedResourceIds,
                      changedResourceKeys,
                    );
                  }
                }
              } else {
                // A delta landed before any full snapshot (e.g. the initial
                // payload was dropped as oversized). There is no baseline to
                // patch, so re-acquire a full snapshot instead of applying the
                // delta to nothing and rendering stubs.
                requestFullStateRecovery(connectionId);
              }
            }
            if (nextResources !== undefined) {
              commitResources(nextResources, changedResourceIds, changedResourceKeys);
            }
            // Sync active alerts from state
            if (message.data.activeAlerts !== undefined) {
              const alertsArray =
                message.data.activeAlerts && Array.isArray(message.data.activeAlerts)
                  ? (message.data.activeAlerts as Alert[])
                  : [];
              // Only the socket can establish the server-owned keyed baseline.
              // REST recovery still refreshes the displayed alerts below.
              rawActiveAlerts = source === 'websocket' ? structuredClone(alertsArray) : null;
              const newAlerts: Record<string, Alert> = {};
              alertsArray.forEach((alert: Alert) => {
                newAlerts[alert.id] = alert;
              });

              commitActiveAlertsTruth(newAlerts);
            } else if (
              'activeAlertsDelta' in message.data &&
              message.data.activeAlertsDelta !== undefined
            ) {
              if (rawActiveAlerts !== null) {
                const appliedAlerts = applyKeyedStateDelta(
                  rawActiveAlerts,
                  message.data.activeAlertsDelta as KeyedStateDelta,
                );
                rawActiveAlerts = appliedAlerts.entries;
                // Cloned on the way out so the alerts store never adopts
                // baseline-owned objects. Applied immediately, input-active or
                // not: alert lifecycle truth does not wait for idle.
                const newAlerts: Record<string, Alert> = {};
                for (const alert of rawActiveAlerts) {
                  newAlerts[alert.id] = structuredClone(alert);
                }
                commitActiveAlertsTruth(newAlerts);
              } else {
                requestFullStateRecovery(connectionId);
              }
            }
            // Sync recently resolved alerts
            if (message.data.recentlyResolved !== undefined) {
              // Received recentlyResolved update

              // Update resolved alerts atomically to prevent race conditions
              const newResolvedAlerts: Record<string, ResolvedAlert> = {};
              if (message.data.recentlyResolved && Array.isArray(message.data.recentlyResolved)) {
                message.data.recentlyResolved.forEach((alert: ResolvedAlert) => {
                  newResolvedAlerts[alert.id] = alert;
                });
              }

              // Clear existing resolved alerts and set new ones
              const currentResolvedIds = Object.keys(recentlyResolved);
              currentResolvedIds.forEach((id) => {
                if (!newResolvedAlerts[id]) {
                  setRecentlyResolved(id, undefined as unknown as ResolvedAlert);
                }
              });

              // Add new resolved alerts
              Object.entries(newResolvedAlerts).forEach(([id, alert]) => {
                setRecentlyResolved(id, alert);
              });

              setState('recentlyResolved', Object.values(newResolvedAlerts));

              // Updated recentlyResolved
            }
            const nextLastUpdate =
              typeof message.data.lastUpdate === 'number' ? message.data.lastUpdate : Date.now();
            if (
              typeof document !== 'undefined' &&
              document.visibilityState !== 'hidden' &&
              isOperatorInputActive()
            ) {
              deferredLastUpdate = nextLastUpdate;
              scheduleDeferredResourceFlush();
            } else {
              deferredLastUpdate = null;
              setState('lastUpdate', nextLastUpdate);
            }
          });
        logger.debug('message', {
          type: message.type,
          hasData: !!message.data,
          resourceCount: message.data?.resources?.length || 0,
        });
      } else if (message.type === 'stateTooLarge') {
        // The server deliberately did not adopt the withheld payload as this
        // client's delta baseline. Invalidate any prior baseline before REST
        // hydration and remain delta-free until a full socket snapshot lands.
        rawServerResources = null;
        deferredResourceIds.clear();
        deferredResourceKeys.clear();
        deferredResourceDeltaQueue.length = 0;
        rawConnectedInfrastructure = null;
        pendingInfrastructureIds.clear();
        pendingInfrastructureFull = false;
        rawActiveAlerts = null;
        oversizedSnapshotObserved = true;
        logger.warn('Server withheld an oversized state payload. Recovering over REST', {
          supersedes: message.data.supersedes,
          bytes: message.data.bytes,
          maxBytes: message.data.maxBytes,
          resourceCount: message.data.resourceCount,
        });
        requestFullStateRecovery(connectionId);
      } else if (message.type === WEBSOCKET.MESSAGE_TYPES.ERROR) {
        logger.debug('error', message.error);
      } else if (message.type === 'ping') {
        // Respond to ping with pong
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'pong', data: { timestamp: Date.now() } }));
        }
      } else if (message.type === 'pong') {
        // Server acknowledged our ping
        logger.debug('Received pong from server');
      } else if (message.type === 'welcome') {
        // Welcome message from server
        logger.info('WebSocket connection established');
      } else if (message.type === 'alert') {
        // Individual alerts now handled via state sync
        logger.debug('New alert received (will sync with next state update)', message.data);
      } else if (message.type === 'alertResolved') {
        // Individual alert resolution now handled via state sync
        logger.info('Alert resolved (will sync with next state update)', {
          alertIdentifier: message.data.alertIdentifier,
        });
      } else if (message.type === 'update:progress') {
        // Update progress event
        setUpdateProgress(message.data);
        logger.info('Update progress:', message.data);
      } else if (message.type === 'node_auto_registered') {
        const node = message.data;
        const nodeName = node.name || node.host;
        const nodeType = node.type === 'pve' ? 'Proxmox VE' : 'Proxmox Backup Server';

        if (
          shouldShowAutoRegisterNotification(message, node, Date.now(), currentConnectionOpenedAtMs)
        ) {
          notificationStore.success(
            `${nodeType} node "${nodeName}" was successfully auto-registered and is now being monitored!`,
            8000,
          );
          eventBus.emit('node_auto_registered', node);
          logger.info('Node auto-registered:', node);
        } else {
          logger.debug('Suppressed stale or duplicate node auto-registration notification', {
            nodeName,
            nodeType: node.type,
            timestamp: message.timestamp,
          });
        }

        eventBus.emit('refresh_nodes');
      } else if (message.type === 'node_deleted' || message.type === 'nodes_changed') {
        // Nodes configuration has changed, refresh the list
        eventBus.emit('refresh_nodes');
      } else if (message.type === 'discovery_update') {
        // Discovery scan completed with new results
        eventBus.emit('discovery_updated', message.data);
      } else if (message.type === 'discovery_started') {
        eventBus.emit('discovery_status', {
          scanning: true,
          subnet: message.data?.subnet,
          timestamp: message.data?.timestamp,
        });
      } else if (message.type === 'discovery_complete') {
        eventBus.emit('discovery_status', {
          scanning: false,
          timestamp: message.data?.timestamp,
        });
      } else if ((message as { type: string }).type === 'ai_discovery_progress') {
        // AI-powered discovery progress update
        eventBus.emit(
          'ai_discovery_progress',
          (message as { data: unknown }).data as import('../types/discovery').DiscoveryProgress,
        );
      } else if (message.type === 'settingsUpdate') {
        // Settings have been updated (e.g., theme change)
        if (message.data?.theme) {
          // Emit event for theme change
          eventBus.emit('theme_changed', message.data.theme);
          logger.info('Theme update received via WebSocket:', message.data.theme);
        }
      } else {
        // Log any unhandled message types in dev mode only
        if (import.meta.env.DEV) {
          // Silently ignore unhandled message types
        }
      }
    } catch (err) {
      logger.error('Failed to process WebSocket message', err);
    }
  };

  const schedulePendingRESTHydration = (connectionId: number, delay: number) => {
    if (!isCurrentConnection(connectionId)) return;
    pendingRestHydrationConnectionId = connectionId;
    if (restHydrationTimeout) return;
    restHydrationTimeout = window.setTimeout(
      () => {
        restHydrationTimeout = 0;
        if (pendingRestHydrationConnectionId !== connectionId) return;
        pendingRestHydrationConnectionId = null;
        requestFullStateRecovery(connectionId);
      },
      Math.max(0, delay),
    );
  };

  // Hydrate display state from `/api/state`, which has no frame-size ceiling.
  // It deliberately does not establish a socket delta baseline: the endpoint
  // builds independently from the snapshot the hub withheld.
  const hydrateFullStateFromREST = async (connectionId: number) => {
    if (!isCurrentConnection(connectionId) || restHydrationConnectionId === connectionId) return;
    restHydrationConnectionId = connectionId;
    try {
      const snapshot = await apiFetchJSON<State>('/api/state');
      if (!isCurrentConnection(connectionId)) return;
      if (rawServerResources !== null) {
        // A deliverable socket snapshot won the race while REST was pending.
        // Keep that connection-native baseline instead of replacing it with an
        // older HTTP response captured before the socket snapshot arrived.
        return;
      }
      if (!snapshot || typeof snapshot !== 'object') {
        logger.error('REST state recovery returned an unusable payload');
        return;
      }
      logger.info('Hydrated full state over REST after an oversized WebSocket snapshot');
      processServerMessage(
        { type: WEBSOCKET.MESSAGE_TYPES.RAW_DATA, data: snapshot },
        connectionId,
        'rest',
      );
    } catch (error) {
      if (!isCurrentConnection(connectionId)) return;
      // Leave the throttle stamp in place; the next marker or baseline-less
      // delta retries after the shared recovery window. Failing here is not
      // fatal — the UI stays in its current state without hammering REST.
      logger.error('REST state recovery failed', error);
    } finally {
      if (restHydrationConnectionId === connectionId) {
        restHydrationConnectionId = null;
      }
      if (isCurrentConnection(connectionId) && pendingRestHydrationConnectionId === connectionId) {
        const remaining = Math.max(0, 30000 - (Date.now() - lastFullStateRecoveryAt));
        schedulePendingRESTHydration(connectionId, remaining);
      }
    }
  };

  // Single throttled entry point for re-acquiring a full snapshot, whichever
  // trigger noticed the gap: a frame dropped as oversized, or a delta arriving
  // with no baseline to patch. Sharing one budget keeps a server that repeatedly
  // emits an undeliverable snapshot from turning into a REST hammer.
  const requestFullStateRecovery = (connectionId: number) => {
    if (!isCurrentConnection(connectionId)) return;
    const now = Date.now();

    if (oversizedSnapshotObserved) {
      if (restHydrationConnectionId === connectionId) {
        // A marker received after this request started may describe a newer
        // state than its response. Coalesce one trailing refresh rather than
        // losing the update or starting concurrent full-state builds.
        pendingRestHydrationConnectionId = connectionId;
        return;
      }
      const remaining = 30000 - (now - lastFullStateRecoveryAt);
      if (remaining > 0) {
        schedulePendingRESTHydration(connectionId, remaining);
        return;
      }
      lastFullStateRecoveryAt = now;
      // The socket has already proven it cannot deliver this estate's snapshot.
      // `requestData` would just queue another frame the guard drops, which is
      // what left large estates stuck on an empty UI, retrying every 30s.
      logger.warn('Recovering full state over REST. Socket snapshot exceeds the inbound limit');
      void hydrateFullStateFromREST(connectionId);
      return;
    }

    if (now - lastFullStateRecoveryAt < 30000) return;
    lastFullStateRecoveryAt = now;

    logger.warn('Received keyed delta without a socket snapshot baseline. Requesting full state');
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'requestData' }));
    }
  };

  const setupWebSocket = (socket: WebSocket, connectionId: number) => {
    if (!isCurrentConnection(connectionId)) return;

    socket.onopen = () => {
      if (!isCurrentConnection(connectionId)) return;
      logger.debug('connect');
      const wasReconnecting = reconnectAttempt > 0;
      setConnected(true);
      setReconnecting(false); // Clear reconnecting state
      reconnectAttempt = 0; // Reset reconnect attempts on successful connection
      isReconnecting = false;
      lastServerActivityAt = Date.now();
      currentConnectionOpenedAtMs = lastServerActivityAt;

      // Start heartbeat to keep connection alive
      clearHeartbeatTimer();
      heartbeatInterval = window.setInterval(() => {
        if (isCurrentConnection(connectionId) && socket.readyState === WebSocket.OPEN) {
          const silenceDuration = Date.now() - lastServerActivityAt;
          if (silenceDuration >= heartbeatTimeoutMs) {
            logger.warn('WebSocket heartbeat timeout, forcing reconnect', {
              silenceMs: silenceDuration,
            });
            socket.close(4000, 'Heartbeat timeout');
            return;
          }
          socket.send(JSON.stringify({ type: 'ping', data: { timestamp: Date.now() } }));
        }
      }, heartbeatIntervalMs);

      // Emit reconnection event so App can refresh alert config
      // This ensures the alert activation state is re-fetched after connection loss
      if (wasReconnecting) {
        logger.info('WebSocket reconnected, emitting event for config refresh');
        eventBus.emit('websocket_reconnected');
      }

      // Alerts will come with the initial state broadcast
      scheduleColdActiveAlertsRecovery(connectionId);
    };

    socket.onmessage = (event) => {
      if (!isCurrentConnection(connectionId)) return;
      if (typeof event.data !== 'string') {
        logger.warn('Ignoring non-text WebSocket payload');
        return;
      }

      if (!isInboundPayloadWithinLimit(event.data)) {
        logger.warn('Ignoring oversized WebSocket payload', {
          characterCount: event.data.length,
          maxBytes: MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES,
        });
        // A dropped frame is almost always the full snapshot: it is the only
        // message that scales with estate size (~2.7 KB per resource, so the
        // 32 MiB guard trips around 12000 resources). Recover over REST rather
        // than waiting for a baseline-less delta to notice, otherwise the first
        // paint on a large estate is an empty UI.
        // The frame may supersede any of the keyed projections. Once it is
        // dropped, none of their socket lineages are safe to patch further.
        rawServerResources = null;
        deferredResourceIds.clear();
        deferredResourceKeys.clear();
        deferredResourceDeltaQueue.length = 0;
        rawConnectedInfrastructure = null;
        pendingInfrastructureIds.clear();
        pendingInfrastructureFull = false;
        rawActiveAlerts = null;
        oversizedSnapshotObserved = true;
        lastServerActivityAt = Date.now();
        requestFullStateRecovery(connectionId);
        return;
      }

      let data;
      try {
        data = JSON.parse(event.data);
      } catch (parseError) {
        logger.error('Failed to parse WebSocket message', parseError);
        return;
      }
      lastServerActivityAt = Date.now();

      processServerMessage(data, connectionId);
    };

    socket.onclose = (event) => {
      if (isDisposed || !retireConnection(connectionId)) return;
      logger.debug('disconnect', { code: event.code, reason: event.reason });
      setConnected(false);

      // Clear heartbeat interval
      clearHeartbeatTimer();

      if (isDisposed) {
        return;
      }

      // Don't try to reconnect if the close was intentional (code 1000)
      if (
        event.code === 1000 &&
        (event.reason === 'Reconnecting' || event.reason === 'Component unmounting')
      ) {
        return;
      }

      // If we get a 1008 (policy violation) close code, it's likely an auth failure
      // Redirect to login page to re-authenticate
      if (event.code === 1008) {
        logger.warn('WebSocket closed due to authentication failure, redirecting to login');
        // Clear auth and reload to trigger login
        if (typeof window !== 'undefined') {
          localStorage.setItem('just_logged_out', 'true');
          window.location.href = '/';
        }
        return;
      }

      if (!hasActiveAlertsSnapshot) {
        void recoverActiveAlertsFromREST();
      }
      handleReconnect();
    };

    socket.onerror = (error) => {
      if (!isCurrentConnection(connectionId)) return;
      // Don't log connection errors if we're already connected
      // Browser may show errors for initial connection attempts even after success
      if (!connected()) {
        logger.debug('error', error);
      }
    };
  };

  // Connect immediately
  connect();

  // Cleanup on unmount
  onCleanup(() => {
    isDisposed = true;
    isReconnecting = false;
    activeConnectionId = null;
    resetConnectionBaseline();
    window.clearTimeout(reconnectDelayTimeout);
    window.clearTimeout(reconnectTimeout);
    window.clearInterval(heartbeatInterval);
    pendingAckTimeouts.forEach((t) => window.clearTimeout(t));
    pendingAckTimeouts.clear();
    activeAlertsRecoveryRequestId += 1;
    activeAlertsRecoveryInFlight = null;
    if (activeAlertsColdHydrationTimeout) {
      window.clearTimeout(activeAlertsColdHydrationTimeout);
      activeAlertsColdHydrationTimeout = 0;
    }
    if (typeof window !== 'undefined') {
      window.removeEventListener(ALERTS_DETECTION_EVENT, handleAlertsDetectionEvent);
      for (const eventName of OPERATOR_INPUT_EVENTS) {
        window.removeEventListener(eventName, handleOperatorInput, { capture: true });
      }
    }
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', handleResourceVisibilityChange);
    }
    if (deferredFlushTimer) {
      window.clearTimeout(deferredFlushTimer);
      deferredFlushTimer = 0;
    }
    if (ws) {
      ws.close(1000, 'Component unmounting');
      ws = null;
    }
  });

  const markTokenRevoked = (
    key: 'dockerRuntimes' | 'agents',
    tokenId: string,
    agentIds: string[],
  ) => {
    if (!agentIds || agentIds.length === 0) return;
    const timestamp = Date.now();
    const targetIds = new Set(agentIds.filter(Boolean));
    setState(
      'resources',
      produce((draft: any[]) => {
        if (!Array.isArray(draft)) return;

        draft.forEach((resource) => {
          if (!resource || typeof resource !== 'object') return;

          const platformData =
            resource.platformData && typeof resource.platformData === 'object'
              ? resource.platformData
              : (resource.platformData = {});
          const agentData =
            platformData.agent && typeof platformData.agent === 'object'
              ? platformData.agent
              : (platformData.agent = {});
          const dockerData =
            platformData.docker && typeof platformData.docker === 'object'
              ? platformData.docker
              : (platformData.docker = {});

          const agentActionId =
            asString(agentData.agentId) ||
            asString(platformData.agentId) ||
            getAgentDiscoveryResourceId(resource?.discoveryTarget) ||
            asString(resource?.discoveryTarget?.agentId) ||
            asString(resource.id);
          const runtimeActionId =
            asString(dockerData.hostSourceId) ||
            asString(platformData.hostSourceId) ||
            asString(resource?.discoveryTarget?.agentId) ||
            (isAppContainerDiscoveryResourceType(resource?.discoveryTarget?.resourceType)
              ? asString(resource?.discoveryTarget?.resourceId)
              : undefined) ||
            asString(resource.id);

          const matchedId = key === 'agents' ? agentActionId : runtimeActionId;
          if (!matchedId || !targetIds.has(matchedId)) return;

          platformData.revokedTokenId = tokenId;
          platformData.tokenRevokedAt = timestamp;
          if (key === 'agents') {
            agentData.revokedTokenId = tokenId;
            agentData.tokenRevokedAt = timestamp;
          } else {
            dockerData.revokedTokenId = tokenId;
            dockerData.tokenRevokedAt = timestamp;
          }
        });
      }),
    );
  };

  return {
    state,
    activeAlerts,
    recentlyResolved,
    connected,
    reconnecting,
    initialDataReceived,
    activeAlertsHydrationStatus,
    updateProgress,
    resourceChange,
    changedResourceIdsSince,
    changedResourceMetaSince,
    shutdown,
    refreshActiveAlerts: () => recoverActiveAlertsFromREST(true),
    reconnect: () => {
      if (isDisposed) return;
      if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
        ws.close(1000, 'Reconnecting');
      }
      clearReconnectTimeout();
      clearReconnectDelayTimeout();
      reconnectAttempt = 0; // Reset attempts for manual reconnect
      isReconnecting = false;
      setConnected(false);
      setReconnecting(true);
      connect();
    },
    switchUrl: (nextUrl: string) => {
      if (!nextUrl || nextUrl === wsUrl) {
        return;
      }

      wsUrl = nextUrl;
      activeAlertsRecoveryRequestId += 1;
      activeAlertsRecoveryInFlight = null;
      if (activeAlertsColdHydrationTimeout) {
        window.clearTimeout(activeAlertsColdHydrationTimeout);
        activeAlertsColdHydrationTimeout = 0;
      }
      lastActiveAlertsRecoveryAt = 0;
      hasActiveAlertsSnapshot = false;
      activeAlertsRevision += 1;
      lastActiveAlertsPayload = {};
      batch(() => {
        setConnected(false);
        setReconnecting(false);
        setInitialDataReceived(false);
        setActiveAlertsHydrationStatus('pending');
        setUpdateProgress(null);
        setResourceChange({
          version: ++resourceChangeVersion,
          changedIds: null,
          changedKeys: null,
        });
        resourceChangeHistory = [];
        setState(reconcile(createInitialState()));
        setActiveAlerts(reconcile({}));
        setRecentlyResolved(reconcile({}));
      });

      clearReconnectTimeout();
      clearReconnectDelayTimeout();
      reconnectAttempt = 0;
      isReconnecting = false;
      ws?.close(1000, 'Reconnecting');
      connect();
    },
    markDockerRuntimesTokenRevoked: (tokenId: string, agentIds: string[]) =>
      markTokenRevoked('dockerRuntimes', tokenId, agentIds),
    markAgentsTokenRevoked: (tokenId: string, agentIds: string[]) =>
      markTokenRevoked('agents', tokenId, agentIds),
    removeAlerts: (predicate: (alert: Alert) => boolean) => {
      const keysToRemove: string[] = [];
      Object.entries(activeAlerts).forEach(([alertIdentifier, alert]) => {
        if (!alert) {
          keysToRemove.push(alertIdentifier);
          return;
        }
        try {
          if (predicate(alert)) {
            clearPendingAck(alertIdentifier);
            keysToRemove.push(alertIdentifier);
          }
        } catch (error) {
          logger.error('Failed to evaluate alert removal predicate', error);
        }
      });

      if (keysToRemove.length > 0) {
        setActiveAlerts(
          produce((draft) => {
            keysToRemove.forEach((key) => {
              delete draft[key];
            });
          }),
        );
      }
    },
    // Method to update an alert locally (e.g., after acknowledgment)
    updateAlert: (alertIdentifier: string, updates: Partial<Alert>) => {
      const existingAlert = activeAlerts[alertIdentifier];
      if (existingAlert) {
        // Track this alert as having pending changes if acknowledgment is changing
        if ('acknowledged' in updates) {
          const previousAckTime = existingAlert.ackTime;
          pendingAckChanges.set(alertIdentifier, {
            ack: !!updates.acknowledged,
            previousAckTime,
          });
          clearPendingAckTimeout(alertIdentifier);
          // Safety valve: if we never hear back from the server (e.g., request failed silently),
          // clear the pending flag after a generous timeout so we eventually resync with reality.
          const pendingTimeout = window.setTimeout(() => {
            if (isDisposed) return;
            if (pendingAckChanges.has(alertIdentifier)) {
              logger.warn(`Clearing stale pending ack change for alert ${alertIdentifier}`);
              clearPendingAck(alertIdentifier);
              notificationStore.error(
                'Server did not confirm the alert acknowledgment in time. Re-syncing from latest data.',
              );
            }
          }, 15000);
          pendingAckTimeouts.set(alertIdentifier, pendingTimeout);
        }
        setActiveAlerts(alertIdentifier, { ...existingAlert, ...updates });
      }
    },
  };
}
