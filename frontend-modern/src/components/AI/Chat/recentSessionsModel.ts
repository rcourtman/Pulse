import type { ChatSession } from '@/api/aiChat';

export const QUICK_RESUME_SESSION_LIMIT = 3;

/**
 * Sessions offered as resumable chats in the Assistant empty state.
 *
 * Pulse-owned background runs (Patrol detection/eval, investigations) arrive
 * with `system: true`; they are forensic logs, not conversations, so the
 * quick-resume list offers user chats only. The full session picker and the
 * Settings sessions panel still list system sessions for inspection.
 */
export const selectQuickResumeSessions = (
  sessions: ChatSession[],
  activeSessionId: string | null,
): ChatSession[] =>
  sessions
    .filter(
      (session) => !session.system && session.id !== activeSessionId && session.message_count > 0,
    )
    .slice(0, QUICK_RESUME_SESSION_LIMIT);
