import { apiFetch, apiFetchJSON } from '@/utils/apiClient';
import {
  assertAPIResponseOK,
  assertAPIResponseOKOrAllowedStatus,
  assertAPIResponseOKOrThrowStatus,
  parseRequiredAPIResponse,
  withAPIErrorStatusFallback,
  withAPIErrorStatusNull,
} from './responseUtils';

/**
 * Agent profile for centralized configuration management.
 */
export interface AgentProfile {
  id: string;
  name: string;
  description?: string;
  config: Record<string, unknown>;
  version?: number;
  created_at: string;
  updated_at: string;
}

/**
 * Assignment linking an agent to a profile.
 */
export interface AgentProfileAssignment {
  agent_id: string;
  profile_id: string;
  updated_at: string;
}

/**
 * Request for AI-assisted profile suggestion.
 */
export interface ProfileSuggestionRequest {
  prompt: string;
}

/**
 * AI-generated profile suggestion.
 */
export interface ProfileSuggestion {
  name: string;
  description: string;
  config: Record<string, unknown>;
  rationale: string[];
}

export interface ConfigKeyDefinition {
  key: string;
  type: string;
  description: string;
  defaultValue?: unknown;
  required: boolean;
  min?: number;
  max?: number;
  pattern?: string;
  enum?: string[];
}

export interface ConfigValidationError {
  key: string;
  message: string;
}

export interface ConfigValidationResult {
  valid: boolean;
  errors: ConfigValidationError[];
  warnings: ConfigValidationError[];
}

export const MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE =
  'Selected profile no longer exists. Refresh and choose another profile.';
export const INVALID_AGENT_PROFILE_LIST_MESSAGE =
  'Invalid agent profile list response from Pulse.';
export const INVALID_AGENT_PROFILE_ASSIGNMENT_LIST_MESSAGE =
  'Invalid agent profile assignment list response from Pulse.';
export const INVALID_AGENT_PROFILE_RESPONSE_MESSAGE =
  'Invalid agent profile response from Pulse.';
export const INVALID_AGENT_PROFILE_SUGGESTION_MESSAGE =
  'Invalid agent profile suggestion response from Pulse.';
export const INVALID_AGENT_PROFILE_SCHEMA_MESSAGE =
  'Invalid agent profile schema response from Pulse.';
export const INVALID_AGENT_PROFILE_VALIDATION_MESSAGE =
  'Invalid agent profile validation response from Pulse.';

type ConfigKeyDefinitionResponse = {
  Key: string;
  Type: string;
  Description: string;
  Default: unknown;
  Required: boolean;
  Min?: number;
  Max?: number;
  Pattern?: string;
  Enum?: string[];
};

type ConfigValidationErrorResponse = {
  Key: string;
  Message: string;
};

type ConfigValidationResultResponse = {
  Valid: boolean;
  Errors?: ConfigValidationErrorResponse[];
  Warnings?: ConfigValidationErrorResponse[];
};

/**
 * API client for agent profiles (Pro feature).
 * Endpoints are gated behind license - returns 402 if not licensed.
 */
export class AgentProfilesAPI {
  private static baseUrl = '/api/admin/profiles';

  private static isRecord(value: unknown): value is Record<string, unknown> {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
  }

  private static requireStringField(
    record: Record<string, unknown>,
    field: string,
    invalidMessage: string,
  ): string {
    if (typeof record[field] !== 'string') {
      throw new Error(invalidMessage);
    }
    return record[field] as string;
  }

  private static requireOptionalStringField(
    record: Record<string, unknown>,
    field: string,
    invalidMessage: string,
  ): string | undefined {
    const value = record[field];
    if (value === undefined || value === null) {
      return undefined;
    }
    if (typeof value !== 'string') {
      throw new Error(invalidMessage);
    }
    return value;
  }

  private static requireOptionalNumberField(
    record: Record<string, unknown>,
    field: string,
    invalidMessage: string,
  ): number | undefined {
    const value = record[field];
    if (value === undefined || value === null) {
      return undefined;
    }
    if (typeof value !== 'number' || !Number.isFinite(value)) {
      throw new Error(invalidMessage);
    }
    return value;
  }

  private static requireArrayResponse<T>(
    response: unknown,
    invalidMessage: string,
  ): T[] {
    if (!Array.isArray(response)) {
      throw new Error(invalidMessage);
    }
    return response as T[];
  }

  private static requireAgentProfileResponse(
    response: unknown,
    invalidMessage: string,
  ): AgentProfile {
    if (!this.isRecord(response)) {
      throw new Error(invalidMessage);
    }
    if (!this.isRecord(response.config)) {
      throw new Error(invalidMessage);
    }

    return {
      id: this.requireStringField(response, 'id', invalidMessage),
      name: this.requireStringField(response, 'name', invalidMessage),
      description: this.requireOptionalStringField(response, 'description', invalidMessage),
      config: response.config as Record<string, unknown>,
      version: this.requireOptionalNumberField(response, 'version', invalidMessage),
      created_at: this.requireStringField(response, 'created_at', invalidMessage),
      updated_at: this.requireStringField(response, 'updated_at', invalidMessage),
    };
  }

