# Introduction

<img src="static/urso-logo.png" width="200" alt="Urso Logo">

Urso is a hardened CLI for provisioning and maintaining GitHub Actions runners on macOS and Linux hosts. It provides a robust, testable, and production-ready way to manage self-hosted runner fleets.

## Why Urso?

- **Platform-focused:** Native integration with `launchd` on macOS and `systemd` on Linux.
- **Dual Workflow:** Operates in a **Local/Free Mode** for standalone use or a **Managed/Cloud Mode** for centralized fleet management via [urso.run](https://urso.run).
- **Hardened Security:** Built with a "security-first" approach, ensuring strict filesystem permissions and secure credential handling.
- **Production Ready:** Includes structured logging, automatic retries, and archive caching to ensure high availability of your CI/CD infrastructure.

## How it Works

Urso acts as an orchestrator for the official GitHub Actions runner. It handles the lifecycle of the runner binaries:

1.  **Download:** Fetches the latest runner versions from GitHub.
2.  **Configure:** Registers the runner with your GitHub organization or repository.
3.  **Manage:** Installs and maintains the runner as a background system service.
4.  **Sync:** Reconciles the desired state (defined in config or via API) with the actual state of the machine.

## Getting Started

To get started with Urso, follow our [Installation Guide](install.md) to set up your first runner.

## Documentation Overview

- **[Installation](install.md):** How to get Urso running on your machine.
- **[Configuration](configuration.md):** Detailed breakdown of `config.yaml` and environment variables.
- **[Command Reference](commands.md):** CLI usage and flag documentation.
- **[Service Management](services.md):** Deep dive into how Urso manages background agents.
- **[Security](security.md):** Security considerations and hardening best practices.

## Contact

For inquiries or support, you can reach out to the Urso team at [hello@urso.run](mailto:hello@urso.run) or contact the founders directly: [adam@urso.run](mailto:adam@urso.run) or [petr@urso.run](mailto:petr@urso.run).