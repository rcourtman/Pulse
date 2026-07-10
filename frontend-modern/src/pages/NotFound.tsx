import type { Component } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';

import { Card } from '@/components/shared/Card';
import { Button } from '@/components/shared/Button';
import { EmptyState } from '@/components/shared/EmptyState';
import { PageHeader } from '@/components/shared/PageHeader';

const NotFound: Component = () => {
  const location = useLocation();
  const navigate = useNavigate();

  const path = () => `${location.pathname}${location.search || ''}`;

  return (
    <div class="space-y-4">
      <PageHeader title="Page Not Found" description="The route you requested does not exist." />
      <Card padding="md">
        <EmptyState
          title="Page not found"
          description={`No route matched ${path()}.`}
          actions={
            <div class="flex flex-wrap items-center gap-2">
              <Button
                variant="primaryFlat"
                size="settingsActionXs"
                class="gap-2 font-semibold"
                onClick={() => navigate('/')}
              >
                Go to workspace
              </Button>
            </div>
          }
        />
      </Card>
    </div>
  );
};

export default NotFound;
