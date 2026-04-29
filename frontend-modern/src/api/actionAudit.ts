import { apiFetchJSON } from '@/utils/apiClient';
import { apiErrorStatus, objectArrayFieldOrEmpty } from './responseUtils';
import type { ActionAuditListResponse, ActionAuditRecord } from '@/types/actionAudit';

export interface ListActionAuditsOptions {
  resourceId?: string;
  since?: string | number | Date;
  limit?: number;
}

type RawActionAuditListResponse = Partial<
  Omit<ActionAuditListResponse, 'audits' | 'available'> & {
    audits?: unknown;
  }
>;

const ACTION_AUDIT_UNAVAILABLE_STATUSES = new Set([401, 402, 403]);

const unavailableActionAuditResponse = (resourceId?: string): ActionAuditListResponse => ({
  audits: [],
  count: 0,
  resourceId,
  available: false,
});

const normalizeLimit = (limit: number | undefined): number | undefined => {
  if (!Number.isFinite(limit ?? NaN) || (limit ?? 0) <= 0) return undefined;
  return Math.trunc(limit ?? 0);
};

const normalizeSince = (since: string | number | Date | undefined): string | undefined => {
  if (since === undefined || since === null || `${since}`.trim() === '') return undefined;
  const date = since instanceof Date ? since : new Date(since);
  return Number.isFinite(date.getTime()) ? date.toISOString() : undefined;
};

const buildActionAuditQuery = (options?: ListActionAuditsOptions): string => {
  const params = new URLSearchParams();
  const resourceId = options?.resourceId?.trim();
  if (resourceId) params.set('resourceId', resourceId);

  const since = normalizeSince(options?.since);
  if (since) params.set('since', since);

  const limit = normalizeLimit(options?.limit);
  if (limit) params.set('limit', String(limit));

  const query = params.toString();
  return query ? `?${query}` : '';
};

const normalizeActionAuditListResponse = (
  raw: RawActionAuditListResponse | null | undefined,
  fallbackResourceId?: string,
): ActionAuditListResponse => {
  const audits = objectArrayFieldOrEmpty<ActionAuditRecord>(raw, 'audits');
  const count = Number.isFinite(raw?.count) ? Number(raw?.count) : audits.length;
  const resourceId =
    typeof raw?.resourceId === 'string' && raw.resourceId.trim()
      ? raw.resourceId.trim()
      : fallbackResourceId;

  return {
    audits,
    count,
    resourceId,
    available: true,
  };
};

export class ActionAuditAPI {
  static async listActionAudits(
    options?: ListActionAuditsOptions,
  ): Promise<ActionAuditListResponse> {
    const resourceId = options?.resourceId?.trim() || undefined;

    try {
      const raw = await apiFetchJSON<RawActionAuditListResponse>(
        `/api/audit/actions${buildActionAuditQuery(options)}`,
        { cache: 'no-store' },
      );
      return normalizeActionAuditListResponse(raw, resourceId);
    } catch (error) {
      const status = apiErrorStatus(error);
      if (status !== null && ACTION_AUDIT_UNAVAILABLE_STATUSES.has(status)) {
        return unavailableActionAuditResponse(resourceId);
      }
      throw error;
    }
  }
}
