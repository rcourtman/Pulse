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

function atxHeading(text) {
  // Edited issue forms still use Markdown: up to three leading spaces and
  // one to six hashes, separated from the text by whitespace. A closing hash
  // run is optional, but only closes a heading when preceded by whitespace.
  const match = text.match(/^ {0,3}#{1,6}(?:[ \t]+(.*)|$)/);
  if (!match) return null;
  return (match[1] || "").replace(/(?:^|[ \t]+)#+[ \t]*$/, "").trim();
}

function declarationLines(body) {
  let fence = null;
  return stripHTMLComments(body).split(/\r?\n/).map((text) => {
    const marker = text.match(/^ {0,3}(`{3,}|~{3,})(.*)$/);
    let delimiter = false;
    const inCode = fence !== null;
    if (fence) {
      if (marker && marker[1][0] === fence.character &&
          marker[1].length >= fence.length && !marker[2].trim()) {
        fence = null;
        delimiter = true;
      }
    } else if (marker && (marker[1][0] === "~" || !marker[2].includes("`"))) {
      fence = { character: marker[1][0], length: marker[1].length };
      delimiter = true;
    }
    // Pasted examples and logs are evidence, not issue-form structure. Keep
    // their text as a value under a real field, but never scan it for fields.
    const declaration = !inCode && !delimiter && !/^(?: {4}|\t)/.test(text);
    return { text, delimiter, declaration, heading: declaration ? atxHeading(text) : null };
  });
}

function firstFieldValue(lines) {
  const value = lines.find((line) => !line.delimiter && line.text.trim());
  return value ? value.text.trim() : "";
}

function extractSectionValue(body, heading, followingHeadings = []) {
  if (!body) return null;
  const lines = declarationLines(body);
  const pattern = new RegExp(`^${escapeRegExp(heading)}$`, "i");
  const start = lines.findIndex((line) => line.heading !== null && pattern.test(line.heading));
  if (start === -1) return null;
  const boundary = followingHeadings.length
    ? new RegExp(`^(?:${followingHeadings.map(escapeRegExp).join("|")})$`, "i") : null;
  let end = start + 1;
  while (end < lines.length && !(lines[end].heading !== null &&
      (!boundary || boundary.test(lines[end].heading)))) {
    end += 1;
  }
  // An empty declared field differs from a legacy report with no field.
  return lines.slice(start + 1, end).map((line) => line.text).join("\n").trim();
}

function stripHTMLComments(value) {
  let stripped = String(value || "");
  let previous;
  do {
    previous = stripped;
    let fence = null;
    let inComment = false;
    // Strip hidden template text, not literal markers in pasted code. In
    // particular, an unfinished HTML response must not swallow the real form
    // fields after its closing Markdown fence. Match the same fences and
    // indentation that declarationLines excludes from metadata declarations.
    stripped = stripped.split(/(\r?\n)/).map((text, index) => {
      if (index % 2) return text;
      const marker = text.match(/^ {0,3}(`{3,}|~{3,})(.*)$/);
      if (!inComment) {
        if (fence) {
          if (marker && marker[1][0] === fence.character &&
              marker[1].length >= fence.length && !marker[2].trim()) {
            fence = null;
          }
          return text;
        }
        if (marker && (marker[1][0] === "~" || !marker[2].includes("`"))) {
          fence = { character: marker[1][0], length: marker[1].length };
          return text;
        }
        // A quoted log is not a top-level form declaration. Its literal
        // comment opener must not hide the unquoted fields which follow it.
        if (/^(?: {4}|\t| {0,3}>)/.test(text)) return text;
      }
      // Match complete, same-line code spans by exact backtick-run length.
      // Looking up the next run avoids rescanning a long pasted line for each
      // unmatched delimiter. Backticks inside a hidden comment cannot open a
      // span: the scanner below skips the comment before considering them.
      const codeEnds = new Map();
      const nextRun = new Map();
      const runs = [...text.matchAll(/`+/g)];
      for (let i = runs.length - 1; i >= 0; i -= 1) {
        const run = runs[i];
        if (nextRun.has(run[0].length)) {
          codeEnds.set(run.index, nextRun.get(run[0].length));
        }
        nextRun.set(run[0].length, run.index + run[0].length);
      }
      const token = /<!--|`+/g;
      let visible = "";
      let offset = 0;
      while (offset < text.length) {
        if (inComment) {
          const end = text.indexOf("-->", offset);
          if (end === -1) break;
          offset = end + 3;
          inComment = false;
        } else {
          token.lastIndex = offset;
          const next = token.exec(text);
          if (!next) return visible + text.slice(offset);
          const start = next.index;
          visible += text.slice(offset, start);
          let escapes = 0;
          for (let i = start - 1; i >= 0 && text[i] === "\\"; i -= 1) escapes += 1;
          const codeEnd = next[0][0] === "`" && escapes % 2 === 0 && codeEnds.get(start);
          if (codeEnd) {
            visible += text.slice(start, codeEnd);
            offset = codeEnd;
          } else if (next[0] === "<!--" && escapes % 2 === 0) {
            offset = start + 4;
            inComment = true;
          } else {
            visible += next[0];
            offset = start + next[0].length;
          }
        }
      }
      return visible;
    }).join("");
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
  return /^(?:v?\d+\.\d+\.\d+\b|(?:pulse|server(?:[ \t]*\+[ \t]*pulse-agent)?)[ \t]+v?\d+\.\d+\.\d+\b|(?:[a-z0-9._/-]+\/)?pulse:|pulse[-_])/i.test(visible)
    ? normalizeVersion(visible) : null;
}

function extractPulseVersion(title, body) {
  if (body) {
    const lines = declarationLines(body);
    const versionHeading = lines.findIndex((line) =>
      line.heading !== null && /^Pulse[ \t]+version$/i.test(line.heading)
    );
    if (versionHeading !== -1) {
      // The explicit running-version field is authoritative, even when it is
      // incomplete. A title may name the old image in an upgrade report, and
      // neighbouring fields may contain an unrelated agent version.
      return normalizeLegacyVersionValue(firstFieldValue(lines.slice(versionHeading + 1)));
    }
    for (let i = 0; i < lines.length; i += 1) {
      if (!lines[i].declaration) continue;
      const line = (lines[i].heading ?? lines[i].text).replace(/\*\*|__/g, "")
        .replace(/^[ \t]*[-*][ \t]+/, "").trim();
      const field = line.match(
        /^Pulse[ \t]*(?:[|-][ \t]*)?version(?:[ \t]*[:|][ \t]*|[ \t]+|$)(.*)$/i
      );
      if (!field) continue;

      // Legacy inline/standalone fields have the same authority as the form.
      // Unknown or incomplete must not borrow an agent/platform version or
      // an upgrade's starting version from the title.
      const inlineValue = field[1].trim();
      if (inlineValue) return normalizeLegacyVersionValue(inlineValue);
      // A declared value may itself be fenced; that does not turn headings
      // elsewhere in a fenced log into declarations.
      return normalizeLegacyVersionValue(firstFieldValue(lines.slice(i + 1)));
    }
  }

  return normalizeVersion(title);
}

function classifyV6FeedbackType(body) {
  const feedbackType = extractSectionValue(body, "Feedback type");
  if (!feedbackType) return null;

  // This is a single-choice field, not the later examples pasted beneath it.
  const normalized = firstFieldValue(declarationLines(feedbackType)).toLowerCase();
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
