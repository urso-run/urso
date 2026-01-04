# Installation

Urso can be installed on macOS and Linux. It supports two main modes of operation: **Local/Free Mode** and **Managed/Cloud Mode**.

## Prerequisites

- **macOS:** macOS 11.0 (Big Sur) or later.
- **Linux:** A distribution with `systemd` (e.g., Ubuntu, Debian, CentOS).
- **GitHub:** A registration token for your organization or repository.

## Quick Install (Recommended)

The easiest way to install Urso is using the official installation script.

### Managed / Cloud Mode
If you are using the [Urso Dashboard](https://urso.run), run the following command with your registration token:

```bash
curl -sSL https://raw.githubusercontent.com/urso-run/urso/main/scripts/install.sh | sh -s -- <YOUR_REGISTRATION_TOKEN>
```

### Local / Free Mode
For standalone use without the cloud dashboard:

```bash
curl -sSL https://raw.githubusercontent.com/urso-run/urso/main/scripts/install.sh | sh
```

## Manual Installation

If you prefer to install the binary manually:

1.  **Download the binary:** Grab the latest release for your platform from the [GitHub Releases](https://github.com/urso-run/urso/releases) page.
2.  **Move to PATH:**
    ```bash
    mv urso /usr/local/bin/urso
    chmod +x /usr/local/bin/urso
    ```
3.  **Initialize:**
    ```bash
    urso init
    ```

## Verifying Installation

Verify that Urso is installed correctly by checking the version:

```bash
urso --version
```

## Next Steps

Once installed, proceed to the [Configuration](configuration.md) guide to set up your runner definitions.