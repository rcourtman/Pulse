import { splitProps, type JSX } from 'solid-js';

interface TlsVerificationWarningBannerProps extends JSX.HTMLAttributes<HTMLDivElement> {
  subject: string;
  remediation?: string;
}

const bannerClass =
  'rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-200';

export function TlsVerificationWarningBanner(props: TlsVerificationWarningBannerProps) {
  const [local, rest] = splitProps(props, ['subject', 'remediation', 'class']);

  return (
    <div
      role="alert"
      class={`${bannerClass} ${local.class ?? ''}`.trim()}
      {...rest}
    >
      <span class="font-medium">TLS verification disabled.</span>{' '}
      Pulse will accept untrusted certificates for {local.subject}. Use this only for
      controlled lab environments.{' '}
      {local.remediation ?? 'Install a trusted certificate before using this in production.'}
    </div>
  );
}

export default TlsVerificationWarningBanner;
