export const AUDIT_WEBHOOK_READONLY_NOTICE_CLASS =
  'rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-200';

export const AUDIT_WEBHOOK_ENDPOINT_CARD_CLASS =
  'flex items-center justify-between gap-3 rounded-md border border-border bg-surface-alt p-3';

export const AUDIT_WEBHOOK_ENDPOINT_ICON_CLASS =
  'p-2 bg-blue-100 dark:bg-blue-900 text-blue-600 dark:text-blue-300 rounded-md shrink-0';

export interface AuditWebhookFeatureGateCopy {
  title: string;
  body: string;
}

export interface AuditWebhookEmptyStateCopy {
  title: string;
}

export function getAuditWebhookFeatureGateCopy(): AuditWebhookFeatureGateCopy {
  return {
    title: 'Audit Webhooks (Pro)',
    body: 'Audit webhooks are part of the audit logging feature set and require Pro.',
  };
}

export function getAuditWebhookEmptyStateCopy(): AuditWebhookEmptyStateCopy {
  return {
    title: 'No audit webhooks configured yet.',
  };
}

export function getAuditWebhookLoadingState() {
  return {
    text: 'Loading audit webhooks…',
  } as const;
}
