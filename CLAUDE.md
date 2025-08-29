# Claude Code Access Information

## CRITICAL: Mock Data System for Testing
**Claude must be able to switch between real and mock data instantly when asked.**

### Quick Commands for Claude to Use
```bash
# Switch to mock mode (simulated data)
/opt/pulse/scripts/toggle-mock.sh on

# Switch back to real Proxmox nodes
/opt/pulse/scripts/toggle-mock.sh off

# Check current mode
/opt/pulse/scripts/toggle-mock.sh status

# Edit mock configuration (number of nodes, VMs, etc.)
/opt/pulse/scripts/toggle-mock.sh edit
```

### When to Use Mock Mode
- **User says "use mock data"** → Run `/opt/pulse/scripts/toggle-mock.sh on` immediately
- **User says "use real nodes"** → Run `/opt/pulse/scripts/toggle-mock.sh off` immediately
- **User says "test with X nodes"** → Edit mock.env, set PULSE_MOCK_NODES=X, restart
- **Testing UI features** → Use mock mode with appropriate node count
- **User will frequently ask to switch** - Do it without questioning

### Mock Configuration (/opt/pulse/mock.env)
```bash
PULSE_MOCK_MODE=true          # Enable/disable
PULSE_MOCK_NODES=7            # Number of nodes (test different UI layouts)
PULSE_MOCK_VMS_PER_NODE=5     # VMs per node
PULSE_MOCK_LXCS_PER_NODE=8    # Containers per node
PULSE_MOCK_RANDOM_METRICS=true # Dynamic changing metrics
PULSE_MOCK_STOPPED_PERCENT=20  # % of stopped guests
```

### UI Testing Scenarios
- **1-4 nodes**: Regular node cards
- **5-9 nodes**: Compact cards (default: 7)
- **10+ nodes**: Ultra-compact list
- **25+ nodes**: Stress test

### Example Usage Patterns
```bash
# User: "use mock data with 15 nodes"
nano /opt/pulse/mock.env  # Set PULSE_MOCK_NODES=15
/opt/pulse/scripts/toggle-mock.sh on
sudo systemctl restart pulse-backend

# User: "back to real"
/opt/pulse/scripts/toggle-mock.sh off

# User: "test with lots of VMs"
nano /opt/pulse/mock.env  # Set PULSE_MOCK_VMS_PER_NODE=20
sudo systemctl restart pulse-backend
```

### How Mock System Works
1. Located in `/opt/pulse/internal/mock/` (gitignored, local only)
2. Generates realistic fake nodes, VMs, containers with popular app names
3. Creates random alerts and changing metrics
4. Integrated with backend-watch.sh for auto-reload
5. Service automatically rebuilds with mock support when enabled

### Known Issue & Workaround
Mock mode shows both real and mock data mixed. If user needs PURE mock:
```bash
/opt/pulse/scripts/toggle-mock-pure.sh on   # Disables real nodes completely
/opt/pulse/scripts/toggle-mock-pure.sh off  # Restores real nodes
```

### IMPORTANT FOR CLAUDE
- **The mock system is for Claude's use during development sessions**
- **User will ask Claude to switch modes frequently** - do it immediately
- **Always use the toggle scripts** - never manually move files
- **After switching, wait ~10-15 seconds** for service to restart
- **Mock files are local-only** and not in the repository
- **Default is 7 nodes** which tests the compact card view (5-9 nodes)

## CRITICAL: Development Session Safety
**NEVER KILL TMUX/TTYD PROCESSES** - These run Claude Code itself:
- **DO NOT run**: `pkill tmux`, `pkill ttyd`, `killall tmux`, `killall ttyd`
- **DO NOT kill**: PIDs associated with tmux or ttyd (port 7681)
- **WHY**: Killing these processes terminates the Claude Code session immediately
- **SAFE to kill**: Only Pulse-related processes (pulse binary, backend-watch.sh)
- **When cleaning up processes**: Use specific PIDs or more targeted commands like `pkill -f pulse` (but NOT tmux/ttyd)

