# Pulse Install/Upgrade/Uninstall Hardening - Findings Report

**Mission**: Harden and modernize Pulse's install/upgrade/uninstall experience and binary distribution pipeline across all deployment models (Docker, bare metal, dev)

**Date**: 2025-11-21
**Status**: Phase 1 - Inventory & Analysis Complete

---

## Executive Summary

After comprehensive analysis of Pulse's build, distribution, and installation infrastructure, the system demonstrates **strong foundational architecture** with well-structured multi-platform builds, comprehensive validation scripts, and thoughtful install/upgrade flows. However, several **critical gaps** exist around runtime validation, error recovery, and documentation alignment that create risk for users.

**Overall Assessment**: 🟡 **MODERATE RISK** - Core infrastructure is solid, but missing guardrails could lead to user-facing failures.

---

## 1. INVENTORY FINDINGS

### 1.1 Binary Build & Distribution Pipeline

#### ✅ **DOCKERFILE** (Strong - `Dockerfile:1-291`)

**What Works Well**:
- **Multi-stage build** cleanly separates frontend → backend → runtime stages
- **Comprehensive platform coverage**:
  - pulse-docker-agent: linux/{amd64,arm64,armv7,armv6,386}
  - pulse-host-agent: linux/{amd64,arm64,armv7,armv6,386}, darwin/{amd64,arm64}, windows/{amd64,arm64,386}
  - pulse-sensor-proxy: linux/{amd64,arm64,armv7,armv6,386}
- **Frontend embedding** correctly copies built assets to `internal/api/frontend-modern/dist` for `go:embed`
- **Version injection** via ldflags for all binaries
- **Windows symlink handling** for .exe files (lines 254-257)
- **BUILD_AGENT flag** allows fast dev builds (line 61)

**Evidence**:
```dockerfile
# Lines 94-137: Host agent - all platforms
COPY --from=backend-builder /app/pulse-host-agent-linux-amd64 /opt/pulse/bin/
...
COPY --from=backend-builder /app/pulse-host-agent-windows-amd64.exe /opt/pulse/bin/
...
# Lines 254-257: Windows symlinks
RUN ln -s pulse-host-agent-windows-amd64.exe /opt/pulse/bin/pulse-host-agent-windows-amd64
```

#### ✅ **BUILD-RELEASE.SH** (Strong - `scripts/build-release.sh:1-361`)

**What Works Well**:
- **Idempotent** - cleans build/release dirs before starting (line 23)
- **Frontend-first build** - ensures embedded assets are ready (lines 26-35)
- **All platforms built** - matches Dockerfile matrix
- **Structured release artifacts**:
  - Per-architecture tarballs with bin/ and scripts/ (lines 83-116)
  - Universal tarball with auto-detect wrapper scripts (lines 119-239)
  - Standalone macOS/Windows archives (lines 278-282)
- **Checksums generated** via sha256sum with GPG signing support (lines 322-353)
- **Helm chart packaging** (lines 306-316)
- **Version embedding** - same ldflags as Dockerfile (lines 56-80)

**Notable Design**:
```bash
# Lines 145-165: Auto-detect wrapper for universal tarball
cat > "$universal_dir/bin/pulse" << 'EOF'
#!/bin/sh
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) exec "$(dirname "$0")/pulse-linux-amd64" "$@" ;;
    aarch64|arm64) exec "$(dirname "$0")/pulse-linux-arm64" "$@" ;;
    ...
esac
EOF
```

#### ✅ **VALIDATE-RELEASE.SH** (Excellent - `scripts/validate-release.sh:1-330`)

**What Works Well**:
- **Comprehensive Docker image validation** (lines 86-127):
  - VERSION file correctness
  - All 26 binaries + 3 Windows symlinks present
  - Scripts executable
  - Version strings embedded correctly
- **Tarball validation** (lines 130-237):
  - All required assets present (install.sh, checksums.txt, tarballs)
  - Tarball contents validated (bin/, scripts/, VERSION)
  - checksums.txt verified
  - Individual .sha256 files match
- **Binary version embedding** tested by executing binaries (lines 246-274)

**Coverage**:
```bash
# Lines 100-102: Binary validation
required="pulse pulse-docker-agent pulse-docker-agent-linux-amd64 ... pulse-sensor-proxy-linux-386"
# 26 binaries + 3 symlinks = 29 artifacts validated
```

#### 🟡 **GAP: No Runtime Binary Validation**

**Finding**: Docker image and release tarballs are validated at build time, but there's **no startup-time validation** that critical binaries exist and are executable.

