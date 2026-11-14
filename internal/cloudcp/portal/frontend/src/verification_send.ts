// Advisory page-local pacing, not account eligibility or delivery evidence.
// Keep mailbox identity aligned with the service: Unicode whitespace trimming,
// ASCII case folding only, and no Unicode/dot/plus-address normalization.
export function trimVerificationEmail(email: string): string {
  return String(email || '').replace(/^\p{White_Space}+|\p{White_Space}+$/gu, '');
}

export function verificationEmailKey(email: string): string {
  return trimVerificationEmail(email).replace(/[A-Z]/g, function(letter) {
    return letter.toLowerCase();
  });
}

interface SendReservation {
  key: string;
  pending: boolean;
  readyAt: number;
}

export function createVerificationSendWindow(now: () => number = () => performance.now()) {
  var reservations = new Map<string, SendReservation>();

  function state(email: string) {
    var key = verificationEmailKey(email);
    var entry = reservations.get(key);
    if (entry && !entry.pending && entry.readyAt <= now()) {
      reservations.delete(key);
      entry = undefined;
    }
    return {
      pending: !!entry?.pending,
      seconds: entry && !entry.pending ? Math.max(0, Math.ceil((entry.readyAt - now()) / 1000)) : 0,
    };
  }

  return {
    state,
    begin(email: string): SendReservation | null {
      var key = verificationEmailKey(email);
      var current = state(email);
      if (!key || current.pending || current.seconds) return null;
      var entry = { key, pending: true, readyAt: 0 };
      reservations.set(key, entry);
      return entry;
    },
    settle(entry: SendReservation, wait: boolean): void {
      if (reservations.get(entry.key) !== entry) return;
      if (!wait) {
        reservations.delete(entry.key);
        return;
      }
      entry.pending = false;
      // Start after the response, conservatively later than server admission.
      entry.readyAt = now() + 60_000;
    },
    active(): boolean {
      Array.from(reservations.keys()).forEach(state);
      return reservations.size > 0;
    },
    clear(): void {
      reservations.clear();
    },
  };
}
