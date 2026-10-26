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

function inlineProjection(tokens) {
  return (tokens || []).map((token) => {
    const raw = token.raw || "";
    // Keep line positions when removing hidden comments. A comment inside a
    // code span is a literal, not an HTML token, and remains evidence.
    if (token.type === "html" && /^<!--/.test(raw)) {
      return raw.replace(/[^\r\n]/g, "");
    }
    if (token.tokens && typeof token.text === "string") {
      const offset = raw.indexOf(token.text);
      if (offset !== -1) {
        return raw.slice(0, offset) + inlineProjection(token.tokens) +
          raw.slice(offset + token.text.length);
      }
    }
    return raw;
  }).join("");
}

function projectDeclarationLines(tokens) {
  const lines = [];
  const append = (text, declaration = false, metadata = text, fieldText = text) => {
    const visible = text.split(/\r?\n/);
    const declared = metadata.split(/\r?\n/);
    const field = fieldText.split(/\r?\n/);
    visible.forEach((value, i) => lines.push({
      text: value,
      metadata: declared[i] || "",
      fieldText: field[i] || "",
      delimiter: false,
      declaration,
      heading: null,
    }));
  };
  for (const token of tokens) {
    if (token.type === "heading") {
      const text = inlineProjection(token.tokens).trim();
      lines.push({ text, metadata: text, delimiter: false, declaration: true,
        // Field names follow their visible labels, just like field values.
        // Ordinary emphasis or a release/help link must not hide a declared
        // topic or let the next form field become part of that topic. Keep
        // literal code, images, struck-out text and raw HTML non-declarative.
        heading: inlineDeclarationText(token.tokens, true).trim(), headingDepth: token.depth });
    } else if (token.type === "paragraph") {
      // Legacy field labels follow their visible emphasis/link text too.
      // Mask every code line for declaration detection, but retain literal
      // code values separately once a real field label has been established.
      append(inlineProjection(token.tokens), true, inlineDeclarationText(token.tokens, false, true),
        inlineDeclarationText(token.tokens, true));
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
          const metadata = inlineDeclarationText(first.tokens, false, true);
          const fieldText = inlineDeclarationText(first.tokens, true);
          append(text.split(/\r?\n/)[0], true, metadata.split(/\r?\n/)[0], fieldText.split(/\r?\n/)[0]);
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

function extractSectionValues(body, heading, followingHeadings = [], report = parseReportMarkdown(body)) {
  if (!body) return [];
  const lines = report.lines;
  const pattern = new RegExp(`^${escapeRegExp(heading)}$`, "i");
  const boundary = followingHeadings.length
    ? new RegExp(`^(?:${[heading, ...followingHeadings].map(escapeRegExp).join("|")})$`, "i") : null;
  const values = [];
  for (let start = 0; start < lines.length; start += 1) {
    if (lines[start].heading === null || !pattern.test(lines[start].heading)) continue;
    let end = start + 1;
    while (end < lines.length && !(lines[end].heading !== null &&
        (!boundary || (lines[end].headingDepth <= lines[start].headingDepth &&
          boundary.test(lines[end].heading))))) {
      end += 1;
    }
    values.push(lines.slice(start + 1, end).map((line) => line.text).join("\n").trim());
    // A deeper, form-named subheading is part of this value, not another
    // declaration. Only resume at the boundary that ended the field.
    start = end - 1;
  }
  return values;
}

function extractSectionValue(body, heading, followingHeadings = [], report = parseReportMarkdown(body)) {
  const values = extractSectionValues(body, heading, followingHeadings, report);
  // An empty declared field differs from a legacy report with no field.
  return values.length ? values[0] : null;
}

function additionalActionableTopicDeclarations(body, report = parseReportMarkdown(body)) {
  // Edited reports can move this field beside any other form field. Context
  // from those fields must not invent a topic or reopen a completed review.
  // Deeper subheadings still belong to the topic, even with a familiar name.
  const values = extractSectionValues(body, "Additional actionable topics", [
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
  if (!values.length) return null;
  // A reporter may append a second field rather than replace the first.
  // Assess each independently: neither an earlier None nor a later empty
  // field can erase a task. Empty/identical copies and field order alone do
  // not create a new task after Community has completed its review.
  return [...new Set(values.filter((value) => isActionableTopicValue(value, report))
    .map((value) => value.replace(/\s+/g, " ").trim()))].sort();
}

function classifyAdditionalActionableTopics(body, report = parseReportMarkdown(body)) {
  const declarations = additionalActionableTopicDeclarations(body, report);
  return declarations === null ? null : declarations.length > 0;
}

function isActionableTopicValue(value, report) {
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

function inlineDeclarationText(tokens, preserveCode = false, maskCode = false) {
  return (tokens || []).map((token) => {
    // A link's visible label is evidence; its destination and title are not.
    // Unwrap only ordinary inline formatting, not images, struck-out values
    // or HTML. Code spans keep their literal text rather than parsing links
    // or comments inside them.
    if (["link", "strong", "em"].includes(token.type)) {
      return inlineDeclarationText(token.tokens, preserveCode, maskCode);
    }
    if (token.type === "html" && /^<!--/.test(token.raw || "")) {
      return token.raw.replace(/[^\r\n]/g, "");
    }
    if (maskCode && token.type === "codespan") return token.raw.replace(/[^\r\n]/g, " ");
    if (preserveCode && token.type === "codespan") return token.raw;
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
      const visibleLine = (lines[i].heading ?? lines[i].fieldText).replace(/\*\*|__/g, "")
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
  const label = {
    owner: context.repo.owner,
    repo: context.repo.repo,
    name,
  };
  try {
    await github.rest.issues.getLabel(label);
  } catch (error) {
    if (error.status !== 404) throw error;
    try {
      await github.rest.issues.createLabel({ ...label, color, description });
    } catch (creationError) {
      // Concurrent reports can both observe a missing repository-wide label.
      // Reconcile only GitHub's explicit name conflict, without replaying the
      // write or treating other validation/access failures as success.
      const errors = creationError.response?.data?.errors;
      if (creationError.status !== 422 || !Array.isArray(errors) || !errors.length ||
          !errors.every((item) => item?.resource === "Label" &&
            item.field === "name" && item.code === "already_exists")) {
        throw creationError;
      }
      const { data: current } = await github.rest.issues.getLabel(label);
      if (typeof current?.name !== "string" ||
          current.name.toLowerCase() !== name.toLowerCase()) {
        throw creationError;
      }
    }
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

  const additionalTopicDeclarations = additionalActionableTopicDeclarations(issue.body, report);
  const hasAdditionalActionableTopics = additionalTopicDeclarations === null
    ? null : additionalTopicDeclarations.length > 0;

  const reportedVersion = extractPulseVersion(issue.title, issue.body, report);
  core.info(`Reported Pulse version: ${reportedVersion || "not found"}`);
  if (latestVersion) core.info(`Latest stable release: ${latestVersion}`);

  return {
    labelNames,
    nextLabels,
    reportedVersion,
    v6FeedbackClass,
    hasAdditionalActionableTopics,
    additionalTopicDeclarations,
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
  const readCurrent = async () => {
    const { data: current } = await github.rest.issues.get(target);
    if (!current || current.number !== issue.number) {
      throw new Error("Current issue identity did not match the triage event");
    }
    return current;
  };
  const decisionInputsMatch = (current) => {
    const labels = new Set((current.labels || []).map((label) => label.name));
    return current.state === "open" && !current.pull_request &&
      ["title", "body"].every((field) => current[field] === issue[field]) &&
      labels.size === before.size && [...before].every((name) => labels.has(name));
  };
  const changed = () => core.info("Issue changed during metadata preparation. Skipping outdated label delta.");
  const current = await readCurrent();
  if (!decisionInputsMatch(current)) {
    changed();
    return;
  }
  if (["updated_at", "comments"].some((field) => current[field] !== issue[field])) {
    // A comment can arrive during label lookup or the first history read,
    // not just before the job. Preserve only an already eligible declaration,
    // after checking review/lifecycle history from its original event again.
    // Version-only decisions and all changed content/labels stay conservative.
    if (!additions.includes(NEEDS_DECOMPOSITION_LABEL) ||
        !Number.isInteger(issue.comments) || current.comments <= issue.comments ||
        !await declarationSurvivesLaterComments(github, context, current, context.payload.issue)) {
      changed();
      return;
    }
    // History is not a transaction with issue reads. Bind its result to the
    // snapshot it checked; further activity stops here without a write/retry.
    const final = await readCurrent();
    if (!decisionInputsMatch(final) ||
        ["updated_at", "comments"].some((field) => final[field] !== current[field])) {
      changed();
      return;
    }
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

async function declarationSurvivesLaterComments(github, context, issue, eventIssue) {
  // A comment can advance the activity count without changing the declaration.
  // The clock alone must not lose its review task, but identical text cannot prove that
  // Community has not already cleared it. Consult complete, bounded history
  // only for this delayed-comment case; never infer a disposition from silence.
  const eventTime = typeof eventIssue.updated_at === "string" ? Date.parse(eventIssue.updated_at) : NaN;
  const currentTime = typeof issue.updated_at === "string" ? Date.parse(issue.updated_at) : NaN;
  // GitHub's clock has second precision. A larger comment count proves later
  // activity even when both timestamps match; its review history still needs
  // checking before either restoring or discarding this declaration.
  if (!Number.isFinite(eventTime) || !Number.isFinite(currentTime) || currentTime < eventTime ||
      !Number.isInteger(eventIssue.comments) || eventIssue.comments < 0 ||
      !Number.isInteger(issue.comments) || issue.comments <= eventIssue.comments ||
      issue.title !== eventIssue.title || issue.body !== eventIssue.body) {
    return false;
  }

  const perPage = 100;
  for (let page = 1; page <= 10; page += 1) {
    const { data: events } = await github.rest.issues.listEvents({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issue.number,
      per_page: perPage,
      page,
    });
    if (!Array.isArray(events) || events.length > perPage) {
      throw new Error("Issue review history was not a complete event page");
    }
    for (const event of events) {
      const time = typeof event?.created_at === "string" ? Date.parse(event.created_at) : NaN;
      if (typeof event?.event !== "string" || !Number.isFinite(time) ||
          (event.event === "unlabeled" && typeof event.label?.name !== "string")) {
        throw new Error("Issue review history contained an unreadable event");
      }
      // Treat same-second dispositions conservatively. A history newer than
      // our issue snapshot, or an intervening lifecycle change, is not proof
      // that this queued declaration is still eligible.
      if (time > currentTime || (time >= eventTime && (
        (event.event === "unlabeled" &&
          event.label.name.toLowerCase() === NEEDS_DECOMPOSITION_LABEL) ||
        ["closed", "reopened", "transferred", "converted_to_discussion"].includes(event.event)
      ))) return false;
    }
    if (events.length < perPage) return true;
  }
  throw new Error("Issue review history exceeded the bounded event read");
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
    additionalTopicDeclarations,
    isBugLike,
  } = buildTriageState(issue, core, null);

  // A new or revised form declaration creates a review task. A later
  // empty field cannot prove that comment topics were dispositioned, and an
  // unrelated or superseded event must not restore a label Community cleared.
  const previousBody = context.payload.changes?.body?.from;
  const eventIsCurrent = typeof issue.updated_at === "string" &&
    issue.body === eventIssue.body &&
    issue.updated_at === eventIssue.updated_at &&
    issue.comments === eventIssue.comments;
  const declarationChanged = hasAdditionalActionableTopics === true && (
    context.payload.action === "opened" ||
    (context.payload.action === "edited" && previousBody !== undefined &&
      JSON.stringify(additionalActionableTopicDeclarations(previousBody) || []) !==
        JSON.stringify(additionalTopicDeclarations))
  );
  // GitHub's updated_at clock has second precision. A review present in the
  // queued event can already have been cleared in the current snapshot even
  // when both clocks (and comment counts) match. That observed removal takes
  // precedence over the queued declaration; do not restore that review task
  // or require another history read to recognise the label difference.
  const reviewClearedAfterEvent = !labelNames.has(NEEDS_DECOMPOSITION_LABEL) &&
    (eventIssue.labels || []).some((label) => label.name === NEEDS_DECOMPOSITION_LABEL);
  const newlyDeclared = declarationChanged && !reviewClearedAfterEvent &&
    !nextLabels.has(NEEDS_DECOMPOSITION_LABEL) &&
    (eventIsCurrent || await declarationSurvivesLaterComments(github, context, issue, eventIssue));
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
    const versionLabel = `${VERSION_LABEL_PREFIX}${reportedVersion}`;
    // GitHub label names stop at 50 characters. A longer declared version
    // still supplies evidence, but cannot be represented as an exact label.
    // Do not truncate it into a different version, or let that API validation
    // failure prevent independent topic review and stale-metadata removal.
    if (versionLabel.length <= 50) {
      await ensureLabel(
        github,
        context,
        versionLabel,
        "0e8a16",
        `Bug reported against Pulse ${reportedVersion}`
      );
      nextLabels.add(versionLabel);
    } else {
      core.info("Declared version exceeds the exact-version label limit; retaining its evidence without a version label.");
    }
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
