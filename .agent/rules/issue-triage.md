---
trigger: always_on
---

# GitHub Issue Triage

When given a GitHub issue or discussion URL, follow this process:

## 1. Check Git History FIRST (before reading issue details)

```bash
# Search for commits referencing the issue number or related keywords
git log --all --since="30 days ago" --format="%h %ai %s" | grep -i "keywords|from|issue"
git log --all --oneline | grep -i "#123\|relevant-keywords" | head -20
```

- Look for commits that reference the issue number
- Check if the issue was already addressed
- Understand recent development context

## 2. Read ALL Comments

```bash
gh issue view 123 --repo owner/repo --json comments --jq '.comments[] | "\(.author.login) (\(.createdAt)): \(.body)"'
```

- NEVER respond based on partial context
- Later comments often contain critical context
- Check if users confirmed fixes or provided updates

## 3. Establish Timeline

- When was the issue created?
- What commits happened after?
- Did users comment after commits to confirm/deny fixes?

## 4. Challenge the Premise

- Does this request make sense architecturally?
- What are the security implications?
- Is this solving the right problem?
- Does similar functionality already exist?
- Is there a configuration or documentation solution instead?

## 5. Commit Message Rules

- Reference issues: "Related to #123" or "Addresses #123"
- NEVER use auto-close keywords: "Fixes #123", "Closes #123", "Resolves #123"
- Users must verify fixes before issues are closed

## 6. Comment Style (when writing responses)

- Direct but user-friendly, write for typical users not developers
- No "we", "us", "our"
- No exclamation marks or enthusiasm
- No bullets/dashes for lists
- No pleasantries ("hope this helps", "let me know")
- Use hedging when uncertain: "might", "could", "possibly"
- For fixes: just say "This will be in the next release"
- NEVER say "build from source" or "pull latest"
- Match the user's effort level in your response
