const VERSION_LABEL_PREFIX = "affects-";
const NEEDS_VERSION_LABEL = "needs-version-info";
const RETEST_LABEL = "needs-retest-on-latest";
const RETEST_COMMENT_MARKER = "<!-- issue-version-triage:v1 -->";
const BUG_LABEL = "bug";
const DOCS_LABEL = "documentation";
const ENHANCEMENT_LABEL = "enhancement";
const MAINTAINER_AUTHOR_ASSOCIATIONS = new Set(["OWNER", "MEMBER", "COLLABORATOR"]);

function escapeRegExp(value) {
  return String(value || "").replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function extractSectionValue(body, heading) {
  if (!body) return null;
  const pattern = new RegExp(
    `^#+\\s*${escapeRegExp(heading)}\\s*$\\n+([\\s\\S]*?)(?=^#+\\s+|$)`,
    "im"
  );
  const match = body.match(pattern);
  if (!match) return null;
  const value = match[1].trim();
  return value || null;
}

function normalizeVersion(value) {
  if (!value) return null;
  const match = String(value).match(/\bv?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)\b/);
  return match ? match[1] : null;
}

function extractPulseVersion(title, body) {
  if (body) {
    const lines = body.split(/\r?\n/);
    for (let i = 0; i < lines.length; i += 1) {
      const line = lines[i] || "";
      if (/pulse\s*(\||-)?\s*version/i.test(line)) {
        const inlineVersion = normalizeVersion(line);
        if (inlineVersion) return inlineVersion;

        for (let j = i + 1; j < Math.min(i + 6, lines.length); j += 1) {
          const nearby = (lines[j] || "").trim();
          if (!nearby) continue;
          const nearbyVersion = normalizeVersion(nearby);
          if (nearbyVersion) return nearbyVersion;
        }
      }
    }

    const headingMatch = body.match(
      /#+\s*Pulse version[\s\S]{0,80}?(\bv?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?\b)/i
    );
    if (headingMatch) return normalizeVersion(headingMatch[1]);

    const legacyMatch = body.match(
      /pulse\s*\|?\s*version[^\n]*?(\bv?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?\b)/i
    );
    if (legacyMatch) return normalizeVersion(legacyMatch[1]);
  }

  return normalizeVersion(title);
}

function classifyV6FeedbackType(body) {
  const feedbackType = extractSectionValue(body, "Feedback type");
  if (!feedbackType) return null;

  const normalized = feedbackType.toLowerCase();
  if (
    normalized.includes("bug") ||
    normalized.includes("regression") ||
    normalized.includes("upgrade / migration issue") ||
    normalized.includes("performance issue")
  ) {
    return BUG_LABEL;
  }
  if (normalized.includes("documentation issue")) {
    return DOCS_LABEL;
  }
  if (
    normalized.includes("ux / workflow friction") ||
    normalized.includes("other actionable feedback")
  ) {
    return ENHANCEMENT_LABEL;
  }
  return null;
}

function parseCore(version) {
  const match = String(version || "").match(/^(\d+)\.(\d+)\.(\d+)/);
  if (!match) return null;
  return [Number(match[1]), Number(match[2]), Number(match[3])];
}

function compareCore(a, b) {
  const av = parseCore(a);
  const bv = parseCore(b);
  if (!av || !bv) return null;
  for (let i = 0; i < 3; i += 1) {
    if (av[i] > bv[i]) return 1;
    if (av[i] < bv[i]) return -1;
  }
  return 0;
}

async function ensureLabel(github, context, name, color, description) {
  try {
    await github.rest.issues.getLabel({
      owner: context.repo.owner,
      repo: context.repo.repo,
      name,
    });
  } catch (error) {
    if (error.status !== 404) throw error;
    await github.rest.issues.createLabel({
      owner: context.repo.owner,
      repo: context.repo.repo,
      name,
      color,
      description,
    });
  }
}

async function hasRetestComment(github, context, issueNumber) {
  const comments = await github.paginate(github.rest.issues.listComments, {
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issueNumber,
    per_page: 100,
  });
  return comments.some((comment) => (comment.body || "").includes(RETEST_COMMENT_MARKER));
}

async function getLatestStableVersion(github, context, core) {
  try {
    const latest = await github.rest.repos.getLatestRelease({
      owner: context.repo.owner,
      repo: context.repo.repo,
    });
    return normalizeVersion(latest.data.tag_name || latest.data.name || "");
  } catch (error) {
    core.warning(`Could not determine latest release: ${error.message}`);
    return null;
  }
}

function buildTriageState(issue, core, latestVersion) {
  const labelNames = new Set((issue.labels || []).map((label) => label.name));
  const nextLabels = new Set(labelNames);
  const v6FeedbackClass = classifyV6FeedbackType(issue.body);
  if (v6FeedbackClass) {
    core.info(`Detected v6 feedback issue class: ${v6FeedbackClass}`);
    nextLabels.add(v6FeedbackClass);
  }

  const reportedVersion = extractPulseVersion(issue.title, issue.body);
  core.info(`Reported Pulse version: ${reportedVersion || "not found"}`);
  core.info(`Latest stable release: ${latestVersion || "unknown"}`);

  return {
    labelNames,
    nextLabels,
    reportedVersion,
    v6FeedbackClass,
    isBugLike: nextLabels.has(BUG_LABEL),
    comparison:
      reportedVersion && latestVersion ? compareCore(reportedVersion, latestVersion) : null,
  };
}

