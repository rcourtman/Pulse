// A credential-free local Ollama protocol double, not a readiness/result stub.
// Pulse must parse streaming typed calls, execute its own tools and evaluate
// their exact nonce/context/continuation before it can issue a verdict.
const assert = require('node:assert/strict');
const http = require('node:http');

function readinessArguments(prompt) {
  const actionable = prompt.split('\n').find((line) => line.startsWith('ACTIONABLE '));
  const input = {};
  for (const match of (actionable || prompt).matchAll(
    /(resource_id|category|severity|evidence_token|nonce)=([^,\s.]+)/g,
  ))
    input[match[1]] = match[2];
  const nonce = /Evaluation nonce: ([a-f0-9]+)/.exec(prompt);
  if (nonce) input.nonce = nonce[1];
  return input;
}

async function startProvider(port, records) {
  let mode = 'pass';
  let sequence = 0;
  const server = http.createServer(async (request, response) => {
    try {
      assert.equal(request.headers.authorization, undefined, 'no provider credential');
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const raw = Buffer.concat(chunks).toString();
      const body = raw ? JSON.parse(raw) : {};
      const record = { path: request.url, method: request.method, mode, body };
      records.push(record);
      response.setHeader('Content-Type', 'application/json');
      if (request.url === '/api/version')
        return response.end(JSON.stringify({ version: 'synthetic-ollama' }));
      if (request.url === '/api/tags')
        return response.end(
          JSON.stringify({
            models: [
              { name: 'qwen3:8b', size: 5_000_000_000, modified_at: '2026-10-13T00:00:00Z' },
            ],
          }),
        );
      if (request.url === '/api/show')
        return response.end(
          JSON.stringify({
            capabilities: ['completion', 'tools'],
            details: { family: 'qwen3', parameter_size: '8B', quantization_level: 'Q4_K_M' },
            model_info: { 'qwen3.context_length': 32768 },
          }),
        );
      assert.equal(request.url, '/api/chat', 'only supported local provider paths');
      const tools = (body.tools || []).map((tool) => tool.function.name);
      const messages = body.messages || [];
      const last = messages.at(-1);
      let call;
      let purpose;
      if (tools.includes('readiness_confirm_result')) {
        const result = JSON.parse(last.content);
        call = {
          name: 'readiness_confirm_result',
          arguments: { accepted: true, finding_id: result.finding_id, nonce: result.nonce },
        };
        purpose = 'readiness-continuation';
      } else if (tools.includes('readiness_record_observation')) {
        const prompt = messages.find((message) => message.role === 'user').content;
        const args = readinessArguments(prompt);
        if (mode === 'wrong-context') args.resource_id = 'healthy-decoy-001';
        call = { name: 'readiness_record_observation', arguments: args };
        purpose = 'readiness-scenario';
      } else if (tools.includes('pulse_query')) {
        purpose = 'patrol';
        if (last.role !== 'tool')
          call = { name: 'pulse_query', arguments: { action: 'list', type: 'agents' } };
      } else if (tools.length === 1) {
        purpose = 'startup-preflight';
        call = { name: tools[0], arguments: {} };
      } else {
        purpose = 'provider-chat';
      }
      record.purpose = purpose;
      const content =
        purpose === 'patrol'
          ? '### Infrastructure Status\nSynthetic fixture: reviewed the supplied host.\n### Key Observations\nNo warning or critical condition in this supplied fixture.\n### Actions Taken\nNo changes requested.'
          : 'Synthetic provider response.';
      if (mode === 'no-tools' && purpose.startsWith('readiness')) call = undefined;
      record.emittedTool = call?.name || null;
      const message = { role: 'assistant', content: call ? '' : content };
      if (call)
        message.tool_calls = [
          { id: `synthetic-call-${++sequence}`, function: { index: 0, ...call } },
        ];
      const end = {
        model: body.model,
        created_at: '2026-10-13T00:00:00Z',
        message,
        done: true,
        done_reason: call ? 'tool_use' : 'stop',
        prompt_eval_count: 100,
        eval_count: 20,
      };
      if (body.stream) {
        response.setHeader('Content-Type', 'application/x-ndjson');
        // Deliver the tool independently of the done event to exercise the
        // actual Ollama streaming parser rather than the non-streaming path.
        response.write(JSON.stringify({ ...end, done: false }) + '\n');
        response.end(
          JSON.stringify({ ...end, message: { role: 'assistant', content: '' } }) + '\n',
        );
      } else response.end(JSON.stringify(end));
    } catch (error) {
      records.push({ error: error.message });
      response.writeHead(500);
      response.end(JSON.stringify({ error: 'Synthetic fixture protocol refused' }));
    }
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(port, '127.0.0.1', resolve);
  });
  return {
    port: server.address().port,
    setMode(next) {
      assert(['pass', 'no-tools', 'wrong-context'].includes(next));
      mode = next;
    },
    async close() {
      server.closeAllConnections();
      await new Promise((resolve) => server.close(resolve));
    },
  };
}

module.exports = { startProvider, readinessArguments };
