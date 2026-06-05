import { describe, expect, it } from 'vitest';
import aiChatEventsSource from '@/api/generated/aiChatEvents.ts?raw';
import type { AIChatStreamEvent, SessionData } from '@/api/generated/aiChatEvents';

describe('AI chat stream event contract', () => {
  it('does not expose the retired explore pre-pass stream event', () => {
    expect(aiChatEventsSource).not.toContain('ExploreStatusData');
    expect(aiChatEventsSource).not.toContain("type: 'explore_status'");
  });

  it('exposes the backend-created session event as a typed stream contract', () => {
    const session: SessionData = { id: 'sess-stream' };
    const event: AIChatStreamEvent = { type: 'session', data: session };

    expect(event.data.id).toBe('sess-stream');
    expect(aiChatEventsSource).toContain('export interface SessionData');
    expect(aiChatEventsSource).toContain("type: 'session'");
  });
});
