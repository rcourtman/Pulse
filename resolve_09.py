#!/usr/bin/env python3
"""
Resolve merge conflicts for parallel-09-logging-consistency.

This branch adds structured logging context fields (.Str("remote_addr", ...) etc.)
and standardizes log message capitalization.

Strategy for each conflict:
- Start with HEAD's version (has robust closeConn(), DecodePayload() etc.)
- Graft in the branch's .Str() logging context fields onto HEAD's log lines
- Adopt the branch's message capitalization (lowercase first letter)
"""
import re
import subprocess
import sys


def get_conflicted_files():
    result = subprocess.run(
        ['git', 'diff', '--name-only', '--diff-filter=U'],
        capture_output=True, text=True
    )
    return [f.strip() for f in result.stdout.strip().split('\n') if f.strip()]


def extract_log_fields(line):
    """Extract .Str/.Int/.Bool/.Float64/.Dur/.Time field calls from a log line."""
    fields = re.findall(r'\.\s*(Str|Int|Bool|Float64|Dur|Time|Strs)\s*\([^)]*\)', line)
    return fields


def extract_full_fields(line):
    """Extract full field call strings like .Str("remote_addr", remoteAddr) from a log line."""
    return re.findall(r'\.\s*(?:Str|Int|Bool|Float64|Dur|Time|Strs)\s*\([^)]*\)', line)


def get_msg_content(line):
    """Extract the .Msg("...") content from a log line."""
    match = re.search(r'\.Msg\("([^"]*)"\)', line)
    return match.group(1) if match else None


def merge_log_line(head_line, branch_line):
    """
    Merge a log line: keep HEAD's structure but add branch's extra fields
    and adopt branch's message text.
    """
    # Extract fields from both
    head_fields = extract_full_fields(head_line)
    branch_fields = extract_full_fields(branch_line)
    
    # Find fields in branch that aren't in head
    head_field_set = set(f.strip() for f in head_fields)
    new_fields = [f for f in branch_fields if f.strip() not in head_field_set]
    
    if not new_fields:
        # No new fields, but maybe adopt branch's message capitalization
        branch_msg = get_msg_content(branch_line)
        head_msg = get_msg_content(head_line)
        if branch_msg and head_msg and branch_msg.lower() == head_msg.lower():
            return head_line.replace(f'.Msg("{head_msg}")', f'.Msg("{branch_msg}")')
        return head_line
    
    # Insert new fields before .Msg(
    msg_match = re.search(r'\.Msg\(', head_line)
    if msg_match:
        insert_pos = msg_match.start()
        fields_str = ''.join(new_fields)
        result = head_line[:insert_pos] + fields_str + head_line[insert_pos:]
        
        # Also adopt branch's message text
        branch_msg = get_msg_content(branch_line)
        head_msg = get_msg_content(result)
        if branch_msg and head_msg:
            result = result.replace(f'.Msg("{head_msg}")', f'.Msg("{branch_msg}")')
        
        return result
    
    return head_line


def resolve_conflict(head_content, branch_content):
    """Resolve a single conflict block by combining both sides."""
    head_stripped = head_content.strip()
    branch_stripped = branch_content.strip()
    
    # If HEAD is empty, accept the branch addition
    if not head_stripped:
        return branch_content
    
    # If branch is empty, keep HEAD (branch removed something)
    if not branch_stripped:
        return head_content
    
    head_lines = head_content.rstrip('\n').split('\n')
    branch_lines = branch_content.rstrip('\n').split('\n')
    
    # Check if both sides have log statements - merge the logging fields
    head_has_log = any('log.' in l for l in head_lines)
    branch_has_log = any('log.' in l for l in branch_lines)
    
    if head_has_log and branch_has_log:
        # Try to merge logging: for each HEAD log line, find matching branch log line
        # and add any extra fields
        result_lines = []
        for h_line in head_lines:
            if 'log.' in h_line and '.Msg(' in h_line:
                # Find corresponding branch line by matching Msg content (case-insensitive)
                h_msg = get_msg_content(h_line) 
                matched = False
                for b_line in branch_lines:
                    if '.Msg(' in b_line:
                        b_msg = get_msg_content(b_line)
                        if h_msg and b_msg and (h_msg.lower() == b_msg.lower() or 
                                                 h_msg.lower().startswith(b_msg.lower()[:20]) or
                                                 b_msg.lower().startswith(h_msg.lower()[:20])):
                            merged = merge_log_line(h_line, b_line)
                            result_lines.append(merged)
                            matched = True
                            break
                if not matched:
                    # Multi-line log - look for fields in branch
                    result_lines.append(h_line)
            elif ('log.' in h_line or '.Err(' in h_line or '.Str(' in h_line or 
                  '.Int(' in h_line or '.Msg(' in h_line) and '.Msg(' not in h_line:
                # Part of a multi-line log call - keep it
                result_lines.append(h_line)
            else:
                result_lines.append(h_line)
        
        # Check if branch has extra lines not in HEAD (like additional fields in multi-line format)
        # For multi-line log calls in branch, check for .Str lines before .Msg
        for b_line in branch_lines:
            b_stripped = b_line.strip()
            if b_stripped.startswith('.Str(') or b_stripped.startswith('.Int(') or b_stripped.startswith('.Bool('):
                # Check this field isn't already in result
                field_content = b_stripped
                already_present = any(field_content in r for r in result_lines)
                if not already_present:
                    # Find where to insert: before the .Msg line
                    for i, r_line in enumerate(result_lines):
                        if '.Msg(' in r_line:
                            result_lines.insert(i, h_line.replace(h_line.strip(), b_stripped))
                            break
        
        return '\n'.join(result_lines) + '\n'
    
    # For non-logging conflicts, we need to keep HEAD's structure (closeConn, DecodePayload)
    # but check if branch has anything HEAD doesn't
    return head_content


def resolve_file(filepath):
    """Resolve all conflicts in a file."""
    with open(filepath, 'r') as f:
        content = f.read()

    max_iterations = 50
    for iteration in range(max_iterations):
        # Match innermost standard conflicts
        pattern = r'<{7} [^\n]*\n((?:(?!<{7})(?!>{7})(?!={7}).)*?)={7}[^\n]*\n((?:(?!<{7})(?!>{7})(?!={7}).)*?)>{7} [^\n]*\n'
        
        def resolver(m):
            return resolve_conflict(m.group(1), m.group(2))
        
        new_content = re.sub(pattern, resolver, content, flags=re.DOTALL)
        
        # Handle malformed markers (no =======)
        if new_content == content:
            pattern2 = r'<{7} [^\n]*\n((?:(?!<{7})(?!>{7}).)*?)>{7} [^\n]*\n'
            new_content = re.sub(pattern2, r'\1', content, flags=re.DOTALL)
        
        if new_content == content:
            break
        content = new_content

    remaining = content.count('<<<<<<<')
    with open(filepath, 'w') as f:
        f.write(content)
    
    if remaining > 0:
        print(f"  WARNING: {remaining} unresolved markers in {filepath}")
        return False
    return True


def main():
    files = get_conflicted_files()
    print(f"Resolving {len(files)} conflicted files for parallel-09-logging-consistency...")
    print("Strategy: keep HEAD structure + graft branch logging context fields")
    
    success = 0
    failed = []
    for f in files:
        print(f"  Processing: {f}")
        if resolve_file(f):
            success += 1
        else:
            failed.append(f)

    print(f"\nResults: {success}/{len(files)} resolved")
    if failed:
        print("Files needing manual review:")
        for f in failed:
            print(f"  - {f}")
    
    return 0 if not failed else 1


if __name__ == '__main__':
    sys.exit(main())
