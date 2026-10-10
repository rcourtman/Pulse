const VERSION_LABEL_PREFIX = "affects-";
const NEEDS_VERSION_LABEL = "needs-version-info";
const RETEST_LABEL = "needs-retest-on-latest";
const NEEDS_DECOMPOSITION_LABEL = "needs-decomposition";
const BUG_LABEL = "bug";
const DOCS_LABEL = "documentation";
const ENHANCEMENT_LABEL = "enhancement";

function escapeRegExp(value) {
  return String(value || "").replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function extractSectionValue(body, heading, followingHeadings = []) {
  if (!body) return null;
  const boundary = followingHeadings.length
    ? followingHeadings.map(escapeRegExp).join("|")
    : "[^\\n]+";
  const pattern = new RegExp(
    `^#+\\s*${escapeRegExp(heading)}\\s*$\\n+([\\s\\S]*?)(?=^#+\\s*(?:${boundary})\\s*$|$)`,
    "im"
  );
  const match = body.match(pattern);
  if (!match) return null;
  const value = match[1].trim();
  return value || null;
}

function stripHTMLComments(value) {
  let stripped = String(value || "");
  let previous;
  do {
    previous = stripped;
    stripped = stripped.replace(/<!--[\s\S]*?(?:-->|$)/g, "");
  } while (stripped !== previous);
  return stripped;
}

function classifyAdditionalActionableTopics(body) {
  const value = extractSectionValue(body, "Additional actionable topics", [
    "Pulse version",
    "Additional context",
    "Logs, screenshots, or diagnostics",
    "Confirmations",
  ]);
  if (value === null) return null;

  const normalized = stripHTMLComments(value)
    .trim()
    .toLowerCase()
    .replace(/[.!]+$/g, "");
  if (!normalized) return false;

  return !new Set([
    "_no response_",
    "n/a",
    "na",
    "no",
    "none",
    "none known",
    "not applicable",
  ]).has(normalized);
}

function normalizeVersion(value) {
  if (!value) return null;
  const match = String(value).match(/\bv?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)\b/i);
  return match ? match[1] : null;
}

function normalizeLegacyVersionValue(value) {
  const visible = String(value).trim().replace(/^[`*_]+/, "");
  // Keep an explicitly shared server/agent version (as in #1788), without
  // mining a sentence such as "unknown; agent version 6.5.0" for a number.
  return /^(?:v?\d+\.\d+\.\d+\b|server(?:[ \t]*\+[ \t]*pulse-agent)?[ \t]+v?\d+\.\d+\.\d+\b|(?:[a-z0-9._/-]+\/)?pulse:|pulse[-_])/i.test(visible)
    ? normalizeVersion(visible) : null;
}

function extractPulseVersion(title, body) {
  if (body) {
    // Hidden template examples and headings are not reporter evidence.
    const lines = stripHTMLComments(body).split(/\r?\n/);
    const versionHeading = lines.findIndex((line) =>
      /^#{1,6}[ \t]+Pulse[ \t]+version[ \t]*$/i.test(line)
    );
    if (versionHeading !== -1) {
      // The explicit running-version field is authoritative, even when it is
      // incomplete. A title may name the old image in an upgrade report, and
      // neighbouring fields may contain an unrelated agent version.
      const value = [];
      for (let i = versionHeading + 1; i < lines.length; i += 1) {
        if (/^#{1,6}[ \t]+/.test(lines[i])) break;
        value.push(lines[i]);
      }
      return normalizeVersion(value.join("\n"));
    }
    for (let i = 0; i < lines.length; i += 1) {
      const line = lines[i].replace(/\*\*|__/g, "")
        .replace(/^[ \t]*(?:#{1,6}[ \t]+|[-*][ \t]+)/, "").trim();
      const field = line.match(
        /^Pulse[ \t]*(?:[|-][ \t]*)?version(?:[ \t]*[:|][ \t]*|[ \t]+|$)(.*)$/i
      );
      if (!field) continue;

      // Legacy inline/standalone fields have the same authority as the form.
      // Unknown or incomplete must not borrow an agent/platform version or
      // an upgrade's starting version from the title.
      const inlineValue = field[1].trim();
      if (inlineValue) return normalizeLegacyVersionValue(inlineValue);
      for (let j = i + 1; j < lines.length; j += 1) {
        const value = lines[j].trim();
        if (!value || /^(?:```|~~~)[\w-]*$/.test(value)) continue;
        // Accept a version or image reference as the first visible value,
        // not a neighbouring heading, named field, log or prose paragraph.
        return normalizeLegacyVersionValue(value);
      }
      return null;
    }
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

