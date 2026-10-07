'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');

const {
  ACTIVE_STATUSES,
  cancelClosedPullRequestRuns,
} = require('./reclaim-closed-pr-capacity.cjs');

function fixture({ state = 'closed', openForHead = [], runs = {}, cancelError, refreshed = 'completed', refreshError, observedRun, runReadError } = {}) {
  const cancelled = [];
  const cancellationAttempts = [];
  const readbacks = [];
  const messages = [];
  const lifecycleReads = [];
  const headReads = [];
  const github = {
    paginate: async (method, input) => method(input),
    rest: {
      pulls: {
        get: async () => {
          lifecycleReads.push(1858);
          return { data: { state: Array.isArray(state) ? state.shift() : state } };
        },
        list: async ({ head }) => {
          headReads.push(head);
          return typeof openForHead === 'function' ? openForHead() : openForHead;
        },
      },
      actions: {
        listWorkflowRunsForRepo: async ({ status }) => runs[status] || [],
        cancelWorkflowRun: async ({ run_id: runId }) => {
          cancellationAttempts.push(runId);
          if (cancelError) throw cancelError;
          cancelled.push(runId);
        },
        getWorkflowRun: async ({ run_id: runId }) => {
          readbacks.push(runId);
          if (cancellationAttempts.includes(runId)) {
            if (refreshError) throw refreshError;
            return { data: { status: refreshed } };
          }
          if (runReadError) throw runReadError;
          const listed = Object.values(runs).flat().find((run) => run.id === runId);
          return { data: observedRun || listed };
        },
      },
    },
  };
  const context = {
    repo: { owner: 'rcourtman', repo: 'Pulse' },
    payload: {
      pull_request: {
        number: 1858,
        head: {
          ref: 'topic/old',
          repo: { full_name: 'rcourtman/Pulse', owner: { login: 'rcourtman' } },
        },
      },
    },
  };
  const core = {
    info: (message) => messages.push(message),
    warning: (message) => messages.push(message),
    setFailed: (message) => messages.push(message),
  };
  return { github, context, core, cancelled, cancellationAttempts, readbacks, messages, lifecycleReads, headReads };
}

function requestError(status) {
  return Object.assign(new Error(`request failed (${status})`), { status });
}

function matchingRun(id = 10) {
  return {
    id,
    name: 'Core E2E Tests',
    event: 'pull_request',
    pull_requests: [{ number: 1858 }],
    status: 'in_progress',
    head_branch: 'topic/old',
    head_repository: { full_name: 'rcourtman/Pulse' },
  };
}

test('cancels only unfinished runs for the exact closed head', async () => {
  const matching = {
    id: 10,
    name: 'Build and Test',
    event: 'pull_request',
    pull_requests: [{ number: 1858 }],
    status: 'queued',
    head_branch: 'topic/old',
    head_repository: { full_name: 'rcourtman/Pulse' },
  };
  const duplicate = { ...matching, status: 'in_progress' };
  const otherBranch = { ...matching, id: 11, head_branch: 'topic/current' };
  const otherRepository = {
    ...matching,
    id: 12,
    head_repository: { full_name: 'contributor/Pulse' },
  };
  const subject = fixture({
    runs: { queued: [matching, otherBranch, otherRepository], in_progress: [duplicate] },
  });

  await cancelClosedPullRequestRuns(subject);

  assert.deepEqual(subject.cancelled, [10]);
  assert.match(subject.messages.at(-1), /Requested cancellation for 1 of 1 unfinished run/);
  assert.deepEqual(ACTIVE_STATUSES, ['queued', 'in_progress']);
});

test('does nothing when the pull request reopened', async () => {
  const subject = fixture({ state: 'open' });
  await cancelClosedPullRequestRuns(subject);
  assert.deepEqual(subject.cancelled, []);
  assert.match(subject.messages[0], /has reopened/);
});

test('does nothing when an open pull request reused the head branch', async () => {
  const subject = fixture({ openForHead: [{ number: 1900 }] });
  await cancelClosedPullRequestRuns(subject);
  assert.deepEqual(subject.cancelled, []);
  assert.match(subject.messages[0], /belongs to an open pull request/);
});

