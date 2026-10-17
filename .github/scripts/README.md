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
- Preserve the immediate pre-write check of issue identity, open state, title,
  body, update time and label set. Separate GitHub label APIs are not atomic;
  this check narrows that race but cannot eliminate it.

Secondary-topic classification still runs independently. Preserving version
labels must not hide a new symptom or restore a completed decomposition task
on an unrelated edit. Retest and human-ownership labels remain Community-owned.

## Checks

Run `npm ci --ignore-scripts` and `npm run test:triage` in this directory. The
workflow uses its pinned lock and runs the tests before obtaining a write token.
The tests exercise the actual `syncLabels` consumer against in-memory GitHub
interfaces, including stale reads, access errors and independent label changes;
they make no provider calls. Workflow activation and live provider acceptance
remain distinct from source tests.
