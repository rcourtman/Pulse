import type { AIChatStreamEvent } from './generated/aiChatEvents';

export const AI_CHAT_DEV_STREAM_FIXTURE_PROMPTS = [
  '/fixture devices',
  '/fixture assistant-stream',
  '/fixture tool-burst',
  '/fixture context-group',
  '/fixture pending-tool',
  '/fixture provider-retry',
  '/fixture stream-idle',
  '/fixture queue-hold',
  '/fixture compacted-artifact',
  '/fixture skipped-tool',
] as const;

const AI_CHAT_DEV_STREAM_FIXTURE_ALIASES: Record<
  string,
  (typeof AI_CHAT_DEV_STREAM_FIXTURE_PROMPTS)[number]
> = {
  '/fixture burst-tool': '/fixture tool-burst',
};

const availableFixtureNames = AI_CHAT_DEV_STREAM_FIXTURE_PROMPTS.map((prompt) =>
  prompt.replace('/fixture ', ''),
);

const DEFAULT_DEV_FIXTURE_STEP_DELAY_MS = 140;
const TEST_FIXTURE_STEP_DELAY_MS = 0;

export interface AIChatDevStreamFixtureOptions {
  model?: string;
  onEvent: (event: AIChatStreamEvent) => void;
  prompt: string;
  signal?: AbortSignal;
  stepDelayMs?: number;
}

const normalizeFixturePrompt = (prompt: string) => prompt.trim().toLowerCase().replace(/\s+/g, ' ');

const canonicalizeFixturePrompt = (prompt: string) => {
  const normalized = normalizeFixturePrompt(prompt);
  return AI_CHAT_DEV_STREAM_FIXTURE_ALIASES[normalized] || normalized;
};

const isAIChatDevStreamFixtureCommand = (prompt: string): boolean => {
  const normalized = normalizeFixturePrompt(prompt);
  return normalized === '/fixture' || normalized.startsWith('/fixture ');
};

export const isAIChatDevStreamFixturePrompt = (prompt: string): boolean => {
  const normalized = canonicalizeFixturePrompt(prompt);
  return AI_CHAT_DEV_STREAM_FIXTURE_PROMPTS.some((fixturePrompt) => fixturePrompt === normalized);
};

const isAIChatDevStreamFixtureAvailable = () =>
  import.meta.env.DEV || import.meta.env.MODE === 'test';

const abortError = () => {
  const error = new Error('The operation was aborted.');
  error.name = 'AbortError';
  return error;
};

const throwIfAborted = (signal?: AbortSignal) => {
  if (signal?.aborted) {
    throw abortError();
  }
};

const waitForFixtureStep = (delayMs: number, signal?: AbortSignal) => {
  throwIfAborted(signal);
  if (delayMs <= 0) return Promise.resolve();

  return new Promise<void>((resolve, reject) => {
    let timeoutId: ReturnType<typeof setTimeout> | undefined;
    const cleanup = () => {
      if (timeoutId !== undefined) {
        clearTimeout(timeoutId);
      }
      signal?.removeEventListener('abort', handleAbort);
    };
    const handleAbort = () => {
      cleanup();
      reject(abortError());
    };

    timeoutId = setTimeout(() => {
      cleanup();
      resolve();
    }, delayMs);
    signal?.addEventListener('abort', handleAbort, { once: true });
  });
};

const assistantFixtureModel = (model?: string) => model?.trim() || 'dev:assistant-stream-fixture';

const buildUnknownFixtureEvents = (prompt: string): AIChatStreamEvent[] => [
  {
    type: 'session',
    data: { id: 'dev-fixture-unknown' },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'request_start',
      message: 'Checking local Assistant fixture.',
    },
  },
  {
    type: 'error',
    data: {
      message: `Unknown Assistant fixture "${prompt}". Available fixtures: ${availableFixtureNames.join(', ')}.`,
    },
  },
];