test('accepts only a proven completion race', async () => {
  const run = matchingRun();
  const conflict = requestError(409);
  const raced = fixture({ runs: { in_progress: [run] }, cancelError: conflict });
  await cancelClosedPullRequestRuns(raced);
  assert.deepEqual(raced.cancellationAttempts, [10]);
  assert.deepEqual(raced.readbacks, [10, 10]);
  assert.match(raced.messages[0], /completed before cancellation/);
  assert.match(raced.messages.at(-1), /Requested cancellation for 0 of 1/);

  const failed = fixture({
    runs: { in_progress: [run] },
    cancelError: conflict,
    refreshed: 'in_progress',
  });
  await assert.rejects(cancelClosedPullRequestRuns(failed), (error) => error === conflict);
  assert.deepEqual(failed.readbacks, [10, 10]);
});

for (const status of [400, 401, 403, 404, 429, 500, undefined, '409']) {
  test(`does not clear cancellation error ${status} with a completed run`, async () => {
    const error = requestError(status);
    const subject = fixture({
      runs: { in_progress: [matchingRun(), matchingRun(11)] },
      cancelError: error,
      refreshed: 'completed',
    });
    await assert.rejects(cancelClosedPullRequestRuns(subject), (caught) => caught === error);
    assert.deepEqual(subject.cancellationAttempts, [10], 'stop before cancelling another run');
    assert.deepEqual(subject.cancelled, []);
    assert.deepEqual(subject.readbacks, [10], 'only admission read, no follow-up API call after the failed cancellation');
    assert.deepEqual(subject.messages, [], 'do not report the failed operation as a completion race');
  });
}

test('does not infer a conflict from an error message', async () => {
  const error = new Error('409');
  const subject = fixture({ runs: { in_progress: [matchingRun()] }, cancelError: error });
  await assert.rejects(cancelClosedPullRequestRuns(subject), (caught) => caught === error);
  assert.deepEqual(subject.readbacks, [10]);
});

test('stops when the conflict readback fails', async () => {
  const error = requestError(403);
  const subject = fixture({
    runs: { in_progress: [matchingRun(), matchingRun(11)] },
    cancelError: requestError(409),
    refreshError: error,
  });
  await assert.rejects(cancelClosedPullRequestRuns(subject), (caught) => caught === error);
  assert.deepEqual(subject.readbacks, [10, 10]);
  assert.deepEqual(subject.cancellationAttempts, [10]);
  assert.deepEqual(subject.messages, []);
});


for (const [label, change] of [
  ['different PR', { pull_requests: [{ number: 1900 }] }],
  ['missing association', { pull_requests: undefined }],
  ['empty fork association', { pull_requests: [] }],
  ['shared association', { pull_requests: [{ number: 1858 }, { number: 1900 }] }],
  ['malformed association', { pull_requests: [null] }],
  ['push event', { event: 'push' }],
  ['completed status', { status: 'completed' }],
  ['invalid run ID', { id: '10' }],
]) {
  test(`identity: ignores ${label} even on the same head branch`, async () => {
    const subject = fixture({ runs: { in_progress: [{ ...matchingRun(), ...change }] } });
    await cancelClosedPullRequestRuns(subject);
    assert.deepEqual(subject.cancellationAttempts, []);
    assert.deepEqual(subject.readbacks, [], 'do not pursue an unrelated or unbound run');
  });
}

for (const [label, change] of [
  ['reassociated', { pull_requests: [{ number: 1900 }] }],
  ['association unavailable', { pull_requests: [] }],
  ['completed', { status: 'completed' }],
  ['changed event', { event: 'push' }],
  ['different returned ID', { id: 11 }],
  ['different returned branch', { head_branch: 'topic/current' }],
  ['different returned repository', { head_repository: { full_name: 'contributor/Pulse' } }],
]) {
  test(`identity: leaves a ${label} run alone at mutation readback`, async () => {
    const subject = fixture({
      runs: { in_progress: [matchingRun()] },
      observedRun: { ...matchingRun(), ...change },
    });
    await cancelClosedPullRequestRuns(subject);
    assert.deepEqual(subject.cancellationAttempts, []);
    assert.deepEqual(subject.readbacks, [10]);
    assert.deepEqual(subject.lifecycleReads, [1858], 'no further reads for the excluded run');
  });
}

