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

// Use the same locked GFM lexer as Pulse's Markdown surfaces, without
// rendering or executing any issue content. Only top-level declarations count.
const { lexer } = require("marked");

function inlineProjection(tokens, maskCode = false) {
  return (tokens || []).map((token) => {
    const raw = token.raw || "";
    // Keep line positions when removing hidden comments. A comment inside a
    // code span is a literal, not an HTML token, and remains evidence.
    if (token.type === "html" && /^<!--/.test(raw)) {
      return raw.replace(/[^\r\n]/g, "");
    }
    if (maskCode && token.type === "codespan") {
      return raw.replace(/[^\r\n]/g, " ");
    }
    if (token.tokens && typeof token.text === "string") {
      const offset = raw.indexOf(token.text);
      if (offset !== -1) {
        return raw.slice(0, offset) + inlineProjection(token.tokens, maskCode) +
          raw.slice(offset + token.text.length);
      }
    }
    return raw;
  }).join("");
}

function declarationLines(body) {
  const lines = [];
  const append = (text, declaration = false, metadata = text) => {
    const visible = text.split(/\r?\n/);
    const declared = metadata.split(/\r?\n/);
    visible.forEach((value, i) => lines.push({
      text: value,
      metadata: declared[i] || "",
      delimiter: false,
      declaration,
      heading: null,
    }));
  };
  for (const token of lexer(String(body || ""), { gfm: true })) {
    if (token.type === "heading") {
      const text = inlineProjection(token.tokens).trim();
      lines.push({ text, metadata: text, delimiter: false, declaration: true, heading: text });
    } else if (token.type === "paragraph") {
      append(inlineProjection(token.tokens), true, inlineProjection(token.tokens, true));
    } else if (token.type === "code") {
      // A real version field may have a fenced or indented value. Its contents
      // remain a value, never new metadata or a section boundary.
      append(token.text);
    } else if (token.type === "list") {
      // Preserve legacy "- **Pulse version**: ..." declarations. Only an
      // item's first line can declare one; lazy/nested copied content cannot.
      for (const item of token.items) {
        const first = item.tokens[0];
        if (first && first.type === "text" && first.tokens) {
          const text = inlineProjection(first.tokens);
          const metadata = inlineProjection(first.tokens, true);
          append(text.split(/\r?\n/)[0], true, metadata.split(/\r?\n/)[0]);
          append(text.split(/\r?\n/).slice(1).join("\n"));
          for (const nested of item.tokens.slice(1)) append(nested.raw || "");
        } else {
          append(item.raw);
        }
      }
    } else if (token.type === "html" && /^\s*<!--/.test(token.raw)) {
      append("");
    } else {
      // Block quotes (including lazy continuation), HTML, tables and nested
      // examples are evidence, not top-level form structure.
      append(token.raw || "");
    }
  }
  return lines;
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

function normalizedTopicField(body) {
  const value = extractSectionValue(body, "Additional actionable topics", [
    "Pulse version",
    "Additional context",
    "Logs, screenshots, or diagnostics",
    "Confirmations",
  ]);
  if (value === null) return null;

  return value
    .trim()
    .toLowerCase()
    .replace(/\s+/g, " ")
    .replace(/[.!]+$/g, "");
}

function classifyAdditionalActionableTopics(body) {
  const normalized = normalizedTopicField(body);
  if (normalized === null) return null;
  if (!normalized || !/[\p{L}\p{N}]/u.test(normalized)) return false;

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
  // Failed-install forms explicitly accept an attempted server archive. Its
  // packaging suffix is not part of the release version. Recognise only the
  // server archive names; companion binaries and signatures cannot establish
  // the attempted server executable's version.
  const filename = visible.match(/^pulse-[^\s`*_]+/i)?.[0];
  if (filename && /\.(?:tar\.gz|tgz|zip|exe|sha256|sig|sshsig)(?:\.|$)/i.test(filename)) {
    const archive = filename.match(
      /^pulse-v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)\.(?:tar\.gz|tgz)$/i
    );
    return archive ? normalizeVersion(archive[1].replace(
      /-linux-(?:386|amd64|arm64|armv6|armv7)$/i, ""
    )) : null;
  }
  // Keep an explicitly shared server/agent version (as in #1788), without
  // mining a sentence such as "unknown; agent version 6.5.0" for a number.
  return /^(?:v?\d+\.\d+\.\d+\b|(?:pulse|server(?:[ \t]*\+[ \t]*pulse-agent)?)[ \t]+v?\d+\.\d+\.\d+\b|(?:[a-z0-9._/-]+\/)?pulse:|pulse[-_])/i.test(visible)
    ? normalizeVersion(visible) : null;
}

function pulseVersionEvidence(title, body) {
  if (body) {
    const lines = declarationLines(body);
    const versionHeading = lines.findIndex((line) =>
      line.heading !== null && /^Pulse[ \t]+version$/i.test(line.heading)
    );
    if (versionHeading !== -1) {
      // The explicit running-version field is authoritative, even when it is
      // incomplete. A title may name the old image in an upgrade report, and
      // neighbouring fields may contain an unrelated agent version.
      return {
        version: normalizeLegacyVersionValue(firstFieldValue(lines.slice(versionHeading + 1))),
        declared: true,
      };
    }
    for (let i = 0; i < lines.length; i += 1) {
      if (!lines[i].declaration) continue;
      const line = (lines[i].heading ?? lines[i].metadata).replace(/\*\*|__/g, "")
        .replace(/^[ \t]*[-*][ \t]+/, "").trim();
      const field = line.match(
        /^Pulse[ \t]*(?:[|-][ \t]*)?version(?:[ \t]*[:|][ \t]*|[ \t]+|$)(.*)$/i
      );
      if (!field) continue;

      // Legacy inline/standalone fields have the same authority as the form.
      // Unknown or incomplete must not borrow an agent/platform version or
      // an upgrade's starting version from the title.
      const visibleLine = (lines[i].heading ?? lines[i].text).replace(/\*\*|__/g, "")
        .replace(/^[ \t]*[-*][ \t]+/, "").trim();
      const visibleField = visibleLine.match(
        /^Pulse[ \t]*(?:[|-][ \t]*)?version(?:[ \t]*[:|][ \t]*|[ \t]+|$)(.*)$/i
      );
      const inlineValue = visibleField ? visibleField[1].trim() : "";
      if (inlineValue) {
        return { version: normalizeLegacyVersionValue(inlineValue), declared: true };
      }
      // A declared value may itself be fenced; that does not turn headings
      // elsewhere in a fenced log into declarations.
      return {
        version: normalizeLegacyVersionValue(firstFieldValue(lines.slice(i + 1))),
        declared: true,
      };
    }
  }

  // A starting version named in an upgrade title is not a running-version
  // declaration. Copied examples may leave no usable body field at all.
  const upgradeFrom = /\b(?:upgrad(?:e|ed|ing)|migrat(?:e|ed|ing))\b[^\n]*\bfrom\b/i.test(String(title || ""));
  return { version: upgradeFrom ? null : normalizeVersion(title), declared: false };
}

function extractPulseVersion(title, body) {
  return pulseVersionEvidence(title, body).version;
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

function buildTriageState(issue, core, latestVersion, feedbackTypeChanged = false) {
  const labelNames = new Set((issue.labels || []).map((label) => label.name));
  const nextLabels = new Set(labelNames);
  const v6FeedbackClass = classifyV6FeedbackType(issue.body);
  // The form's original choice is not a whole-thread disposition. Community
  // may have reclassified the report since intake. An unrelated edit, reopen
  // or delayed event must not restore a conflicting form label. A current
  // change of feedback type can add its new class, but never removes existing
  // classes: the report may contain several independently actionable topics.
  const hasClassification = [BUG_LABEL, DOCS_LABEL, ENHANCEMENT_LABEL]
    .some((name) => labelNames.has(name));
  if (v6FeedbackClass && (!hasClassification || feedbackTypeChanged)) {
    core.info(`Detected v6 feedback issue class: ${v6FeedbackClass}`);
    nextLabels.add(v6FeedbackClass);
  }

  const hasAdditionalActionableTopics = classifyAdditionalActionableTopics(issue.body);

  const { version: reportedVersion, declared: hasDeclaredPulseVersion } =
    pulseVersionEvidence(issue.title, issue.body);
  core.info(`Reported Pulse version: ${reportedVersion || "not found"}`);
  if (latestVersion) core.info(`Latest stable release: ${latestVersion}`);

  return {
    labelNames,
    nextLabels,
    reportedVersion,
    hasDeclaredPulseVersion,
    v6FeedbackClass,
    hasAdditionalActionableTopics,
    isBugLike: nextLabels.has(BUG_LABEL),
    comparison:
      reportedVersion && latestVersion ? compareCore(reportedVersion, latestVersion) : null,
  };
}

function isExactVersionLabel(label) {
  return /^affects-\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(label);
}

// Apply only this event's classification delta. Replacing the complete label
// set would overwrite Community changes made after the event was queued.
async function applyLabelDelta(github, context, issue, before, after, core) {
  const target = {
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issue.number,
  };
  const additions = [...after].filter((name) => !before.has(name)).sort();
  const removals = [...before].filter((name) => !after.has(name)).sort();
  if (!additions.length && !removals.length) return;

  // Label lookup/creation can take time after the initial report read. Do not
  // apply that prepared decision to a report since edited, closed or reviewed.
  // This narrows the race; GitHub's separate label APIs are not a transaction.
  const { data: current } = await github.rest.issues.get(target);
  if (!current || current.number !== issue.number) {
    throw new Error("Current issue identity did not match the triage event");
  }
  const currentLabels = new Set((current.labels || []).map((label) => label.name));
  if (current.state !== "open" || current.pull_request ||
      ["title", "body", "updated_at"].some((field) => current[field] !== issue[field]) ||
      currentLabels.size !== before.size ||
      [...before].some((name) => !currentLabels.has(name))) {
    core.info("Issue changed during metadata preparation. Skipping outdated label delta.");
    return;
  }

  if (additions.length) {
    await github.rest.issues.addLabels({ ...target, labels: additions });
  }
  for (const name of removals) {
    try {
      await github.rest.issues.removeLabel({ ...target, name });
    } catch (error) {
      // Another synchronizer may already have removed this label.
      if (error.status !== 404) throw error;
    }
  }
}

async function newlyDeclaredTopics({ github, context, issue, eventIsCurrent, core }) {
  const eventIssue = context.payload.issue;
  const previousBody = context.payload.changes?.body?.from;
  const eventTopics = normalizedTopicField(eventIssue.body);
  const declared = classifyAdditionalActionableTopics(eventIssue.body) === true && (
    context.payload.action === "opened" ||
    (context.payload.action === "edited" && typeof previousBody === "string" &&
      normalizedTopicField(previousBody) !== eventTopics)
  );
  if (!declared || eventTopics !== normalizedTopicField(issue.body)) return false;
  if (eventIsCurrent) return true;

  // A comment or unrelated edit can overtake a queued declaration. That does
  // not dispose of its topics, but an explicit label removal may have done so.
  // Read every page of this issue's events rather than guessing from its age
  // or checking only the latest page. Missing/failed history must not authorise
  // a mutation. Keep the normal fresh pre-write issue check as well.
  const timestamp = (value) => typeof value === "string" &&
    /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/.test(value) ? Date.parse(value) : NaN;
  const declaredAt = timestamp(eventIssue.updated_at);
  const currentAt = timestamp(issue.updated_at);
  if (!Number.isFinite(declaredAt) || !Number.isFinite(currentAt) || currentAt < declaredAt ||
      (issue.labels || []).some((label) => label.name === NEEDS_DECOMPOSITION_LABEL)) return false;

  const events = await github.paginate(github.rest.issues.listEvents, {
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issue.number,
    per_page: 100,
  });
  if (!Array.isArray(events)) throw new Error("Issue event history was unavailable");
  const reviewed = events.some((event) => {
    if (event.event !== "unlabeled" || event.label?.name !== NEEDS_DECOMPOSITION_LABEL) return false;
    const removedAt = timestamp(event.created_at);
    return !Number.isFinite(removedAt) || removedAt >= declaredAt;
  });
  if (reviewed) {
    core.info("Community removed the decomposition label after this declaration; preserving its disposition.");
    return false;
  }
  return true;
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
  const previousBody = context.payload.changes?.body?.from;
  const eventIsCurrent = typeof issue.updated_at === "string" &&
    issue.body === eventIssue.body &&
    issue.updated_at === eventIssue.updated_at;
  const feedbackTypeChanged = eventIsCurrent && context.payload.action === "edited" &&
    typeof previousBody === "string" &&
    classifyV6FeedbackType(previousBody) !== classifyV6FeedbackType(issue.body);
  const {
    labelNames,
    nextLabels,
    reportedVersion,
    hasDeclaredPulseVersion,
    hasAdditionalActionableTopics,
    isBugLike,
  } = buildTriageState(issue, core, null, feedbackTypeChanged);

  // A new or materially edited nonempty field needs review even if the old
  // declaration already had topics. Comparing only its boolean classification
  // loses an added symptom after Community disposed of the earlier topics.
  // Ignore case/whitespace and hidden comments. A delayed declaration can still
  // need review; use explicit label history to distinguish that from a task
  // Community has cleared. This flags review, not a new issue or a disposition.
  const newlyDeclared = hasAdditionalActionableTopics === true && await newlyDeclaredTopics({
    github, context, issue, eventIsCurrent, core,
  });
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
    await applyLabelDelta(github, context, issue, labelNames, nextLabels, core);
    return;
  }

  // The original field is not the whole thread: Community may have established
  // later affected versions from retests. Only a current edit that changes the
  // declaration can correct its previous version; an unrelated edit, reopen or
  // superseded event must not replay the original field over that evidence.
  const previousVersion = typeof previousBody === "string"
    ? pulseVersionEvidence("", previousBody) : null;
  const versionCorrected = eventIsCurrent && context.payload.action === "edited" &&
    hasDeclaredPulseVersion && previousVersion !== null &&
    (previousVersion.declared !== hasDeclaredPulseVersion ||
      previousVersion.version !== reportedVersion);
  if (!versionCorrected && [...labelNames].some(isExactVersionLabel)) {
    core.info("No current version-field correction; preserving established whole-thread version metadata.");
    await applyLabelDelta(github, context, issue, labelNames, nextLabels, core);
    return;
  }

  if (versionCorrected && previousVersion.declared && previousVersion.version) {
    nextLabels.delete(`${VERSION_LABEL_PREFIX}${previousVersion.version}`);
  }
  if (reportedVersion) {
    await ensureLabel(
      github,
      context,
      `${VERSION_LABEL_PREFIX}${reportedVersion}`,
      "0e8a16",
      `Bug reported against Pulse ${reportedVersion}`
    );
    nextLabels.add(`${VERSION_LABEL_PREFIX}${reportedVersion}`);
    nextLabels.delete(NEEDS_VERSION_LABEL);
  } else if ([...nextLabels].some(isExactVersionLabel)) {
    // Correcting this field to unknown does not invalidate an independently
    // established version from another part of the report.
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
  await applyLabelDelta(github, context, issue, labelNames, nextLabels, core);
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
