'use strict';

const ACTIVE_STATUSES = ['queued', 'in_progress'];

function belongsToClosedPullRequest(run, pullRequestNumber, headRepository, headBranch) {
  return (
    Number.isSafeInteger(run.id) && run.id > 0 &&
    run.event === 'pull_request' &&
    ACTIVE_STATUSES.includes(run.status) &&
    run.head_branch === headBranch &&
    run.head_repository?.full_name === headRepository &&
    Array.isArray(run.pull_requests) && run.pull_requests.length > 0 &&
    run.pull_requests.every((pullRequest) => pullRequest?.number === pullRequestNumber)
  );
}

async function cancelClosedPullRequestRuns({ github, context, core }) {
  const pullRequest = context.payload.pull_request;
  if (!pullRequest || !Number.isInteger(pullRequest.number)) {
    core.setFailed('The close event has no valid pull request number.');
    return;
  }

  const { owner, repo } = context.repo;
  const current = await github.rest.pulls.get({
    owner,
    repo,
    pull_number: pullRequest.number,
  });
  if (current.data.state !== 'closed') {
    core.info(`PR #${pullRequest.number} has reopened; leaving its runs alone.`);
    return;
  }

  const headRepository = pullRequest.head?.repo?.full_name;
  const headOwner = pullRequest.head?.repo?.owner?.login;
  const headBranch = pullRequest.head?.ref;
  if (!headRepository || !headOwner || !headBranch) {
    core.warning('The closed pull request has no durable head identity; no runs cancelled.');
    return;
  }

  // A branch can be reused immediately after closure. An open PR for the same
  // repository and branch takes precedence over this stale close event.
  const openForHead = await github.paginate(github.rest.pulls.list, {
    owner,
    repo,
    state: 'open',
    head: `${headOwner}:${headBranch}`,
    per_page: 100,
  });
  if (openForHead.length > 0) {
    core.info('The head now belongs to an open pull request; no runs cancelled.');
    return;
  }

  const candidates = new Map();
  for (const status of ACTIVE_STATUSES) {
    const runs = await github.paginate(github.rest.actions.listWorkflowRunsForRepo, {
      owner,
      repo,
      event: 'pull_request',
      status,
      per_page: 100,
    });
    for (const run of runs) {
      if (belongsToClosedPullRequest(run, pullRequest.number, headRepository, headBranch)) {
        candidates.set(run.id, run);
      } else if (
        run.head_branch === headBranch &&
        run.head_repository?.full_name === headRepository
      ) {
        core.info('A same-head run has no active, exclusive closed-PR association; leaving it alone.');
      }
    }
  }

  let cancellationRequests = 0;
  for (const run of candidates.values()) {
    // Lists are discovery, not mutation authority: a reused branch can have
    // another PR's runs, and a discovered run may have completed meanwhile.
    // Missing associations (including some fork responses) stay untouched.
    const observed = await github.rest.actions.getWorkflowRun({ owner, repo, run_id: run.id });
    if (
      observed.data.id !== run.id ||
      !belongsToClosedPullRequest(observed.data, pullRequest.number, headRepository, headBranch)
    ) {
      core.info(`Run ${run.id} no longer has an active, exclusive closed-PR association; leaving it alone.`);
      continue;
    }
    const latest = await github.rest.pulls.get({ owner, repo, pull_number: pullRequest.number });
    if (latest.data.state !== 'closed') {
      core.info(`PR #${pullRequest.number} is no longer closed; stopping cancellation.`);
      break;
    }
    const reused = await github.paginate(github.rest.pulls.list, {
      owner, repo, state: 'open', head: `${headOwner}:${headBranch}`, per_page: 100,
    });
    if (reused.length > 0) {
      core.info('The head now belongs to an open pull request; stopping cancellation.');
      break;
    }

    try {
      await github.rest.actions.cancelWorkflowRun({ owner, repo, run_id: run.id });
      cancellationRequests += 1;
      core.info(`Requested cancellation of ${run.name} run ${run.id} (${run.status}).`);
    } catch (error) {
      // Completion can race cancellation. Suppress only that proven terminal
      // conflict; a later completed run cannot clear a refusal, rate limit,
      // transport error or unknown cancellation result. Stop without another
      // API call for those errors, even if the run has since completed.
      if (error?.status !== 409) {
        throw error;
      }
      const refreshed = await github.rest.actions.getWorkflowRun({
        owner,
        repo,
        run_id: run.id,
      });
      if (refreshed.data.status !== 'completed') {
        throw error;
      }
      core.info(`Run ${run.id} completed before cancellation.`);
    }
  }
  core.info(
    `Requested cancellation for ${cancellationRequests} of ${candidates.size} ` +
      `unfinished run(s) for PR #${pullRequest.number}.`,
  );
}

module.exports = { ACTIVE_STATUSES, cancelClosedPullRequestRuns };
