# Urso Project Plan

This file tracks the development tasks for the `urso` project.

## Completed Milestones

### V1: Core CLI and TDD Foundation
- [x] Implement basic CLI structure (`init`, `run`, `install`).
- [x] Refactor core logic into a testable, decoupled `urso` package.
- [x] Add comprehensive unit tests for `init` and `run` commands.
- [x] Resolve all `golangci-lint` issues.

### V2: Hardening and API Implementation
- [x] Implement structured, leveled logging with `slog`.
- [x] Securely handle token resolution (flags and env vars only).
- [x] Propagate `context.Context` for timeouts and cancellation.
- [x] Inject `http.Client` dependency.
- [x] Implement macOS (`launchd`) service installation logic using modern `bootstrap/bootout` commands.
- [x] Implement and unit-test the `APIClient` for all required endpoints with exponential backoff retries.
- [x] Implement and unit-test the `CredentialStore` for local storage with restricted permissions.
- [x] Refactor `install` command to perform the full API-driven workflow, including secure merging of local and remote configs.
- [x] Migrate CLI to `spf13/cobra` for professional flag and command management.
- [x] Decouple configuration parsing and path expansion for better testability.
- [x] Implement resilient synchronization that collects and reports multiple runner errors using `errors.Join`.
- [x] Implement GitHub runner archive caching in `~/.urso/cache` with version validation against latest releases.

## Next Steps

The client-side logic and core synchronization engine are now robust and hardened. The focus moves to integration and maintenance.

- [ ] **Live Integration Testing**
  - [ ] Validate the `urso install` command against the live Urso API.
  - [ ] Verify that the `launchd` service correctly starts the sync process on load.
  - [ ] Test the archive caching logic over several days to ensure seamless updates.

- [ ] **Maintenance & Polish**
  - [ ] Periodically review `launchd` log output for any edge cases in runner configuration.
  - [ ] Monitor GitHub runner release patterns to ensure the 24-hour version check remains optimal.