import type { Component } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';

import { Card } from '@/components/shared/Card';
import { Button } from '@/components/shared/Button';
import { PageHeader } from '@/components/shared/PageHeader';

const NotFound: Component = () => {
  const location = useLocation();
  const navigate = useNavigate();

  const path = () => `${location.pathname}${location.search || ''}`;

  return (
    <div class="flex min-h-[50vh] items-center justify-center px-4 py-8">
      <Card padding="lg" class="w-full max-w-md text-center">
        <div class="space-y-4">
          <div class="space-y-2">
            <PageHeader
              title="Page not found"
              description="We could not find that page in Pulse. Check the address or return to your workspace."
              descriptionVisibility="always"
              updateDocumentTitle={false}
              class="items-center! justify-center! text-center sm:flex-col!"
              titleClass="w-full text-center text-xl! font-semibold!"
              descriptionClass="text-sm! font-normal!"
            />
            <p data-testid="not-found-path" class="pt-1 font-mono text-xs break-all text-muted">
              No route matched {path()}.
            </p>
          </div>
          <div class="flex justify-center pt-1">
            <Button
              variant="primaryFlat"
              size="settingsActionXs"
              class="gap-2 font-semibold"
              onClick={() => navigate('/')}
            >
              Go to workspace
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
};

export default NotFound;
