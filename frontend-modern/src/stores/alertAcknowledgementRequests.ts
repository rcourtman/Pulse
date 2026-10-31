import { batch, createSignal } from 'solid-js';

// Card and group requests keep their alerts busy for this long after they
// settle, as the overview cards always have. A global request does not.
const SETTLED_HOLD_MS = 1500;

export type AlertAcknowledgementRequestKind = 'alert' | 'group' | 'bulk';

export interface AlertAcknowledgementRequest {
  readonly kind: AlertAcknowledgementRequestKind;
  readonly alertIdentifiers: readonly string[];
}

// The acknowledgement and restore requests the app has in flight, by the
// alerts each one carries. An alert belongs to at most one unsettled request,
// and that request reports its outcome, so card, group and global actions
// leave an owned alert out instead of sending it again. The owner outlives any
// one view, so a remounted overview still sees a request its predecessor sent.
// It ends with the alert scope that holds it: reset() on an organization
// switch or shutdown drops every claim, and a request from the old scope then
// releases nothing and is no longer current.
export function createAlertAcknowledgementRequests() {
  const [requests, setRequests] = createSignal<ReadonlySet<AlertAcknowledgementRequest>>(new Set());
  const ownerByAlert = new Map<string, AlertAcknowledgementRequest>();
  // Alerts just settled by a card or group request, by that request. A later
  // settle replaces the entry, so an older timer cannot end a newer hold, and
  // an alert a newer request carries stays busy whatever the old timer does.
  const [settledHolds, setSettledHolds] = createSignal<
    ReadonlyMap<string, AlertAcknowledgementRequest>
  >(new Map());
  const holdTimers = new Set<ReturnType<typeof setTimeout>>();

  const isOwned = (alertIdentifier: string) => ownerByAlert.has(alertIdentifier);

  // Claims the given alerts that no unsettled request carries, in order.
  // Returns null when every one is already carried.
  const claim = (
    kind: AlertAcknowledgementRequestKind,
    alertIdentifiers: readonly string[],
  ): AlertAcknowledgementRequest | null => {
    const unowned = [...new Set(alertIdentifiers)].filter((id) => !ownerByAlert.has(id));
    if (unowned.length === 0) {
      return null;
    }
    const request: AlertAcknowledgementRequest = { kind, alertIdentifiers: unowned };
    unowned.forEach((id) => ownerByAlert.set(id, request));
    setRequests((previous) => new Set(previous).add(request));
    return request;
  };

  // False once the request has settled or its alert scope has ended.
  const isCurrent = (request: AlertAcknowledgementRequest) => requests().has(request);

  // Releases exactly this request's alerts. A card or group request holds
  // them briefly; a global request frees them at once.
  const release = (request: AlertAcknowledgementRequest) => {
    if (!requests().has(request)) {
      return;
    }
    request.alertIdentifiers.forEach((id) => {
      if (ownerByAlert.get(id) === request) {
        ownerByAlert.delete(id);
      }
    });
    batch(() => {
      setRequests((previous) => {
        const next = new Set(previous);
        next.delete(request);
        return next;
      });
      if (request.kind !== 'bulk') {
        setSettledHolds((previous) => {
          const next = new Map(previous);
          request.alertIdentifiers.forEach((id) => next.set(id, request));
          return next;
        });
      }
    });
    if (request.kind === 'bulk') {
      return;
    }
    const timer = setTimeout(() => {
      holdTimers.delete(timer);
      setSettledHolds((previous) => {
        const next = new Map(previous);
        request.alertIdentifiers.forEach((id) => {
          if (next.get(id) === request) {
            next.delete(id);
          }
        });
        return next;
      });
    }, SETTLED_HOLD_MS);
    holdTimers.add(timer);
  };

  // Alerts a card shows as processing: carried by any unsettled request, or
  // just settled by a card or group request.
  const processingAlerts = () => {
    const processing = new Set(settledHolds().keys());
    requests().forEach((request) => {
      request.alertIdentifiers.forEach((id) => processing.add(id));
    });
    return processing;
  };

  const bulkInFlight = () => {
    for (const request of requests()) {
      if (request.kind === 'bulk') {
        return true;
      }
    }
    return false;
  };

  const reset = () => {
    holdTimers.forEach((timer) => clearTimeout(timer));
    holdTimers.clear();
    ownerByAlert.clear();
    batch(() => {
      setRequests(new Set<AlertAcknowledgementRequest>());
      setSettledHolds(new Map<string, AlertAcknowledgementRequest>());
    });
  };

  return { claim, isCurrent, isOwned, release, processingAlerts, bulkInFlight, reset };
}

export type AlertAcknowledgementRequests = ReturnType<typeof createAlertAcknowledgementRequests>;
