import { describe, expect, it, beforeEach } from 'vitest';
import { aiChatStore } from '@/stores/aiChat';

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
    aiChatStore.addContextItem('node', 'node-1', 'node-1', { name: 'node-1' });
    expect(aiChatStore.contextItems).toHaveLength(2);
    expect(aiChatStore.context.targetId).toBe('node-1');

    aiChatStore.removeContextItem('node-1');
    expect(aiChatStore.contextItems).toHaveLength(1);
    expect(aiChatStore.context.targetId).toBe('vm-101');

    aiChatStore.removeContextItem('vm-101');
    expect(aiChatStore.contextItems).toHaveLength(0);
    expect(aiChatStore.context.targetId).toBeUndefined();
  });

  it('setTargetContext and openForTarget derive a sensible name', () => {
    aiChatStore.setTargetContext('vm', 'vm-101', { guestName: 'my-guest' });
    expect(aiChatStore.contextItems).toHaveLength(1);
    expect(aiChatStore.contextItems[0].name).toBe('my-guest');
    expect(aiChatStore.context.targetId).toBe('vm-101');

    aiChatStore.openForTarget('node', 'node-1', { name: 'delly' });
    expect(aiChatStore.isOpen).toBe(true);
    expect(aiChatStore.contextItems).toHaveLength(2);
    expect(aiChatStore.contextItems[1].name).toBe('delly');
  });

  it('opens with a pre-filled prompt', () => {
    aiChatStore.openWithPrompt('hello', { targetType: 'vm', targetId: 'vm-101' });
    expect(aiChatStore.isOpen).toBe(true);
    expect(aiChatStore.context.initialPrompt).toBe('hello');
    expect(aiChatStore.context.targetId).toBe('vm-101');
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
});
