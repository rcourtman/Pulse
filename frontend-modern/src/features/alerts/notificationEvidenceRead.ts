// Alert configuration and delivery evidence reads share the API client's maximum two-minute Retry-After
// allowance. Past this deadline an unanswered header/body is unavailable, not
// evidence that delivery is healthy or that there were no attempts.
export const NOTIFICATION_EVIDENCE_READ_TIMEOUT_MS = 150_000;

export class NotificationEvidenceTimeoutError extends Error {
  constructor() {
    super('Notification evidence could not be read within the time limit.');
    this.name = 'NotificationEvidenceTimeoutError';
  }
}

// One owner per independent evidence stream. Settle locally as well as aborting
// the transport: a stalled body or an uncooperative reader must not hold the UI
// or let a late result replace a subsequent refresh. Never use for mutations.
export function createNotificationEvidenceReader() {
  let cancelCurrent: (() => void) | undefined;
  const cancel = () => cancelCurrent?.();

  const read = async <T>(fetcher: (signal: AbortSignal) => Promise<T>): Promise<T> => {
    cancel();
    const controller = new AbortController();
    let rejectRead!: (error: Error) => void;
    const interrupted = new Promise<never>((_resolve, reject) => {
      rejectRead = reject;
    });
    const interrupt = (error: Error) => {
      clearTimeout(timer);
      rejectRead(error);
      controller.abort();
    };
    const timer = setTimeout(
      () => interrupt(new NotificationEvidenceTimeoutError()),
      NOTIFICATION_EVIDENCE_READ_TIMEOUT_MS,
    );
    const retire = () => interrupt(new DOMException('Evidence read retired.', 'AbortError'));
    cancelCurrent = retire;
    try {
      return await Promise.race([interrupted, fetcher(controller.signal)]);
    } finally {
      clearTimeout(timer);
      if (cancelCurrent === retire) cancelCurrent = undefined;
    }
  };

  return { read, cancel };
}
