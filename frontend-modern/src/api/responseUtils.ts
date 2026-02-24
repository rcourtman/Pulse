type APIErrorPayload = {
  error?: unknown;
  message?: unknown;
};

function extractErrorMessage(payload: APIErrorPayload): string | null {
  if (typeof payload.error === 'string' && payload.error.trim()) {
    return payload.error.trim();
  }

  if (payload.error && typeof payload.error === 'object') {
    const nestedError = payload.error as { message?: unknown };
    if (typeof nestedError.message === 'string' && nestedError.message.trim()) {
      return nestedError.message.trim();
    }
  }

  if (typeof payload.message === 'string' && payload.message.trim()) {
    return payload.message.trim();
  }

  return null;
}

export async function readAPIErrorMessage(response: Response, fallback: string): Promise<string> {
  try {
    const text = await response.text();
    const trimmed = text.trim();
    if (!trimmed) {
      return fallback;
    }

    try {
      const parsed = JSON.parse(trimmed) as APIErrorPayload;
      return extractErrorMessage(parsed) ?? trimmed;
    } catch {
      return trimmed;
    }
  } catch {
    return fallback;
  }
}

export async function parseOptionalJSON<T>(
  response: Response,
  emptyValue: T,
  parseErrorMessage: string,
): Promise<T> {
  const text = await response.text();
  if (!text.trim()) {
    return emptyValue;
  }

  try {
    return JSON.parse(text) as T;
  } catch {
    throw new Error(parseErrorMessage);
  }
}