function buildTriageState(issue, core, latestVersion) {
  const labelNames = new Set((issue.labels || []).map((label) => label.name));
  const nextLabels = new Set(labelNames);
  const v6FeedbackClass = classifyV6FeedbackType(issue.body);
  if (v6FeedbackClass) {
    core.info(`Detected v6 feedback issue class: ${v6FeedbackClass}`);
    nextLabels.add(v6FeedbackClass);
  }

  const hasAdditionalActionableTopics = classifyAdditionalActionableTopics(issue.body);

  const reportedVersion = extractPulseVersion(issue.title, issue.body);
  core.info(`Reported Pulse version: ${reportedVersion || "not found"}`);
  if (latestVersion) core.info(`Latest stable release: ${latestVersion}`);

  return {
    labelNames,
    nextLabels,
    reportedVersion,
    v6FeedbackClass,
    hasAdditionalActionableTopics,
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

// Apply only this event's classification delta. Replacing the complete label
// set would overwrite Community changes made after the event was queued.
async function applyLabelDelta(github, context, issue, before, after) {
  const target = {
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issue.number,
  };
  const additions = [...after].filter((name) => !before.has(name)).sort();
  if (additions.length) {
    await github.rest.issues.addLabels({ ...target, labels: additions });
  }
  for (const name of [...before].filter((name) => !after.has(name)).sort()) {
    try {
      await github.rest.issues.removeLabel({ ...target, name });
    } catch (error) {
      // Another synchronizer may already have removed this label.
      if (error.status !== 404) throw error;
    }
  }
}

async function syncLabels({ github, context, core }) {
  const eventIssue = context.payload.issue;
  // Jobs can start after another edit, comment or Community disposition.
  // Read the report and its labels together; never fall back to stale event
  // metadata when this read fails.
  const { data: issue } = await github.rest.issues.get({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: eventIssue.number,
  });
  if (!issue || issue.number !== eventIssue.number) {
    throw new Error("Current issue identity did not match the triage event");
  }
  if (issue.state !== "open" || issue.pull_request) {
    core.info("Report is no longer an open issue. Skipping label sync.");
    return;
  }
  const {
    labelNames,
    nextLabels,
    reportedVersion,
    hasAdditionalActionableTopics,
    isBugLike,
  } = buildTriageState(issue, core, null);

  // A form declaration creates a review task only when it is new. A later
  // empty field cannot prove that comment topics were dispositioned, and an
  // unrelated or superseded event must not restore a label Community cleared.
  const previousBody = context.payload.changes?.body?.from;
  const eventIsCurrent = typeof issue.updated_at === "string" &&
    issue.body === eventIssue.body &&
    issue.updated_at === eventIssue.updated_at;
  const newlyDeclared = eventIsCurrent && hasAdditionalActionableTopics === true && (
    context.payload.action === "opened" ||
    (context.payload.action === "edited" && previousBody !== undefined &&
      classifyAdditionalActionableTopics(previousBody) !== true)
  );
  if (newlyDeclared) {
    core.info("Issue newly declares additional actionable topics; decomposition is required.");
    nextLabels.add(NEEDS_DECOMPOSITION_LABEL);
    await ensureLabel(
      github,
      context,
      NEEDS_DECOMPOSITION_LABEL,
      "fbca04",
      "Issue declares additional actionable topics that need linked dispositions"
    );
  }

  if (!isBugLike) {
    core.info("Issue is not bug-like after classification. Skipping version triage.");
    await applyLabelDelta(github, context, issue, labelNames, nextLabels);
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
    keepOnlyReportedVersionLabel(nextLabels, reportedVersion);
    nextLabels.add(`${VERSION_LABEL_PREFIX}${reportedVersion}`);
    nextLabels.delete(NEEDS_VERSION_LABEL);
  } else {
    await ensureLabel(
      github,
      context,
      NEEDS_VERSION_LABEL,
      "fbca04",
      "Issue is missing required Pulse version metadata"
    );
    nextLabels.add(NEEDS_VERSION_LABEL);
  }

  // Retest labels are community-owned: version metadata cannot establish
  // relevant-fix availability or reconcile the live reporter conversation.
  await applyLabelDelta(github, context, issue, labelNames, nextLabels);
}

// Compatibility entry points for callers using the old helper API. Version
// metadata alone cannot justify a public retest request. Community owns
// whole-thread review, relevant-fix and release-inclusion verification.
async function postRetestComment({ core }) {
  core.info("Version-only retest posting is retired; Community owns follow-up.");
}

async function postEligibleRetestComments({ core }) {
  core.info("Version-only retest sweep is retired; Community owns follow-up.");
  return { eligibleCount: 0, postedCount: 0 };
}

module.exports = {
  postEligibleRetestComments,
  syncLabels,
  postRetestComment,
  internals: {
    BUG_LABEL,
    DOCS_LABEL,
    ENHANCEMENT_LABEL,
    NEEDS_DECOMPOSITION_LABEL,
    NEEDS_VERSION_LABEL,
    RETEST_LABEL,
    VERSION_LABEL_PREFIX,
    buildTriageState,
    classifyAdditionalActionableTopics,
    classifyV6FeedbackType,
    compareCore,
    extractPulseVersion,
    normalizeVersion,
  },
};
