import { Show, type Component } from 'solid-js';

// A return message describes the attempt, not what the current session or
// sign-in options allow. Never render arbitrary query text.
export function readSSOReturnError(params: URLSearchParams): string {
  const protocol =
    params.get('saml') === 'error' ? 'saml' : params.get('oidc') === 'error' ? 'oidc' : undefined;
  if (!protocol) return '';

  switch (params.get(`${protocol}_error`)) {
    case 'email_restricted':
      return 'Your account email is not permitted to access Pulse.';
    case 'domain_restricted':
      return 'Your email domain is not allowed for Pulse access.';
    case 'email_unverified':
      return 'Your identity provider has not verified your email address. Verify it there, then sign in again.';
    case 'group_restricted':
      return 'Your account is not part of an authorized group to use Pulse.';
    case 'invalid_state':
      return 'The sign-in attempt expired. Please try again.';
    case 'exchange_failed':
      return 'We could not complete the sign-in request. Please try again shortly.';
    case 'session_failed':
      return 'Login succeeded but we could not create a session. Try again.';
    case 'invalid_id_token':
      return 'ID token verification failed. The server log records the exact reason for this login attempt.';
    case 'issuer_mismatch':
      return 'The token was issued by a different issuer than the one configured. The server log shows the expected and received issuer values for this login attempt.';
    case 'audience_mismatch':
      return 'The token was issued for a different client ID than the one configured in Pulse. Check the client ID against your provider application.';
    case 'token_expired':
      return 'The identity provider returned an already-expired token. Check that the clocks on Pulse and the provider are in sync.';
    case 'invalid_signature_alg':
      return "The identity provider is issuing HS256 tokens. Configure it to sign ID tokens with RS256 (see your IdP's OIDC settings).";
    case 'invalid_nonce':
      return 'Security validation failed (nonce mismatch). Please try again.';
    case 'saml_validation_failed':
      return 'SAML assertion validation failed. Please try again or contact an administrator.';
    case 'provider_not_found':
      return 'SSO provider not found or not configured.';
    case 'provider_not_initialized':
      return 'SSO provider is not properly configured. Contact your administrator.';
    default:
      return 'Single sign-on failed. Please try again or contact an administrator.';
  }
}

export const SSOReturnError: Component<{ message?: string; duringCheck?: boolean }> = (props) => (
  <Show when={props.message}>
    <div
      role="alert"
      class="rounded-md bg-red-50 dark:bg-red-900/25 border border-red-200 dark:border-red-800 px-3 py-2 text-sm dark:text-red-300"
      classList={{
        'mt-4 text-left text-red-700': props.duringCheck,
        'text-red-600': !props.duringCheck,
      }}
    >
      <Show when={props.duringCheck}>
        <p class="mb-1 font-semibold">Last single sign-on attempt</p>
      </Show>
      {props.message}
    </div>
  </Show>
);
