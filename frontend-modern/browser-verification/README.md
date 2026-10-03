# Browser verification receipts

Each commit that changes user-visible frontend source adds one receipt here.
The file name is the fingerprint of the content the receipt verified, so no
two changes write the same file and open frontend pull requests cannot
conflict on their receipts.

Create the skeleton for the staged change, then fill it in after the browser
pass:

```bash
python3 scripts/release_control/browser_verification_guard.py --write-template
```

Do not edit, rename or delete another change's receipt in a commit that adds
one. A receipt is read from the commit that recorded it, so receipts whose
commits are already on `main` are dead weight; remove them in a change that
records none:

```bash
python3 scripts/release_control/browser_verification_guard.py --prune
```

The rules live in the Frontend Browser Verification Gate section of
`docs/release-control/v6/internal/CANONICAL_DEVELOPMENT_PROTOCOL.md`.
