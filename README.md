<img src="./assets/urso-logo.png" width="200" alt="Urso Logo">

[![codecov](https://codecov.io/gh/urso-run/urso/graph/badge.svg?token=HYC27GSMBM)](https://codecov.io/gh/urso-run/urso)

# Urso – macOS GitHub Actions Runner Orchestrator

Urso is a hardened CLI for provisioning and maintaining GitHub Actions runners on macOS hosts. It can operate in two modes:

1. **Local / Free Mode** – Operates standalone. You provide a `config.yaml` plus GitHub registration/removal tokens and Urso reconciles runners accordingly.
2. **Managed / Cloud Mode** – The machine is registered with `https://urso.run`, credentials are cached locally, and Urso pulls runner definitions and tokens from the API. Configuration is kept in memory and does not require or modify `config.yaml`.

> **Platform scope:** macOS (darwin) is the only supported operating system. All tooling, release artifacts, and service management are optimized for launchd. Linux/Windows support is explicitly out of scope.

---

## Table of Contents

1. [Why Urso?](#why-urso)
2. [Architecture Overview](#architecture-overview)
3. [Installation & Quick Start](#installation--quick-start)
4. [Command Reference](#command-reference)
5. [Configuration and State Layout](#configuration-and-state-layout)
6. [Local vs. Managed Workflow](#local-vs-managed-workflow)
7. [Service Management](#service-management)
8. [Security Considerations](#security-considerations)
9. [Testing & Tooling](#testing--tooling)
10. [Roadmap & Open Questions](#roadmap--open-questions)
11. [Commercial Use](#commercial-use)
12. [Contributing](#contributing)

---

## Why Urso?

- **Mac-focused:** Launchd integration, Apple silicon support, and filesystem permissions aligned with macOS security expectations.
- **Production-ready:** Structured logging, strict config validation, retrying API client, GitHub runner archive caching, and error fan-out reporting.
- **Testable-by-design:** CLI logic is decoupled behind interfaces (`Syncer`, `APIClient`, `CredentialStore`, `ServiceManager`, etc.), enabling unit tests without shelling out.
- **Dual workflow:** Works for hobby use (just point at a YAML file) and for enterprise deployments via the Urso dashboard/API.

---

## Architecture Overview

| Component | Responsibility |
|-----------|----------------|
| `cmd/urso` | Cobra-based CLI wiring, dependency injection, flag parsing. |
| `urso.ConfigStore` | Handles `config.yaml` discovery, read/write, and path expansion. |
| `urso.CLI` | Implements `init`, `run`, and `install` command logic, delegating to injected collaborators. |
| `urso.RunnerSyncer` | Core reconciliation engine: plans runner additions/removals, downloads archives, installs services. |
| `urso.DashboardAPIClient` | Talks to `https://urso.run` with retry/backoff, fetching runner config and GitHub tokens. |
| `urso.FileSystemCredentialStore` | Persists machine ID/token returned by the Urso API. |
| `urso.GithubAPIDownloader` | Maintains a cached GitHub Actions runner archive in the Urso home directory. |
| `urso.LaunchdManager` | Generates a plist and uses `launchctl bootstrap/bootout` to manage the background agent. |

Key design decisions:

- **macOS only:** Non-darwin builds should fail fast; service management is launchd-specific.
- **Single source of truth for paths:** The “Urso Home” (`~/.urso` by default) stores configs, credentials, logs, and cache.
- **Urso Home as trust boundary:** The `--urso-home` flag (defaulting to `~/.urso`) defines where all runners, logs, and cache are stored. Standalone configuration is relative to this root.
- **Error aggregation:** Runner creation/removal errors are joined so operators see every issue after a reconciliation pass.

---

## Installation & Quick Start

The easiest way to install Urso on macOS is via the installation script:

```bash
# For Managed / Cloud Mode
curl -sSL https://raw.githubusercontent.com/urso-run/urso/main/scripts/install.sh | sh -s -- <YOUR_REGISTRATION_TOKEN>

# For Local / Free Mode
curl -sSL https://raw.githubusercontent.com/urso-run/urso/main/scripts/install.sh | sh
```

### Manual Quick Start

If you prefer manual installation or want to run from source:

```bash
# 1. Build from source (macOS only)
make build

# 2. Initialize local state (~/.urso/config.yaml)
./urso init

# 3. Choose your mode:

# 3a. Local mode: provide GitHub tokens manually
export GITHUB_REGISTER_TOKEN=...
export GITHUB_REMOVE_TOKEN=...
~/.urso/bin/urso run

# 3b. Managed mode: install as a service (idempotent)
~/.urso/bin/urso install --urso-registration-token <YOUR_REGISTRATION_TOKEN>

# 4. (Optional) Add Urso to your PATH
echo 'export PATH="$HOME/.urso/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

---

## Command Reference

### `urso init`

- Creates a starter `config.yaml` inside the Urso home directory.
- If a config already exists, prompts before overwriting.
- **Prerequisite** for Local / Free Mode. Not required for Managed / Cloud Mode.

### `urso run`

- **Dual behavior:**
  - If machine credentials are absent → operate in local/free mode by reading `config.yaml` and using CLI/env GitHub tokens.
  - If credentials are present → operate in managed mode. It pulls runner definitions from the API and ignores `config.yaml` entirely.
- All state is derived from `--urso-home` (default: `~/.urso`).
- Executes a single reconciliation cycle; repeating runs are handled by launchd when installed.

### `urso install`

- Steps:
  1. Register machine with Urso API using `--urso-registration-token` (JWT). If already registered, re-use saved credentials.
  2. Persist machine credentials securely.
  3. Fetch remote runner configuration and GitHub tokens and perform an initial sync.
  4. Install/refresh the launchd service so it runs `urso run` automatically.
- Should be **idempotent**: running it multiple times refreshes configuration and service definitions without side effects.

### `urso uninstall`

- Removes the launchd service plist.
- Stops the background agent.

---

## Configuration and State Layout

### Urso Home (default: `~/.urso`)

| Path | Purpose |
|------|---------|
| `config.yaml` | Local runner definitions (Standalone mode only). |
| `credentials.json` | Machine ID/token issued by Urso API (managed mode). |
| `cache/actions-runner.tar.gz` & `cache/version.txt` | Cached GitHub runner archive and version metadata. |
| `logs/com.urso-run.urso.log` | launchd stdout/stderr (defined in plist). |

### Sample `config.yaml`

```yaml
runners:
  - name: "default-runner"
    url: "https://github.com/my-org"
    group: "Default"
    labels:
      - self-hosted
      - macos
      - arm64
```

*Configuration no longer includes `rootDir`; runners are always placed in `runners/` relative to the Urso home directory.*

### Validation Rules

- Runner `name` and `url` are required.
- Additional validation (labels, groups) should be tightened in future iterations.
- YAML parsing enforces strict field checking (`KnownFields(true)`) to catch typos or legacy configuration keys (like `rootDir`).

---

## Local vs. Managed Workflow

| Aspect | Local / Free Mode | Managed / Cloud Mode |
|--------|-------------------|-------------------------|
| Credentials | Not present | `credentials.json` populated via `install`. |
| Runner Source | `config.yaml` (inside Urso Home). | Remote API runners (kept in memory). |
| GitHub Tokens | CLI flags or env vars (`GITHUB_REGISTER_TOKEN`, `GITHUB_REMOVE_TOKEN`). | Fetched from Urso API per run (register/remove tokens). |
| Typical Command | `urso run --urso-home <path>` | `urso install` (once), then launchd invokes `urso run`. |
| Use Case | Personal/lab machines, no dashboard integration. | Production fleets managed centrally. |

Workflow logic:

1. `urso run` checks for credentials via `CredentialStore`.
2. If absent → Local mode: Read `config.yaml`, require manual tokens, run sync.
3. If present → Managed mode: Load machine ID/token, fetch runners and tokens from API, run sync.

---

## Service Management

- **Launchd plist** is generated at `~/Library/LaunchAgents/com.urso-run.urso.plist`.
- Program arguments should be updated to `["/path/to/urso", "run"]`. Because `run` auto-detects managed mode, no extra flags are needed once credentials exist.
- Logging:
  - Free/local mode (interactive CLI) writes structured logs to stdout and fatal errors to stderr.
  - Managed/launchd mode captures both stdout and stderr into `~/Library/Logs/com.urso-run.urso.log` (per the plist). Tail with `tail -f ~/Library/Logs/com.urso-run.urso.log` or `log stream --predicate 'process == "urso"'`.
- `install` currently runs:
  1. `launchctl bootout gui/<uid> <plist>` (best-effort).
  2. `launchctl bootstrap gui/<uid> <plist>`.
- Future enhancements:
  - Decide whether `install` should also start the service immediately (current behavior) and document manual `launchctl kickstart` commands.
  - Provide explicit `urso uninstall` or `urso service stop|start` helpers.

---

## Security Considerations

- **Permissions:** Config, credentials, and cache directories should all be `0700`. Runner directories currently use `0755`; tighten to `0700` to avoid leaking GitHub runner secrets.
- **Credential storage:** `credentials.json` is written with `0600`. Handle errors carefully and avoid printing secrets in logs.
- **Logging:** Structured logging (`slog`) is configured to stdout; ensure sensitive data isn’t logged.
- **API retries:** `DashboardAPIClient` retries transient failures (5xx/timeouts) with exponential backoff and aborts on 4xx responses.

---

## Testing & Tooling

- `make test` → `go test -v -race -count=1 ./...`
- `make lint` → `golangci-lint` with an extensive ruleset (see `.golangci.yml`).
- GitHub Actions workflows (`.github/workflows/*.yml`) run lint, unit tests, CodeQL, vuln scans, and release builds.
- Release pipeline uses GoReleaser (macOS targets only) with SBOM + cosign signing.

---

## Roadmap & Open Questions

### High-priority (from previous TODO + new analysis)

1. **Dual-mode `run` implementation (Completed)**
   - Auto-detect credentials, fetch remote config/tokens when managed.
   - Handle missing `config.yaml` in managed mode by using defaults.

2. **Idempotent `install` workflow (Completed)**
   - Skip re-registration if credentials exist.
   - Ensure launchd always runs `urso run`.
 
3. **Urso Home abstraction (Completed)**
   - Add `--urso-home` flag and removed `--config`.
   - Centralize path derivation for config, credentials, cache, logs, and tests.
   - Removed `rootDir` from configuration; it's now strictly relative to Urso Home.

4. **Documentation refresh (Completed)**
   - Keep this file as the single source of truth; deprecate `TODO.md` and `HANDOFF_SUMMARY.md`.

5. **Better validation & error reporting**
   - YAML known-field checking is enforced.
   - Improve filesystem error messages (`FileSystemMachine`).
   - Filter non-runner directories when determining existing runners.

6. **Security tightening**
   - Restrict runner directory permissions to `0700`.
   - Honor temporary download directories in `GithubAPIDownloader` or refactor signature.

7. **Testing gaps**
   - Add coverage for downloader caching, machine inspector errors, and CLI failure paths.
   - Introduce service-level tests if feasible.

### Open Questions
 
- **Credential rotation?** Define how to rotate machine credentials if compromised (probably via `install` re-run).

Document decisions in this README as they are made.

---

## Commercial Use

For use within a business or commercial environment, a commercial license is required. Contact the Urso team via [https://urso.run](https://urso.run) for pricing and licensing details.

---

## Contributing

1. Fork the repo and clone locally.
2. Install Go (matching the version in `go.mod`) and `golangci-lint`.
3. Run `make lint` and `make test` before submitting a PR.
4. Keep README as the authoritative documentation; avoid divergent docs in separate files.
5. Follow the coding patterns already established (interfaces for side effects, table-driven tests, context propagation).

Thanks for helping improve Urso!
