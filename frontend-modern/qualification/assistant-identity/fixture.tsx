// Synthetic component journey: real reducer and transcript, no application server.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { AIChatAPI, type StreamEvent } from '../../src/api/aiChat';
import { useChat } from '../../src/components/AI/Chat/hooks/useChat';
import { ChatMessages } from '../../src/components/AI/Chat/ChatMessages';
import type { ChatMessage } from '../../src/components/AI/Chat/types';
import '../../src/index.css';

let dispatch: (event: StreamEvent) => void;
AIChatAPI.chat = async (_prompt, _session, _model, onEvent) => {
  dispatch = onEvent;
  await new Promise(() => {}); // Keep stream open until the fixture is disposed.
};
const noOp = () => {};
function Fixture() {
  const chat = useChat({ sessionId: 'synthetic-identity' });
  const [override, setOverride] = createSignal<ChatMessage[] | null>(null);
  const a = { name: 'pulse_query', input: 'client', output: 'client evidence', success: true };
  const b = { name: 'pulse_alerts', input: 'alerts', output: 'alert evidence', success: true };
  const pendingA = { id: 'a', name: a.name, input: a.input };
  const pendingB = { id: 'b', name: b.name, input: b.input };
  const workflow = {
    type: 'workflow_status' as const,
    workflowStatus: { phase: 'provider_start', message: 'Starting' },
  };
  const message = (extra: Partial<ChatMessage>): ChatMessage => ({
    id: 'shared',
    role: 'assistant',
    content: '',
    timestamp: new Date('2026-01-01T00:00:00Z'),
    ...extra,
  });
  Object.assign(window, {
    identityFixture: {
      start: () => {
        void chat.sendMessage('Synthetic identity check');
      },
      fire: (event: StreamEvent) => dispatch(event),
      snapshot: () => chat.messages().find((m) => m.role === 'assistant'),
      evidence: () => [a, b],
      shared: (stage: number) =>
        setOverride([
          message({
            toolCalls: stage === 0 ? [] : stage === 1 ? [a] : [a, b],
            pendingTools: stage === 0 ? [pendingA, pendingB] : stage === 1 ? [pendingB] : [],
            streamEvents: [
              ...(stage < 3 ? [workflow] : []),
              stage === 0
                ? { type: 'pending_tool', toolId: 'a', pendingTool: pendingA }
                : { type: 'tool', toolId: 'a', tool: a },
              stage < 2
                ? { type: 'pending_tool', toolId: 'b', pendingTool: pendingB }
                : { type: 'tool', toolId: 'b', tool: b },
            ],
          }),
        ]),
    },
  });
  return (
    <main style={{ height: '100vh', display: 'flex', 'flex-direction': 'column' }}>
      <ChatMessages
        messages={override() ?? chat.messages()}
        onApprove={(id, approval) => chat.updateApproval(id, approval.toolId, { removed: true })}
        onSkip={(id, toolId) => chat.updateApproval(id, toolId, { removed: true })}
        onAnswerQuestion={noOp}
        onSkipQuestion={noOp}
      />
    </main>
  );
}
render(() => <Fixture />, document.getElementById('root')!);