**Evidence**:
- `Dockerfile` copies binaries but doesn't verify checksums at runtime
- `cmd/pulse/main.go` doesn't check if /opt/pulse/bin binaries exist before serving download endpoints
- Missing health check for binary availability

**Impact**: **MEDIUM** - If binaries are corrupted or missing (filesystem issue, manual tampering, incomplete upgrade), download endpoints will fail with generic 404 instead of clear diagnostics.

**Recommendation**: Add startup validation in `cmd/pulse/main.go`:
```go
// Validate critical binaries on startup
func validateDownloadableBinaries() error {
    required := []string{
        "/opt/pulse/bin/pulse-docker-agent-linux-amd64",
        "/opt/pulse/bin/pulse-host-agent-linux-amd64",
        // ... key platforms
    }
    for _, path := range required {
        if _, err := os.Stat(path); err != nil {
            return fmt.Errorf("critical binary missing: %s: %w", path, err)
        }
    }
    return nil
}
```

---

### 1.2 Download Endpoints (`internal/api/router.go`)

#### ✅ **DOWNLOAD HANDLERS** (Strong - `router.go:993-1007, 3280-3545`)

**What Works Well**:
- **Public endpoints** correctly configured (no auth required) - lines 1330-1337
- **Install scripts served** from /opt/pulse/scripts (lines 3280-3309)
- **Binary downloads** with architecture detection (lines 3312-3379, 3446-3545)
- **Checksum headers** via X-Checksum-Sha256 (router.go:3357-3375)
- **Path traversal protection** with regex validation (lines 3461-3470)
- **Automatic binary restoration** via `agentbinaries.EnsureHostAgentBinaries()` (line 3477)
- **Detailed error messages** with troubleshooting steps (lines 3485-3512)

**Evidence**:
```go
// Lines 3357-3375: Checksum calculation and header
hasher := sha256.New()
io.Copy(hasher, file)
checksum := hex.EncodeToString(hasher.Sum(nil))
w.Header().Set("X-Checksum-Sha256", checksum)
http.ServeContent(w, req, filepath.Base(candidate), info.ModTime(), file)
```

#### 🟡 **GAP: Inconsistent Windows Binary Naming**

**Finding**: Windows binaries have dual naming (.exe and symlink without .exe), but installer scripts don't consistently handle both.

**Evidence**:
- `Dockerfile:254-257` creates symlinks: `pulse-host-agent-windows-amd64` → `pulse-host-agent-windows-amd64.exe`
- `install-host-agent.sh:292` requests `?platform=windows&arch=amd64`
- `router.go:3528-3531` checks for .exe if Windows detected, but relies on correct platformParam

**Impact**: **LOW** - Current implementation works, but fragile if installer doesn't pass correct platform param

**Recommendation**: Normalize in download handler to always serve .exe for Windows:
```go
if strings.Contains(platformParam, "windows") && !strings.HasSuffix(candidate, ".exe") {
    pathsToCheck = append(pathsToCheck, candidate+".exe")
}
```

---

### 1.3 Install/Upgrade Scripts

#### ✅ **INSTALL-DOCKER-AGENT.SH** (Excellent - `scripts/install-docker-agent.sh:1-2042`)

**What Works Well**:
- **Auto-detection** of Docker vs Podman with user prompt (lines 843-900)
- **Podman redirect** - chains to install-container-agent.sh automatically (lines 929-986)
- **Service user management** with marker file to track created users (lines 346-523)
- **Multi-target support** - can report to multiple Pulse servers (lines 99-193, 1414-1498)
- **Docker socket validation** before service start (lines 753-831)
- **Snap Docker support** with home directory relocation (lines 525-585)
- **Docker group membership** with automatic creation for Snap (lines 587-661)
- **Polkit rule** creation for non-root systemd management (lines 713-751)
- **Checksum verification** with fallback (lines 1645-1727)
- **Re-enrollment API call** to clear removed host blocks (lines 1751-1791)
- **OpenRC support** for Alpine (lines 1845-1933)
- **Unraid auto-start** via /boot/config/go.d (lines 1822-1844)
- **Detailed error messages** with quoted commands throughout

**Evidence**:
```bash
# Lines 787-796: Socket validation with sudo as service user
test_output=$(eval "env $env_prefix sudo -u $SERVICE_USER_ACTUAL docker version --format '{{.Server.Version}}'" 2>&1)
test_exitcode=$?
if [[ $test_exitcode -eq 0 ]]; then
    log_success "Docker socket access confirmed for $SERVICE_USER_ACTUAL"
```