const buildDeviceCountFixtureEvents = (model?: string): AIChatStreamEvent[] => [
  {
    type: 'session',
    data: { id: 'dev-fixture-assistant-stream' },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'request_start',
      message: 'Preparing Pulse context.',
    },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'context',
      message: 'Reading current Pulse inventory with pulse_query.',
      tool: 'pulse_query',
    },
  },
  {
    type: 'thinking',
    data: { text: 'Checking device-node and block-device evidence before answering.' },
  },
  {
    type: 'tool_start',
    data: {
      id: 'fixture-tool-devices',
      name: 'pulse_read',
      input: '{}',
      raw_input: 'pulse_read(target_host="current_resource", command="ls /dev | wc',
    },
  },
  {
    type: 'tool_progress',
    data: {
      id: 'fixture-tool-devices',
      name: 'pulse_read',
      phase: 'running',
      message: 'Running command.',
      input:
        '{"target_host":"current_resource","command":"ls /dev | wc -l && lsblk -d -o NAME,TYPE,SIZE"}',
      raw_input:
        'pulse_read(target_host="current_resource", command="ls /dev | wc -l && lsblk -d -o NAME,TYPE,SIZE")',
    },
  },
  {
    type: 'tool_end',
    data: {
      id: 'fixture-tool-devices',
      name: 'pulse_read',
      input:
        '{"target_host":"current_resource","command":"ls /dev | wc -l && lsblk -d -o NAME,TYPE,SIZE"}',
      raw_input:
        'pulse_read(target_host="current_resource", command="ls /dev | wc -l && lsblk -d -o NAME,TYPE,SIZE")',
      output: ['4358', 'NAME    TYPE SIZE', 'sda     disk 1.8T', 'nvme0n1 disk 931.5G'].join('\n'),
      success: true,
    },
  },
  {
    type: 'content',
    data: {
      text: 'There are 4,358 entries under `/dev`. The block-device view shows 2 disk devices: `sda` and `nvme0n1`.',
    },
  },
  {
    type: 'done',
    data: {
      session_id: 'dev-fixture-assistant-stream',
      model: assistantFixtureModel(model),
      input_tokens: 4358,
      output_tokens: 943,
    },
  },
];

const buildSkippedToolFixtureEvents = (model?: string): AIChatStreamEvent[] => [
  {
    type: 'session',
    data: { id: 'dev-fixture-skipped-tool' },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'request_start',
      message: 'Preparing Pulse context.',
    },
  },
  {
    type: 'tool_start',
    data: {
      id: 'fixture-tool-skipped',
      name: 'pulse_read',
      input: '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
      raw_input: 'pulse_read(target_host="current_resource", command="ls /dev | wc -l")',
    },
  },
  {
    type: 'tool_cancel',
    data: {
      id: 'fixture-tool-skipped',
      name: 'pulse_read',
      reason: 'current_resource unavailable',
    },
  },
  {
    type: 'content',
    data: {
      text: 'I could not inspect the current resource because no resource context was attached to this chat turn.',
    },
  },
  {
    type: 'done',
    data: {
      session_id: 'dev-fixture-skipped-tool',
      model: assistantFixtureModel(model),
      input_tokens: 41,
      output_tokens: 27,
    },
  },
];

const buildToolBurstFixtureEvents = (model?: string): AIChatStreamEvent[] => [
  {
    type: 'session',
    data: { id: 'dev-fixture-tool-burst' },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'request_start',
      message: 'Preparing Pulse context.',
    },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'provider_start',
      message: 'Sent request to OpenRouter; waiting for the first token.',
      model: assistantFixtureModel(model),
    },
  },
  {
    type: 'tool_start',
    data: {
      id: 'fixture-tool-burst',
      name: 'pulse_read',
      input: '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
      raw_input: 'pulse_read(target_host="current_resource", command="ls /dev | wc -l")',
    },
  },
  {
    type: 'tool_end',
    data: {
      id: 'fixture-tool-burst',
      name: 'pulse_read',
      input: '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
      raw_input: 'pulse_read(target_host="current_resource", command="ls /dev | wc -l")',
      output: '4358',
      success: true,
    },
  },
  {
    type: 'content',
    data: {
      text: 'The burst fixture completed a fast `pulse_read` command and kept the tool transition visible.',
    },
  },
  {
    type: 'done',
    data: {
      session_id: 'dev-fixture-tool-burst',
      model: assistantFixtureModel(model),
      input_tokens: 64,
      output_tokens: 31,
    },
  },
];

