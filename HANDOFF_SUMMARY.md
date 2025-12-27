### Project `urso` - Handoff Summary

**Project Goal:** `urso` is a command-line tool for macOS that manages self-hosted GitHub Actions runners. It has two primary modes of operation: a local mode using a `config.yaml` file, and a managed mode where it is installed as a service and gets its configuration from a central API.

**Current State:** The project is in a very good state. The client-side logic for both the local (`run`) and API-driven (`install`) workflows is complete, fully unit-tested, and follows Go best practices. The codebase is clean and all linter issues have been resolved.

**Architectural Decisions & Patterns:**

1.  **macOS Only:** The tool is explicitly for macOS (`darwin`). We do not need to support other operating systems like Linux.
2.  **TDD and Decoupling:** The entire application has been refactored for testability. Core logic is in the `urso` package, decoupled from side effects (network, filesystem, OS commands) via interfaces. `main.go` is only for dependency injection and wiring. The key interfaces are:
    *   `Syncer`: The core runner reconciliation logic.
    *   `APIClient`: For communicating with the `https://urso.run` API.
    *   `CredentialStore`: For securely saving/loading the machine ID and token.
    *   `ServiceManager`: For installing the `launchd` service on macOS.
    *   `RunnerExecutor`, `MachineInspector`, `ActionsDownloader`: For abstracting shell commands and filesystem interactions.
3.  **Command Structure:**
    *   `urso init`: Creates a default `~/.urso/config.yaml`.
    *   `urso run`: Reads `config.yaml` and synchronizes runners once. Tokens are provided by flags or environment variables. This is the **local/manual** mode.
    *   `urso install`: The **API-driven** setup command. It:
        1.  Requires `init` to have been run first to establish a `rootDir`.
        2.  Registers the machine with the API via a JWT.
        3.  Saves the returned machine credentials.
        4.  Fetches runner configuration and GitHub tokens from the API.
        5.  Merges the API's runners with the `rootDir` from the local `config.yaml`.
        6.  Performs an initial sync.
        7.  Installs a `launchd` service.
4.  **Configuration Precedence:**
    *   **`rootDir`**: The local `config.yaml` is the *only* source of truth for this. The API is not trusted to set this value.
    *   **Tokens (for `run` command):** Command-line flag > Environment Variable.
    *   **Tokens (for `install` command):** Fetched directly from the Urso API.

**Testing:**

*   We use table-driven tests and helper functions to keep test complexity low.
*   We use spies for all our interfaces to test logic in isolation.
*   We use `httptest` to mock the API server for our `APIClient` tests.
*   We run `make test` and `make lint` after all changes.

**Immediate Next Step:**

The `install` command currently configures the `launchd` service to run `urso run`, which is incorrect for the managed workflow. The next task is to finalize the managed service workflow:

1.  **Add a `--managed` flag** to the `run` command.
2.  **Implement the managed logic:** When `urso run --managed` is called, it must:
    *   Ignore any `--config` flag.
    *   Load machine credentials using the `CredentialStore`.
    *   Fetch runner config and GitHub tokens from the API using the `APIClient`.
    *   Load the local `config.yaml` *only* to get the `rootDir`.
    *   Merge the `rootDir` with the API-provided runners.
    *   Execute the sync logic.
3.  **Update the `launchd` template** in `service.go` to call `urso run --managed`.
4.  **Add unit tests** for this new `--managed` flag logic in `TestCLI_Run`.