test('identity: stops unchanged on a refused run admission read', async () => {
  const error = requestError(403);
  const subject = fixture({
    runs: { in_progress: [matchingRun(), matchingRun(11)] }, runReadError: error,
  });
  await assert.rejects(cancelClosedPullRequestRuns(subject), (caught) => caught === error);
  assert.deepEqual(subject.cancellationAttempts, []);
  assert.deepEqual(subject.readbacks, [10]);
  assert.deepEqual(subject.lifecycleReads, [1858]);
});

test('lifecycle: stops if the PR reopens during run discovery', async () => {
  const subject = fixture({ state: ['closed', 'open'], runs: { in_progress: [matchingRun()] } });
  await cancelClosedPullRequestRuns(subject);
  assert.deepEqual(subject.cancellationAttempts, []);
  assert.deepEqual(subject.lifecycleReads, [1858, 1858]);
});

test('lifecycle: stops if an open PR reuses the branch during discovery', async () => {
  let lists = 0;
  const subject = fixture({
    openForHead: () => ++lists === 1 ? [] : [{ number: 1900 }],
    runs: { in_progress: [matchingRun()] },
  });
  await cancelClosedPullRequestRuns(subject);
  assert.deepEqual(subject.cancellationAttempts, []);
  assert.equal(lists, 2);
});

test('lifecycle: rechecks closure before every cancellation, not just the first', async () => {
  const subject = fixture({
    state: ['closed', 'closed', 'open'],
    runs: { in_progress: [matchingRun(), matchingRun(11)] },
  });
  await cancelClosedPullRequestRuns(subject);
  assert.deepEqual(subject.cancellationAttempts, [10]);
  assert.deepEqual(subject.readbacks, [10, 11]);
  assert.deepEqual(subject.lifecycleReads, [1858, 1858, 1858]);
});

test('lifecycle: rechecks branch reuse before every cancellation', async () => {
  let lists = 0;
  const subject = fixture({
    openForHead: () => ++lists < 3 ? [] : [{ number: 1900 }],
    runs: { in_progress: [matchingRun(), matchingRun(11)] },
  });
  await cancelClosedPullRequestRuns(subject);
  assert.deepEqual(subject.cancellationAttempts, [10]);
  assert.equal(lists, 3);
});


for (const status of [403, 429]) {
  for (const phase of ['closure', 'head reuse']) {
    test(`lifecycle: stops on ${status} from the fresh ${phase} read`, async () => {
      const error = requestError(status);
      const subject = fixture({ runs: { in_progress: [matchingRun(), matchingRun(11)] } });
      const method = phase === 'closure' ? 'get' : 'list';
      const original = subject.github.rest.pulls[method];
      let calls = 0;
      subject.github.rest.pulls[method] = async (input) => {
        if (++calls === 2) throw error;
        return original(input);
      };
      await assert.rejects(cancelClosedPullRequestRuns(subject), (caught) => caught === error);
      assert.deepEqual(subject.cancellationAttempts, []);
      assert.deepEqual(subject.readbacks, [10], 'no later run or fallback read after refusal');
    });
  }
}

test('identity: still reclaims every exclusively associated run, including a fork', async () => {
  const run = { ...matchingRun(), head_repository: { full_name: 'contributor/Pulse' } };
  const subject = fixture({ runs: { in_progress: [run, { ...run, id: 11 }] } });
  subject.context.payload.pull_request.head.repo = {
    full_name: 'contributor/Pulse', owner: { login: 'contributor' },
  };
  await cancelClosedPullRequestRuns(subject);
  assert.deepEqual(subject.cancelled, [10, 11]);
  assert.deepEqual(subject.readbacks, [10, 11]);
  assert.deepEqual(subject.headReads, Array(3).fill('contributor:topic/old'));
  assert.match(subject.messages.at(-1), /Requested cancellation for 2 of 2/);
});
