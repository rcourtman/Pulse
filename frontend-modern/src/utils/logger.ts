import { LOG_LEVELS, type LogLevel } from '@/constants';

interface LogEntry {
  level: LogLevel;
  timestamp: Date;
  message: string;
  data?: any;
  stack?: string;
}

class Logger {
  private static instance: Logger;
  private logLevel: LogLevel = 'INFO';
  private logs: LogEntry[] = [];
  private maxLogs = 1000;
  private isDevelopment = import.meta.env.DEV;

  private constructor() {
    // Set log level from environment or localStorage
    const savedLevel = localStorage.getItem('logLevel') as LogLevel;
    if (savedLevel && savedLevel in LOG_LEVELS) {
      this.logLevel = savedLevel;
    } else if (this.isDevelopment) {
      this.logLevel = 'DEBUG';
    }
  }

  static getInstance(): Logger {
    if (!Logger.instance) {
      Logger.instance = new Logger();
    }
    return Logger.instance;
  }

  setLevel(level: LogLevel): void {
    this.logLevel = level;
    localStorage.setItem('logLevel', level);
  }

  getLevel(): LogLevel {
    return this.logLevel;
  }

  private shouldLog(level: LogLevel): boolean {
    return LOG_LEVELS[level] >= LOG_LEVELS[this.logLevel];
  }

  private formatMessage(level: LogLevel, message: string, data?: any): string {
    const timestamp = new Date().toISOString();
    const prefix = `[${timestamp}] [${level}]`;
    
    if (data !== undefined) {
      return `${prefix} ${message} ${JSON.stringify(data, null, 2)}`;
    }
    return `${prefix} ${message}`;
  }

  private addToHistory(entry: LogEntry): void {
    this.logs.push(entry);
    if (this.logs.length > this.maxLogs) {
      this.logs.shift();
    }
  }

  private log(level: LogLevel, message: string, data?: any): void {
    if (!this.shouldLog(level)) return;

    const entry: LogEntry = {
      level,
      timestamp: new Date(),
      message,
      data,
    };

    // Add to history
    this.addToHistory(entry);

    // Console output
    const formattedMessage = this.formatMessage(level, message, data);
    
    switch (level) {
      case 'DEBUG':
        console.debug(formattedMessage);
        break;
      case 'INFO':
        console.info(formattedMessage);
        break;
      case 'WARN':
        console.warn(formattedMessage);
        break;
      case 'ERROR':
        console.error(formattedMessage);
        if (data instanceof Error) {
          entry.stack = data.stack;
          console.error(data.stack);
        }
        break;
    }
  }

  debug(message: string, data?: any): void {
    this.log('DEBUG', message, data);
  }

  info(message: string, data?: any): void {
    this.log('INFO', message, data);
  }

  warn(message: string, data?: any): void {
    this.log('WARN', message, data);
  }

  error(message: string, error?: Error | any): void {
    this.log('ERROR', message, error);
  }

  // Performance timing helper
  time(label: string): () => void {
    const start = performance.now();
    this.debug(`[TIMER] ${label} started`);
    
    return () => {
      const end = performance.now();
      const duration = end - start;
      this.debug(`[TIMER] ${label} completed`, { duration: `${duration.toFixed(2)}ms` });
    };
  }

  // Group related logs
  group(label: string): () => void {
    if (this.shouldLog('DEBUG')) {
      console.group(label);
    }
    
    return () => {
      if (this.shouldLog('DEBUG')) {
        console.groupEnd();
      }
    };
  }

  // Get log history
  getHistory(level?: LogLevel): LogEntry[] {
    if (!level) return [...this.logs];
    
    const minLevel = LOG_LEVELS[level];
    return this.logs.filter(log => LOG_LEVELS[log.level] >= minLevel);
  }

  // Clear log history
  clearHistory(): void {
    this.logs = [];
  }

  // Export logs as CSV
  exportLogs(level?: LogLevel): string {
    const logs = this.getHistory(level);
    const headers = ['Timestamp', 'Level', 'Message', 'Data'];
    const rows = logs.map(log => [
      log.timestamp.toISOString(),
      log.level,
      log.message,
      log.data ? JSON.stringify(log.data) : '',
    ]);
    
    return [
      headers.join(','),
      ...rows.map(row => row.map(cell => `"${cell.replace(/"/g, '""')}"`).join(',')),
    ].join('\n');
  }

  // Network request logging
  logRequest(method: string, url: string, options?: any): void {
    this.debug(`[API] ${method} ${url}`, options);
  }

  logResponse(method: string, url: string, status: number, duration: number): void {
    const level = status >= 400 ? 'ERROR' : 'DEBUG';
    this.log(level, `[API] ${method} ${url} - ${status}`, { duration: `${duration}ms` });
  }

  // WebSocket logging
  logWebSocket(event: 'connect' | 'disconnect' | 'message' | 'error', data?: any): void {
    const level = event === 'error' ? 'ERROR' : 'DEBUG';
    this.log(level, `[WebSocket] ${event}`, data);
  }

  // Component lifecycle logging
  logComponent(component: string, event: 'mount' | 'unmount' | 'update', data?: any): void {
    this.debug(`[Component] ${component} - ${event}`, data);
  }

  // Feature usage tracking
  trackFeature(feature: string, action: string, data?: any): void {
    this.info(`[Feature] ${feature} - ${action}`, data);
  }
}

// Export singleton instance
export const logger = Logger.getInstance();

// Export only the used convenience function
export const logError = (message: string, error?: Error | any) => logger.error(message, error);