const buildContextGroupFixtureEvents = (model?: string): AIChatStreamEvent[] => [
  {
    type: 'session',
    data: { id: 'dev-fixture-context-group' },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'request_start',
      message: 'Preparing Pulse context.',
    },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'context',
      message: 'Gathering resource details and recent metrics.',
      tool: 'pulse_get_resource_details',
    },
  },
  {
    type: 'tool_start',
    data: {
      id: 'fixture-tool-context-resource',
      name: 'pulse_get_resource_details',
      input: '{"resource_id":"vm-101"}',
      raw_input: 'pulse_get_resource_details(resource_id="vm-101")',
    },
  },
  {
    type: 'tool_end',
    data: {
      id: 'fixture-tool-context-resource',
      name: 'pulse_get_resource_details',
      input: '{"resource_id":"vm-101"}',
      raw_input: 'pulse_get_resource_details(resource_id="vm-101")',
      output: '{"name":"web-101","status":"running","node":"pve-1"}',
      success: true,
    },
  },
  {
    type: 'tool_start',
    data: {
      id: 'fixture-tool-context-metrics',
      name: 'pulse_get_metrics_history',
      input: '{"resource_id":"vm-101","metric":"cpu","range":"1h"}',
      raw_input: 'pulse_get_metrics_history(resource_id="vm-101", metric="cpu", range="1h")',
    },
  },
  {
    type: 'tool_end',
    data: {
      id: 'fixture-tool-context-metrics',
      name: 'pulse_get_metrics_history',
      input: '{"resource_id":"vm-101","metric":"cpu","range":"1h"}',
      raw_input: 'pulse_get_metrics_history(resource_id="vm-101", metric="cpu", range="1h")',
      output: '{"average":42,"peak":67,"unit":"percent"}',
      success: true,
    },
  },
  {
    type: 'content',
    data: {
      text: 'The context-group fixture gathered the resource identity and recent CPU history as one compact context activity row.',
    },
  },
  {
    type: 'done',
    data: {
      session_id: 'dev-fixture-context-group',
      model: assistantFixtureModel(model),
      input_tokens: 82,
      output_tokens: 33,
    },
  },
];

const buildPendingToolFixtureEvents = (model?: string): AIChatStreamEvent[] => [
  {
    type: 'session',
    data: { id: 'dev-fixture-pending-tool' },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'request_start',
      message: 'Preparing Pulse context.',
    },
  },
  {
    type: 'tool_start',
    data: {
      id: 'fixture-tool-pending',
      name: 'pulse_read',
      input: '{}',
      raw_input: 'pulse_read(target_host="current_resource", command="ls /dev | wc',
    },
  },
  {
    type: 'tool_progress',
    data: {
      id: 'fixture-tool-pending',
      name: 'pulse_read',
      phase: 'running',
      message: 'Running command.',
      input:
        '{"target_host":"current_resource","command":"ls /dev | wc -l && lsblk -d -o NAME,TYPE,SIZE"}',
      raw_input:
        'pulse_read(target_host="current_resource", command="ls /dev | wc -l && lsblk -d -o NAME,TYPE,SIZE")',
    },
  },
  {
    type: 'tool_end',
    data: {
      id: 'fixture-tool-pending',
      name: 'pulse_read',
      input:
        '{"target_host":"current_resource","command":"ls /dev | wc -l && lsblk -d -o NAME,TYPE,SIZE"}',
      raw_input:
        'pulse_read(target_host="current_resource", command="ls /dev | wc -l && lsblk -d -o NAME,TYPE,SIZE")',
      output: ['4358', 'NAME    TYPE SIZE', 'sda     disk 1.8T', 'nvme0n1 disk 931.5G'].join('\n'),
      success: true,
    },
  },
  {
    type: 'content',
    data: {
      text: 'The pending-tool fixture kept the running tool row inspectable before completion.',
    },
  },
  {
    type: 'done',
    data: {
      session_id: 'dev-fixture-pending-tool',
      model: assistantFixtureModel(model),
      input_tokens: 88,
      output_tokens: 34,
    },
  },
];

