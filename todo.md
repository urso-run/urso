# Urso Project TODO

## Robustness & Error Handling
- [x] **Hostname Error Handling**: In `urso/cli.go`, handle potential errors from `os.Hostname()` in the `Uninstall` method instead of ignoring them.
- [x] **Non-interactive Sudo**: Ensure `runSvc` in `urso/executor.go` doesn't hang in background/service mode if `sudo` requires a password.
- [ ] **Runner Health Check**: Implement a post-start check in `urso/runner.go` to verify the runner successfully connected to GitHub Actions.

## Performance & Efficiency
- [ ] **Native Extraction**: Replace external `tar` command dependency with Go's `archive/tar` and `compress/gzip` packages.

## Code Consistency & Maintainability
- [x] **Unify Structs**: Consolidate `RunnerConfig` and `apiRunnerConfig` into a single shared struct in `urso/urso.go`.
- [x] **Centralize Defaults**: Consolidate `DefaultRootDir` and `defaultUrsoHome` logic to avoid path confusion.
- [x] **Clean Up Redundant Logic**: Remove redundant `os.Chmod` calls in `urso/runner.go` since `MkdirAll` already sets permissions.

## Developer Experience (DX)
- [x] **Interactive Init**: Enhance `urso init` to optionally prompt for the GitHub URL and labels instead of providing a static template.
