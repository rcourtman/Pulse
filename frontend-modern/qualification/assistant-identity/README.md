# Assistant identity browser regression

From the repository root (Node 24):

```sh
npm ci --ignore-scripts
npm ci --ignore-scripts --prefix frontend-modern
npx playwright install chromium
pulse-heavy-run -- node scripts/check-assistant-identity.mjs
```

This standalone Vite fixture imports the current checkout's real `useChat` and
`ChatMessages`, including `MessageItem`, approval cards and tool detail rows.
It is not a production entry point and needs no backend, credentials, provider,
customer captures or live infrastructure. The only replaced API is the chat
stream callback; all browser API/external HTTP requests are blocked and fail the
check. Approval completion is synthetic, not evidence of an authorised action.

Checks at 1440, 900 and 390 pixels cover concurrent same-name invocation IDs,
progress/completion isolation, retaining a sibling approval after completion,
settled rendered results, and shared input/output evidence after removal of a
workflow row. Both evidence rows are expanded and document overflow is checked.
The shared-object journey deliberately retains references, matching the original
regression rather than concealing it with cloned fixtures.

Output defaults to `tmp/assistant-identity`; set `ASSISTANT_IDENTITY_OUTPUT` to
an absolute path to retain a candidate-specific run. A successful run writes
screenshots and `receipt.json` with browser version and SHA-256 source hashes.
Run on the actual backport checkout: a main receipt does not qualify release
content. This is a component/reducer browser proof, not the release admission
attestation, a production-build check, SSE transport qualification, provider
reasoning proof, or installed end-to-end action verification.

Regression sensitivity can be checked by temporarily substituting either
`ChatMessages.tsx` or `hooks/useChat.ts` from the parent of fix `33b852f66b`:
each must fail independently. Restore files afterwards; never admit those
experimental substitutions as candidate content.
