import { Component, JSX, ErrorBoundary as SolidErrorBoundary } from 'solid-js';
import { logError } from '@/utils/logger';
import { SectionHeader } from '@/components/shared/SectionHeader';

interface ErrorBoundaryProps {
  children: JSX.Element;
  fallback?: (error: Error, reset: () => void) => JSX.Element;
  onError?: (error: Error) => void;
}

const DefaultErrorFallback: Component<{ error: Error; reset: () => void }> = (props) => {
  return (
    <div class="min-h-screen flex items-center justify-center bg-slate-100 dark:bg-slate-900 p-4">
      <div class="max-w-md w-full bg-white dark:bg-slate-800 rounded-md shadow-sm p-6">
        <div class="flex items-center mb-4">
          <svg
            class="w-12 h-12 text-red-500 mr-3"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
            />
          </svg>
          <div>
            <SectionHeader
              title="Something went wrong"
              description="An unexpected error occurred"
              size="md"
              titleClass="text-slate-900 dark:text-slate-100"
              descriptionClass="text-sm text-slate-600 dark:text-slate-400"
            />
          </div>
        </div>

        <div class="bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 rounded p-3 mb-4">
          <p class="text-sm text-red-800 dark:text-red-200">
            Please try again or reload the page. If the problem persists, contact your administrator.
          </p>
        </div>

        <div class="flex gap-2">
          <button
            type="button"
            onClick={props.reset}
            class="flex-1 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
          >
            Try Again
          </button>
          <button
            type="button"
            onClick={() => window.location.reload()}
            class="flex-1 px-4 py-2 bg-slate-600 text-white rounded hover:bg-slate-700 transition-colors"
          >
            Reload Page
          </button>
        </div>

        <div class="mt-4 text-xs text-slate-500 dark:text-slate-400 leading-relaxed">
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

// Component-specific error boundary with more context
export const ComponentErrorBoundary: Component<{
  name: string;
  children: JSX.Element;
}> = (props) => {
  return (
    <ErrorBoundary
      fallback={(error, reset) => (
        <div class="p-4 bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 rounded">
          <div class="flex items-center mb-2">
            <svg
              class="w-5 h-5 text-red-500 mr-2"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <SectionHeader
              title={`Error in ${props.name}`}
              size="sm"
              titleClass="text-red-800 dark:text-red-200"
            />
          </div>
          <p class="text-xs text-red-700 dark:text-red-300 mb-2">{error.message}</p>
          <button
            type="button"
            onClick={reset}
            class="text-xs px-2 py-1 bg-red-600 text-white rounded hover:bg-red-700 transition-colors"
          >
            Retry
          </button>
        </div>
      )}
      onError={(error) => {
        logError(`Error in component ${props.name}`, error);
      }}
    >
      {props.children}
    </ErrorBoundary>
  );
};
