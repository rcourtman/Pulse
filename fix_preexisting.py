#!/usr/bin/env python3
"""
Fix pre-existing conflict markers in committed files.
These were left from the parallel-05-error-handling merge.

For each conflict:
- If it's between HEAD (the pre-merge state) and parallel-05-error-handling,
  we want the parallel-05 side (error handling improvements) since that was
  the branch being merged.
- If no ======= separator, just strip the markers.
"""
import re
import sys
import os


def fix_file(filepath):
    with open(filepath, 'r') as f:
        content = f.read()
    
    original_count = content.count('<<<<<<<')
    if original_count == 0:
        return True
    
    max_iterations = 50
    for iteration in range(max_iterations):
        # Standard conflict markers with ======= 
        pattern = r'<{7} [^\n]*\n((?:(?!<{7})(?!>{7})(?!={7}).)*?)={7}[^\n]*\n((?:(?!<{7})(?!>{7})(?!={7}).)*?)>{7} [^\n]*\n'
        
        def pick_branch(m):
            head_content = m.group(1)
            branch_content = m.group(2)
            # For parallel-05-error-handling conflicts, prefer the branch side
            # (those were the error handling improvements being merged in)
            if not branch_content.strip():
                return head_content
            if not head_content.strip():
                return branch_content
            # Prefer the branch side (error handling improvements)
            return branch_content
        
        new_content = re.sub(pattern, pick_branch, content, flags=re.DOTALL)
        
        # Malformed markers (no =======)
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
        print(f"  WARNING: {remaining} markers remain in {filepath}")
        return False
    else:
        print(f"  Fixed {original_count} conflict(s) in {filepath}")
        return True


def main():
    files = [
        "frontend-modern/src/api/ai.ts",
        "frontend-modern/src/api/aiChat.ts",
        "frontend-modern/src/utils/__tests__/apiClient.org.test.ts",
        "frontend-modern/src/utils/apiClient.ts",
        "internal/cloudcp/account/tenant_handlers.go",
        "internal/cloudcp/admin/handlers.go",
        "internal/cloudcp/auth/handlers.go",
        "internal/hostagent/commands.go",
        "internal/hostagent/mdadm_test.go",
        "internal/license/conversion/store.go",
        "internal/notifications/queue.go",
        "internal/notifications/webhook_enhanced.go",
        "internal/notifications/webhook_enhanced_test.go",
        "internal/remoteconfig/client.go",
        "internal/sensors/power.go",
        "internal/ssh/knownhosts/manager.go",
        "internal/updates/manager.go",
    ]
    
    print(f"Fixing {len(files)} files with pre-existing conflict markers...")
    success = 0
    failed = []
    for f in files:
        if not os.path.exists(f):
            print(f"  SKIP (not found): {f}")
            continue
        if fix_file(f):
            success += 1
        else:
            failed.append(f)
    
    print(f"\nResults: {success} fixed, {len(failed)} need manual attention")
    if failed:
        for f in failed:
            print(f"  - {f}")
    return 0 if not failed else 1


if __name__ == '__main__':
    sys.exit(main())
