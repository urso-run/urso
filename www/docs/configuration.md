# Configuration

Urso is designed to be flexible, supporting two primary configuration workflows: **Local Mode** and **Managed Mode**.

## Urso Home Directory

By default, Urso stores all its configuration, credentials, cache, and logs in the `~/.urso` directory. You can override this using the `--urso-home` flag or the `URSO_HOME` environment variable.

## Local Mode (config.yaml)

In Local Mode, your runners are defined in a `config.yaml` file located in your Urso Home directory.

### Sample `config.yaml`

```yaml
runners:
  - name: "macos-arm64-1"
    url: "https://github.com/your-org"
    group: "Default"
    labels:
      - self-hosted
      - macos
      - arm64

  - name: "macos-arm64-2"
    url: "https://github.com/your-org"
    labels:
      - self-hosted
      - macos
```

### Environment Variables

In Local Mode, you must provide GitHub tokens via environment variables for Urso to register or remove runners:

- `GITHUB_REGISTER_TOKEN`: The token required to add a new runner to your org/repo.
- `GITHUB_REMOVE_TOKEN`: The token required to unregister a runner.

## Managed Mode

In Managed Mode (Cloud Mode), Urso retrieves its configuration from the [Urso Dashboard](https://urso.run).

- Local `config.yaml` files are ignored.
- Tokens are fetched automatically from the Urso API on each sync.
- Credentials for the machine are stored in `credentials.json` within the Urso Home directory.

To switch to Managed Mode, simply run the `urso install` command with your registration token.