// Settings API Types - Keep in sync with Go backend

export interface ServerSettings {
  backend: {
    port: number;
    host: string;
  };
  frontend: {
    port: number;
    host: string;
  };
}

export interface MonitoringSettings {
  // Note: PVE polling is hardcoded to 10s server-side
  concurrentPolling: boolean;
  backupPollingCycles: number;
  backupPollingIntervalMs: number;
  backupPollingEnabled: boolean;
  metricsRetentionDays: number;
}

export interface LoggingSettings {
  level: string;
  file: string;
  maxSize: number;
  maxBackups: number;
  maxAge: number;
  compress: boolean;
}

export interface SecuritySettings {
  apiToken: string;
  allowedOrigins: string[];
  iframeEmbedding: string;
  enableAuthentication: boolean;
}

export interface Settings {
  server: ServerSettings;
  monitoring: MonitoringSettings;
  logging: LoggingSettings;
  security: SecuritySettings;
}

export interface SettingsCapabilities {
  canRestart: boolean;
  canValidatePorts: boolean;
  requiresRestart: boolean;
}

export interface SettingsResponse {
  current: Settings;
  defaults: Settings;
  capabilities: SettingsCapabilities;
}

export interface SettingsUpdateRequest {
  server?: Partial<ServerSettings>;
  monitoring?: Partial<MonitoringSettings>;
  logging?: Partial<LoggingSettings>;
  security?: Partial<SecuritySettings>;
}

export interface SettingsUpdateResponse {
  success: boolean;
  requiresRestart: boolean;
  message?: string;
}
