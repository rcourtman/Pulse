import { parseJSONTextSafe } from './responseUtils';

export interface JSONEventStreamOptions<T> {
  onEvent: (event: T) => boolean | void;
  onParseError?: (line: string) => void;
  onTrailingParseError?: (line: string) => void;
  onTimeout?: () => void;
  onComplete?: () => void;
  timeoutMs?: number;
}

export async function consumeJSONEventStream<T>(
  response: Response,
  options: JSONEventStreamOptions<T>,
): Promise<void> {
  const reader = response.body?.getReader();
  if (!reader) {
    throw new Error('No response body');
  }

  const decoder = new TextDecoder();
  let buffer = '';
  const timeoutMs = options.timeoutMs ?? 300000;
  let lastEventTime = Date.now();

  const readWithTimeout = async (): Promise<ReadableStreamReadResult<Uint8Array>> => {
    let timeoutId: ReturnType<typeof setTimeout> | undefined;
    try {
      return await Promise.race([
        reader.read(),
        new Promise<never>((_, reject) => {
          timeoutId = setTimeout(() => reject(new Error('Read timeout')), timeoutMs);
        }),
      ]);
    } finally {
      if (timeoutId !== undefined) {
        clearTimeout(timeoutId);
      }
    }
  };

  const processMessages = (chunk: string): boolean => {
    const normalizedBuffer = chunk.replace(/\r\n/g, '\n');
    const messages = normalizedBuffer.split('\n\n');
    buffer = messages.pop() || '';

    for (const message of messages) {
      if (!message.trim() || message.trim().startsWith(':')) {
        continue;
      }

      const dataLines = message.split('\n').filter((line) => line.startsWith('data: '));
      for (const line of dataLines) {
        const jsonStr = line.slice(6);
        if (!jsonStr.trim()) {
          continue;
        }

        const event = parseJSONTextSafe<T>(jsonStr);
        if (!event) {
          options.onParseError?.(line);
          continue;
        }

        if (options.onEvent(event)) {
          return true;
        }
      }
    }

    return false;
  };

  try {
    for (;;) {
      if (Date.now() - lastEventTime > timeoutMs) {
        options.onTimeout?.();
        break;
      }

      let result: ReadableStreamReadResult<Uint8Array>;
      try {
        result = await readWithTimeout();
      } catch (error) {
        if ((error as Error).message === 'Read timeout') {
          break;
        }
        throw error;
      }

      const { done, value } = result;
      if (done) {
        break;
      }

      lastEventTime = Date.now();
      buffer += decoder.decode(value, { stream: true });
      if (processMessages(buffer)) {
        return;
      }
    }

    const trailing = buffer.trim();
    if (trailing.startsWith('data: ')) {
      const event = parseJSONTextSafe<T>(trailing.slice(6));
      if (event) {
        if (options.onEvent(event)) {
          return;
        }
      } else {
        options.onTrailingParseError?.(trailing);
      }
    }

    options.onComplete?.();
  } finally {
    reader.releaseLock();
  }
}
