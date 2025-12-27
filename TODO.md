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
- [x] Implement macOS (`launchd`) service installation logic.
- [x] Implement and unit-test the `APIClient` for all required endpoints.
- [x] Implement and unit-test the `CredentialStore` for local storage.
- [x] Refactor `install` command to perform the full API-driven workflow, including secure merging of local and remote configs.
- [x] Add comprehensive unit tests for the `install` command's orchestration logic.

## Next Steps

The client-side logic for both local (`run`) and API-driven (`install`) workflows is complete and unit-tested. The primary remaining task is to implement the specific workflow for the installed service to use.

- [ ] **Finalize the Managed Service Workflow**
  - [ ] Add a `--managed` flag to the `run` command.
  - [ ] When `urso run --managed` is executed, the `CLI.Run` method should:
    1. Ignore the `--config` flag and local `config.yaml` for runner definitions.
    2. Load the machine ID and token from the `CredentialStore`.
    3. Fetch the runner configuration and GitHub tokens from the API.
    4. Load the local `config.yaml` **only to read the `rootDir`**.
    5. Merge the trusted `rootDir` with the runners fetched from the API.
    6. Execute the sync logic with the merged configuration and fetched tokens.
  - [ ] Update the `launchd.plist` template in `service.go` to call `urso run --managed`.
  - [ ] Add comprehensive unit tests for this new flag and logic in `TestCLI_Run`.

- [ ] **Live Integration Testing**
  - [ ] Validate the `urso install` command against the live Urso API (once backend JWT `kid` issue is resolved).
  - [ ] Validate the `urso run --managed` workflow after it is installed as a service.