#### ✅ **INSTALL-HOST-AGENT.SH** (Excellent - `scripts/install-host-agent.sh:1-980`)

**What Works Well**:
- **Cross-platform** - Linux (systemd/rc.local/Unraid), macOS (launchd), Windows (WSL)
- **Keychain integration** on macOS with interactive prompt (lines 466-544)
- **Wrapper script** for Keychain token retrieval (lines 553-594)
- **SELinux support** with restorecon after install (lines 405-410)
- **Version comparison** with upgrade detection (lines 254-289)
- **Checksum verification** with clear error messages (lines 355-398)
- **Service validation** with 10-second wait + API lookup (lines 806-891)
- **Unraid persistence** via /boot/config/go (lines 683-731)
- **rc.local fallback** for non-systemd systems (lines 732-803)
- **Detailed troubleshooting** in validation failure (lines 901-928)

**Evidence**:
```bash
# Lines 356-375: Checksum verification
if [[ "$EXPECTED_CHECKSUM" == "$ACTUAL_CHECKSUM" ]]; then
    log_success "Checksum verified (SHA256: ${ACTUAL_CHECKSUM:0:16}...)"
else
    log_error "Checksum mismatch!"
    echo "  Expected: $EXPECTED_CHECKSUM"
    echo "  Got:      $ACTUAL_CHECKSUM"
```

#### 🟡 **GAP: No Rollback on Failed Upgrade**

**Finding**: Install scripts overwrite binaries immediately without preserving old versions. If download succeeds but binary is corrupted, service won't start and old binary is gone.

**Evidence**:
- `install-host-agent.sh:402` - `sudo install -m 0755 "$TEMP_BINARY" "$AGENT_PATH"` overwrites immediately
- `install-docker-agent.sh:1748` - `chmod +x "$AGENT_PATH"` after download, no backup
- No `.bak` or versioned backup created before replacement

**Impact**: **HIGH** - Failed upgrades leave system in broken state with no automatic recovery path.

**Recommendation**: Add backup/restore logic:
```bash
# Before overwriting
if [[ -f "$AGENT_PATH" ]]; then
    BACKUP_PATH="${AGENT_PATH}.bak-$(date +%s)"
    cp "$AGENT_PATH" "$BACKUP_PATH"
    log_info "Backed up existing binary to $BACKUP_PATH"
fi

# After download + checksum verification
sudo install -m 0755 "$TEMP_BINARY" "$AGENT_PATH"

# Validate new binary works
if ! "$AGENT_PATH" --version &>/dev/null; then
    log_error "New binary failed validation, rolling back"
    mv "$BACKUP_PATH" "$AGENT_PATH"
    exit 1
fi
```

#### 🟡 **GAP: Installer Scripts Not Versioned**

**Finding**: Install scripts are served from /opt/pulse/scripts in Docker image, but there's no version tracking. If user runs old installer against new server, compatibility issues may arise.

**Evidence**:
- `router.go:3291` serves static file: `http.ServeFile(w, req, "/opt/pulse/scripts/install-docker-agent.sh")`
- No version header or compatibility check
- Scripts don't validate server version before proceeding

**Impact**: **MEDIUM** - Users running cached install scripts may encounter issues with API changes or new features.

**Recommendation**:
1. Add version comment to top of scripts: `# pulse-installer-version: 4.32.2`
2. Add server version check early in script:
```bash
SERVER_VERSION=$(curl -fsSL "$PULSE_URL/api/version" 2>/dev/null || echo "unknown")
log_info "Server version: $SERVER_VERSION"
# Optionally warn if major version mismatch
```

---

### 1.4 Frontend Embedding (`internal/api/frontend_embed.go`)

#### ✅ **GO:EMBED PIPELINE** (Excellent - `frontend_embed.go:1-201`)

**What Works Well**:
- **Correct embed path**: `//go:embed all:frontend-modern/dist` (line 19)
- **Filesystem override** for development via PULSE_FRONTEND_DIR (lines 58-61)
- **Dev proxy support** via FRONTEND_DEV_SERVER (lines 28-54)
- **SPA routing** - serves index.html for unknown paths (lines 177-195)
- **Cache headers** - immutable for hashed assets, no-cache for HTML (lines 161-169)
- **Content-type detection** for .js, .css, .svg, .json (lines 143-157)

**Build Flow Validation**:
1. `Dockerfile:17-19` - Frontend built in node:20-alpine stage
2. `Dockerfile:44` - Dist copied to `internal/api/frontend-modern/dist`
3. `frontend_embed.go:19` - `//go:embed all:frontend-modern/dist`
4. `build-release.sh:26-35` - Frontend built, then copied before Go build

