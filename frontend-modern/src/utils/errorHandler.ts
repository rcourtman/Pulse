import { logger } from './logger';

export interface ErrorContext {
  component?: string;
  action?: string;
  data?: unknown;
}

export class AppError extends Error {
  public readonly context: ErrorContext;
  public readonly isOperational: boolean;

  constructor(message: string, context: ErrorContext = {}, isOperational = true) {
    super(message);
    this.name = 'AppError';
    this.context = context;
    this.isOperational = isOperational;
  }
}

export function handleError(error: unknown, context: ErrorContext = {}): void {
  if (error instanceof AppError) {
    logger.error(`[${context.component || 'Unknown'}] ${error.message}`, {
      ...error.context,
      ...context,
      error: error.stack,
    });
  } else if (error instanceof Error) {
    logger.error(`[${context.component || 'Unknown'}] ${error.message}`, {
      ...context,
      error: error.stack,
    });
  } else {
    logger.error(`[${context.component || 'Unknown'}] Unknown error`, {
      ...context,
      error: String(error),
    });
  }
}

export function handleAsyncError<T>(promise: Promise<T>, context: ErrorContext = {}): Promise<T> {
  return promise.catch((error) => {
    handleError(error, context);
    throw error;
  });
}

export function createErrorBoundary(component: string) {
  return (action: string) => (error: unknown) => {
    handleError(error, { component, action });
  };
}