  private static requireAgentProfileAssignmentResponse(
    response: unknown,
    invalidMessage: string,
  ): AgentProfileAssignment {
    if (!this.isRecord(response)) {
      throw new Error(invalidMessage);
    }

    return {
      agent_id: this.requireStringField(response, 'agent_id', invalidMessage),
      profile_id: this.requireStringField(response, 'profile_id', invalidMessage),
      updated_at: this.requireStringField(response, 'updated_at', invalidMessage),
    };
  }

  private static requireConfigValidationItems(
    value: unknown,
    invalidMessage: string,
  ): ConfigValidationError[] {
    return this.requireArrayResponse<unknown>(value, invalidMessage).map((item) => {
      if (!this.isRecord(item)) {
        throw new Error(invalidMessage);
      }
      return {
        key: this.requireStringField(item, 'Key', invalidMessage),
        message: this.requireStringField(item, 'Message', invalidMessage),
      };
    });
  }

  /**
   * List all agent profiles.
   */
  static async listProfiles(): Promise<AgentProfile[]> {
    const response = await withAPIErrorStatusFallback<AgentProfile[]>(
      apiFetchJSON<AgentProfile[]>(`${this.baseUrl}/`),
      402,
      [],
    );
    return this.requireArrayResponse<unknown>(response, INVALID_AGENT_PROFILE_LIST_MESSAGE).map(
      (profile) =>
        this.requireAgentProfileResponse(profile, INVALID_AGENT_PROFILE_LIST_MESSAGE),
    );
  }

  /**
   * Get a single profile by ID.
   */
  static async getProfile(id: string): Promise<AgentProfile | null> {
    const response = await withAPIErrorStatusNull<AgentProfile>(
      apiFetchJSON<AgentProfile>(`${this.baseUrl}/${encodeURIComponent(id)}`),
      404,
    );
    if (response === null) {
      return null;
    }
    return this.requireAgentProfileResponse(response, INVALID_AGENT_PROFILE_RESPONSE_MESSAGE);
  }

