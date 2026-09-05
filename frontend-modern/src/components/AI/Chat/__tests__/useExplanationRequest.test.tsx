import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { aiChatStore } from '@/stores/aiChat';
import { eventBus } from '@/stores/events';
import {
  EXPLAIN_SELECTED_ISSUE_PROMPT,
  useExplanationRequest,
} from '../hooks/useExplanationRequest';

describe('explicit issue explanations', () => {
  beforeEach(() => {
    const request = aiChatStore.explanationRequestSignal();
    if (request) aiChatStore.ackExplanationRequest(request.id);
    aiChatStore.close();
    aiChatStore.clearContext();
  });
  afterEach(cleanup);

  const mount = (
    send = vi.fn().mockResolvedValue(true),
    prepare = vi.fn().mockResolvedValue(undefined),
  ) => {
    const onError = vi.fn();
    render(() => {
      useExplanationRequest({ isOpen: aiChatStore.isOpenSignal, send, prepare, onError });
      return <div />;
    });
    return { send, prepare, onError };
  };

  it('only attaches context for ordinary opens', async () => {
    const { send } = mount();
    aiChatStore.open({ targetId: 'vm-1' });
    await Promise.resolve();
    expect(send).not.toHaveBeenCalled();
  });

  it('sends the explicit request once with finding and action context', async () => {
    const { send } = mount();
    const resources = [{ id: 'vm-1', type: 'vm' }];
    const actions = [{ actionId: 'action-1', approvalId: 'approval-1' }];
    aiChatStore.explain({
      targetId: 'vm-1',
      findingId: 'finding-1',
      autonomousMode: true,
      handoffContext: 'Current finding evidence',
      handoffResources: resources,
      handoffActions: actions,
      handoffMetadata: { kind: 'patrol_finding' },
    });
    await waitFor(() => expect(send).toHaveBeenCalledTimes(1));
    expect(send).toHaveBeenCalledWith(EXPLAIN_SELECTED_ISSUE_PROMPT, 'finding-1', {
      autonomousMode: false,
      handoffContext: 'Current finding evidence',
      handoffResources: resources,
      handoffActions: actions,
      handoffMetadata: { kind: 'patrol_finding' },
    });
    aiChatStore.close();
    aiChatStore.open();
    await Promise.resolve();
    expect(send).toHaveBeenCalledTimes(1);
  });

  it('captures the selected issue while initialization is pending and never clears a newer handoff', async () => {
    let ready!: () => void;
    const prepare = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          ready = resolve;
        }),
    );
    const { send } = mount(vi.fn().mockResolvedValue(true), prepare);
    aiChatStore.explain({ findingId: 'finding-a', handoffContext: 'Evidence A' });
    await waitFor(() => expect(prepare).toHaveBeenCalledOnce());
    aiChatStore.open({ findingId: 'finding-b', handoffContext: 'Evidence B' });
    ready();
    await waitFor(() => expect(send).toHaveBeenCalledOnce());
    expect(send.mock.calls[0][1]).toBe('finding-a');
    expect(send.mock.calls[0][2].handoffContext).toBe('Evidence A');
    expect(aiChatStore.context.handoffContext).toBe('Evidence B');
  });

  it('retains context after a rejected send and allows an explicit retry', async () => {
    const { send } = mount(vi.fn().mockResolvedValueOnce(false).mockResolvedValue(true));
    aiChatStore.explain({ findingId: 'finding-1', handoffContext: 'Evidence' });
    await waitFor(() => expect(send).toHaveBeenCalledOnce());
    expect(aiChatStore.context.handoffContext).toBe('Evidence');
    aiChatStore.explain(aiChatStore.context);
    await waitFor(() => expect(send).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(aiChatStore.context.handoffContext).toBeUndefined());
  });

  it('does not dispatch old tenant evidence after an organisation switch during initialization', async () => {
    let ready!: () => void;
    const prepare = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          ready = resolve;
        }),
    );
    const { send } = mount(vi.fn().mockResolvedValue(true), prepare);
    aiChatStore.explain({ handoffContext: 'Private evidence for org A' });
    await waitFor(() => expect(prepare).toHaveBeenCalledOnce());
    eventBus.emit('org_switched', 'org-b');
    ready();
    await Promise.resolve();
    await Promise.resolve();
    expect(send).not.toHaveBeenCalled();
    expect(aiChatStore.explanationRequestSignal()).toBeNull();
    expect(aiChatStore.context.handoffContext).toBeUndefined();
  });

  it('reports preparation failure without sending or silently retrying', async () => {
    const error = new Error('offline');
    const { send, onError } = mount(vi.fn(), vi.fn().mockRejectedValue(error));
    aiChatStore.explain({ handoffContext: 'Evidence' });
    await waitFor(() => expect(onError).toHaveBeenCalledWith(error));
    expect(send).not.toHaveBeenCalled();
    expect(aiChatStore.context.handoffContext).toBe('Evidence');
  });
});
