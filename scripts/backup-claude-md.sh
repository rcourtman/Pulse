#!/bin/bash
# Daily LOCAL backup script for CLAUDE.md
# Keeps last 30 days of backups in a LOCAL directory (never committed)

set -e

CLAUDE_MD="/opt/pulse/CLAUDE.md"
BACKUP_DIR="$HOME/.claude-md-backups"  # User's home directory, not in repo
DATE=$(date +%Y-%m-%d)
BACKUP_FILE="${BACKUP_DIR}/CLAUDE.md.${DATE}"

# Create backup directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

# Only create backup if CLAUDE.md exists
if [ ! -f "$CLAUDE_MD" ]; then
    echo "Error: $CLAUDE_MD not found"
    exit 1
fi

# Only create backup if one doesn't exist for today
if [ -f "$BACKUP_FILE" ]; then
    echo "Backup already exists for today: $BACKUP_FILE"
    exit 0
fi

# Create backup
cp "$CLAUDE_MD" "$BACKUP_FILE"
echo "Created backup: $BACKUP_FILE"

# Remove backups older than 30 days
find "$BACKUP_DIR" -name "CLAUDE.md.*" -type f -mtime +30 -delete
echo "Cleaned up backups older than 30 days"

# Show current backups
echo ""
echo "Current backups in $BACKUP_DIR:"
ls -lh "$BACKUP_DIR" | tail -n +2 | wc -l
echo "backups found"
