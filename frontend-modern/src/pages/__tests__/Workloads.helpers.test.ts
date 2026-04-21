import { describe, expect, it } from 'vitest';
import appSource from '@/App.tsx?raw';
import workloadsPageSource from '@/pages/Workloads.tsx?raw';
import workloadsSurfaceSource from '@/components/Dashboard/Dashboard.tsx?raw';

describe('workloads page route shell', () => {
  it('keeps App routing on a page shell instead of an inline workloads view', () => {
    expect(appSource).toContain("const WorkloadsPage = lazy(() => import('./pages/Workloads'));");
    expect(appSource).toContain('<Route path={ROOT_WORKLOADS_PATH} component={WorkloadsPage} />');
    expect(appSource).not.toContain(
      'const WorkloadsView = () => <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />;',
    );
    expect(workloadsPageSource).toContain(
      "import { Dashboard as WorkloadsSurface } from '@/components/Dashboard/Dashboard';",
    );
    expect(workloadsPageSource).toContain(
      '<WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />',
    );
    expect(workloadsSurfaceSource).toContain("import { PageHeader } from '@/components/shared/PageHeader';");
    expect(workloadsSurfaceSource).toContain('<PageHeader');
    expect(workloadsSurfaceSource).toContain('title="Workloads"');
  });
});
