import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

/* ------------------------------------------------------------------ */
/*  Mocks                                                              */
/* ------------------------------------------------------------------ */

const logErrorMock = vi.hoisted(() => vi.fn());

vi.mock('@/utils/logger', () => ({
  logError: logErrorMock,
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

/** Component that throws on render to trigger error boundaries. */
function ThrowingComponent(props: { error?: Error }): JSX.Element {
  const err = props.error ?? new Error('Test explosion');
  throw err;
}

/** Simple child that renders normally. */
function GoodChild(): JSX.Element {
  return <div data-testid="good-child">All good</div>;
}

/**
 * Component that throws on first render but succeeds after reset.
 * Uses a module-level flag so that after error boundary resets,
 * the re-render succeeds.
 */
let throwOnceFlag = true;
function ThrowOnceComponent(): JSX.Element {
  if (throwOnceFlag) {
    throwOnceFlag = false;
    throw new Error('First render fails');
  }
  return <div data-testid="recovered">Recovered successfully</div>;
}

/* ------------------------------------------------------------------ */
/*  Tests — ErrorBoundary                                              */
/* ------------------------------------------------------------------ */

describe('ErrorBoundary', () => {
  beforeEach(() => {
    logErrorMock.mockReset();
  });

  afterEach(cleanup);

  async function importComponent() {
    return await import('../ErrorBoundary');
  }

  /* ---------- Happy path: renders children when no error ---------- */

  it('renders children when no error is thrown', async () => {
    const { ErrorBoundary } = await importComponent();

    render(() => (
      <ErrorBoundary>
        <GoodChild />
      </ErrorBoundary>
    ));

    expect(screen.getByTestId('good-child')).toBeInTheDocument();
    expect(screen.getByText('All good')).toBeInTheDocument();
  });

  /* ---------- Default fallback on error ---------- */

  it('renders default fallback UI when a child throws', async () => {
    const { ErrorBoundary } = await importComponent();

    render(() => (
      <ErrorBoundary>
        <ThrowingComponent />
      </ErrorBoundary>
    ));

    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    expect(screen.getByText('An unexpected error occurred')).toBeInTheDocument();
    expect(screen.getByText('Try Again')).toBeInTheDocument();
    expect(screen.getByText('Reload Page')).toBeInTheDocument();
    expect(screen.getByText(/Technical details are suppressed/)).toBeInTheDocument();
  });

  /* ---------- Logs error via logError ---------- */

  it('calls logError exactly once when a child throws', async () => {
    const { ErrorBoundary } = await importComponent();

    render(() => (
      <ErrorBoundary>
        <ThrowingComponent />
      </ErrorBoundary>
    ));

    expect(logErrorMock).toHaveBeenCalledTimes(1);
    expect(logErrorMock).toHaveBeenCalledWith('Error boundary caught error', expect.any(Error));
  });

  /* ---------- Custom onError callback ---------- */

  it('calls custom onError handler when provided', async () => {
    const { ErrorBoundary } = await importComponent();
    const onError = vi.fn();

    render(() => (
      <ErrorBoundary onError={onError}>
        <ThrowingComponent />
      </ErrorBoundary>
    ));

    expect(onError).toHaveBeenCalledTimes(1);
    expect(onError).toHaveBeenCalledWith(expect.any(Error));
  });

  /* ---------- Custom fallback ---------- */

  it('renders custom fallback when provided', async () => {
    const { ErrorBoundary } = await importComponent();

    render(() => (
      <ErrorBoundary
        fallback={(error, reset) => (
          <div data-testid="custom-fallback">
            <span>Custom: {error.message}</span>
            <button onClick={reset}>Custom Reset</button>
          </div>
        )}
      >
        <ThrowingComponent error={new Error('kaboom')} />
      </ErrorBoundary>
    ));

    expect(screen.getByTestId('custom-fallback')).toBeInTheDocument();
    expect(screen.getByText('Custom: kaboom')).toBeInTheDocument();
    expect(screen.getByText('Custom Reset')).toBeInTheDocument();
    // Default fallback should NOT appear
    expect(screen.queryByText('Something went wrong')).not.toBeInTheDocument();
  });

  /* ---------- Custom fallback reset actually recovers ---------- */

  it('custom fallback reset recovers the component tree', async () => {
    const { ErrorBoundary } = await importComponent();

    // Reset the throw-once flag so ThrowOnceComponent throws first, succeeds after reset
    throwOnceFlag = true;

    render(() => (
      <ErrorBoundary
        fallback={(_error, reset) => (
          <button data-testid="custom-reset-btn" onClick={reset}>
            Reset
          </button>
        )}
      >
        <ThrowOnceComponent />
      </ErrorBoundary>
    ));

    // Should be in error state
    expect(screen.getByTestId('custom-reset-btn')).toBeInTheDocument();
    expect(screen.queryByTestId('recovered')).not.toBeInTheDocument();

    // Click reset — component should recover
    fireEvent.click(screen.getByTestId('custom-reset-btn'));
    expect(screen.getByTestId('recovered')).toBeInTheDocument();
    expect(screen.queryByTestId('custom-reset-btn')).not.toBeInTheDocument();
  });

  /* ---------- Reload Page calls window.location.reload ---------- */

  it('Reload Page button calls window.location.reload', async () => {
    const { ErrorBoundary } = await importComponent();

    const reloadMock = vi.fn();
    const originalLocation = window.location;
    // jsdom marks location.reload as non-configurable, so replace the whole object
    Object.defineProperty(window, 'location', {
      value: { ...originalLocation, reload: reloadMock },
      writable: true,
      configurable: true,
    });

    try {
      render(() => (
        <ErrorBoundary>
          <ThrowingComponent />
        </ErrorBoundary>
      ));

      fireEvent.click(screen.getByText('Reload Page'));
      expect(reloadMock).toHaveBeenCalledTimes(1);
    } finally {
      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
        configurable: true,
      });
    }
  });

  /* ---------- Try Again button triggers reset and recovers ---------- */

  it('Try Again button resets the error boundary and recovers', async () => {
    const { ErrorBoundary } = await importComponent();

    throwOnceFlag = true;

    render(() => (
      <ErrorBoundary>
        <ThrowOnceComponent />
      </ErrorBoundary>
    ));

    // Should be showing default error fallback
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    expect(screen.queryByTestId('recovered')).not.toBeInTheDocument();

    // Click "Try Again" — should recover
    fireEvent.click(screen.getByText('Try Again'));
    expect(screen.getByTestId('recovered')).toBeInTheDocument();
    expect(screen.queryByText('Something went wrong')).not.toBeInTheDocument();
  });

  /* ---------- Combined onError + custom fallback ---------- */

  it('fires onError even when a custom fallback is provided', async () => {
    const { ErrorBoundary } = await importComponent();
    const onError = vi.fn();

    render(() => (
      <ErrorBoundary
        onError={onError}
        fallback={(error) => <div data-testid="custom">Custom: {error.message}</div>}
      >
        <ThrowingComponent error={new Error('combined test')} />
      </ErrorBoundary>
    ));

    // Custom fallback should render
    expect(screen.getByTestId('custom')).toBeInTheDocument();
    expect(screen.getByText('Custom: combined test')).toBeInTheDocument();

    // onError should still have been called
    expect(onError).toHaveBeenCalledTimes(1);
    expect(onError).toHaveBeenCalledWith(expect.objectContaining({ message: 'combined test' }));

    // logError should also have fired
    expect(logErrorMock).toHaveBeenCalledTimes(1);
  });

  /* ---------- No onError when not provided ---------- */

  it('does not throw when onError is not provided', async () => {
    const { ErrorBoundary } = await importComponent();

    // Should not throw even without onError
    expect(() => {
      render(() => (
        <ErrorBoundary>
          <ThrowingComponent />
        </ErrorBoundary>
      ));
    }).not.toThrow();

    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
  });
});

/* ------------------------------------------------------------------ */
/*  Tests — ComponentErrorBoundary                                     */
/* ------------------------------------------------------------------ */

describe('ComponentErrorBoundary', () => {
  beforeEach(() => {
    logErrorMock.mockReset();
  });

  afterEach(cleanup);

  async function importComponent() {
    return await import('../ErrorBoundary');
  }

  /* ---------- Happy path: renders children ---------- */

  it('renders children when no error is thrown', async () => {
    const { ComponentErrorBoundary } = await importComponent();

    render(() => (
      <ComponentErrorBoundary name="TestWidget">
        <GoodChild />
      </ComponentErrorBoundary>
    ));

    expect(screen.getByTestId('good-child')).toBeInTheDocument();
  });

  /* ---------- Shows component name in error UI ---------- */

  it('displays the component name in the error fallback', async () => {
    const { ComponentErrorBoundary } = await importComponent();

    render(() => (
      <ComponentErrorBoundary name="Dashboard">
        <ThrowingComponent />
      </ComponentErrorBoundary>
    ));

    expect(screen.getByText('Error in Dashboard')).toBeInTheDocument();
  });

  /* ---------- Shows error message ---------- */

  it('displays the error message in the fallback', async () => {
    const { ComponentErrorBoundary } = await importComponent();

    render(() => (
      <ComponentErrorBoundary name="Widget">
        <ThrowingComponent error={new Error('socket timeout')} />
      </ComponentErrorBoundary>
    ));

    expect(screen.getByText('socket timeout')).toBeInTheDocument();
  });

  /* ---------- Has Retry button ---------- */

  it('shows a Retry button in the error fallback', async () => {
    const { ComponentErrorBoundary } = await importComponent();

    render(() => (
      <ComponentErrorBoundary name="Chart">
        <ThrowingComponent />
      </ComponentErrorBoundary>
    ));

    expect(screen.getByText('Retry')).toBeInTheDocument();
  });

  /* ---------- Logs error with component name ---------- */

  it('calls logError exactly twice: base + component-specific', async () => {
    const { ComponentErrorBoundary } = await importComponent();

    render(() => (
      <ComponentErrorBoundary name="MetricsPanel">
        <ThrowingComponent />
      </ComponentErrorBoundary>
    ));

    // Exactly 2 calls: base ErrorBoundary log + ComponentErrorBoundary onError log
    expect(logErrorMock).toHaveBeenCalledTimes(2);
    expect(logErrorMock).toHaveBeenCalledWith('Error boundary caught error', expect.any(Error));
    expect(logErrorMock).toHaveBeenCalledWith('Error in component MetricsPanel', expect.any(Error));
  });

  /* ---------- Retry button recovers the component ---------- */

  it('Retry button resets the error boundary and recovers', async () => {
    const { ComponentErrorBoundary } = await importComponent();

    throwOnceFlag = true;

    render(() => (
      <ComponentErrorBoundary name="RecoverWidget">
        <ThrowOnceComponent />
      </ComponentErrorBoundary>
    ));

    // Should be showing compact error fallback
    expect(screen.getByText('Error in RecoverWidget')).toBeInTheDocument();
    expect(screen.queryByTestId('recovered')).not.toBeInTheDocument();

    // Click "Retry" — should recover
    fireEvent.click(screen.getByText('Retry'));
    expect(screen.getByTestId('recovered')).toBeInTheDocument();
    expect(screen.queryByText('Error in RecoverWidget')).not.toBeInTheDocument();
  });

  /* ---------- Does not show default full-page fallback ---------- */

  it('renders compact inline fallback, not the full-page default', async () => {
    const { ComponentErrorBoundary } = await importComponent();

    render(() => (
      <ComponentErrorBoundary name="Sidebar">
        <ThrowingComponent />
      </ComponentErrorBoundary>
    ));

    // Should NOT show the full-page default fallback elements
    expect(screen.queryByText('Something went wrong')).not.toBeInTheDocument();
    expect(screen.queryByText('Reload Page')).not.toBeInTheDocument();
    expect(screen.queryByText('Try Again')).not.toBeInTheDocument();

    // Should show the compact inline fallback
    expect(screen.getByText('Error in Sidebar')).toBeInTheDocument();
    expect(screen.getByText('Retry')).toBeInTheDocument();
  });
});
