import { describe, expect, it } from 'vitest';
import aiChatEventsSource from '@/api/generated/aiChatEvents.ts?raw';

describe('AI chat stream event contract', () => {
  it('does not expose the retired explore pre-pass stream event', () => {
    expect(aiChatEventsSource).not.toContain('ExploreStatusData');
    expect(aiChatEventsSource).not.toContain("type: 'explore_status'");
  });
});
