/** Only final HTTP status establishes an access failure, never transport text. */
export function getAPIReadAccessErrorMessage(error: unknown): string | null {
  if (!error || typeof error !== 'object' || !('status' in error)) return null;
  switch (error.status) {
    case 401:
      return 'Sign in again or check your API token.';
    case 403:
      return 'Access denied. Check your permissions and license plan.';
    default:
      return null;
  }
}
