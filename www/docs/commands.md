# Command Reference

This page provides a detailed reference for all commands available in the Urso CLI.

## Global Flags

These flags are available on all commands:

| Flag | Shorthand | Description | Default |
|------|-----------|-------------|---------|
| `--urso-home` | None | Base directory for Urso configuration, state, and logs. | `~/.urso` |
| `--help` | `-h` | Display help for the command. | N/A |
| `--version` | `-v` | Display version information. | N/A |

---

## `urso init`

Initializes the Urso home directory with a starter configuration.

### Usage
```bash
urso init [flags]
```

### Description
Creates a default `config.yaml` file in your Urso home directory. If a configuration file already exists, Urso will prompt you for confirmation before overwriting it. During initialization, you will be guided through an interactive walkthrough to set up your GitHub URL and initial runner labels.

---

## `urso run`

Executes a single reconciliation cycle to synchronize runners.

### Usage
```bash
urso run [flags]
```

### Flags
| Flag | Description |
|------|-------------|
| `--github-register-token` | GitHub token used to register new runners (Local Mode only). |
| `--github-remove-token` | GitHub token used to remove existing runners (Local Mode only). |

### Description
`urso run` is the core command of the orchestrator. It performs the following steps:
1.  **Mode Detection:** Checks for machine credentials to decide between Local or Managed mode.
2.  **Configuration Loading:** Loads runner definitions from `config.yaml` (Local) or the Urso API (Managed).
3.  **Synchronization:** Compares desired state with the current machine state.
4.  **Execution:** Adds missing runners, removes obsolete ones, and ensures background services are running.

---

## `urso install`

Registers the machine and installs Urso as a persistent system service.

### Usage
```bash
urso install [flags]
```

### Flags
| Flag | Description |
|------|-------------|
| `--urso-registration-token` | **Required.** The JWT token from the Urso Dashboard used to register this machine. |

### Description
This command is used to set up **Managed Mode**:
1.  Registers the machine with the Urso API.
2.  Persists machine credentials securely in `credentials.json`.
3.  Initializes the Vector observability configuration.
4.  Installs and starts a system service (`launchd` on macOS, `systemd` on Linux) that automatically executes `urso run` in the background.

---

## `urso uninstall`

Removes the Urso service and cleans up local runners.

### Usage
```bash
urso uninstall
```

### Description
Performs a clean removal of Urso from the system:
1.  Stops and removes the background system service agent.
2.  Uninstalls the Vector observability service if present.
3.  Attempts to unregister and delete all local runner instances from GitHub (requires API access or cached tokens).
4.  Cleans up the service configuration files.

---

## `urso completion`

Generates shell completion scripts.

### Usage
```bash
urso completion [bash|zsh|fish|powershell]
```

### Description
Generates a completion script for Urso for the specified shell. For example, to enable Zsh completions:
```bash
urso completion zsh > ~/.zsh/completions/_urso
```
