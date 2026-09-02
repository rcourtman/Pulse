'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');

const {
  ACTIVE_STATUSES,
  cancelClosedPullRequestRuns,
} = require('./reclaim-closed-pr-capacity.cjs');

function fixture({ state = 'closed', openForHead = [], runs = {}, cancelError, refreshed = 'completed' } = {}) {
  const cancelled = [];
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
          if (cancelError) throw cancelError;
          cancelled.push(runId);
        },
        getWorkflowRun: async () => ({ data: { status: refreshed } }),
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
  return { github, context, core, cancelled, messages };
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
  const run = {
    id: 10,
    name: 'Core E2E Tests',
    status: 'in_progress',
    head_branch: 'topic/old',
    head_repository: { full_name: 'rcourtman/Pulse' },
  };
  const raced = fixture({ runs: { in_progress: [run] }, cancelError: new Error('409') });
  await cancelClosedPullRequestRuns(raced);
  assert.match(raced.messages[0], /completed before cancellation/);
  assert.match(raced.messages.at(-1), /Requested cancellation for 0 of 1/);

  const failed = fixture({
    runs: { in_progress: [run] },
    cancelError: new Error('authentication failed'),
    refreshed: 'in_progress',
  });
  await assert.rejects(cancelClosedPullRequestRuns(failed), /authentication failed/);
});
