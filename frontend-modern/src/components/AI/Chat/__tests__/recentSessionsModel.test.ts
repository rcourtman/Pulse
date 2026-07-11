import { describe, expect, it } from 'vitest';
import type { ChatSession } from '@/api/aiChat';
import {
  QUICK_RESUME_SESSION_LIMIT,
  selectQuickResumeSessions,
} from '../recentSessionsModel';

const session = (overrides: Partial<ChatSession> & { id: string }): ChatSession => ({
  title: overrides.id,
  created_at: '2026-07-11T06:00:00Z',
  updated_at: '2026-07-11T06:00:00Z',
  message_count: 5,
  ...overrides,
});

describe('selectQuickResumeSessions', () => {
  it('excludes Pulse-owned background sessions from the quick-resume list', () => {
    const sessions = [
      session({ id: 'patrol-main', title: '# Deterministic Triage Results Scanned 70', system: true, message_count: 200 }),
      session({ id: 'user-chat-1' }),
    ];

    const result = selectQuickResumeSessions(sessions, null);

    expect(result.map((s) => s.id)).toEqual(['user-chat-1']);
  });

  it('excludes the active session and empty sessions', () => {
    const sessions = [
      session({ id: 'active-chat' }),
      session({ id: 'empty-chat', message_count: 0 }),
      session({ id: 'resumable-chat' }),
    ];

    const result = selectQuickResumeSessions(sessions, 'active-chat');

    expect(result.map((s) => s.id)).toEqual(['resumable-chat']);
  });

  it('caps the list at the quick-resume limit', () => {
    const sessions = Array.from({ length: QUICK_RESUME_SESSION_LIMIT + 2 }, (_, index) =>
      session({ id: `chat-${index}` }),
    );

    expect(selectQuickResumeSessions(sessions, null)).toHaveLength(QUICK_RESUME_SESSION_LIMIT);
  });
});
