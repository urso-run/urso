# Security

Security is a core pillar of the Urso project. As an orchestrator managing CI/CD infrastructure, Urso is designed with a "security-first" approach to protect your host machines and your GitHub Actions secrets.

## Principles of Operation

Urso follows the principle of least privilege and implements several hardening measures to ensure the integrity of your runner environment.

### 1. Filesystem Permissions

Urso strictly manages filesystem permissions to prevent unauthorized access to sensitive configuration and runner data.

- **Urso Home (`~/.urso`):** The root directory and all subdirectories are created with `0700` (drwx------) permissions, ensuring only the owner can read or enter these directories.
- **Credentials (`credentials.json`):** Machine-specific tokens used to communicate with the Urso API are stored with `0600` (rw-------) permissions.
- **Runner Directories:** Each GitHub Actions runner directory is initialized with `0700` permissions. This is critical because these directories contain the `.runner` file, which holds the temporary GitHub registration secrets.

### 2. Credential Handling

Urso minimizes the exposure of sensitive tokens:

- **Tokens in Memory:** GitHub registration and removal tokens are fetched only when needed and are not persisted to disk in Local Mode (unless provided via environment variables managed by the user).
- **No Secrets in Logs:** Urso's structured logging (`slog`) is carefully audited to ensure that machine tokens, GitHub tokens, and runner secrets are never printed to stdout or written to log files.
- **Secure API Communication:** All communication with `https://urso.run` is performed over TLS.

### 3. Non-Interactive Execution

To support secure background operation on Linux, Urso uses `sudo -n` (non-interactive) when executing the official GitHub `svc.sh` scripts. This prevents the application from hanging on a password prompt and allows administrators to provide targeted, passwordless sudo access only to the specific scripts required for runner management.

## Hardening Best Practices

While Urso provides a secure foundation, we recommend the following practices for production runner hosts:

### Use Dedicated Users

Always run Urso and its managed runners under a dedicated, non-privileged system user. Avoid running the orchestrator as `root`. The GitHub Actions runner is designed to run as a standard user, and Urso inherits this model.

### Firewall & Network Security

- **Egress Only:** GitHub Actions runners generally only require egress (outbound) access to GitHub's IP ranges. 
- **Urso API:** Ensure the host can reach `https://urso.run` on port 443.
- **No Ingress:** No inbound ports need to be opened for Urso or GitHub runners to function.

### Regular Updates

Urso automatically fetches the latest versions of the GitHub Actions runner binaries during the synchronization process. However, you should also keep the Urso binary itself updated to benefit from the latest security patches and platform improvements.

### Monitoring

In Managed Mode, Urso integrates with **Vector** to forward logs to a central dashboard. Monitor these logs for unexpected errors or unauthorized configuration changes.

## Reporting Vulnerabilities

If you discover a security vulnerability within Urso, please do not open a public issue. Instead, follow the reporting instructions in our [Security Policy](https://github.com/urso-run/urso/blob/main/SECURITY.md).