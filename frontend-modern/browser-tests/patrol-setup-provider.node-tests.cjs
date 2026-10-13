const assert = require('node:assert/strict');
const { test } = require('node:test');
const { startProvider, readinessArguments } = require('./patrol-setup-provider.cjs');

test('copies the typed observation and nonce without punctuation', () => {
  assert.deepEqual(
    readinessArguments(
      'resource_id=node-protocol, category=reliability, severity=warning, evidence_token=token, nonce=abc123.',
    ),
    {
      resource_id: 'node-protocol',
      category: 'reliability',
      severity: 'warning',
      evidence_token: 'token',
      nonce: 'abc123',
    },
  );
});

test('takes the actionable context row, not healthy decoys or earlier fields', () => {
  const prompt =
    'Evaluation nonce: abcd1234\nhealthy resource_id=decoy severity=warning\nACTIONABLE resource_id=pbs-lab-a category=backup severity=critical evidence_token=failed-job\nhealthy-decoy-049 cpu=20%';
  assert.deepEqual(readinessArguments(prompt), {
    resource_id: 'pbs-lab-a',
    category: 'backup',
    severity: 'critical',
    evidence_token: 'failed-job',
    nonce: 'abcd1234',
  });
});

test('the protocol double emits calls, not a fabricated Pulse verdict', async (t) => {
  const records = [];
  const provider = await startProvider(0, records);
  t.after(() => provider.close());
  const origin = `http://127.0.0.1:${provider.port}`;
  const request = (name, content, role = 'user') =>
    fetch(origin + '/api/chat', {
      method: 'POST',
      body: JSON.stringify({
        model: 'qwen3:8b',
        stream: true,
        tools: [{ function: { name } }],
        messages: [{ role, content }],
      }),
    });
  const read = async (response) =>
    (await response.text())
      .trim()
      .split('\n')
      .map((line) => JSON.parse(line));
  const prompt =
    'resource_id=node-protocol, category=reliability, severity=warning, evidence_token=token, nonce=abc123.';
  const pass = await read(await request('readiness_record_observation', prompt));
  assert.equal(pass[0].done, false);
  assert.equal(pass[1].done, true);
  assert.equal(pass[0].message.tool_calls[0].function.name, 'readiness_record_observation');
  assert.equal(pass[0].message.tool_calls[0].function.arguments.nonce, 'abc123');
  assert.equal(pass[0].max_verified_mode, undefined);
  assert.equal(pass[0].status, undefined);

  provider.setMode('wrong-context');
  const wrong = await read(await request('readiness_record_observation', prompt));
  assert.equal(wrong[0].message.tool_calls[0].function.arguments.resource_id, 'healthy-decoy-001');
  provider.setMode('no-tools');
  const noTools = await read(await request('readiness_record_observation', prompt));
  assert.equal(noTools[0].message.tool_calls, undefined);

  provider.setMode('pass');
  const continued = await read(
    await request(
      'readiness_confirm_result',
      JSON.stringify({ finding_id: 'backend-issued-finding', nonce: 'fresh-backend-nonce' }),
      'tool',
    ),
  );
  assert.deepEqual(continued[0].message.tool_calls[0].function.arguments, {
    accepted: true,
    finding_id: 'backend-issued-finding',
    nonce: 'fresh-backend-nonce',
  });
  const firstRun = await read(await request('pulse_query', 'Synthetic host estate'));
  assert.deepEqual(firstRun[0].message.tool_calls[0].function.arguments, {
    action: 'list',
    type: 'agents',
  });
  const toolResult = await read(
    await request('pulse_query', JSON.stringify({ success: true }), 'tool'),
  );
  assert.equal(toolResult[0].message.tool_calls, undefined);
  assert.match(toolResult[0].message.content, /No changes requested/);
  assert.equal(
    records.some((record) => record.error),
    false,
  );
});
