import { parseJSONTextSafe } from './responseUtils';

export interface JSONEventStreamOptions<T> {
  onEvent: (event: T) => boolean | void;
  onParseError?: (line: string) => void;
  onTrailingParseError?: (line: string) => void;
  onTimeout?: () => void;
  onComplete?: () => void;
  timeoutMs?: number;
  yieldBetweenEvents?: boolean | ((event: T) => boolean);
}

const yieldToBrowser = () =>
  new Promise<void>((resolve) => {
    setTimeout(resolve, 1);
  });

const shouldYieldAfterEvent = <T,>(
  event: T,
  options: JSONEventStreamOptions<T>,
  hasBufferedEventAfterThisOne: boolean,
) => {
  if (!hasBufferedEventAfterThisOne || !options.yieldBetweenEvents) return false;
  if (typeof options.yieldBetweenEvents === 'function') {
    return options.yieldBetweenEvents(event);
  }
  return true;
};

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

  const processMessages = async (chunk: string): Promise<boolean> => {
    const normalizedBuffer = chunk.replace(/\r\n/g, '\n');
    const messages = normalizedBuffer.split('\n\n');
    buffer = messages.pop() || '';

    const messageHasData = (message: string) =>
      !!message
        .split('\n')
        .find((line) => line.startsWith('data: ') && !!line.slice(6).trim());

    for (let messageIndex = 0; messageIndex < messages.length; messageIndex += 1) {
      const message = messages[messageIndex];
      if (!message.trim() || message.trim().startsWith(':')) {
        continue;
      }

      const dataLines = message.split('\n').filter((line) => line.startsWith('data: '));
      for (let lineIndex = 0; lineIndex < dataLines.length; lineIndex += 1) {
        const line = dataLines[lineIndex];
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
        const hasBufferedEventAfterThisOne =
          dataLines.slice(lineIndex + 1).some((nextLine) => !!nextLine.slice(6).trim()) ||
          messages.slice(messageIndex + 1).some(messageHasData);
        if (shouldYieldAfterEvent(event, options, hasBufferedEventAfterThisOne)) {
          await yieldToBrowser();
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
      if (await processMessages(buffer)) {
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
