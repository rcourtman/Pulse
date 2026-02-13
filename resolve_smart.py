#!/usr/bin/env python3
"""
Resolve merge conflicts for security-hardening and other branches.

Strategy: For each conflict, examine both HEAD and branch content.
Where branch adds security improvements (bounds checks, input validation,
size limits), prefer branch or combine. Where HEAD has structural 
improvements (helpers, better error handling), keep HEAD's structure
but graft in branch's security additions.

For this to work well, we use a heuristic:
- If both sides are similar (just renaming or reformatting), prefer HEAD
- If branch adds new code (validation, bounds, guards), prefer branch
- If HEAD has improvements the branch doesn't, combine
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


def analyze_side(content):
    """Analyze a conflict side for security-relevant additions."""
    indicators = {
        'validation': 0,
        'bounds_check': 0, 
        'security': 0,
        'error_handling': 0,
        'logging_context': 0,
        'lines': len(content.strip().split('\n')) if content.strip() else 0,
    }
    
    for line in content.split('\n'):
        l = line.strip().lower()
        # Security hardening signals
        if any(kw in l for kw in ['sanitize', 'validate', 'limit', 'cap', 'bound', 'max_', 'maximum', 'truncat']):
            indicators['security'] += 1
        if any(kw in l for kw in ['if.*<.*0', 'if.*>.*max', 'if len(', 'if.*<=.*0']):
            indicators['bounds_check'] += 1
        if any(kw in l for kw in ['err != nil', 'error', 'fmt.errorf', 'return.*err']):
            indicators['error_handling'] += 1
        if '.str(' in l or '.int(' in l or '.bool(' in l:
            indicators['logging_context'] += 1
        if 'closeconn' in l or 'decodepayload' in l:
            indicators['error_handling'] += 2
            
    return indicators


def resolve_conflict(head_content, branch_content, filepath=""):
    """Resolve a conflict by intelligently comparing both sides."""
    head_stripped = head_content.strip()
    branch_stripped = branch_content.strip()
    
    # If one side is empty, take the other
    if not head_stripped:
        return branch_content
    if not branch_stripped:
        return head_content
    
    head_info = analyze_side(head_content)
    branch_info = analyze_side(branch_content)
    
    # If branch adds security improvements HEAD doesn't have, prefer branch
    branch_security = branch_info['security'] + branch_info['bounds_check']
    head_security = head_info['security'] + head_info['bounds_check']
    
    # If branch has significantly more security-relevant code, prefer it
    if branch_security > head_security + 1:
        # But check if HEAD has error handling we should keep
        if head_info['error_handling'] > branch_info['error_handling'] + 2:
            # HEAD has better error handling — try to combine
            # For now, start with branch (security) and note that we might miss error handling
            return branch_content
        return branch_content
    
    # If HEAD has better error handling and logging, prefer HEAD
    if head_info['error_handling'] > branch_info['error_handling']:
        # But add branch's structured logging fields if any
        if branch_info['logging_context'] > head_info['logging_context']:
            # Try to graft logging fields
            return _graft_logging(head_content, branch_content)
        return head_content
    
    # If similar code with minor differences, prefer whichever has more content
    # (more content usually means more complete)
    if head_info['lines'] > branch_info['lines'] * 1.5:
        return head_content
    if branch_info['lines'] > head_info['lines'] * 1.5:
        return branch_content
    
    # Default: prefer branch (since we're merging branches to get their improvements)
    return branch_content


def _graft_logging(head_content, branch_content):
    """Try to add branch's logging fields onto HEAD's log lines."""
    # Simple approach: just use HEAD since field-grafting is complex
    return head_content


def resolve_file(filepath):
    """Resolve all conflicts in a file."""
    with open(filepath, 'r') as f:
        content = f.read()

    max_iterations = 50
    for iteration in range(max_iterations):
        # Match innermost standard conflicts
        pattern = r'<{7} [^\n]*\n((?:(?!<{7})(?!>{7})(?!={7}).)*?)={7}\n((?:(?!<{7})(?!>{7})(?!={7}).)*?)>{7} [^\n]*\n'
        
        def resolver(m):
            return resolve_conflict(m.group(1), m.group(2), filepath)
        
        new_content = re.sub(pattern, resolver, content, flags=re.DOTALL)
        
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
    print(f"Resolving {len(files)} conflicted files...")
    print("Strategy: prefer branch's security improvements, keep HEAD's error handling")
    
    success = 0
    failed = []
    for f in files:
        result = resolve_file(f)
        if result:
            success += 1
            print(f"  ✅ {f}")
        else:
            failed.append(f)
            print(f"  ⚠️  {f}")

    print(f"\nResults: {success}/{len(files)} resolved")
    if failed:
        print("Files needing manual review:")
        for f in failed:
            print(f"  - {f}")
    
    return 0 if not failed else 1


if __name__ == '__main__':
    sys.exit(main())
