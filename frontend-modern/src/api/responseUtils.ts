type APIErrorPayload = {
  error?: unknown;
  message?: unknown;
};

type APIErrorLike = {
  status?: unknown;
};

type APIResponseLike = {
  status?: unknown;
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

export function apiErrorStatus(error: unknown): number | null {
  if (!error || typeof error !== 'object') {
    return null;
  }

  const status = (error as APIErrorLike).status;
  if (typeof status !== 'number' || !Number.isInteger(status) || status < 100 || status > 599) {
    return null;
  }

  return status;
}

export function isAPIErrorStatus(error: unknown, expectedStatus: number): boolean {
  return apiErrorStatus(error) === expectedStatus;
}

export function apiResponseStatus(response: APIResponseLike | null | undefined): number | null {
  if (!response || typeof response !== 'object') {
    return null;
  }

  const status = response.status;
  if (typeof status !== 'number' || !Number.isInteger(status) || status < 100 || status > 599) {
    return null;
  }

  return status;
}

export function isAPIResponseStatus(
  response: APIResponseLike | null | undefined,
  expectedStatus: number,
): boolean {
  return apiResponseStatus(response) === expectedStatus;
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
