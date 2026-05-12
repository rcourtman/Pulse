import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const sectionSource = readFileSync(
  resolve(__dirname, '..', 'MaintenanceVerificationSection.tsx'),
  'utf-8',
);

const overviewTabSource = readFileSync(
  resolve(__dirname, '..', 'ResourceDetailDrawerOverviewTab.tsx'),
  'utf-8',
);

const apiClientSource = readFileSync(
  resolve(__dirname, '..', '..', '..', 'api', 'maintenanceVerification.ts'),
  'utf-8',
);

describe('MaintenanceVerificationSection', () => {
  it('routes list/rerun/review through the canonical maintenanceVerification API client', () => {
    expect(sectionSource).toContain("from '@/api/maintenanceVerification'");
    expect(sectionSource).toContain('listMaintenanceVerificationsForResource');
    expect(sectionSource).toContain('rerunMaintenanceVerification');
    expect(sectionSource).toContain('reviewMaintenanceVerification');
  });

  it('keeps the section out of the parent Suspense fallback by using createNonSuspendingQuery', () => {
    expect(sectionSource).toContain('createNonSuspendingQuery');
    expect(sectionSource).not.toContain('createResource<');
  });

  it('renders an empty state for resources with no reports yet', () => {
    expect(sectionSource).toContain('No verification reports yet.');
  });

  it('surfaces the deterministic evidence counts the report exposes', () => {
    expect(sectionSource).toContain('Critical alerts');
    expect(sectionSource).toContain('Warning alerts');
    expect(sectionSource).toContain('Critical findings');
    expect(sectionSource).toContain('Warning findings');
    expect(sectionSource).toContain('Failed actions');
  });

  it('exposes both operator actions (rerun + mark reviewed) the MVP committed to', () => {
    expect(sectionSource).toContain('data-testid="maintenance-verification-rerun"');
    expect(sectionSource).toContain('data-testid="maintenance-verification-review"');
  });

  it('hides the review button once the report has been reviewed', () => {
    expect(sectionSource).toContain("when={!report.userOutcome}");
    expect(sectionSource).toContain("when={report.userOutcome === 'reviewed'}");
  });

  it('renders the patrolRunTodo breadcrumb when the sentinel surfaced one', () => {
    expect(sectionSource).toContain('report.evidence.patrolRunTodo');
  });
});

describe('maintenanceVerification API client', () => {
  it('encodes the resource id segment so canonical ids with colons survive', () => {
    expect(apiClientSource).toContain('encodeURIComponent(resourceId)');
    expect(apiClientSource).toContain('encodeURIComponent(reportId)');
  });

  it('exposes the four operations the section needs', () => {
    expect(apiClientSource).toContain('export async function listMaintenanceVerificationsForResource');
    expect(apiClientSource).toContain('export async function reviewMaintenanceVerification');
    expect(apiClientSource).toContain('export async function rerunMaintenanceVerification');
  });

  it('pins the four-state status enum the UI branches on', () => {
    expect(apiClientSource).toContain("'pending'");
    expect(apiClientSource).toContain("'healthy'");
    expect(apiClientSource).toContain("'needs_review'");
    expect(apiClientSource).toContain("'failed_verification'");
  });
});

describe('ResourceDetailDrawerOverviewTab integration', () => {
  it('renders MaintenanceVerificationSection directly under the operator-state section', () => {
    expect(overviewTabSource).toContain("from './MaintenanceVerificationSection'");
    expect(overviewTabSource).toContain('<MaintenanceVerificationSection resourceId={resource.id} />');
    const operatorIndex = overviewTabSource.indexOf('<ResourceOperatorStateSection');
    const verificationIndex = overviewTabSource.indexOf('<MaintenanceVerificationSection');
    expect(operatorIndex).toBeGreaterThan(0);
    expect(verificationIndex).toBeGreaterThan(operatorIndex);
  });
});