**Evidence**:
```go
// Lines 63-68: Subdirectory extraction
fsys, err := fs.Sub(embeddedFrontend, "frontend-modern/dist")
if err != nil {
    return nil, err
}
return http.FS(fsys), nil
```

#### ✅ **NO GAPS IDENTIFIED** - Frontend embedding is correctly implemented and validated

---

### 1.5 Release Workflow (`.github/workflows/create-release.yml`)

#### ✅ **RELEASE PIPELINE** (Excellent - `create-release.yml:1-551`)

**What Works Well**:
- **Version guard** - ensures VERSION file matches requested version (lines 46-63)
- **Preflight tests** before release creation (lines 65-206):
  - Frontend linting
  - Backend tests
  - Integration tests (Docker compose up, API validation)
  - Playwright smoke tests
- **Multi-platform Docker builds** via buildx: linux/amd64, linux/arm64 (line 247)
- **Build cache** via GHCR (lines 250-251)
- **Artifact upload** with retry logic built into gh CLI (lines 483-517)
- **Draft release** - manual publish step prevents accidental releases (line 423)
- **Release validation** via separate workflow (lines 539-551)

**Evidence**:
```yaml
# Lines 91-103: Frontend embedding validation in CI
- name: Build frontend bundle for Go embed
  run: |
    npm --prefix frontend-modern run build
    rm -rf internal/api/frontend-modern
    mkdir -p internal/api/frontend-modern
    cp -r frontend-modern/dist internal/api/frontend-modern/
```

#### 🟡 **GAP: No Binary Integrity Check After Upload**

**Finding**: Release workflow uploads assets to GitHub releases, but doesn't re-download and verify checksums after upload completes.

**Evidence**:
- `create-release.yml:483-517` - Uploads checksums.txt and assets
- No verification step that uploaded checksums.txt matches downloaded artifacts
- validate-release-assets.yml called but unclear if it downloads from GitHub

**Impact**: **LOW-MEDIUM** - Corrupt uploads could go undetected until users report issues.

**Recommendation**: Add post-upload verification:
```yaml
- name: Verify uploaded artifacts
  run: |
    TAG="${{ needs.extract_version.outputs.tag }}"
    TEMP_DIR=$(mktemp -d)
    gh release download "${TAG}" -D "$TEMP_DIR"
    cd "$TEMP_DIR"
    sha256sum -c checksums.txt || {
      echo "::error::Uploaded artifacts failed checksum verification"
      exit 1
    }
```

---

## 2. PLATFORM/ARCHITECTURE SUPPORT MATRIX

### 2.1 Comprehensive Matrix

| Binary | linux/amd64 | linux/arm64 | linux/armv7 | linux/armv6 | linux/386 | darwin/amd64 | darwin/arm64 | windows/amd64 | windows/arm64 | windows/386 |
|--------|-------------|-------------|-------------|-------------|-----------|--------------|--------------|---------------|---------------|-------------|
| **pulse** (server) | ✅ Built | ✅ Built | ✅ Built | ✅ Built | ✅ Built | ❌ N/A | ❌ N/A | ❌ N/A | ❌ N/A | ❌ N/A |
| **pulse-docker-agent** | ✅ Built | ✅ Built | ✅ Built | ✅ Built | ✅ Built | ❌ N/A | ❌ N/A | ❌ N/A | ❌ N/A | ❌ N/A |
| **pulse-host-agent** | ✅ Built | ✅ Built | ✅ Built | ✅ Built | ✅ Built | ✅ Built | ✅ Built | ✅ Built | ✅ Built | ✅ Built |
| **pulse-sensor-proxy** | ✅ Built | ✅ Built | ✅ Built | ✅ Built | ✅ Built | ❌ N/A | ❌ N/A | ❌ N/A | ❌ N/A | ❌ N/A |

**Total Binaries Built**: 29 (5 Linux server + 5 docker-agent + 10 host-agent + 5 sensor-proxy + 3 Windows .exe + 1 default docker-agent)

**Docker Image Platforms**: linux/amd64, linux/arm64 (Dockerfile multi-stage builds for both)

---

## 3. CRITICAL GAPS & RISKS

### 3.1 HIGH PRIORITY

| ID | Finding | Evidence | Impact | Recommendation |
|----|---------|----------|--------|----------------|
| **H1** | No rollback on failed upgrade | `install-host-agent.sh:402` overwrites binary immediately | Service left broken with no recovery | Add backup/restore logic with validation |
| **H2** | No startup binary validation | `cmd/pulse/main.go` doesn't check /opt/pulse/bin binaries exist | Download endpoints fail with unclear errors | Add validateDownloadableBinaries() startup check |

