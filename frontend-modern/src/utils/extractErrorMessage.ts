/**
 * Extract a user-friendly error message from an API response.
 * Handles various response formats including JSON with error/message fields.
 *
 * @param response - The Response object from fetch
 * @param defaultMessage - Default message if extraction fails
 * @returns The extracted error message
 */
export async function extractErrorMessage(
  response: Response,
  defaultMessage?: string
): Promise<string> {
  const fallback = defaultMessage || `Failed with status ${response.status}`;

  try {
    const text = await response.text();
    if (!text?.trim()) {
      return fallback;
    }

    // Try to parse as JSON first
    try {
      const parsed = JSON.parse(text);
      // Check for common error field names
      if (typeof parsed?.error === 'string' && parsed.error.trim()) {
        return parsed.error.trim();
      }
      if (typeof parsed?.message === 'string' && parsed.message.trim()) {
        return parsed.message.trim();
      }
    } catch {
      // Not JSON, fall through to use raw text
    }

    // Use raw text if it's non-empty
    return text.trim();
  } catch {
    // Body read failed
    return fallback;
  }
}
