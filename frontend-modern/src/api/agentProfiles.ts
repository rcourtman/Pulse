import { apiFetch, apiFetchJSON } from '@/utils/apiClient';
import { readAPIErrorMessage } from './responseUtils';

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

    /**
     * List all agent profiles.
     */
    static async listProfiles(): Promise<AgentProfile[]> {
        try {
            const response = await apiFetchJSON<AgentProfile[]>(`${this.baseUrl}/`);
            return response || [];
        } catch (err) {
            // Handle 402 gracefully - means not licensed
            if (err instanceof Error && err.message.includes('402')) {
                return [];
            }
            throw err;
        }
    }

    /**
     * Get a single profile by ID.
     */
    static async getProfile(id: string): Promise<AgentProfile | null> {
        try {
            return await apiFetchJSON<AgentProfile>(`${this.baseUrl}/${encodeURIComponent(id)}`);
        } catch (err) {
            if (err instanceof Error && err.message.includes('404')) {
                return null;
            }
            throw err;
        }
    }

    /**
     * Create a new profile.
     */
    static async createProfile(name: string, config: Record<string, unknown>, description?: string): Promise<AgentProfile> {
        const response = await apiFetch(`${this.baseUrl}/`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, description, config }),
        });

        if (!response.ok) {
            throw new Error(await readAPIErrorMessage(response, `Failed to create profile: ${response.status}`));
        }

        return response.json();
    }

    /**
     * Update an existing profile.
     */
    static async updateProfile(id: string, name: string, config: Record<string, unknown>, description?: string): Promise<AgentProfile> {
        const response = await apiFetch(`${this.baseUrl}/${encodeURIComponent(id)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id, name, description, config }),
        });

        if (!response.ok) {
            throw new Error(await readAPIErrorMessage(response, `Failed to update profile: ${response.status}`));
        }

        return response.json();
    }

    /**
     * Delete a profile.
     */
    static async deleteProfile(id: string): Promise<void> {
        const response = await apiFetch(`${this.baseUrl}/${encodeURIComponent(id)}`, {
            method: 'DELETE',
        });

        if (!response.ok && response.status !== 204) {
            throw new Error(await readAPIErrorMessage(response, `Failed to delete profile: ${response.status}`));
        }
    }

    /**
     * List all profile assignments.
     */
    static async listAssignments(): Promise<AgentProfileAssignment[]> {
        try {
            const response = await apiFetchJSON<AgentProfileAssignment[]>(`${this.baseUrl}/assignments`);
            return response || [];
        } catch (err) {
            if (err instanceof Error && err.message.includes('402')) {
                return [];
            }
            throw err;
        }
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

        if (!response.ok) {
            throw new Error(await readAPIErrorMessage(response, `Failed to assign profile: ${response.status}`));
        }
    }

    /**
     * Remove profile assignment from an agent.
     */
    static async unassignProfile(agentId: string): Promise<void> {
        const response = await apiFetch(`${this.baseUrl}/assignments/${encodeURIComponent(agentId)}`, {
            method: 'DELETE',
        });

        if (!response.ok && response.status !== 204) {
            throw new Error(await readAPIErrorMessage(response, `Failed to unassign profile: ${response.status}`));
        }
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

        if (!response.ok) {
            if (response.status === 503) {
                throw new Error('Pulse Assistant service is not available. Please check Pulse Assistant settings.');
            }
            throw new Error(await readAPIErrorMessage(response, `Failed to get suggestion: ${response.status}`));
        }

        return response.json();
    }

    /**
     * Fetch config schema definitions for agent profiles.
     */
    static async getConfigSchema(): Promise<ConfigKeyDefinition[]> {
        const response = await apiFetchJSON<ConfigKeyDefinitionResponse[]>(`${this.baseUrl}/schema`);
        const defs = response || [];
        return defs.map(def => ({
            key: def.Key,
            type: def.Type,
            description: def.Description,
            defaultValue: def.Default,
            required: def.Required,
            min: def.Min,
            max: def.Max,
            pattern: def.Pattern,
            enum: def.Enum,
        }));
    }

    /**
     * Validate a config without saving.
     */
    static async validateConfig(config: Record<string, unknown>): Promise<ConfigValidationResult> {
        const response = await apiFetchJSON<ConfigValidationResultResponse>(`${this.baseUrl}/validate`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(config),
        });

        if (!response) {
            return { valid: true, errors: [], warnings: [] };
        }

        return {
            valid: response.Valid,
            errors: (response.Errors || []).map(err => ({ key: err.Key, message: err.Message })),
            warnings: (response.Warnings || []).map(err => ({ key: err.Key, message: err.Message })),
        };
    }
}