### 3.2 MEDIUM PRIORITY

| ID | Finding | Evidence | Impact | Recommendation |
|----|---------|----------|--------|----------------|
| **M1** | Installer scripts not versioned | `router.go:3291` serves static script, no version header | Compatibility issues with old installers | Add version header + server version check |
| **M2** | No post-upload integrity check | `create-release.yml:483-517` uploads but doesn't re-verify | Corrupt uploads undetected | Add gh release download + checksum verification |
| **M3** | Inconsistent Windows binary naming | Dual .exe / non-.exe naming handled inconsistently | Fragile if platform param wrong | Normalize in download handler |

### 3.3 LOW PRIORITY

| ID | Finding | Evidence | Impact | Recommendation |
|----|---------|----------|--------|----------------|
| **L1** | Docker compose health check uses wget | `docker-compose.yml:19` uses wget, not always available in minimal images | Health check may fail in alpine variants | Use curl or both: `curl || wget` |
| **L2** | No version skew detection | Installers don't check if agent version matches server | Users may run mismatched versions | Add version compatibility check to installer |

---

## 4. STRENGTHS TO PRESERVE

1. **Comprehensive platform coverage** - 10 platforms for host-agent is excellent
2. **Checksum verification** - installers verify downloads via X-Checksum-Sha256 header
3. **Auto-repair** - `agentbinaries.EnsureHostAgentBinaries()` automatically restores missing binaries
4. **Service user management** - docker-agent installer creates dedicated user with marker file
5. **Socket validation** - docker-agent validates Docker socket access before starting service
6. **Keychain integration** - macOS installer securely stores tokens
7. **Multi-target support** - docker-agent can report to multiple Pulse servers
8. **Detailed error messages** - installers provide troubleshooting steps and quoted commands
9. **validate-release.sh** - comprehensive pre-release validation
10. **Idempotent operations** - installers can be re-run safely

---

## 5. RECOMMENDATIONS SUMMARY

### Immediate Actions (1-2 days)

1. **Add startup binary validation** (H2):
   - Create `internal/validators/binaries.go`
   - Call from `cmd/pulse/main.go` before HTTP server starts
   - Fail fast with clear error if critical binaries missing

2. **Add rollback logic to installers** (H1):
   - Backup existing binary before overwrite
   - Validate new binary executes `--version` successfully
   - Restore backup if validation fails

### Short-term (1 week)

3. **Version installer scripts** (M1):
   - Add `# pulse-installer-version: X.Y.Z` header
   - Add server version check early in script
   - Log version mismatch warnings

4. **Add post-upload verification** (M2):
   - Download artifacts from GitHub after upload
   - Verify checksums match
   - Fail workflow if mismatch detected

### Medium-term (2-4 weeks)

5. **Normalize Windows binary handling** (M3):
   - Always serve .exe for Windows in download handler
   - Update installers to expect .exe consistently

6. **Add version skew detection** (L2):
   - Installers check `/api/version` endpoint
   - Warn if major version mismatch
   - Block if breaking API changes

### Long-term (1-2 months)

7. **Integration testing**:
   - Test upgrade path: v4.31 → v4.32 → v4.33
   - Test downgrade with rollback
   - Test cross-platform downloads from single server

8. **Documentation audit**:
   - Align docs with current install flows
   - Document recovery procedures
   - Create upgrade troubleshooting guide

---

## 6. EVIDENCE INDEX

All findings are traceable to specific file:line references:

- `Dockerfile:1-291` - Multi-stage build with all platforms
- `scripts/build-release.sh:1-361` - Release tarball generation
- `scripts/validate-release.sh:1-330` - Pre-release validation
- `scripts/install-docker-agent.sh:1-2042` - Docker agent installer
- `scripts/install-host-agent.sh:1-980` - Host agent installer
- `internal/api/router.go:993-1007, 3280-3545` - Download endpoints
- `internal/api/frontend_embed.go:1-201` - Frontend embedding
- `.github/workflows/create-release.yml:1-551` - Release pipeline

---

## 7. NEXT STEPS

1. **Review findings** with team - prioritize H1, H2 fixes
2. **Create implementation plan** with effort estimates
3. **Begin local validation** - build all targets, test upgrade flows
4. **Draft PRs** for high-priority fixes
5. **Update documentation** to reflect current install flows

**End of Findings Report**