function keepOnlyReportedVersionLabel(nextLabels, reportedVersion) {
  const keep = `${VERSION_LABEL_PREFIX}${reportedVersion}`;
  for (const label of [...nextLabels]) {
    if (
      /^affects-\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(label) &&
      label !== keep
    ) {
      nextLabels.delete(label);
    }
  }
}

function buildRetestCommentBody(reportedVersion, latestVersion) {
  return [
    RETEST_COMMENT_MARKER,
    "Thanks for the report.",
    "",
    `I can see this was reported on **v${reportedVersion}**, while the latest stable release is **v${latestVersion}**.`,
    `Please retest on **v${latestVersion}** and comment with:`,
    "",
    "- whether the issue still reproduces",
    "- updated logs/diagnostics",
    "- exact running image tag or digest",
    "",
    "If there is no reporter follow-up after 7 days, this issue may be auto-closed until new confirmation is provided.",
    "",
    "If it still reproduces on the latest version, I will keep this open as an active regression.",
  ].join("\n");
}

function canPostRetestComment(issue, action) {
  const authorAssociation = String(issue.author_association || "").toUpperCase();
  return (
    (action === "opened" || action === "reopened") &&
    !MAINTAINER_AUTHOR_ASSOCIATIONS.has(authorAssociation)
  );
}

async function syncLabels({ github, context, core }) {
  const issue = context.payload.issue;
  const latestVersion = await getLatestStableVersion(github, context, core);
  const {
    labelNames,
    nextLabels,
    reportedVersion,
    v6FeedbackClass,
    isBugLike,
    comparison,
  } = buildTriageState(issue, core, latestVersion);

  if (!isBugLike) {
    core.info("Issue is not bug-like after classification. Skipping version triage.");
    if (v6FeedbackClass && !labelNames.has(v6FeedbackClass)) {
      await github.rest.issues.setLabels({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: issue.number,
        labels: [...nextLabels].sort(),
      });
    }
    return;
  }

  if (reportedVersion) {
    await ensureLabel(
      github,
      context,
      `${VERSION_LABEL_PREFIX}${reportedVersion}`,
      "0e8a16",
      `Bug reported against Pulse ${reportedVersion}`
    );
    await ensureLabel(
      github,
      context,
      RETEST_LABEL,
      "d93f0b",
      "Reporter should retest on current latest stable release"
    );

    keepOnlyReportedVersionLabel(nextLabels, reportedVersion);
    nextLabels.add(`${VERSION_LABEL_PREFIX}${reportedVersion}`);
    nextLabels.delete(NEEDS_VERSION_LABEL);

    if (comparison !== null && comparison < 0) {
      nextLabels.add(RETEST_LABEL);
    } else {
      nextLabels.delete(RETEST_LABEL);
    }
  } else {
    await ensureLabel(
      github,
      context,
      NEEDS_VERSION_LABEL,
      "fbca04",
      "Issue is missing required Pulse version metadata"
    );
    nextLabels.add(NEEDS_VERSION_LABEL);
    nextLabels.delete(RETEST_LABEL);
  }

  await github.rest.issues.setLabels({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issue.number,
    labels: [...nextLabels].sort(),
  });
}

async function postRetestComment({ github, context, core }) {
  const issue = context.payload.issue;
  const action = context.payload.action || "";
  if (!canPostRetestComment(issue, action)) {
    core.info("Public retest guidance is disabled for this issue event.");
    return;
  }

  const latestVersion = await getLatestStableVersion(github, context, core);
  const { reportedVersion, isBugLike, comparison } = buildTriageState(
    issue,
    core,
    latestVersion
  );

  if (!isBugLike) {
    core.info("Issue is not bug-like after classification. Skipping public retest guidance.");
    return;
  }
  if (!reportedVersion) {
    core.info("Issue is missing Pulse version metadata. Skipping public retest guidance.");
    return;
  }
  if (comparison === null || comparison >= 0) {
    core.info("Issue is already on the latest stable core or newer. Skipping public retest guidance.");
    return;
  }
  if (await hasRetestComment(github, context, issue.number)) {
    core.info("Retest guidance comment already exists.");
    return;
  }

  await github.rest.issues.createComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issue.number,
    body: buildRetestCommentBody(reportedVersion, latestVersion),
  });
}

module.exports = {
  syncLabels,
  postRetestComment,
  internals: {
    BUG_LABEL,
    DOCS_LABEL,
    ENHANCEMENT_LABEL,
    NEEDS_VERSION_LABEL,
    RETEST_COMMENT_MARKER,
    RETEST_LABEL,
    VERSION_LABEL_PREFIX,
    buildRetestCommentBody,
    buildTriageState,
    canPostRetestComment,
    classifyV6FeedbackType,
    compareCore,
    extractPulseVersion,
    normalizeVersion,
  },
};
