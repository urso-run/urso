# Service Management

Urso is designed to run as a reliable background agent. It leverages the native service managers of the host operating system to ensure that runners are always synchronized and persistent across reboots.

## Platform Integration

Urso automatically detects the operating system and integrates with the appropriate service manager:

- **macOS:** Integrates with `launchd` as a User Agent.
- **Linux:** Integrates with `systemd` as a User Service.

## The Urso Service

When you run `urso install`, a system service is created that executes `urso run` on a recurring schedule or ensures it stays alive in the background.

### macOS (launchd)

On macOS, Urso creates a property list (plist) file at:
`~/Library/LaunchAgents/com.urso-run.urso.plist`

- **Logs:** Combined stdout and stderr are written to `~/Library/Logs/com.urso-run.urso.log`.
- **Management:** You can manually interact with the service using `launchctl`:
  ```bash
  # Kickstart the service immediately
  launchctl kickstart -k gui/$(id -u)/com.urso-run.urso
  ```

### Linux (systemd)

On Linux, Urso creates a unit file at:
`~/.config/systemd/user/com.urso-run.urso.service`

- **Logs:** Managed by the systemd journal.
- **Management:** Use `systemctl --user` to interact with the service:
  ```bash
  # Check service status
  systemctl --user status com.urso-run.urso

  # View real-time logs
  journalctl --user -u com.urso-run.urso -f
  ```

## Runner Services

In addition to the main Urso orchestrator service, each individual GitHub Actions runner managed by Urso is also installed as a separate system service.

- **Naming:** These services are typically named using the runner name provided in your configuration.
- **Lifecycle:** Urso handles the installation, starting, stopping, and uninstallation of these runner services automatically during the `sync` process.

!!! warning "Permissions"
    On Linux, the GitHub Actions runner scripts (`svc.sh`) require `sudo` to install and manage systemd services. Urso uses `sudo -n` (non-interactive) to perform these operations. Ensure the user running Urso has the necessary passwordless sudo permissions for these scripts if required.

## Observability with Vector

If the `vector` binary is present in your system `PATH` during installation, Urso will also configure and manage a **Vector** service.

- **Config:** Located at `~/.urso/vector/vector.yaml`.
- **Function:** Vector monitors the logs from both Urso and the individual GitHub runners, forwarding them to the Urso Dashboard for centralized monitoring.
- **Service Name:** `com.urso-run.vector` (managed via launchd or systemd).

## Troubleshooting

If services are not starting correctly:

1.  **Check Urso Logs:** Review the logs in `~/.urso/logs/` or via `journalctl`.
2.  **Verify PATH:** Ensure the `urso` binary is in a persistent location and its path is correctly reflected in the service configuration.
3.  **Manual Sync:** Run `urso run` manually in your terminal to see if any immediate errors occur during reconciliation.