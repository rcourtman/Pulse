#!/bin/bash
# session-handoff.sh — Merge a session handoff entry into MEMORY.md under flock.
#
# Usage: scripts/session-handoff.sh <session_id> <handoff_file>
#
# The handoff_file should contain a short markdown snippet (written by Claude)
# with: date, branch, summary, in-progress items, decisions.
#
# This script:
#   1. Acquires flock on a lockfile (safe for parallel sessions)
#   2. Reads the existing MEMORY.md
#   3. Inserts the new entry into the "## Session Handoff Log" section
#   4. Trims to MAX_ENTRIES (oldest dropped)
#   5. Atomic-writes back via temp file + mv
#
# Exit 0 on success, 1 on error.

set -eo pipefail

MAX_ENTRIES=8
SECTION_HEADER="## Session Handoff Log"
ENTRY_DELIM="---"

SESSION_ID="${1:-}"
HANDOFF_FILE="${2:-}"

if [ -z "$SESSION_ID" ] || [ -z "$HANDOFF_FILE" ]; then
  echo "Usage: session-handoff.sh <session_id> <handoff_file>" >&2
  exit 1
fi

if [ ! -f "$HANDOFF_FILE" ]; then
  echo "Error: handoff file not found: $HANDOFF_FILE" >&2
  exit 1
fi

# Validate handoff file is not empty
if [ ! -s "$HANDOFF_FILE" ]; then
  echo "Error: handoff file is empty" >&2
  exit 1
fi

# Resolve memory file path
MEMORY_DIR="$HOME/.claude/projects/-Volumes-Development-pulse-repos-pulse/memory"
MEMORY_FILE="$MEMORY_DIR/MEMORY.md"
LOCK_FILE="$MEMORY_DIR/.handoff.lock"

mkdir -p "$MEMORY_DIR"

# Perform the merge under flock
(
  flock -w 10 200 || { echo "Error: could not acquire handoff lock" >&2; exit 1; }

  # If MEMORY.md doesn't exist, create a minimal one
  if [ ! -f "$MEMORY_FILE" ]; then
    echo "# Pulse Development Notes" > "$MEMORY_FILE"
  fi

  TMPFILE=$(mktemp "$MEMORY_DIR/.memory.XXXXXX")

  # Use awk to parse MEMORY.md and merge the new entry.
  # Pass the handoff file as a separate awk input (read via FILENAME/NR/FNR).
  awk -v section_header="$SECTION_HEADER" \
      -v entry_delim="$ENTRY_DELIM" \
      -v max_entries="$MAX_ENTRIES" \
      -v handoff_file="$HANDOFF_FILE" \
  '
  BEGIN {
    phase = "before"
    section_found = 0
    before_lines = 0
    section_entries = 0
    after_lines = 0
    current_entry = ""

    # Read new entry from handoff file
    new_entry = ""
    while ((getline line < handoff_file) > 0) {
      if (new_entry != "") {
        new_entry = new_entry "\n" line
      } else {
        new_entry = line
      }
    }
    close(handoff_file)
  }

  phase == "before" && $0 == section_header {
    phase = "section"
    section_found = 1
    next
  }

  phase == "before" {
    before[++before_lines] = $0
    next
  }

  phase == "section" && /^#+ / {
    # Hit any markdown heading — switch to after
    phase = "after"
    after[++after_lines] = $0
    next
  }

  phase == "section" {
    # Parse entries delimited by ---
    if ($0 == entry_delim) {
      if (current_entry != "") {
        section_entries++
        entries[section_entries] = current_entry
        current_entry = ""
      }
    } else if ($0 != "") {
      if (current_entry != "") {
        current_entry = current_entry "\n" $0
      } else {
        current_entry = $0
      }
    }
    next
  }

  phase == "after" {
    after[++after_lines] = $0
    next
  }

  END {
    # Capture any trailing entry without final ---
    if (current_entry != "") {
      section_entries++
      entries[section_entries] = current_entry
    }

    # Helper: emit the handoff section block
    # (called at the right position depending on section_found)
    # We use a function-like approach via a flag

    if (section_found) {
      # Section existed — print before, strip trailing blanks
      last_content = before_lines
      while (last_content > 0 && before[last_content] == "") {
        last_content--
      }
      for (i = 1; i <= last_content; i++) {
        print before[i]
      }
    } else {
      # Section did not exist — insert after line 1 (title)
      if (before_lines >= 1) {
        print before[1]
      }
    }

    # Print the handoff section (exactly one blank line before header)
    print ""
    print section_header
    print ""
    print entry_delim
    print new_entry

    keep = max_entries - 1
    printed = 0
    for (i = 1; i <= section_entries && printed < keep; i++) {
      e = entries[i]
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", e)
      if (e != "") {
        print entry_delim
        print entries[i]
        printed++
      }
    }

    print entry_delim

    if (section_found) {
      # Print everything after the section
      if (after_lines > 0) {
        print ""
      }
      for (i = 1; i <= after_lines; i++) {
        print after[i]
      }
    } else {
      # Print remaining "before" lines (lines 2+) as the rest of the file
      print ""
      for (i = 2; i <= before_lines; i++) {
        print before[i]
      }
    }
  }
  ' "$MEMORY_FILE" > "$TMPFILE"

  mv "$TMPFILE" "$MEMORY_FILE"

) 200>"$LOCK_FILE"

# Clean up the handoff temp file
rm -f "$HANDOFF_FILE"

echo "Handoff entry merged into MEMORY.md (session: $SESSION_ID)"
