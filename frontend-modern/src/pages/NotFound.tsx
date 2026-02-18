import type { Component } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';

import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { DASHBOARD_PATH, buildRecoveryPath, buildInfrastructurePath } from '@/routing/resourceLinks';

const NotFound: Component = () => {
  const location = useLocation();
  const navigate = useNavigate();

  const path = () => `${location.pathname}${location.search || ''}`;
  const recoveryTarget = () => {
    const p = location.pathname || '';
    if (p.startsWith('/replication') || p.startsWith('/proxmox/replication')) {
      return buildRecoveryPath({ view: 'events', mode: 'remote' });
    }
    return buildRecoveryPath();
  };

  return (
    <Card padding="md">
      <EmptyState
        title="Page not found"
        description={`No route matched ${path()}.`}
        actions={
          <div class="flex flex-wrap items-center gap-2">
            <button
              type="button"
              class="inline-flex items-center gap-2 rounded-md border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 shadow-sm hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
              onClick={() => navigate(recoveryTarget())}
            >
              Go to Recovery
            </button>
            <button
              type="button"
              class="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-1.5 text-xs font-semibold text-white shadow-sm hover:bg-blue-700"
              onClick={() => navigate(buildInfrastructurePath())}
            >
              Go to Infrastructure
            </button>
            <button
              type="button"
              class="inline-flex items-center gap-2 rounded-md border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 shadow-sm hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
              onClick={() => navigate(DASHBOARD_PATH)}
            >
              Go to Dashboard
            </button>
          </div>
        }
      />
    </Card>
  );
};

export default NotFound;