  /**
   * Create a new profile.
   */
  static async createProfile(
    name: string,
    config: Record<string, unknown>,
    description?: string,
  ): Promise<AgentProfile> {
    const response = await apiFetch(`${this.baseUrl}/`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, description, config }),
    });

    const parsed = await parseRequiredAPIResponse(
      response,
      `Failed to create profile: ${response.status}`,
      'Failed to parse created profile',
    );
    return this.requireAgentProfileResponse(parsed, INVALID_AGENT_PROFILE_RESPONSE_MESSAGE);
  }

  /**
   * Update an existing profile.
   */
  static async updateProfile(
    id: string,
    name: string,
    config: Record<string, unknown>,
    description?: string,
  ): Promise<AgentProfile> {
    const response = await apiFetch(`${this.baseUrl}/${encodeURIComponent(id)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id, name, description, config }),
    });

    const parsed = await parseRequiredAPIResponse(
      response,
      `Failed to update profile: ${response.status}`,
      'Failed to parse updated profile',
    );
    return this.requireAgentProfileResponse(parsed, INVALID_AGENT_PROFILE_RESPONSE_MESSAGE);
  }

  /**
   * Delete a profile.
   */
  static async deleteProfile(id: string): Promise<void> {
    const response = await apiFetch(`${this.baseUrl}/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });

    await assertAPIResponseOKOrAllowedStatus(
      response,
      204,
      `Failed to delete profile: ${response.status}`,
    );
  }

  /**
   * List all profile assignments.
   */
  static async listAssignments(): Promise<AgentProfileAssignment[]> {
    const response = await withAPIErrorStatusFallback<AgentProfileAssignment[]>(
      apiFetchJSON<AgentProfileAssignment[]>(`${this.baseUrl}/assignments`),
      402,
      [],
    );
    return this.requireArrayResponse<unknown>(
      response,
      INVALID_AGENT_PROFILE_ASSIGNMENT_LIST_MESSAGE,
    ).map((assignment) =>
      this.requireAgentProfileAssignmentResponse(
        assignment,
        INVALID_AGENT_PROFILE_ASSIGNMENT_LIST_MESSAGE,
      ),
    );
  }

  /**
   * Assign a profile to an agent.
   */
  static async assignProfile(agentId: string, profileId: string): Promise<void> {
    const response = await apiFetch(`${this.baseUrl}/assignments`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ agent_id: agentId, profile_id: profileId }),
    });

    await assertAPIResponseOKOrThrowStatus(
      response,
      404,
      MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE,
      `Failed to assign profile: ${response.status}`,
    );
  }

  /**
   * Remove profile assignment from an agent.
   */
  static async unassignProfile(agentId: string): Promise<void> {
    const response = await apiFetch(`${this.baseUrl}/assignments/${encodeURIComponent(agentId)}`, {
      method: 'DELETE',
    });

    await assertAPIResponseOKOrAllowedStatus(
      response,
      204,
      `Failed to unassign profile: ${response.status}`,
    );
  }

  /**
   * Get AI-assisted profile suggestion.
   * Requires AI to be enabled and running.
   */
  static async suggestProfile(request: ProfileSuggestionRequest): Promise<ProfileSuggestion> {
    const response = await apiFetch(`${this.baseUrl}/suggestions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    });

    await assertAPIResponseOKOrThrowStatus(
      response,
      503,
      'Pulse Assistant service is not available. Please check Pulse Assistant settings.',
      `Failed to get suggestion: ${response.status}`,
    );

    const parsed = await parseRequiredAPIResponse(
      response,
      `Failed to get suggestion: ${response.status}`,
      'Failed to parse profile suggestion',
    );
    if (!this.isRecord(parsed) || !this.isRecord(parsed.config)) {
      throw new Error(INVALID_AGENT_PROFILE_SUGGESTION_MESSAGE);
    }
    return {
      name: this.requireStringField(parsed, 'name', INVALID_AGENT_PROFILE_SUGGESTION_MESSAGE),
      description: this.requireStringField(
        parsed,
        'description',
        INVALID_AGENT_PROFILE_SUGGESTION_MESSAGE,
      ),
      config: parsed.config as Record<string, unknown>,
      rationale: this.requireArrayResponse<unknown>(
        parsed.rationale,
        INVALID_AGENT_PROFILE_SUGGESTION_MESSAGE,
      ).map((entry) => {
        if (typeof entry !== 'string') {
          throw new Error(INVALID_AGENT_PROFILE_SUGGESTION_MESSAGE);
        }
        return entry;
      }),
    };
  }

  /**
   * Fetch config schema definitions for agent profiles.
   */
  static async getConfigSchema(): Promise<ConfigKeyDefinition[]> {
    const response = await apiFetch(`${this.baseUrl}/schema`);
    await assertAPIResponseOK(response, 'Failed to fetch profile schema');
    const defs = this.requireArrayResponse<ConfigKeyDefinitionResponse>(
      await response.json(),
      INVALID_AGENT_PROFILE_SCHEMA_MESSAGE,
    );
    return defs.map((def) => {
      if (!this.isRecord(def)) {
        throw new Error(INVALID_AGENT_PROFILE_SCHEMA_MESSAGE);
      }

      return {
        key: this.requireStringField(def, 'Key', INVALID_AGENT_PROFILE_SCHEMA_MESSAGE),
        type: this.requireStringField(def, 'Type', INVALID_AGENT_PROFILE_SCHEMA_MESSAGE),
        description: this.requireStringField(def, 'Description', INVALID_AGENT_PROFILE_SCHEMA_MESSAGE),
        defaultValue: def.Default,
        required: (() => {
          const value = def.Required;
          if (typeof value !== 'boolean') {
            throw new Error(INVALID_AGENT_PROFILE_SCHEMA_MESSAGE);
          }
          return value;
        })(),
        min: this.requireOptionalNumberField(def, 'Min', INVALID_AGENT_PROFILE_SCHEMA_MESSAGE),
        max: this.requireOptionalNumberField(def, 'Max', INVALID_AGENT_PROFILE_SCHEMA_MESSAGE),
        pattern: this.requireOptionalStringField(def, 'Pattern', INVALID_AGENT_PROFILE_SCHEMA_MESSAGE),
        enum: (() => {
          const value = def.Enum;
          if (value === undefined || value === null) {
            return undefined;
          }
          return this.requireArrayResponse<unknown>(value, INVALID_AGENT_PROFILE_SCHEMA_MESSAGE).map(
            (entry) => {
              if (typeof entry !== 'string') {
                throw new Error(INVALID_AGENT_PROFILE_SCHEMA_MESSAGE);
              }
              return entry;
            },
          );
        })(),
      };
    });
  }

  /**
   * Validate a config without saving.
   */
  static async validateConfig(config: Record<string, unknown>): Promise<ConfigValidationResult> {
    const response = await apiFetch(
      `${this.baseUrl}/validate`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      },
    );
    await assertAPIResponseOK(response, 'Failed to validate profile config');
    const parsed = (await response.json()) as ConfigValidationResultResponse;

    if (!this.isRecord(parsed) || typeof parsed.Valid !== 'boolean') {
      throw new Error(INVALID_AGENT_PROFILE_VALIDATION_MESSAGE);
    }

    return {
      valid: parsed.Valid,
      errors:
        parsed.Errors === undefined
          ? []
          : this.requireConfigValidationItems(
              parsed.Errors,
              INVALID_AGENT_PROFILE_VALIDATION_MESSAGE,
            ),
      warnings:
        parsed.Warnings === undefined
          ? []
          : this.requireConfigValidationItems(
              parsed.Warnings,
              INVALID_AGENT_PROFILE_VALIDATION_MESSAGE,
            ),
    };
  }
}
