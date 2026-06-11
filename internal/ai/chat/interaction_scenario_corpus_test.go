package chat

// Interaction-quality scenario corpus — the chat-feel counterpart of the
// Discovery corpus (internal/servicediscovery/scenario_corpus_test.go).
//
// Each scenario drives one full ExecuteStream turn against a scripted
// provider and pins the browser-facing event stream: the sequence and
// content of what a user visibly experiences in the transcript and footer.
// The teeth are deliberately at the stream boundary, not at internal
// helpers, so refactors of the agentic loop (including Patrol-phase work on
// the shared loop) cannot silently regress the interaction feel while unit
// tests stay green.
//
// When an interaction-quality fix lands (streaming stability, status
// restraint, error surfacing, fallback hygiene, tool-noise rules), add a
// scenario here that states the user-visible promise and would fail without
// the fix.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// scriptedProviderCall is one provider attempt: the events it streams and
// the error it returns. A scripted provider replays calls in order and
// repeats the last call once the script is exhausted (so final-summary
// retries behave deterministically).
type scriptedProviderCall struct {
	events []providers.StreamEvent
	err    error
}

func scriptedProvider(calls ...scriptedProviderCall) *stubServiceProvider {
	idx := 0
	return &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			call := calls[len(calls)-1]
			if idx < len(calls) {
				call = calls[idx]
			}
			idx++
			for _, ev := range call.events {
				callback(ev)
			}
			return call.err
		},
	}
}

func providerContentDone(text string) scriptedProviderCall {
	return scriptedProviderCall{events: []providers.StreamEvent{
		{Type: "content", Data: providers.ContentEvent{Text: text}},
		{Type: "done", Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1}},
	}}
}

func providerQueryToolCall(callID string) scriptedProviderCall {
	return scriptedProviderCall{events: []providers.StreamEvent{
		{Type: "tool_start", Data: providers.ToolStartEvent{ID: callID, Name: "pulse_query"}},
		{Type: "done", Data: providers.DoneEvent{
			ToolCalls: []providers.ToolCall{{
				ID:    callID,
				Name:  "pulse_query",
				Input: map[string]interface{}{"action": "topology"},
			}},
		}},
	}}
}

func providerEmptyDone() scriptedProviderCall {
	return scriptedProviderCall{events: []providers.StreamEvent{
		{Type: "done", Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1}},
	}}
}

type interactionScenario struct {
	name string
	// promise states the user-visible behavior this scenario pins, in plain
	// language. If a change makes this scenario fail, that promise broke.
	promise  string
	prompt   string
	calls    []scriptedProviderCall
	maxTurns int
	wantErr  bool
	// orderedTypes must appear in the recorded event stream as a
	// subsequence (other events may interleave).
	orderedTypes []string
	// forbiddenTypes must not appear in the stream at all.
	forbiddenTypes []string
	// answerMustContain / answerMustNotContain run against the concatenated
	// content events — the text the user actually reads.
	answerMustContain    []string
	answerMustNotContain []string
	// streamMustContain runs against the serialized event log (type|payload
	// per line), for promises about event payloads rather than answer text.
	streamMustContain []string
}

func interactionScenarios() []interactionScenario {
	return []interactionScenario{
		{
			name:              "plain answer turn",
			promise:           "a simple question streams an answer and a terminal done stamped with the model route and context window — no errors, no tool noise",
			prompt:            "is pulse healthy",
			calls:             []scriptedProviderCall{providerContentDone("Pulse is healthy.")},
			orderedTypes:      []string{"session", "workflow_state", "content", "done"},
			forbiddenTypes:    []string{"error", "tool_start", "tool_end"},
			answerMustContain: []string{"Pulse is healthy."},
			streamMustContain: []string{`"model":"openai:test"`, `"context_limit_tokens"`},
		},
		{
			name:              "greeting answers directly without tool noise",
			promise:           "a greeting gets a direct reply; tools are offered (model-owned manifest) but none are narrated into the stream",
			prompt:            "hi",
			calls:             []scriptedProviderCall{providerContentDone("Hi! How can I help with your infrastructure?")},
			orderedTypes:      []string{"session", "content", "done"},
			forbiddenTypes:    []string{"error", "tool_start", "tool_end"},
			answerMustContain: []string{"How can I help"},
		},
		{
			name:    "tool turn shows compact tool events then the synthesized answer",
			promise: "a tool-using turn renders as tool_start/tool_end with the real tool name, followed by the model's answer — never raw provider call ids in the answer",
			prompt:  "what does my inventory look like",
			calls: []scriptedProviderCall{
				providerQueryToolCall("call_a1b2c3d4"),
				providerContentDone("Your inventory is empty right now."),
			},
			maxTurns:             4,
			orderedTypes:         []string{"session", "tool_start", "tool_end", "content", "done"},
			forbiddenTypes:       []string{"error"},
			answerMustContain:    []string{"inventory is empty"},
			answerMustNotContain: []string{"call_a1b2c3d4"},
			streamMustContain:    []string{"pulse_query"},
		},
		{
			name:    "post-tool model turn marks the status handoff",
			promise: "after tools run, the footer status hands off from the last tool to the model composing the answer — a provider that reasons server-side without streaming events never leaves a stale tool status on screen",
			prompt:  "summarize my inventory",
			calls: []scriptedProviderCall{
				providerQueryToolCall("call_e5f6a7b8"),
				providerContentDone("Inventory summarized."),
			},
			maxTurns:          4,
			orderedTypes:      []string{"session", "tool_start", "tool_end", "workflow_state", "content", "done"},
			forbiddenTypes:    []string{"error"},
			streamMustContain: []string{"model_processing", "Working on the response with the gathered results"},
		},
		{
			name:    "no-narrative turn falls back to a clean operator sentence",
			promise: "when the model runs tools but never writes an answer, the user reads one clean sentence — not raw JSON, tool-result dumps, or provider call ids",
			prompt:  "check the topology",
			calls: []scriptedProviderCall{
				providerQueryToolCall("call_27f0f389aa"),
				providerEmptyDone(),
			},
			maxTurns:          4,
			orderedTypes:      []string{"session", "tool_start", "tool_end", "content", "done"},
			forbiddenTypes:    []string{"error"},
			answerMustContain: []string{"didn't return a written summary"},
			answerMustNotContain: []string{
				"call_27f0f389aa",
				`{"`,
			},
		},
		{
			name:    "provider failure before events retries invisibly",
			promise: "a provider stream that dies before producing anything is retried without surfacing an error or a dead turn to the user",
			prompt:  "is pulse healthy",
			calls: []scriptedProviderCall{
				{err: context.DeadlineExceeded},
				providerContentDone("Pulse is healthy."),
			},
			orderedTypes:      []string{"session", "content", "done"},
			forbiddenTypes:    []string{"error"},
			answerMustContain: []string{"Pulse is healthy."},
		},
		{
			name:    "terminal provider failure surfaces one clear error",
			promise: "when the provider truly fails mid-answer, the user sees exactly one error event instead of a silent stall",
			prompt:  "is pulse healthy",
			calls: []scriptedProviderCall{
				{
					events: []providers.StreamEvent{
						{Type: "content", Data: providers.ContentEvent{Text: "Pulse i"}},
					},
					err: context.DeadlineExceeded,
				},
			},
			wantErr:      true,
			orderedTypes: []string{"session", "error"},
		},
	}
}