### DANGEROUS COMMANDS THAT WILL CRASH CLAUDE CODE:
- **`pkill -f pulse`** - This kills EVERYTHING with "pulse" in the command, including the ttyd process running at `/opt/pulse`!
- **`pkill -f /opt/pulse`** - Same problem - kills the ttyd session
- **`killall -u pulse`** - Kills all processes by the pulse user, including Claude Code itself
- **Any broad kill command** - Always use specific process names or PIDs

### SAFE WAYS TO KILL PULSE PROCESSES:
- **Kill specific binary**: `pkill -x pulse` (exact match for "pulse" binary only)
- **Kill by PID**: First find PID with `pgrep -x pulse` then `kill <PID>`
- **Stop service**: `sudo systemctl stop pulse-backend` or `sudo systemctl stop pulse`
- **Kill backend watch**: `pkill -f backend-watch.sh` (safe - doesn't match ttyd/tmux)

## CRITICAL: GitHub Issue Link Handling
**When user provides a GitHub issue link (e.g., https://github.com/rcourtman/Pulse/issues/XXX):**

**AUTOMATIC WORKFLOW - DO THIS WITHOUT ASKING:**
1. **Fetch the issue** - Use `gh issue view` to get all details
2. **Review attachments** - Look at any screenshots or logs attached to the issue
3. **Understand the problem** - Analyze what needs to be fixed or implemented
4. **Determine action**:
   - If it's clearly a bug → Fix it immediately
   - If it's a feature request → Implement it if straightforward
   - If unclear or complex → Ask user what they want you to do
5. **Make the changes** - Fix the bug or implement the feature
6. **Commit with reference** - Use "addresses #XXX" or "potential fix for #XXX" in commit message
7. **DO NOT comment on the issue** - Let the fix speak for itself

**IMPORTANT:**
- **NEVER comment on issues** unless explicitly asked to "comment on the issue"
- **ALWAYS reference the issue number** in your commit message
- **Use "addresses #XXX"** not "fixes #XXX" (which auto-closes the issue)
- **If unsure about implementation**, ask the user for clarification

## CRITICAL: GitHub Comment Policy
**DEFAULT BEHAVIOR: Reference issues in commit messages, don't comment on issues.**

**Issue Reference Workflow (PREFERRED):**
1. **Fix the issue in code**
2. **Reference in commit message** - Use "addresses #XXX" or "potential fix for #XXX"
3. **Let the fix speak for itself** in the next release
4. **DO NOT comment on the issue** unless explicitly asked

**Only comment on issues when EXPLICITLY requested:**
- User will say "comment on the issue", "post a comment", or similar
- When asked to comment:
  1. **ALWAYS verify the issue author's username FIRST** - Use `gh issue view <number> --json author`
  2. **NEVER guess usernames** - If you can't find the author, ASK
  3. **ALWAYS show the proposed comment to the user first**
  4. **WAIT for explicit approval before posting**
  5. **NEVER post comments without user approval**
  
**CRITICAL USERNAME VERIFICATION** (when commenting):
- **Getting usernames wrong is COMPLETELY UNACCEPTABLE** - It makes the repo owner look incompetent
- **ALWAYS use `gh issue view <number> --json author`** to verify the exact username
- **NEVER make up usernames like "icebreaker2" or "swtrse"** when you haven't verified them

**When checking issues:**
1. Check and gather all relevant information
2. **ALWAYS look at attached screenshots/images** - They often contain critical details
   - Use `wget -O /tmp/image.png "<image_url>"` to download GitHub issue images
   - Then use the Read tool on the downloaded image file to view/OCR it
   - Images from GitHub issue attachments redirect to S3, wget handles this properly
3. Present findings to the user clearly
4. Fix the issue and reference it in commit message
5. Only comment if user explicitly asks you to

## CRITICAL: Issue Management Policy
**NEVER close issues without user confirmation of the fix:**
1. **DO NOT close issues** just because you think they're fixed
2. **WAIT for the reporter to confirm** the fix works for them
3. **Comment on the issue** saying "potential fix in version X.X.X, can you test and confirm?"
4. **Only close when**:
   - The reporter confirms it's fixed
   - The issue is clearly a duplicate
   - The issue is very old (6+ months) with no response
   - You get explicit permission from the repo owner to close it
5. **When closing old issues**, use: "Closing due to inactivity. Please reopen if still experiencing this issue."
6. **ALWAYS mention reopening** - End every issue closure with: "Feel free to reopen if this is still an issue" or similar
7. **Be patient** - users may take days or weeks to test fixes
8. **Example closing messages:**
   - "Closing as this relates to v3 which is no longer supported. Feel free to reopen if you still need this."
   - "Closing as stale. Please reopen if you're still experiencing this issue."
   - "Should be fixed in v4.1.8. Please reopen if the issue persists after updating."

## CRITICAL: Understanding Before Action
**Before making ANY recommendations or implementing features:**
1. **UNDERSTAND the existing implementation** - Check how things currently work
2. **ASK questions** if uncertain about design decisions
3. **VERIFY assumptions** by examining the actual code
4. **CONSIDER the implications** of any changes
5. **RESPECT the developer's design choices** - They know their app better than you

## CRITICAL: Preventing Hallucinations and False Information
**To avoid making incorrect claims about Pulse:**
1. **ALWAYS verify in code first** - Never claim what Pulse does/doesn't support without checking
2. **Say "I'm not sure, let me check"** when uncertain - Better to admit uncertainty than be wrong
3. **Search codebase before answering** - Use Grep/Read to verify features exist before discussing them
4. **Quote the actual code** - When explaining functionality, show the relevant code snippet
5. **Double-check critical claims** - For important statements about how Pulse works, verify twice
6. **If you can't find evidence, ASK** - Don't assume or guess about features or behavior

**Never give opinions or make changes without first understanding:**
- How the feature currently works
- Why it was designed that way
- What security/UX/technical constraints exist
- What the actual problem is (not what you assume it is)

## CRITICAL: Commit Messages and Fix Claims
**NEVER claim something is fixed in commit messages until users confirm:**
1. **DO NOT use "fix:" or "fixed"** in commit messages unless users have verified the fix works
2. **Use "attempt to address"** or "potential fix for" instead
3. **Wait for user confirmation** before claiming anything is resolved
4. **Track issues properly** - Keep them open until users confirm resolution
5. **Test thoroughly** before even claiming a potential fix
6. **Be honest about uncertainty** - If you're not sure it's fixed, say so
7. **ALWAYS reference the issue number** in commit messages (e.g., "addresses #123", "potential fix for #456")
   - This creates automatic links in GitHub so users can track changes
   - Use "addresses #XXX" or "related to #XXX" not "fixes #XXX" (which auto-closes)
8. **AVOID certainty in comments** - Don't say "found the issue" or "fixed it", say "looks like" or "should address this"

## CRITICAL: ProxmoxVE Community Script Requirements
**NEVER change these without coordinating with the ProxmoxVE team:**

### Binary Location
- **MUST be at**: `/opt/pulse/bin/pulse`
- **NOT**: `/opt/pulse/pulse` (v4.3.7 bug that broke everything)
- **NOT**: `/usr/local/bin/pulse` (that's just a symlink)
- The ProxmoxVE script expects this exact path - changing it breaks their deployment

### Service Name
- **ProxmoxVE uses**: `pulse` (NOT pulse-backend)
- **Our install.sh uses**: `pulse`
- **Manual installs might use**: `pulse-backend`
- **Code MUST detect both** and handle either service name

### Configuration Location
- **Config directory**: `/etc/pulse/`
- **Data directory**: `/etc/pulse/`
- **NOT**: `/opt/pulse/` for config (that's just for the binary)

### User and Permissions
- **Service runs as**: `pulse` user (non-root)
- **NO sudo access** - the pulse user has no sudo privileges
- **Shell access**: Removed (`/bin/false`)
- **NEVER attempt sudo** in any code paths

### Authentication Setup
- ProxmoxVE script may pre-configure API_TOKEN in the service file
- If API_TOKEN is already set, Quick Security Setup should be skipped
- They handle auth setup their own way - respect their configuration

### What Breaks ProxmoxVE Script
1. **Changing binary path** from `/opt/pulse/bin/pulse`
2. **Hardcoding service name** as `pulse-backend`
3. **Requiring sudo** for any operations
4. **Forcing Quick Security Setup** when API_TOKEN exists
5. **Changing config directory** from `/etc/pulse`

### ProxmoxVE Script Installation Method
They use their own installation approach:
1. Downloads our release tarball from GitHub
2. Extracts to `/opt/pulse/`
3. Creates systemd service named `pulse` (NOT pulse-backend)
4. Creates `pulse` user with no shell access
5. May pre-configure API_TOKEN in the service
6. Expects binary at `/opt/pulse/bin/pulse`

### Recent Issues They've Had
- **v4.3.2**: Binary path changed from `/opt/pulse/pulse` to `/opt/pulse/bin/pulse`
- **v4.3.7**: We broke it again by installing to wrong path
- **Multiple versions**: Service name confusion (pulse vs pulse-backend)
- **Authentication**: They want to set API_TOKEN themselves, not use our UI

### Testing ProxmoxVE Compatibility
Before ANY release that changes paths or service handling:
```bash
# Create fresh ProxmoxVE container and test their script
ssh root@delly "pct create <id> /var/lib/vz/template/cache/debian-12-standard_12.7-1_amd64.tar.zst --hostname pulse-test --memory 1024 --cores 2 --rootfs local-zfs:4 --net0 name=eth0,bridge=vmbr0,ip=dhcp --unprivileged 1 --features nesting=1 && pct start <id>"

# Install via their script (bash -c $(...) pulse)
# Verify:
# - Binary is at /opt/pulse/bin/pulse
# - Service name is 'pulse'
# - Can start without sudo errors
```

### GitHub Issue History
- **#6833**: Main complaint about constant binary path changes
- **#6859**: Their PR to update for v4.3.2+
- They've had to update their script MULTIPLE times due to our changes
- They're understandably frustrated with the instability

## CRITICAL: Security Model Understanding
**Pulse v4 Security Architecture:**
- Node credentials (passwords, API tokens) are stored ENCRYPTED at rest
- The frontend NEVER receives actual credentials - only `hasPassword: true` or `hasToken: true`
- When editing nodes, the UI cannot display existing credentials
- The ONLY way to extract credentials is through the export API endpoint
- Export/import endpoints MUST require authentication (API_TOKEN) to prevent credential theft
- **Never suggest or implement features that would expose credentials to the frontend UI**
- **Always verify security implications before making recommendations**

## TypeScript Code Standards
- **NEVER use `any` type** - Always use proper TypeScript types
- **Type everything correctly** - All variables, parameters, and return values must be properly typed
- **Use type guards** - When dealing with union types, use proper type guards (e.g., `'property' in object`)
- **No implicit any** - Ensure TypeScript strict mode catches missing types
- **Prefer interfaces over type aliases** for object shapes
- **Use generics** when appropriate instead of `any`

## CRITICAL: THIS IS A PERMANENT DEVELOPMENT ENVIRONMENT

### 🔥 DEVELOPMENT MODE IS ALWAYS ON 🔥
**This machine (debian-go) is configured as a permanent development environment:**

- **Dev mode runs automatically** - You don't need to start anything
- **Auto-restarts on crash** - systemd keeps it running always
- **Survives reboots** - Starts automatically on boot
- **No commands to remember** - It just works

### Development URLs (Always Available):
- **Frontend**: http://192.168.0.123:7655 (with hot-reload - changes appear instantly!)
- **Backend API**: http://localhost:7656 (API only)
- **Logs**: `tail -f /opt/pulse/dev.log`

### How It Works:
- **systemd service**: `pulse-dev.service` runs permanently
- **Script**: `/opt/pulse/scripts/hot-dev.sh` handles everything
- **Hot-reload**: Frontend changes appear instantly without rebuilding
- **Backend changes**: Restart the service to rebuild
- **Port management**: Automatically cleans up ports on each restart
- **Mock support**: Loads `/opt/pulse/mock.env` if mock mode is enabled

### 🎭 MOCK DATA TOGGLE (USE THIS FREQUENTLY!)

**CLAUDE: You should toggle between mock and real data often during development:**

```bash
# WHEN USER SAYS "use mock data" or "test with fake nodes":
/opt/pulse/scripts/toggle-mock.sh on

# WHEN USER SAYS "use real data" or "back to real":
/opt/pulse/scripts/toggle-mock.sh off

# CHECK what mode you're in:
/opt/pulse/scripts/toggle-mock.sh status
```

**What Mock Mode Gives You:**
- 7 fake nodes (pve1-pve7) with no real infrastructure
- 35 VMs (5 per node) with popular app names
- 56 containers (8 per node) with realistic metrics
- Random changing metrics every 2 seconds
- Mock alerts that auto-generate
- Perfect for testing UI layouts and features

**Switching takes 5 seconds** - The service auto-restarts when you toggle.

**To test different UI layouts:**
```bash
# Edit the mock config
/opt/pulse/scripts/toggle-mock.sh edit
# Change PULSE_MOCK_NODES to:
#   1-4 nodes: Regular cards
#   5-9 nodes: Compact cards (default: 7)
#   10+ nodes: List view
#   25+ nodes: Stress test
# Then restart: sudo systemctl restart pulse-dev
```

#### Building & Testing
```bash
# Create a release (only when ready to ship)
/opt/pulse/scripts/build-release.sh

# Run the complete test suite
/opt/pulse/scripts/run-tests.sh
```

### Service Management:
```bash
# Check status
sudo systemctl status pulse-dev

# View logs
tail -f /opt/pulse/dev.log

# Restart (needed after backend changes)
sudo systemctl restart pulse-dev

# Emergency stop
sudo systemctl stop pulse-dev

# Emergency start
sudo systemctl start pulse-dev
```


### IMPORTANT NOTES:
- **This is NOT for production machines** - Only for debian-go dev environment
- **Port 7655**: Always running with hot-reload
- **Port 7656**: Always running backend API (no embedded frontend)
- **Production service removed**: No pulse-backend.service on this machine
- **Flag file**: `/opt/pulse/.dev-mode` marks this as a dev machine
- **Everything is consolidated**: One service, one main script, simple toggles

### Production Build Testing
**For actual releases, use the build-release script:**
```bash
./scripts/build-release.sh  # Creates release artifacts with proper versioning
# This is only needed at release time, not during development
```

### IMPORTANT: Frontend Embed Location (for production builds only)
**The Go binary embeds frontend files from `/opt/pulse/internal/api/frontend-modern/dist`**
- The build-release.sh script handles this automatically at release time
- Never manually copy files unless debugging

## Development Environment (Current Machine - debian-go)
- **Development Port**: 7655 (frontend with hot-reload)
- **Backend API Port**: 7656 (API only)
- **Production Port**: 7655 (embedded frontend + API)
- **Access URL**: http://192.168.0.123:7655
- **Service**: `sudo systemctl restart pulse-backend` (for production only)
- **Logs**: `tail -f /opt/pulse/pulse.log`

## Available SSH Nodes

Claude Code has SSH access to the following nodes:

### Proxmox Cluster
- **delly** (delly.lan) - Part of a Proxmox cluster with minipc
  - User: root
  - Access: SSH key-based authentication

### Standalone Proxmox Node
- **pimox** (pimox.lan / 192.168.0.2) - Standalone Proxmox VE node
  - User: root
  - Access: SSH key-based authentication

### Proxmox Backup Server (PBS)
- **PBS Docker** (192.168.0.8) - PBS running in Docker container
  - User: root
  - Access: SSH key-based authentication (already configured)
  - Container name: pbs
  - Docker commands: `ssh root@192.168.0.8 "docker exec pbs <command>"`
  - Web Interface: https://192.168.0.8:8007
  - Purpose: Production PBS instance for testing PBS integration

## Test LXC Container

### Testing Policy
**ALWAYS test fixes on actual Proxmox test containers before claiming they work**
- Container IDs change - check with `pct list`
- Create fresh test LXCs whenever needed
- Test in real environments, don't assume fixes work
- Clean up test containers when done

### Pulse Test Container
- **Container ID**: 130 on delly
- **Hostname**: pulse-test
- **IP Address**: 192.168.0.152
- **Pulse Version**: v4.0.0-rc.1
- **Web Interface**: http://192.168.0.152:7655
- **Purpose**: Permanent test container for Pulse development and testing
- **Service**: Running as systemd service (pulse.service)

### Docker Builder Container
- **Container ID**: 135 on delly
- **Hostname**: docker-builder
- **IP Address**: 192.168.0.174
- **Purpose**: Dedicated container for building multi-arch Docker images
- **Docker buildx**: Pre-configured with multiarch builder
- **Architectures**: Builds for linux/amd64, linux/arm64, linux/arm/v7
- **Docker Hub Access**: Already logged in as rcourtman with push permissions
- **Access Method**: SSH through delly (container 135)
- **Note**: Direct SSH to 192.168.0.174 doesn't work, must go through delly

## Testing Tools

Automated testing tools are available in `/opt/pulse/testing-tools/`:

**Automated testing tools in `/opt/pulse/testing-tools/`**
- Email, API, UI button, alerts, thresholds, mobile responsiveness tests available
- Run comprehensive tests after significant changes

## Commit Message Guidelines

### NEVER USE ALARMIST LANGUAGE IN COMMITS
- **DO NOT use**: "CRITICAL", "SECURITY FIX", "URGENT", "SEVERE", "VULNERABILITY"
- **DO NOT**: Create panic or alarm users unnecessarily
- **DO NOT**: Make it seem like user data was compromised
- **DO use**: Calm, professional language that describes the improvement
- **Good example**: "fix: remove sensitive data from logs"
- **Bad example**: "CRITICAL SECURITY FIX: passwords exposed in logs!!!"
- **Remember**: Alarmist commits damage user trust and make the project look unprofessional

When fixing security-related issues:
- Focus on the improvement, not the problem
- Use terms like "improve", "enhance", "update" rather than "fix critical vulnerability"
- Be factual without being dramatic
- Remember that public commit history affects project reputation

## Git Repository Workflow

### Repository Structure
- **Single public repo**: `Pulse` (https://github.com/rcourtman/Pulse.git)
- All development, testing, and releases happen in this repository

### Development Workflow
1. **Main branch development** - Direct commits to main (small team, move fast)
2. **Testing releases** - Use RC/pre-release tags for testing (e.g., v4.1.0-rc.1)
3. **Stable releases** - Full releases when ready for production (e.g., v4.1.0)

### Release Strategy
- **RC releases** - Mark as pre-release in GitHub to prevent auto-updates
- **Stable releases** - Regular releases for production use
- **Version tags** - Always use semantic versioning with 'v' prefix


## Update Mechanism Design
**IMPORTANT**: Pulse does NOT perform self-updates from the UI. Instead:
- The UI detects the deployment type (ProxmoxVE, Docker, systemd, etc.)
- Shows deployment-specific update instructions when updates are available
- ProxmoxVE users type `update` in console
- Docker users pull new image and recreate container
- Manual/systemd users re-run the install script

**Why**: Security constraints prevent self-updates:
- ProxmoxVE containers run as non-root user without sudo
- Docker containers cannot restart themselves
- Systemd services cannot restart without privileges
This design is intentional and should not be "fixed" - it ensures proper security boundaries.

## CRITICAL: Remove Redundant Code
**ALWAYS remove old/redundant code when refactoring or consolidating:**
- **Check for duplicate components** - If you update one component, check if there are others doing the same thing
- **Delete unused files immediately** - Don't leave old versions lying around
- **Search for imports** - Before deleting, ensure no other files import it
- **Common patterns to watch for**:
  - Multiple components with similar names (e.g., PVENodeTable vs NodeSummaryTable)
  - Old implementations left behind after refactoring
  - Duplicate utility functions in different files
- **Why this matters**: Leaving redundant code causes bugs where you update one file but the app uses another
- **Example**: We just had PVENodeTable, PBSNodeTable, and NodeSummaryTable all doing the same thing

## Important Instructions
- **NEVER create documentation files** (*.md) unless explicitly requested by the user
- **NEVER create README files** or other docs to explain changes - just explain in the response
- **DO NOT create markdown files to document findings** - Just explain in the response instead
- **DO NOT create analysis or optimization docs** - The user hates unnecessary documentation
- **ALWAYS prefer editing existing files** over creating new ones
- **RUN TESTS** after making significant changes using the testing tools

## Documentation Style Guidelines
When writing or updating documentation:
- **BE CONCISE** - Get to the point, no fluff
- **PRACTICAL** - Show how to use it, not theory
- **NO CORPORATE SPEAK** - Write like a developer, not a PR department
- **ESSENTIALS ONLY** - What users actually need to know
- **GOOD EXAMPLES** - Real commands and configs that work
- **AVOID**: Long introductions, obvious advice, redundant sections, unnecessary verbosity

## Creating Releases
- **ALWAYS USE THE RELEASE CHECKLIST** - There is a `/opt/pulse/RELEASE_CHECKLIST.md` that MUST be followed step-by-step when creating any release
- **MANDATORY - NO EXCEPTIONS**: You MUST open and follow the checklist line by line for EVERY release (stable, RC, or patch)
- **THIS CHECKLIST IS LOCAL-ONLY** - The RELEASE_CHECKLIST.md file exists ONLY on this development machine and should NEVER be committed to the repository
- **Purpose of the checklist**: Ensures consistent, complete releases with all binaries, proper Docker tags, and testing
- **NEVER create a release without following the checklist** - This ensures proper testing, artifact generation, and documentation
- **START HERE**: When asked to create a release, your FIRST action should be: `Read /opt/pulse/RELEASE_CHECKLIST.md`
- The checklist includes critical steps like:
  - Running tests before release
  - Building release artifacts with `./scripts/build-release.sh`
  - Uploading artifacts to GitHub releases
  - Testing installation methods
  - Proper version management
  - Docker multi-arch builds
- Skipping the checklist results in incomplete releases missing binaries that users need
- **The checklist is in .gitignore** - This ensures it stays local and doesn't get accidentally committed

## GitHub PR and Comment Style
When writing comments on GitHub PRs or issues:
- **Keep it casual and human** - Don't sound like an AI assistant
- **Be humble** - Never oversell or sound big-headed about changes
- **NO emoji checkmarks** (✅) or bullet points with formal structure
- **Explain what went wrong honestly** - Users appreciate transparency
- **Talk like a developer**, not a corporate PR person
- **Avoid dramatic language** - Don't say "completely redesigned", "major overhaul", etc.
- **Use simple descriptions** - "fixed", "changed", "updated" instead of "revolutionized", "transformed"
- **Default response for bug reports**: "thanks for reporting, fixing for the next release"
- **When explaining fixes**: Be specific but humble - "fixed the token display issue" not "completely revolutionized the authentication system"
- **Examples of BAD style**: "Thanks for the feedback! I've completely redesigned the system: ✅ Now uses..."
- **Examples of GOOD style**: "hey @username, thanks for the detailed report. You're right that [problem]. Fixed in the new RC."
- **Avoid**: Overly enthusiastic tone, formatted lists, corporate speak, overselling changes
- **Use**: Lowercase, informal tone, minimal punctuation, get straight to the point
- **AVOID DASHES IN SENTENCES** - Don't write "thanks - really appreciate it", use commas or just flow naturally
- **BAD**: "thanks for the help - means a lot", "fixed the bug - should work now"  
- **GOOD**: "thanks for the help, means a lot", "fixed the bug, should work now"
- **NO EXCLAMATION MARKS** - Avoid using ! in comments, keep it casual without fake enthusiasm
- **BAD**: "thanks!", "fixed it!", "works now!"
- **GOOD**: "thanks", "fixed it", "works now"
- **Acknowledge when users are right** - "You're right that..." shows respect and humility
- **Don't oversell improvements** - Let the changes speak for themselves

## Docker Build Process
- Use container 135 on delly for multi-arch Docker builds
- Build for linux/amd64, linux/arm64, linux/arm/v7
- Tag appropriately for stable vs RC releases
