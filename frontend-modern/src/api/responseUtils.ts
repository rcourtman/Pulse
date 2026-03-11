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

type APIRecordLike = Record<string, unknown>;

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

export async function assertAPIResponseOK(response: Response, fallback: string): Promise<void> {
  if (response.ok) {
    return;
  }

  throw new Error(await readAPIErrorMessage(response, fallback));
}

export async function parseRequiredAPIResponse<T>(
  response: Response,
  requestErrorMessage: string,
  parseErrorMessage: string,
): Promise<T> {
  await assertAPIResponseOK(response, requestErrorMessage);
  return parseRequiredJSON(response, parseErrorMessage);
}

export async function parseOptionalAPIResponse<T>(
  response: Response,
  emptyValue: T,
  requestErrorMessage: string,
  parseErrorMessage: string,
): Promise<T> {
  await assertAPIResponseOK(response, requestErrorMessage);
  return parseOptionalJSON(response, emptyValue, parseErrorMessage);
}

export async function parseRequiredAPIResponseOrNull<T>(
  response: Response,
  nullStatus: number,
  requestErrorMessage: string,
  parseErrorMessage: string,
): Promise<T | null> {
  if (isAPIResponseStatus(response, nullStatus)) {
    return null;
  }

  return parseRequiredAPIResponse(response, requestErrorMessage, parseErrorMessage);
}

export async function parseOptionalAPIResponseOrNull<T>(
  response: Response,
  nullStatus: number,
  requestErrorMessage: string,
  parseErrorMessage: string,
): Promise<T | null> {
  if (isAPIResponseStatus(response, nullStatus)) {
    return null;
  }

  return parseOptionalAPIResponse(response, null, requestErrorMessage, parseErrorMessage);
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

export function arrayOrEmpty<T>(value: unknown): T[] {
  return Array.isArray(value) ? (value as T[]) : [];
}

export function arrayOrUndefined<T>(value: unknown): T[] | undefined {
  return Array.isArray(value) ? (value as T[]) : undefined;
}

export function objectArrayFieldOrEmpty<T>(value: unknown, field: string): T[] {
  if (!value || typeof value !== 'object') {
    return [];
  }

  return arrayOrEmpty<T>((value as APIRecordLike)[field]);
}

export function trimmedString(value: unknown): string {
  return typeof value === 'string' ? value.trim() : value == null ? '' : String(value).trim();
}

export function optionalTrimmedString(value: unknown): string | undefined {
  const normalized = trimmedString(value);
  return normalized.length > 0 ? normalized : undefined;
}

export function strictString(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback;
}

export function strictBoolean(value: unknown, fallback = false): boolean {
  return typeof value === 'boolean' ? value : fallback;
}

export function finiteNumberOrUndefined(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

export function coerceTimestampMillis(value: unknown, fallback: number): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }

  const normalized = optionalTrimmedString(value);
  if (normalized) {
    const parsed = Date.parse(normalized);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }

  return fallback;
}

export function stringArray(value: unknown): string[] {
  return arrayOrEmpty<unknown>(value).filter((item): item is string => typeof item === 'string');
}

export function stringRecordOrUndefined(value: unknown): Record<string, string> | undefined {
  if (!value || typeof value !== 'object') {
    return undefined;
  }

  const entries = Object.entries(value).filter(([, item]): item is string => typeof item === 'string');
  if (entries.length === 0) {
    return undefined;
  }

  return Object.fromEntries(entries);
}

export function normalizeStructuredAPIError(
  payload: unknown,
  fallbackStatus: number,
  fallbackCode = 'request_failed',
): { code: string; message: string; details?: Record<string, string> } {
  const fallbackMessage = `Request failed (${fallbackStatus})`;
  if (!payload || typeof payload !== 'object') {
    return {
      code: fallbackCode,
      message: fallbackMessage,
    };
  }

  const record = payload as APIRecordLike;

  return {
    code: optionalTrimmedString(record.code) ?? fallbackCode,
    message: optionalTrimmedString(record.message) ?? fallbackMessage,
    details: stringRecordOrUndefined(record.details),
  };
}

export function promoteLegacyAlertIdentifier<
  T extends { alertIdentifier?: string } & Record<string, unknown>,
>(record: T & { alert_identifier?: string }): T {
  const alertIdentifier =
    optionalTrimmedString(record.alertIdentifier) ??
    optionalTrimmedString(record.alert_identifier);
  const { alert_identifier: _alertIdentifier, ...rest } = record;
  return alertIdentifier ? ({ ...rest, alertIdentifier } as T) : (rest as T);
}

export function parseJSONTextSafe<T>(text: string): T | null {
  if (!text.trim()) {
    return null;
  }

  try {
    return JSON.parse(text) as T;
  } catch {
    return null;
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

export async function parseRequiredJSON<T>(
  response: Response,
  parseErrorMessage: string,
): Promise<T> {
  const text = await response.text();
  if (!text.trim()) {
    throw new Error(parseErrorMessage);
  }

  try {
    return JSON.parse(text) as T;
  } catch {
    throw new Error(parseErrorMessage);
  }
}

export async function parseJSONSafe<T>(response: Response): Promise<T | null> {
  const text = await response.text();
  return parseJSONTextSafe(text);
}
