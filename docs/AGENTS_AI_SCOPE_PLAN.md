AGENTS AI SCOPE PROFILE PLAN

Context
- Agent profiles exist today and are managed via AgentProfilesAPI.
- Current UI copy implied AI could auto-create profiles, but that flow does not exist.
- Goal: add AI-assisted suggestions that are always reviewed and explicitly created by the user.

Goals
- Provide an AI "Suggest profile" flow that drafts a scope profile and explains the rationale.
- Keep user in control: no auto-creation, no auto-assignment, no silent changes.
- Integrate with existing profile CRUD (AgentProfilesAPI.createProfile / assignProfile).

Non-Goals
- No background or automatic profile creation.
- No automatic assignment to agents.
- No backend changes to agent config schema in this phase.

Proposed UX
Entry points
- Agent Profiles page: "Suggest profile" button next to "New Profile".
- Optional: Unified Agents table row action "Suggest profile from this agent" to seed context.

Flow
1) User clicks "Suggest profile".
2) Modal opens with:
   - Prompt text area (optional) + default prompt template.
   - Scope of inputs toggles (agent telemetry, current profile examples, etc.).
3) AI returns a draft:
   - name
   - description (why this profile exists)
   - config JSON
   - rationale bullets (what signals led to the configuration)
4) User reviews and can edit name/description/config.
5) User clicks "Create profile".
6) Optional follow-up: "Assign to agents" (separate explicit action).

UX requirements
- Clear labels: "Suggest" or "Draft", never "Auto-create".
- Show JSON in a code editor-style area with validation errors.
- "Create profile" disabled until JSON validates.
- Provide copy/export for the config JSON.

Data inputs (minimal viable)
- User prompt text.
- Selected agent IDs (if starting from an agent).
- Basic agent metadata: hostname, platform, versions, tags, types (host/docker/k8s).

Data inputs (nice to have)
- Recent health signals: last seen, status, error flags.
- Existing profile list to avoid duplicates and suggest edits.
- Links to the agent config schema (so suggestions are valid).

AI output contract (server side)
- A stable JSON envelope with:
  - name: string
  - description: string
  - config: object
  - rationale: string[]
- If model returns invalid JSON, backend should retry once and then return a friendly error.

API proposal
- POST /api/admin/profiles/suggestions
  - body: { prompt, agentIds?: string[], includeTelemetry?: boolean }
  - response: { name, description, config, rationale }
- This endpoint can be a thin wrapper around the internal LLM service.
- If not licensed, return 402 and the UI should show the Pro gating.

Frontend plan
1) Add "Suggest profile" button to AgentProfilesPanel.
2) Create SuggestProfileModal component:
   - prompt input
   - loading state
   - response preview (name, description, rationale)
   - editable config text area with validation
3) On "Create profile", call AgentProfilesAPI.createProfile.
4) On success, refresh profile list and show toast.
5) Optional: allow "Assign to selected agents" step.

Backend plan (minimal)
1) Add suggestion endpoint that:
   - Gathers agent context (if agentIds provided).
   - Calls LLM with template.
   - Returns validated JSON.
2) Ensure prompt redaction of secrets/tokens.
3) Log prompt usage for auditing (excluding secrets).

Safety and product constraints
- Never apply changes without a user click.
- Show a "This is a draft" warning.
- Document that AI outputs are suggestions and may need adjustments.
- Respect licensing (Pro feature) and return 402 if unlicensed.

Testing
- Unit: modal renders, validation errors, createProfile call.
- Unit: suggestion endpoint response mapping.
- Integration: suggestion -> create profile -> list refresh.
- Regression: no auto-assignment when suggestion is created.

Open questions
- Which telemetry fields are safe/useful to include by default?
- Should suggestions be limited to host agents only?
- Do we need a schema-aware editor to reduce invalid configs?

---

## Implementation Summary (Completed)

### Backend Changes
1. **New file**: `internal/api/profile_suggestions.go`
   - `ProfileSuggestionHandler` - handles AI-assisted profile suggestions
   - `SuggestionRequest` / `ProfileSuggestion` types for API contract
   - Prompt template includes available config keys and their types
   - Parses LLM JSON response and validates config

2. **Modified file**: `internal/api/config_profiles.go`
   - Added `suggestionHandler` field to `ConfigProfileHandler`
   - Added `SetAIHandler()` method to inject AI capability
   - Added routing for `POST /suggestions` in `ServeHTTP()`

3. **Modified file**: `internal/api/router.go`
   - Wired AI handler to profile handler: `r.configProfileHandler.SetAIHandler(r.aiHandler)`

### Frontend Changes
1. **Modified file**: `frontend-modern/src/api/agentProfiles.ts`
   - Added `ProfileSuggestionRequest` and `ProfileSuggestion` interfaces
   - Added `suggestProfile()` method to `AgentProfilesAPI`

2. **New file**: `frontend-modern/src/components/Settings/SuggestProfileModal.tsx`
   - Modal with prompt input and example prompts
   - Loading state during AI call
   - Preview of suggested profile with name, description, config JSON, and rationale
   - "Draft" warning banner
   - "Use This Profile" button to accept suggestion

3. **Modified file**: `frontend-modern/src/components/Settings/AgentProfilesPanel.tsx`
   - Added "Suggest Profile" button (purple, with Sparkles icon) next to "New Profile"
   - Added `showSuggestModal` state
   - Added `handleSuggest()` and `handleSuggestionAccepted()` handlers
   - When suggestion is accepted, pre-fills the create profile form

### UX Flow
1. User clicks "Suggest Profile" button
2. Modal opens with prompt input and example prompts
3. User describes what they need
4. AI generates a profile suggestion
5. User reviews name, description, config JSON, and rationale
6. User clicks "Use This Profile"
7. Modal closes, create profile form opens pre-filled with suggestion
8. User can edit and then click "Create Profile"
