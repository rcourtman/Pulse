# Issue metadata synchroniser

`issue-version-triage.cjs` classifies issue-form metadata. It does not read the
whole conversation, send replies, request retests, close reports or determine
whether a fix is available. Community reconciles comments, images and releases.

## Version evidence

- When no affected-version label exists, classify the current report's explicit
  Pulse version. Missing evidence may add `needs-version-info`; that label is
  not proof that the complete conversation lacks the version.
- An original form is not the whole thread. Unrelated edits, reopens, missing
  previous-body evidence and superseded events preserve established versions,
  including versions established from later retests.
- Only a still-current `edited` event whose previous body establishes a changed
  version declaration may retire the label for that previous declaration. It
  cannot prune other affected versions. Adding a declaration does not prove
  that independently established labels were wrong.
- Correcting the field to unknown or incomplete adds `needs-version-info` only
  when no other affected-version evidence remains.
- An explicit versioned `pulse` or `pulse-pro` server image can supply this
  field's version, including a tag pinned to a digest. An agent, helper or MCP
  artifact cannot. A mutable tag such as `latest` or a digest alone does not
  supply a release version; do not borrow a later number from adjacent prose.
  This parses supplied metadata, not proof that the image is running or that
  a tag currently resolves to those bytes. It performs no registry lookup.
- For a free-form report without a body declaration, accept only an explicitly
  named Pulse server version/image or a concise release title such as
  `Bug on v6.5.0`. Platform, agent and chart versions are not server versions;
  upgrade/migration titles do not establish the failing executable. An unknown
  body declaration remains authoritative. This conservative fallback neither
  clears established whole-thread labels nor sends a diagnostic question.
- A version field containing a transition or alternatives (for example,
  `6.4.5 → 6.5.0` or `6.4.5 / 6.5.0`) does not identify the failing release.
  Do not choose its first endpoint, including server-image and archive forms.
  A stated current version with separate history, such as
  `6.5.0 (upgraded from 6.4.5)`, still supplies evidence. An ambiguous field
  does not erase versions already established by Community or request a retest.
- The same transition check applies to title fallback: naming the source
  (`Pulse 6.4.5 → 6.5.0 fails`) does not make it the affected release. A mutable
  or unknown destination is also ambiguous. Explicit body fields still take
  precedence, and an unrelated agent or platform version beside a named Pulse
  version does not turn that title into a range.
- Preserve the immediate pre-write check of issue identity, open state, title,
  body, update time and label set. Separate GitHub label APIs are not atomic;
  this check narrows that race but cannot eliminate it.

Repository label definitions can be created by concurrent issue jobs. A definite
GitHub duplicate-name conflict gets one readback of that exact label, not another
creation attempt. Other access, validation and transport failures remain failures;
a missing or mismatched readback cannot authorise issue metadata. The normal fresh
pre-write issue check still runs afterwards. This avoids losing an intake signal
merely because another report first needed the same label.

Secondary-topic classification still runs independently. Preserving version
labels must not hide a new symptom or restore a completed decomposition task
on an unrelated edit. Retest and human-ownership labels remain Community-owned.
Emphasis or inline code around an otherwise empty answer (`**None**`, `N/A`)
does not declare a topic. This is limited to the known empty answers: links,
struck-out text, additional list items and later symptoms still surface
review. An empty answer never clears an existing decomposition task or disposes
of topics elsewhere in the conversation.

A comment or unrelated edit can overtake the event that declared a new topic.
When that same topic field remains in the live report, a delayed declaration
reads the complete, issue-scoped label event history before adding its review
signal. A removal of `needs-decomposition` at or after the declaration preserves
Community's disposition; an earlier removal cannot dispose of the new topic.
Missing timestamps or failed history reads do not authorise a delayed addition.
This remains a review signal, not a disposition, and the normal pre-write issue
check still applies. Separate provider reads and writes are not atomic.

## Checks

Run `npm ci --ignore-scripts` and `npm run test:triage` in this directory. The
workflow uses its pinned lock and runs the tests before obtaining a write token.
The tests exercise the actual `syncLabels` consumer against in-memory GitHub
interfaces, including stale reads, access errors and independent label changes;
they make no provider calls. Workflow activation and live provider acceptance
remain distinct from source tests.
