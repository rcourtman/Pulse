import { Component, JSX, ErrorBoundary as SolidErrorBoundary } from 'solid-js';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';

import { Button } from '@/components/shared/Button';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { logError } from '@/utils/logger';
import { SectionHeader } from '@/components/shared/SectionHeader';

interface ErrorBoundaryProps {
  children: JSX.Element;
  fallback?: (error: Error, reset: () => void) => JSX.Element;
  onError?: (error: Error) => void;
}

const DefaultErrorFallback: Component<{ error: Error; reset: () => void }> = (props) => {
  return (
    <div class="min-h-screen flex items-center justify-center bg-base p-4">
      <div class="max-w-md w-full bg-surface rounded-md shadow-sm p-6">
        <div class="flex items-center mb-4">
          <AlertTriangleIcon class="mr-3 h-12 w-12 text-red-500" aria-hidden="true" />
          <div>
            <SectionHeader
              title="Something went wrong"
              description="An unexpected error occurred"
              size="md"
              titleClass="text-base-content"
              descriptionClass="text-sm text-muted"
            />
          </div>
        </div>

        <CalloutCard
          tone="danger"
          scale="compact"
          padding="sm"
          class="mb-4"
          description="Please try again or reload the page. If the problem persists, contact your administrator."
        />

        <div class="flex gap-2">
          <Button onClick={props.reset} variant="primary" size="md" class="flex-1">
            Try Again
          </Button>
          <Button
            onClick={() => window.location.reload()}
            variant="secondary"
            size="md"
            class="flex-1"
          >
            Reload Page
          </Button>
        </div>

        <div class="mt-4 text-xs text-muted leading-relaxed">
          Technical details are suppressed in this view. Check server logs for full context.
        </div>
      </div>
    </div>
  );
};

export const ErrorBoundary: Component<ErrorBoundaryProps> = (props) => {
  return (
    <SolidErrorBoundary
      fallback={(error, reset) => {
        // Log the error
        logError('Error boundary caught error', error);

        // Call custom error handler if provided
        if (props.onError) {
          props.onError(error);
        }

        // Render custom or default fallback
        if (props.fallback) {
          return props.fallback(error, reset);
        }

        return <DefaultErrorFallback error={error} reset={reset} />;
      }}
    >
      {props.children}
    </SolidErrorBoundary>
  );
};

// Route-level error boundary — keeps the app shell (sidebar, header, nav) intact
// when a page component crashes. Renders inline within the content area.
export const RouteErrorBoundary: Component<{ children: JSX.Element }> = (props) => {
  return (
    <ErrorBoundary
      fallback={(_error, reset) => (
        <div class="flex min-h-[60vh] items-center justify-center p-4">
          <div class="max-w-md w-full bg-surface rounded-md shadow-sm p-6">
            <div class="flex items-center mb-4">
              <AlertTriangleIcon class="mr-3 h-10 w-10 text-red-500" aria-hidden="true" />
              <SectionHeader
                title="This page couldn't load"
                description="An unexpected error occurred"
                size="md"
                titleClass="text-base-content"
                descriptionClass="text-sm text-muted"
              />
            </div>
            <CalloutCard
              tone="danger"
              scale="compact"
              padding="sm"
              class="mb-4"
              description="Try again, or navigate to another page using the sidebar."
            />
            <Button onClick={reset} variant="primary" size="md" class="w-full">
              Try Again
            </Button>
          </div>
        </div>
      )}
    >
      {props.children}
    </ErrorBoundary>
  );
};

// Component-specific error boundary with more context
export const ComponentErrorBoundary: Component<{
  name: string;
  children: JSX.Element;
}> = (props) => {
  return (
    <ErrorBoundary
      fallback={(error, reset) => (
        <CalloutCard
          tone="danger"
          scale="compact"
          padding="md"
          icon={<AlertTriangleIcon class="h-5 w-5" aria-hidden="true" />}
          title={`Error in ${props.name}`}
          description={error.message}
        >
          <Button onClick={reset} variant="danger" size="xs" class="mt-1">
            Retry
          </Button>
        </CalloutCard>
      )}
      onError={(error) => {
        logError(`Error in component ${props.name}`, error);
      }}
    >
      {props.children}
    </ErrorBoundary>
  );
};