const buildProviderRetryFixtureEvents = (): AIChatStreamEvent[] => [
  {
    type: 'session',
    data: { id: 'dev-fixture-provider-retry' },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'request_start',
      message: 'Preparing Pulse context.',
    },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'provider_start',
      message: 'Sent request to DeepSeek; waiting for the first token.',
      provider: 'deepseek',
      model: 'deepseek:deepseek-chat',
    },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'provider_retry',
      message: 'DeepSeek failed before output; retrying through OpenRouter.',
      provider: 'deepseek',
      model: 'deepseek:deepseek-chat',
      failed_model: 'deepseek:deepseek-chat',
      next_model: 'openrouter:deepseek/deepseek-chat',
      attempt: 2,
      max_attempts: 3,
      retry_after_ms: 3200,
    },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'provider_start',
      message: 'Retrying through OpenRouter with deepseek/deepseek-chat.',
      provider: 'openrouter',
      model: 'openrouter:deepseek/deepseek-chat',
    },
  },
  {
    type: 'content',
    data: {
      text: 'The provider retry fixture switched to OpenRouter after the direct provider failed before output.',
    },
  },
  {
    type: 'done',
    data: {
      session_id: 'dev-fixture-provider-retry',
      model: 'openrouter:deepseek/deepseek-chat',
      input_tokens: 73,
      output_tokens: 29,
    },
  },
];

const buildStreamIdleFixtureEvents = (model?: string): AIChatStreamEvent[] => [
  {
    type: 'session',
    data: { id: 'dev-fixture-stream-idle' },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'request_start',
      message: 'Preparing Pulse context.',
    },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'provider_start',
      message: 'Sent request to OpenRouter; waiting for the first token.',
      provider: 'openrouter',
      model: assistantFixtureModel(model),
    },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'stream_idle',
      message: 'Assistant is still working; waiting for the next stream event.',
    },
  },
  {
    type: 'content',
    data: {
      text: 'The stream-idle fixture kept visible progress alive while the provider stayed quiet.',
    },
  },
  {
    type: 'done',
    data: {
      session_id: 'dev-fixture-stream-idle',
      model: assistantFixtureModel(model),
      input_tokens: 41,
      output_tokens: 19,
    },
  },
];

const buildQueueHoldFixtureEvents = (model?: string): AIChatStreamEvent[] => [
  {
    type: 'session',
    data: { id: 'dev-fixture-queue-hold' },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'request_start',
      message: 'Preparing Pulse context.',
    },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'provider_start',
      message: 'Sent request to OpenRouter; waiting for the first token.',
      provider: 'openrouter',
      model: assistantFixtureModel(model),
    },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'stream_idle',
      message: 'Holding the Assistant turn open so queued follow-ups can be reordered.',
    },
  },
  {
    type: 'content',
    data: {
      text: 'The queue-hold fixture kept the turn active long enough to inspect queued follow-up controls.',
    },
  },
  {
    type: 'done',
    data: {
      session_id: 'dev-fixture-queue-hold',
      model: assistantFixtureModel(model),
      input_tokens: 44,
      output_tokens: 23,
    },
  },
];

const buildCompactedArtifactFixtureEvents = (model?: string): AIChatStreamEvent[] => [
  {
    type: 'session',
    data: { id: 'dev-fixture-compacted-artifact' },
  },
  {
    type: 'workflow_state',
    data: {
      phase: 'request_start',
      message: 'Preparing Pulse context.',
    },
  },
  {
    type: 'content',
    data: {
      text: "I'llcheckthedevicenodesinsidethecontainertoanswerthat.Letmecounttheentriesin/devandlisttheblockdevices.",
    },
  },
  {
    type: 'tool_start',
    data: {
      id: 'fixture-tool-compacted',
      name: 'pulse_read',
      input:
        '{"target_host":"current_resource","command":"ls /dev | wc -l && lsblk -d -o NAME,TYPE,SIZE"}',
      raw_input:
        'pulse_read(target_host="current_resource", command="ls /dev | wc -l && lsblk -d -o NAME,TYPE,SIZE")',
    },
  },
  {
    type: 'tool_end',
    data: {
      id: 'fixture-tool-compacted',
      name: 'pulse_read',
      input:
        '{"target_host":"current_resource","command":"ls /dev | wc -l && lsblk -d -o NAME,TYPE,SIZE"}',
      raw_input:
        'pulse_read(target_host="current_resource", command="ls /dev | wc -l && lsblk -d -o NAME,TYPE,SIZE")',
      output: ['4358', 'NAME    TYPE SIZE', 'sda     disk 1.8T', 'nvme0n1 disk 931.5G'].join('\n'),
      success: true,
    },
  },
  {
    type: 'content',
    data: {
      text: 'There are 4,358 entries under `/dev`, including 2 block devices: `sda` and `nvme0n1`.',
    },
  },
  {
    type: 'done',
    data: {
      session_id: 'dev-fixture-compacted-artifact',
      model: assistantFixtureModel(model),
      input_tokens: 96,
      output_tokens: 42,
    },
  },
];

