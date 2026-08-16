# Security Review Finding Template

## Finding

- Title:
- Reviewer:
- Review date:
- Tested commit:
- Pulse deployment mode:
- Affected boundary:
- Suggested severity:

## Summary

Describe the security property that fails and the resulting impact.

## Required access

Describe the attacker's starting access, credentials, role, organization, and
network position.

## Reproduction

Provide the smallest safe sequence that demonstrates the behavior. Use
disposable local data and replace all secrets with inert placeholders.

## Expected behavior

Describe the expected authorization, isolation, storage, or failure behavior.

## Observed behavior

Describe what happened, including stable logs or response fields that support
the finding. Do not include live credentials or customer data.

## Impact

Explain what an attacker can read, change, execute, or prevent. Separate
confirmed impact from plausible follow-on impact.

## Source and evidence

List relevant files, functions, tests, requests, and command output.

## Suggested containment or remediation

Describe immediate containment and a durable fix when known.

## Disclosure preference

State whether the report may be published after remediation and whether the
reviewer wants attribution.
