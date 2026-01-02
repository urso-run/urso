# Urso Project TODO

## Robustness & Error Handling
- [ ] **Systemd Compatibility**: Update `systemdTemplate` in `urso/service.go` to be compatible with versions older than v240 (remove or make `append:` prefix conditional).
- [ ] **Hostname Error Handling**: In `urso/cli.go`, handle potential errors from `os.Hostname()` in the `Uninstall` method instead of ignoring them.
- [ ] **Non-interactive Sudo**: Ensure `runSvc` in `urso/executor.go` doesn't hang in background/service mode if `sudo` requires a password.
- [ ] **Runner Health Check**: Implement a post-start check in `urso/runner.go` to verify the runner successfully connected to GitHub Actions.

## Performance & Efficiency
- [ ] **Parallel Synchronization**: Use `errgroup.Group` in `urso/runner.go` to parallelize runner creation and removal.
- [ ] **Native Extraction**: Replace external `tar` command dependency with Go's `archive/tar` and `compress/gzip` packages.

## Security
- [ ] **Secure Credential Storage**: Explore using system keychains (macOS Keychain, Linux Secret Service) instead of plain JSON files for machine tokens.
- [ ] **GitHub API Authentication**: Support using a `GITHUB_TOKEN` for `fetchLatestRelease` in `urso/github.go` to avoid rate limiting.

## Code Consistency & Maintainability
- [ ] **Unify Structs**: Consolidate `RunnerConfig` and `apiRunnerConfig` into a single shared struct in `urso/urso.go`.
- [ ] **Configurable API URL**: Remove hardcoded `https://urso.run` in `main.go` and allow overrides via environment variables or flags.
- [ ] **Centralize Defaults**: Consolidate `DefaultRootDir` and `defaultUrsoHome` logic to avoid path confusion.
- [ ] **Clean Up Redundant Logic**: Remove redundant `os.Chmod` calls in `urso/runner.go` since `MkdirAll` already sets permissions.

## Developer Experience (DX)
- [ ] **Improved Log Formatting**: Better distinguish between `urso` logs and output from external scripts (`config.sh`, `svc.sh`).
- [ ] **Interactive Init**: Enhance `urso init` to optionally prompt for the GitHub URL and labels instead of providing a static template.