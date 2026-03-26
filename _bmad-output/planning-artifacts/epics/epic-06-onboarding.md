# Epic 6: Developer Onboarding — Zero-Friction Setup

**Status:** Planned
**Grava ID:** grava-157a
**Matrix Score:** 3.85
**FRs covered:** FR26, FR27, FR28

## Goal

Non-technical users and developers can set up a fully operational Grava environment (including Claude CLI or Gemini CLI) in under 5 minutes using a single automated install script on macOS, Linux, or Windows — with OS/arch detection, all dependency installation, and final validation via `grava doctor`.

## Deliverables

| Deliverable | FR | Description |
|-------------|----|-------------|
| Shell install script (macOS/Linux) | FR26 | Single-execution setup: AI CLI + Grava |
| PowerShell install script (Windows) | FR26/FR27 | Windows x86-64 equivalent |
| OS/arch detection logic | FR27 | Auto-select binary/package source |
| AI backend selection | FR26 | Claude CLI or Gemini CLI prompt |
| Install validation | FR28 | Run `grava doctor` and report pass/fail with remediation |

## Target Platforms

| Platform | Architecture | Installer |
|----------|-------------|-----------|
| macOS | ARM (M1/M2/M3) | Shell script |
| macOS | x86-64 | Shell script |
| Linux (Debian/Ubuntu) | x86-64, ARM | Shell script + apt |
| Linux (RHEL/Fedora) | x86-64 | Shell script + dnf/yum |
| Windows | x86-64 | PowerShell script |

## NFR Ownership

| NFR | Role |
|-----|------|
| NFR7 (Install Speed) | *Owned* — full setup in <5 minutes on ≥10 Mbps |
| NFR8 (Install Reliability) | *Owned* — first-attempt success, no elevated privileges beyond package manager bootstrap |

## Dependencies

- Epic 1 complete (binary must compile to single static binary — NFR6)
- Epic 5 complete (doctor must be implemented for FR28 install validation)

## Parallel Track

- Can proceed in parallel with Epic 7 (Git Sync) after Epic 5 ships
- Can proceed in parallel with Epic 9 (Worktree Orchestration) after Epic 5 ships

## Key Implementation Notes

- Install script must be idempotent: safe to re-run on already-configured system
- No `sudo` required beyond initial package manager setup (NFR8)
- Final step: `grava doctor` must exit 0 for install to report success
- Remediation steps must be actionable — not "contact support" but specific CLI commands to fix

## Stories

### Story 6.1: Automated Install Script for macOS and Linux *(grava-2d42)*

As a developer or non-technical user on macOS or Linux,
I want a single shell command to install Grava and my chosen AI CLI backend,
So that I can be fully operational without manual dependency management.

**Acceptance Criteria:**

**Given** a clean macOS ARM, macOS x86, Ubuntu, or RHEL machine with ≥10 Mbps connection
**When** I run `curl -sSL <install-script-url> | sh`
**Then** the script detects OS and architecture automatically and selects the correct Grava binary
**And** the script prompts me to choose Claude CLI or Gemini CLI as my AI backend, then installs the selected one
**And** the script installs all required dependencies without requiring `sudo` beyond initial package manager bootstrap
**And** the full install completes in <5 minutes (NFR7)
**And** re-running the script on an already-configured system is idempotent — no duplicate entries, no config overwrite

---

### Story 6.2: PowerShell Install Script for Windows *(grava-084c)*

As a developer on Windows x86-64,
I want a PowerShell script to install Grava and my chosen AI CLI backend,
So that I have the same zero-friction setup experience as macOS/Linux users.

**Acceptance Criteria:**

**Given** a clean Windows x86-64 machine with PowerShell 5.1+ and ≥10 Mbps connection
**When** I run the PowerShell install script
**Then** the script detects Windows x86-64 and downloads the correct Grava binary
**And** the script prompts for AI backend selection (Claude CLI or Gemini CLI) and installs the chosen one
**And** the script completes without requiring elevated Administrator privileges beyond initial package manager bootstrapping (e.g., winget)
**And** the full install completes in <5 minutes (NFR7)
**And** re-running is idempotent (NFR8)

---

### Story 6.3: Install Validation via grava doctor *(grava-4d02)*

As a developer,
I want the install script to automatically validate my environment when it completes,
So that I know immediately if my setup is correct or what needs to be fixed.

**Acceptance Criteria:**

**Given** the install script has run to completion
**When** the final validation step executes `grava doctor`
**Then** if all checks pass, the script prints `✓ Grava is ready. Run 'grava init' to initialize your first project.`
**And** if any check fails, the script prints a clear `✗ Setup incomplete` message with the specific failing check and an actionable remediation command (not "contact support")
**And** the script exits with code `0` on success and `1` on validation failure
**And** `grava doctor` output is included verbatim in the install log for debugging