const buildFixtureEvents = (prompt: string, model?: string): AIChatStreamEvent[] => {
  const normalized = normalizeFixturePrompt(prompt);
  if (normalized === '/fixture compacted-artifact') {
    return buildCompactedArtifactFixtureEvents(model);
  }
  if (normalized === '/fixture skipped-tool') {
    return buildSkippedToolFixtureEvents(model);
  }
  if (normalized === '/fixture tool-burst') {
    return buildToolBurstFixtureEvents(model);
  }
  if (normalized === '/fixture context-group') {
    return buildContextGroupFixtureEvents(model);
  }
  if (normalized === '/fixture pending-tool') {
    return buildPendingToolFixtureEvents(model);
  }
  if (normalized === '/fixture provider-retry') {
    return buildProviderRetryFixtureEvents();
  }
  if (normalized === '/fixture stream-idle') {
    return buildStreamIdleFixtureEvents(model);
  }
  if (normalized === '/fixture queue-hold') {
    return buildQueueHoldFixtureEvents(model);
  }
  return buildDeviceCountFixtureEvents(model);
};

const fixtureStepDelay = (
  normalizedPrompt: string,
  event: AIChatStreamEvent,
  defaultDelayMs: number,
): number => {
  if (defaultDelayMs <= 0) return 0;
  if (
    normalizedPrompt === '/fixture provider-retry' &&
    event.type === 'workflow_state' &&
    event.data.phase === 'provider_retry'
  ) {
    return 1600;
  }
  if (normalizedPrompt === '/fixture pending-tool' && event.type === 'tool_progress') {
    return 2200;
  }
  if (normalizedPrompt === '/fixture context-group' && event.type === 'tool_start') {
    return 900;
  }
  if (normalizedPrompt === '/fixture context-group' && event.type === 'tool_end') {
    return 180;
  }
  if (
    normalizedPrompt === '/fixture stream-idle' &&
    event.type === 'workflow_state' &&
    event.data.phase === 'stream_idle'
  ) {
    return 1800;
  }
  if (
    normalizedPrompt === '/fixture queue-hold' &&
    event.type === 'workflow_state' &&
    event.data.phase === 'stream_idle'
  ) {
    return 10000;
  }
  if (normalizedPrompt === '/fixture pending-tool') return defaultDelayMs;
  if (normalizedPrompt !== '/fixture tool-burst') return defaultDelayMs;
  if (event.type === 'tool_start') return 0;
  return defaultDelayMs;
};

export const maybeRunAIChatDevStreamFixture = async (
  options: AIChatDevStreamFixtureOptions,
): Promise<boolean> => {
  const normalizedPrompt = canonicalizeFixturePrompt(options.prompt);
  if (!isAIChatDevStreamFixtureAvailable() || !isAIChatDevStreamFixtureCommand(options.prompt)) {
    return false;
  }

  const delayMs =
    options.stepDelayMs ??
    (import.meta.env.MODE === 'test'
      ? TEST_FIXTURE_STEP_DELAY_MS
      : DEFAULT_DEV_FIXTURE_STEP_DELAY_MS);
  const events = isAIChatDevStreamFixturePrompt(normalizedPrompt)
    ? buildFixtureEvents(normalizedPrompt, options.model)
    : buildUnknownFixtureEvents(normalizeFixturePrompt(options.prompt));

  for (let index = 0; index < events.length; index += 1) {
    const event = events[index];
    throwIfAborted(options.signal);
    options.onEvent(event);
    if (index < events.length - 1) {
      await waitForFixtureStep(fixtureStepDelay(normalizedPrompt, event, delayMs), options.signal);
    }
  }

  return true;
};
