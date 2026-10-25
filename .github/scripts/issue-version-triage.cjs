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
const { Lexer, lexer } = require("marked");

// Keep one immutable Markdown snapshot per classification decision. Reports can
// contain long copied logs; reparsing that body for each field adds work without
// adding evidence. Never share this context across reports or event revisions.
function parseReportMarkdown(body) {
  const tokens = lexer(String(body || ""), { gfm: true });
  return { lines: projectDeclarationLines(tokens), links: tokens.links };
}

function lexFieldValue(value, report) {
  // Reference links can be defined anywhere in the report. Extracting a field
  // must not discard that context and turn visible declarations into missing
  // versions or invented topics. Use only the locked parser's definitions;
  // destinations are never fetched or treated as declaration text.
  const fieldLexer = new Lexer({ gfm: true });
  // Field lexing can add definitions. Do not let it mutate the shared report
  // definitions or change the meaning of a sibling field.
  fieldLexer.tokens.links = Object.assign(Object.create(null), report.links);
  return fieldLexer.lex(value);
}

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

function projectDeclarationLines(tokens) {
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
  for (const token of tokens) {
    if (token.type === "heading") {
      const text = inlineProjection(token.tokens).trim();
      lines.push({ text, metadata: text, delimiter: false, declaration: true,
        heading: text, headingDepth: token.depth });
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
    } else if (token.type === "def" ||
        (token.type === "html" && /^\s*<!--/.test(token.raw))) {
      // Reference definitions resolve visible labels but do not render as
      // field content. Their URLs must not invent secondary issue topics.
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

function extractSectionValue(body, heading, followingHeadings = [], report = parseReportMarkdown(body)) {
  if (!body) return null;
  const lines = report.lines;
  const pattern = new RegExp(`^${escapeRegExp(heading)}$`, "i");
  const start = lines.findIndex((line) => line.heading !== null && pattern.test(line.heading));
  if (start === -1) return null;
  const boundary = followingHeadings.length
    ? new RegExp(`^(?:${followingHeadings.map(escapeRegExp).join("|")})$`, "i") : null;
  let end = start + 1;
  while (end < lines.length && !(lines[end].heading !== null &&
      (!boundary || (lines[end].headingDepth <= lines[start].headingDepth &&
        boundary.test(lines[end].heading))))) {
    end += 1;
  }
  // An empty declared field differs from a legacy report with no field.
  return lines.slice(start + 1, end).map((line) => line.text).join("\n").trim();
}

function additionalActionableTopicsValue(body, report = parseReportMarkdown(body)) {
  // Edited reports can move this field beside any other form field. Context
  // from those fields must not invent a topic or reopen a completed review.
  // Deeper subheadings still belong to the topic, even with a familiar name.
  return extractSectionValue(body, "Additional actionable topics", [
    "Describe the bug",
    "Steps or original sequence",
    "Expected behavior",
    "Feedback type",
    "Pulse version",
    "Last known working Pulse version",
    "Agent version",
    "Image tag or digest",
    "Running Pulse container image tag or digest",
    "Install path",
    "Installation type",
    "OS / environment",
    "Installer or helper source",
    "Upgrade attempt and recovery",
    "License tier",
    "What happened?",
    "Steps or original sequence / usage context",
    "What did you expect instead?",
    "Additional context",
    "Logs, screenshots, or diagnostics",
    "Confirmations",
    "Problem",
    "Current workflow",
    "Desired outcome",
    "Alternatives considered",
  ], report);
}

function classifyAdditionalActionableTopics(body, report = parseReportMarkdown(body)) {
  const value = additionalActionableTopicsValue(body, report);
  if (value === null) return null;

  // Ordinary inline formatting does not turn "None" into a second topic.
  // Keep block evidence, images and any qualifying text intact for review;
  // only a single paragraph can supply an explicit no-topic response.
  const blocks = lexFieldValue(value, report);
  const text = blocks.length === 1 && blocks[0].type === "paragraph"
    ? inlineDeclarationText(blocks[0].tokens) : value;
  const normalized = text
    .trim()
    .toLowerCase()
    .replace(/[.!]+$/g, "");
  if (!normalized || !/[\p{L}\p{N}]/u.test(normalized)) return false;

  return !new Set([
    "_no response_",
    "no response",
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

function declaredVersionIsTransition(value) {
  const first = normalizeVersion(value);
  if (!first) return false;
  const remaining = value.slice(value.indexOf(first) + first.length);
  // A source -> target sequence is not a declaration of the failing runtime.
  // Neither endpoint proves that the upgrade completed or which version was
  // observed after recovery. Do not infer either one, or mine another field.
  return /^[ \t]*(?:[-=]>|[→←↔]|&(?:rarr|larr|harr|#8594|#x2192);|to\b)[ \t]*(?:v?\d|(?:pulse(?:[ \t]+pro)?|server)[ \t]+v?\d|(?:[a-z0-9._/-]+\/)?pulse(?:-pro)?[:_-]v?\d|unknown\b|latest\b)/i.test(remaining);
}

function inlineDeclarationText(tokens) {
  return (tokens || []).map((token) => {
    // A link's visible label is evidence; its destination and title are not.
    // Unwrap only ordinary inline formatting, not images, struck-out values
    // or HTML. Code spans keep their literal text rather than parsing links
    // or comments inside them.
    if (["link", "strong", "em"].includes(token.type)) {
      return inlineDeclarationText(token.tokens);
    }
    if (["codespan", "escape"].includes(token.type)) return token.text;
    return token.raw || "";
  }).join("");
}

function normalizeLegacyVersionValue(value, report) {
  const raw = String(value).trim();
  const blocks = lexFieldValue(raw, report);
  const text = blocks.length === 1 && blocks[0].type === "paragraph"
    ? inlineDeclarationText(blocks[0].tokens) : raw;
  const visible = text.trim().replace(/^[`*_]+/, "");
  // Failed-install forms explicitly accept an attempted server archive. Its
  // packaging suffix is not part of the release version. Recognise only the
  // Community and Pro server archive names; companion binaries and signatures
  // cannot establish the attempted server executable's version.
  const filename = visible.match(/^pulse-[^\s`*_]+/i)?.[0];
  if (filename && /\.(?:tar\.gz|tgz|zip|exe|sha256|sig|sshsig)(?:\.|$)/i.test(filename)) {
    if (declaredVersionIsTransition(visible)) return null;
    const archive = filename.match(
      /^pulse-(?:pro-)?v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)\.(?:tar\.gz|tgz)$/i
    );
    return archive ? normalizeVersion(archive[1].replace(
      /-linux-(?:386|amd64|arm64|armv6|armv7)$/i, ""
    )) : null;
  }
  // Keep an explicitly shared server/agent version (as in #1788), without
  // mining a sentence such as "unknown; agent version 6.5.0" for a number.
  // The complete version must follow its server prefix immediately. An
  // unversioned/incomplete image or a companion binary cannot borrow an
  // agent/platform number later in the value (or one in the registry name).
  const declared = visible.match(
    /^(?:(?:pulse|server(?:[ \t]*\+[ \t]*pulse-agent)?)[ \t]+|(?:[a-z0-9._/-]+\/)?pulse(?:-pro)?:[ \t]*|pulse(?:-pro)?[-_])?v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)\b/i
  );
  if (!declared) return null;
  const version = declared[1];
  return declaredVersionIsTransition(version + visible.slice(declared[0].length))
    ? null : normalizeVersion(version);
}

function extractTitlePulseVersion(title) {
  const text = String(title || "").trim();
  // A starting version in an upgrade title is not a running declaration.
  if (/\b(?:upgrad(?:e|ed|ing)|migrat(?:e|ed|ing))\b[^\n]*\bfrom\b/i.test(text)) return null;

  const version = String.raw`v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?\b`;
  // A title can also name the agent, OS or monitored platform. Prefer a
  // server-qualified value, rather than taking its first arbitrary number.
  const declared = text.match(new RegExp(
    String.raw`\bpulse(?:[ \t]+pro)?[ \t]+(?:version[ \t]*[:=]?[ \t]*)?(${version})`, "i"
  )) || text.match(new RegExp(String.raw`\bpulse(?:-pro)?:(${version})`, "i"));
  if (declared) return declaredVersionIsTransition(text.slice(declared.index))
    ? null : normalizeVersion(declared[1]);

  // Retain legacy version-first titles and the neutral "Bug on v..." form.
  // Do not search through another product's name to reach its version.
  const neutral = text.replace(/^\[(?:bug|v6 pre-release)\][ \t]*:?[ \t]*/i, "");
  const leading = neutral.match(new RegExp(String.raw`^(?:bug[ \t]+on[ \t]+)?(${version})`, "i"));
  return leading && !declaredVersionIsTransition(neutral) ? normalizeVersion(leading[1]) : null;
}

function extractPulseVersion(title, body, report = parseReportMarkdown(body)) {
  if (body) {
    const lines = report.lines;
    const versionHeading = lines.findIndex((line) =>
      line.heading !== null && /^Pulse[ \t]+version$/i.test(line.heading)
    );
    if (versionHeading !== -1) {
      // The explicit running-version field is authoritative, even when it is
      // incomplete. A title may name the old image in an upgrade report, and
      // neighbouring fields may contain an unrelated agent version.
      return normalizeLegacyVersionValue(firstFieldValue(lines.slice(versionHeading + 1)), report);
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
      if (inlineValue) return normalizeLegacyVersionValue(inlineValue, report);
      // A declared value may itself be fenced; that does not turn headings
      // elsewhere in a fenced log into declarations.
      return normalizeLegacyVersionValue(firstFieldValue(lines.slice(i + 1)), report);
    }
  }

  return extractTitlePulseVersion(title);
}

function classifyV6FeedbackType(body, report = parseReportMarkdown(body)) {
  const feedbackType = extractSectionValue(body, "Feedback type", [], report);
  if (!feedbackType) return null;

  // This is a single-choice field, not free-form evidence. A link's target
  // or title must not turn documentation/workflow feedback into a bug, and
  // prose, images or a withdrawn selection cannot supply a selected option.
  const selected = firstFieldValue(parseReportMarkdown(feedbackType).lines);
  const blocks = lexFieldValue(selected, report);
  const visible = blocks.length === 1 && blocks[0].type === "paragraph"
    ? inlineDeclarationText(blocks[0].tokens) : selected;
  const normalized = visible.trim().toLowerCase().replace(/[ \t]+/g, " ");
  switch (normalized) {
    case "bug":
    case "regression":
    case "bug / regression":
    case "upgrade / migration issue":
    case "performance issue":
      return BUG_LABEL;
    case "documentation issue":
      return DOCS_LABEL;
    case "ux / workflow friction":
    case "other actionable feedback":
      return ENHANCEMENT_LABEL;
    default:
      return null;
  }
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
  const report = parseReportMarkdown(issue.body);
  const v6FeedbackClass = classifyV6FeedbackType(issue.body, report);
  if (v6FeedbackClass) {
    core.info(`Detected v6 feedback issue class: ${v6FeedbackClass}`);
    nextLabels.add(v6FeedbackClass);
  }

  const additionalTopicsValue = additionalActionableTopicsValue(issue.body, report);
  const hasAdditionalActionableTopics = classifyAdditionalActionableTopics(issue.body, report);

  const reportedVersion = extractPulseVersion(issue.title, issue.body, report);
  core.info(`Reported Pulse version: ${reportedVersion || "not found"}`);
  if (latestVersion) core.info(`Latest stable release: ${latestVersion}`);

  return {
    labelNames,
    nextLabels,
    reportedVersion,
    v6FeedbackClass,
    hasAdditionalActionableTopics,
    additionalTopicsValue,
    isBugLike: nextLabels.has(BUG_LABEL),
    comparison:
      reportedVersion && latestVersion ? compareCore(reportedVersion, latestVersion) : null,
  };
}

function keepOnlyReportedVersionLabel(nextLabels, reportedVersion) {
  const keep = reportedVersion ? `${VERSION_LABEL_PREFIX}${reportedVersion}` : null;
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
    additionalTopicsValue,
    isBugLike,
  } = buildTriageState(issue, core, null);

  // A new or revised form declaration creates a review task. A later
  // empty field cannot prove that comment topics were dispositioned, and an
  // unrelated or superseded event must not restore a label Community cleared.
  const previousBody = context.payload.changes?.body?.from;
  const topicText = (value) => (value || "").replace(/\s+/g, " ").trim();
  const eventIsCurrent = typeof issue.updated_at === "string" &&
    issue.body === eventIssue.body &&
    issue.updated_at === eventIssue.updated_at;
  const newlyDeclared = eventIsCurrent && hasAdditionalActionableTopics === true && (
    context.payload.action === "opened" ||
    (context.payload.action === "edited" && previousBody !== undefined &&
      topicText(additionalActionableTopicsValue(previousBody)) !== topicText(additionalTopicsValue))
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
    await applyLabelDelta(github, context, issue, labelNames, nextLabels, core);
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

  // A correction to unknown, incomplete or absent version evidence retires
  // this synchronizer's old exact-version assertions too. It must not leave
  // a known affected version beside its missing-version classification.
  keepOnlyReportedVersionLabel(nextLabels, reportedVersion);

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
