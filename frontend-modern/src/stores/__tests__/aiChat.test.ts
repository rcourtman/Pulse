import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it, beforeEach } from 'vitest';
import { aiChatStore } from '@/stores/aiChat';
import { eventBus } from '@/stores/events';

const aiChatSource = readFileSync(
  resolve(process.cwd(), 'src/components/AI/Chat/index.tsx'),
  'utf8',
);

describe('aiChatStore', () => {
  beforeEach(() => {
    aiChatStore.close();
    aiChatStore.clearContext();
    aiChatStore.clearAllContext();
    aiChatStore.clearConversation();
    aiChatStore.setMessages([]);
    aiChatStore.registerInput(null);
    aiChatStore.setEnabled(false);
  });

  it('opens, closes, and toggles', () => {
    expect(aiChatStore.isOpen).toBe(false);
    aiChatStore.open();
    expect(aiChatStore.isOpen).toBe(true);
    aiChatStore.toggle();
    expect(aiChatStore.isOpen).toBe(false);
    aiChatStore.toggle();
    expect(aiChatStore.isOpen).toBe(true);
    aiChatStore.close();
    expect(aiChatStore.isOpen).toBe(false);
  });

  it('sets legacy context and clears it', () => {
    aiChatStore.setContext({ targetType: 'vm', targetId: 'vm-101', context: { name: 'vm-101' } });
    expect(aiChatStore.context.targetType).toBe('vm');
    expect(aiChatStore.context.targetId).toBe('vm-101');
    aiChatStore.clearContext();
    expect(aiChatStore.context.targetType).toBeUndefined();
  });

  it('adds context items without duplicates and updates legacy context', () => {
    expect(aiChatStore.contextItems).toHaveLength(0);
    expect(aiChatStore.hasContextItem('vm-101')).toBe(false);

    aiChatStore.addContextItem('vm', 'vm-101', 'vm-101', { guestName: 'vm-101', a: 1 });
    expect(aiChatStore.contextItems).toHaveLength(1);
    expect(aiChatStore.hasContextItem('vm-101')).toBe(true);
    expect(aiChatStore.context.targetId).toBe('vm-101');
    expect(aiChatStore.context.context).toMatchObject({ a: 1 });

    aiChatStore.addContextItem('vm', 'vm-101', 'vm-101', { guestName: 'vm-101', a: 2 });
    expect(aiChatStore.contextItems).toHaveLength(1);
    expect(aiChatStore.contextItems[0].data).toMatchObject({ a: 2 });
    expect(aiChatStore.context.targetId).toBe('vm-101');
    expect(aiChatStore.context.context).toMatchObject({ a: 2 });
  });

  it('removes context items and keeps legacy context consistent', () => {
    aiChatStore.addContextItem('vm', 'vm-101', 'vm-101', { name: 'vm-101' });
    aiChatStore.addContextItem('agent', 'agent-1', 'agent-1', { name: 'agent-1' });
    expect(aiChatStore.contextItems).toHaveLength(2);
    expect(aiChatStore.context.targetId).toBe('agent-1');

    aiChatStore.removeContextItem('agent-1');
    expect(aiChatStore.contextItems).toHaveLength(1);
    expect(aiChatStore.context.targetId).toBe('vm-101');

    aiChatStore.removeContextItem('vm-101');
    expect(aiChatStore.contextItems).toHaveLength(0);
    expect(aiChatStore.context.targetId).toBeUndefined();
  });

  it('setTargetContext and openForTarget replace context (not accumulate)', () => {
    aiChatStore.setTargetContext('vm', 'vm-101', { guestName: 'my-guest' });
    expect(aiChatStore.contextItems).toHaveLength(1);
    expect(aiChatStore.contextItems[0].name).toBe('my-guest');
    expect(aiChatStore.context.targetId).toBe('vm-101');

    // openForTarget should replace, not add to existing context
    aiChatStore.openForTarget('agent', 'agent-1', { name: 'pve-node' });
    expect(aiChatStore.isOpen).toBe(true);
    expect(aiChatStore.contextItems).toHaveLength(1);
    expect(aiChatStore.contextItems[0].name).toBe('pve-node');
    expect(aiChatStore.context.targetId).toBe('agent-1');
  });

  it('prefers guestName, then name, then target id for target context labels', () => {
    aiChatStore.setTargetContext('vm', 'vm-101', {
      guestName: 'guest-first',
      name: 'ignored-name',
    });
    expect(aiChatStore.contextItems[0].name).toBe('guest-first');

    aiChatStore.openForTarget('agent', 'agent-1', { name: 'node-name' });
    expect(aiChatStore.contextItems[0].name).toBe('node-name');

    aiChatStore.openForTarget('storage', 'storage-1');
    expect(aiChatStore.contextItems[0].name).toBe('storage-1');
  });

  it('opens with a pre-filled prompt', () => {
    aiChatStore.openWithPrompt('hello', {
      targetType: 'vm',
      targetId: 'vm-101',
      briefing: {
        sourceLabel: 'Pulse Patrol',
        title: 'Investigation record attached',
        actionLabel: 'Open Patrol provider settings',
        actionHref: '/settings/system-ai',
        suggestedPrompts: ['Explain recent changes and correlations'],
      },
      handoffMetadata: {
        kind: 'patrol_finding',
      },
    });
    expect(aiChatStore.isOpen).toBe(true);
    expect(aiChatStore.context.initialPrompt).toBe('hello');
    expect(aiChatStore.context.targetId).toBe('vm-101');
    expect(aiChatStore.context.briefing?.title).toBe('Investigation record attached');
    expect(aiChatStore.context.briefing?.actionLabel).toBe('Open Patrol provider settings');
    expect(aiChatStore.context.briefing?.actionHref).toBe('/settings/system-ai');
    expect(aiChatStore.context.briefing?.suggestedPrompts).toEqual([
      'Explain recent changes and correlations',
    ]);
    expect(aiChatStore.context.handoffMetadata).toMatchObject({
      kind: 'patrol_finding',
    });
  });

  it('threads autoSendInitialPrompt through openWithPrompt and clears it on clearAutoSendFlag', () => {
    // Action-style entry points (Explain, Investigate, eventually Verify
    // fix) set autoSendInitialPrompt: true so the chat surface submits
    // immediately on open instead of pre-filling the input and waiting
    // for Enter. Discuss-style entries leave it falsy.
    aiChatStore.openWithPrompt('Explain this Patrol finding', {
      targetType: 'vm',
      targetId: 'vm-100',
      autoSendInitialPrompt: true,
    });

    expect(aiChatStore.context.initialPrompt).toBe('Explain this Patrol finding');
    expect(aiChatStore.context.autoSendInitialPrompt).toBe(true);

    aiChatStore.clearAutoSendFlag();

    expect(aiChatStore.context.autoSendInitialPrompt).toBeUndefined();
    // The initialPrompt is independent — clearAutoSendFlag must not also
    // clear the prompt itself (the chat effect still needs to read and
    // submit it before its own clearInitialPrompt runs).
    expect(aiChatStore.context.initialPrompt).toBe('Explain this Patrol finding');
  });

  it('defaults autoSendInitialPrompt to undefined when not provided', () => {
    aiChatStore.openWithPrompt('discuss this', {
      targetType: 'vm',
      targetId: 'vm-200',
    });
    expect(aiChatStore.context.autoSendInitialPrompt).toBeUndefined();
  });

  it('preserves scoped autonomous-mode overrides for pre-filled prompts', () => {
    aiChatStore.openWithPrompt('brief me', {
      targetType: 'dashboard',
      targetId: 'pulse-brief',
      autonomousMode: false,
    });

    expect(aiChatStore.isOpen).toBe(true);
    expect(aiChatStore.context.initialPrompt).toBe('brief me');
    expect(aiChatStore.context.autonomousMode).toBe(false);

    aiChatStore.clearInitialPrompt();

    expect(aiChatStore.context.initialPrompt).toBeUndefined();
    expect(aiChatStore.context.autonomousMode).toBe(false);
  });

  it('preserves model-only handoff context and resources for pre-filled prompts', () => {
    aiChatStore.openWithPrompt('discuss this incident', {
      targetType: 'storage',
      targetId: 'storage-1',
      autonomousMode: false,
      handoffContext:
        '[Alert Incident Context]\nTimeline Event 1: 2026-05-07T00:02:00Z | Command | Command event recorded',
      handoffResources: [{ id: 'storage-1', name: 'tank', type: 'storage', node: 'nas-1' }],
      handoffActions: [{ findingId: 'finding-1', approvalId: 'approval-1' }],
    });

    expect(aiChatStore.context.initialPrompt).toBe('discuss this incident');
    expect(aiChatStore.context.handoffContext).toContain('[Alert Incident Context]');
    expect(aiChatStore.context.handoffResources).toEqual([
      { id: 'storage-1', name: 'tank', type: 'storage', node: 'nas-1' },
    ]);
    expect(aiChatStore.context.handoffActions).toEqual([
      { findingId: 'finding-1', approvalId: 'approval-1' },
    ]);

    aiChatStore.clearInitialPrompt();

    expect(aiChatStore.context.initialPrompt).toBeUndefined();
    expect(aiChatStore.context.handoffContext).toContain('Command event recorded');
    expect(aiChatStore.context.handoffResources).toEqual([
      { id: 'storage-1', name: 'tank', type: 'storage', node: 'nas-1' },
    ]);
    expect(aiChatStore.context.handoffActions).toEqual([
      { findingId: 'finding-1', approvalId: 'approval-1' },
    ]);
  });

  it('clears request handoff payloads while preserving safe visible context', () => {
    aiChatStore.openWithPrompt('discuss this incident', {
      targetType: 'storage',
      targetId: 'storage-1',
      autonomousMode: false,
      findingId: 'finding-1',
      briefing: {
        sourceLabel: 'Pulse Patrol',
        title: 'Patrol finding on tank',
      },
      handoffContext:
        '[Alert Incident Context]\nTimeline Event 1: 2026-05-07T00:02:00Z | Command | Command event recorded',
      handoffResources: [{ id: 'storage-1', name: 'tank', type: 'storage', node: 'nas-1' }],
      handoffActions: [{ findingId: 'finding-1', approvalId: 'approval-1' }],
      handoffMetadata: {
        kind: 'patrol_run',
        runId: 'run-runtime-error',
        runType: 'Scoped run',
        runStatus: 'error',
        runtimeFailure: true,
      },
    });

    aiChatStore.clearRequestHandoffPayload();

    expect(aiChatStore.context.handoffContext).toBeUndefined();
    expect(aiChatStore.context.handoffResources).toBeUndefined();
    expect(aiChatStore.context.handoffActions).toBeUndefined();
    expect(aiChatStore.context.handoffMetadata).toBeUndefined();
    expect(aiChatStore.context.targetId).toBe('storage-1');
    expect(aiChatStore.context.findingId).toBe('finding-1');
    expect(aiChatStore.context.autonomousMode).toBe(false);
    expect(aiChatStore.context.briefing?.title).toBe('Patrol finding on tank');
  });

  it('focusInput returns false when closed and true when open with a registered element', () => {
    const textarea = document.createElement('textarea');
    document.body.appendChild(textarea);
    aiChatStore.registerInput(textarea);

    expect(aiChatStore.focusInput()).toBe(false);
    aiChatStore.open();
    expect(aiChatStore.focusInput()).toBe(true);
    expect(document.activeElement).toBe(textarea);

    textarea.remove();
  });

  it('resets assistant drawer session state on org switch', () => {
    const previousSessionId = aiChatStore.sessionId;

    aiChatStore.open();
    aiChatStore.addContextItem('vm', 'vm-101', 'vm-101', { name: 'vm-101' });
    aiChatStore.setMessages([
      {
        id: 'msg-1',
        role: 'user',
        content: 'hello',
        timestamp: new Date('2026-04-08T10:00:00.000Z'),
      },
    ]);
    aiChatStore.setTitle('Session title');

    eventBus.emit('org_switched', 'org-2');

    expect(aiChatStore.messages).toHaveLength(0);
    expect(aiChatStore.contextItems).toHaveLength(0);
    expect(aiChatStore.context.targetId).toBeUndefined();
    expect(aiChatStore.title).toBe('');
    expect(aiChatStore.sessionId).not.toBe(previousSessionId);
    expect(localStorage.getItem('pulse:ai_chat_session_id')).toBe(aiChatStore.sessionId);
  });

  it('keeps closed assistant resource reads on the websocket/cache path', () => {
    expect(aiChatSource).toContain('getGlobalWebSocketStore');
    expect(aiChatSource).toContain('getCachedUnifiedResources');
    expect(aiChatSource).not.toContain('useResources()');
  });
});
