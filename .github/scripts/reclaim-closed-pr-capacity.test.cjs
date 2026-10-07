'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');

const {
  ACTIVE_STATUSES,
  cancelClosedPullRequestRuns,
} = require('./reclaim-closed-pr-capacity.cjs');

function fixture({ state = 'closed', openForHead = [], runs = {}, cancelError, refreshed = 'completed', refreshError } = {}) {
  const cancelled = [];
  const cancellationAttempts = [];
  const readbacks = [];
  const messages = [];
  const github = {
    paginate: async (method, input) => method(input),
    rest: {
      pulls: {
        get: async () => ({ data: { state } }),
        list: async () => openForHead,
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
          if (refreshError) throw refreshError;
          return { data: { status: refreshed } };
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
  return { github, context, core, cancelled, cancellationAttempts, readbacks, messages };
}

function requestError(status) {
  return Object.assign(new Error(`request failed (${status})`), { status });
}

function matchingRun(id = 10) {
  return {
    id,
    name: 'Core E2E Tests',
    status: 'in_progress',
    head_branch: 'topic/old',
    head_repository: { full_name: 'rcourtman/Pulse' },
  };
}

test('cancels only unfinished runs for the exact closed head', async () => {
  const matching = {
    id: 10,
    name: 'Build and Test',
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
  assert.deepEqual(raced.readbacks, [10]);
  assert.match(raced.messages[0], /completed before cancellation/);
  assert.match(raced.messages.at(-1), /Requested cancellation for 0 of 1/);

  const failed = fixture({
    runs: { in_progress: [run] },
    cancelError: conflict,
    refreshed: 'in_progress',
  });
  await assert.rejects(cancelClosedPullRequestRuns(failed), (error) => error === conflict);
  assert.deepEqual(failed.readbacks, [10]);
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
    assert.deepEqual(subject.readbacks, [], 'no follow-up API call after the failed cancellation');
    assert.deepEqual(subject.messages, [], 'do not report the failed operation as a completion race');
  });
}

test('does not infer a conflict from an error message', async () => {
  const error = new Error('409');
  const subject = fixture({ runs: { in_progress: [matchingRun()] }, cancelError: error });
  await assert.rejects(cancelClosedPullRequestRuns(subject), (caught) => caught === error);
  assert.deepEqual(subject.readbacks, []);
});

test('stops when the conflict readback fails', async () => {
  const error = requestError(403);
  const subject = fixture({
    runs: { in_progress: [matchingRun(), matchingRun(11)] },
    cancelError: requestError(409),
    refreshError: error,
  });
  await assert.rejects(cancelClosedPullRequestRuns(subject), (caught) => caught === error);
  assert.deepEqual(subject.readbacks, [10]);
  assert.deepEqual(subject.cancellationAttempts, [10]);
  assert.deepEqual(subject.messages, []);
});