func TestInteractionScenarioCorpus(t *testing.T) {
	for _, sc := range interactionScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			store, err := NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatalf("session store: %v", err)
			}
			service := NewService(Config{
				AIConfig:      &config.AIConfig{ChatModel: "openai:test", ControlLevel: config.ControlLevelControlled},
				StateProvider: &mockStateProvider{},
				AgentServer:   &mockAgentServer{},
			})
			service.sessions = store
			service.provider = scriptedProvider(sc.calls...)
			service.started = true

			var eventLog []string
			var answer strings.Builder
			req := ExecuteRequest{SessionID: "interaction-corpus", Prompt: sc.prompt}
			if sc.maxTurns > 0 {
				req.MaxTurns = sc.maxTurns
			}
			execErr := service.ExecuteStream(context.Background(), req, func(event StreamEvent) {
				eventLog = append(eventLog, event.Type+"|"+string(event.Data))
				if event.Type == "content" {
					var data ContentData
					if err := json.Unmarshal(event.Data, &data); err == nil {
						answer.WriteString(data.Text)
					}
				}
			})

			if sc.wantErr && execErr == nil {
				t.Fatalf("promise %q: expected the turn to error, got nil\nevents:\n%s", sc.promise, strings.Join(eventLog, "\n"))
			}
			if !sc.wantErr && execErr != nil {
				t.Fatalf("promise %q: turn failed: %v\nevents:\n%s", sc.promise, execErr, strings.Join(eventLog, "\n"))
			}

			// Ordered subsequence over event types.
			next := 0
			for _, line := range eventLog {
				if next >= len(sc.orderedTypes) {
					break
				}
				if strings.HasPrefix(line, sc.orderedTypes[next]+"|") {
					next++
				}
			}
			if next < len(sc.orderedTypes) {
				t.Fatalf("promise %q: event stream missing %q (in order %v)\nevents:\n%s",
					sc.promise, sc.orderedTypes[next], sc.orderedTypes, strings.Join(eventLog, "\n"))
			}

			for _, forbidden := range sc.forbiddenTypes {
				for _, line := range eventLog {
					if strings.HasPrefix(line, forbidden+"|") {
						t.Fatalf("promise %q: stream contains forbidden event type %q\nevents:\n%s",
							sc.promise, forbidden, strings.Join(eventLog, "\n"))
					}
				}
			}
			if sc.wantErr {
				errorEvents := 0
				for _, line := range eventLog {
					if strings.HasPrefix(line, "error|") {
						errorEvents++
					}
				}
				if errorEvents != 1 {
					t.Fatalf("promise %q: want exactly one error event, got %d\nevents:\n%s",
						sc.promise, errorEvents, strings.Join(eventLog, "\n"))
				}
			}

			answerText := answer.String()
			for _, want := range sc.answerMustContain {
				if !strings.Contains(answerText, want) {
					t.Fatalf("promise %q: answer missing %q, got %q", sc.promise, want, answerText)
				}
			}
			for _, forbidden := range sc.answerMustNotContain {
				if strings.Contains(answerText, forbidden) {
					t.Fatalf("promise %q: answer contains forbidden %q, got %q", sc.promise, forbidden, answerText)
				}
			}
			stream := strings.Join(eventLog, "\n")
			for _, want := range sc.streamMustContain {
				if !strings.Contains(stream, want) {
					t.Fatalf("promise %q: event stream missing %q\nevents:\n%s", sc.promise, want, stream)
				}
			}
		})
	}
}
