# Urso Project Plan

This file tracks the development tasks for the `urso` project.

## Completed Tasks

- [x] Implement basic CLI structure (`init`, `run`, `install`).
- [x] Use the standard library `flag` package for command-line parsing.
- [x] Add `version` and `help` commands to the CLI.
- [x] Enhance the `init` command to use the `~/.urso` directory and add an overwrite confirmation prompt.
- [x] Refactor the project to isolate core logic from the `main` package into a dedicated `urso` package.
- [x] Decouple the `init` command's logic from the filesystem using interfaces (`ConfigStore`) to enable TDD.
- [x] Decouple the `run` command's logic from the filesystem and `os.exec` using interfaces (`MachineInspector`, `RunnerExecutor`, etc.).
- [x] Add comprehensive unit tests for the `init` command logic.
- [x] Add comprehensive unit tests for the `run` command's `SyncRunners` logic.
- [x] Fix all outstanding `golangci-lint` issues.

## Next Steps

### 1. High Priority: Improve Logging

- [x] Introduce `slog.Logger` as a dependency into the `RunnerSyncer` and `CLI` structs.
- [x] Replace all `log.Printf` calls with structured, leveled logging (e.g., `logger.Info`, `logger.Warn`, `logger.Error`).
- [x] Decouple the `LiveRunnerExecutor` from writing directly to `os.Stdout` by making the output `io.Writer` configurable.

### 2. Medium Priority: Configuration and Token Management

- [x] Move the logic for resolving tokens (flag > env var) from `main.go` into the `urso` package.
- [x] Add support for `githubRegisterToken` and `githubRemoveToken` in the `config.yaml` file.
- [x] Establish a clear order of precedence for token resolution (e.g., flag > environment variable > config file).

### 3. Low Priority: Complete the `install` Command

- [ ] **Implement `install` Command (API-Driven Workflow):**
  - [ ] Define an `APIClient` interface to abstract calls to the Urso Dashboard API.
  - [ ] Define a `CredentialStore` interface to abstract secure local storage of the machine ID and token.
  - [ ] Refactor the `CLI.Install` method to orchestrate the full installation flow:
    1. Register the machine via the API using a JWT.
    2. Save the returned machine ID and token using the `CredentialStore`.
    3. Fetch the runner configuration from the API.
    4. Fetch the GitHub tokens from the API.
    5. Call the existing `syncer.Sync()` method with the fetched config and tokens.
    6. Install the `launchd` service to run a managed sync command in the background.
  - [ ] Write comprehensive unit tests for the `Install` method using spies for all dependencies.

### 4. Code Cleanup and Minor Refinements

- [ ] Inject a shared `http.Client` as a dependency for the `GithubAPIDownloader` to improve efficiency and testability.
- [ ] Propagate a `context.Context` from the top-level commands down through the application logic to enable consistent timeout and cancellation handling.
