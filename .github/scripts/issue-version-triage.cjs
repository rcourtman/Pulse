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

function extractPulseVersion(title, body) {
  if (body) {
    const lines = body.split(/\r?\n/);
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
      return normalizeVersion(stripHTMLComments(value.join("\n")));
    }
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

  const hasAdditionalActionableTopics = classifyAdditionalActionableTopics(issue.body);
  if (hasAdditionalActionableTopics === true) {
    core.info("Issue declares additional actionable topics; decomposition is required.");
    nextLabels.add(NEEDS_DECOMPOSITION_LABEL);
  } else if (hasAdditionalActionableTopics === false) {
    nextLabels.delete(NEEDS_DECOMPOSITION_LABEL);
  }

  const reportedVersion = extractPulseVersion(issue.title, issue.body);
  core.info(`Reported Pulse version: ${reportedVersion || "not found"}`);
  core.info(`Latest stable release: ${latestVersion || "unknown"}`);

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
  const issue = context.payload.issue;
  const latestVersion = await getLatestStableVersion(github, context, core);
  const {
    labelNames,
    nextLabels,
    reportedVersion,
    hasAdditionalActionableTopics,
    isBugLike,
  } = buildTriageState(issue, core, latestVersion);

  if (hasAdditionalActionableTopics === true) {
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
