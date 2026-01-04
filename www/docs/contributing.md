# Contributing

Thank you for your interest in improving Urso! We welcome contributions from the community, whether they are bug reports, feature requests, or code improvements.

## Development Workflow

To start contributing to Urso, follow these steps:

1.  **Fork the Repository:** Create a fork of the [urso-run/urso](https://github.com/urso-run/urso) repository on GitHub.
2.  **Clone Locally:**
    ```bash
    git clone https://github.com/your-username/urso.git
    cd urso
    ```
3.  **Install Dependencies:** Ensure you have Go (matching the version in `go.mod`) and `golangci-lint` installed on your machine.
4.  **Create a Branch:** Create a new branch for your feature or bug fix:
    ```bash
    git checkout -b feature/my-new-feature
    ```

## Coding Standards

To maintain high code quality and consistency, please adhere to the following patterns already established in the project:

- **Interface-Driven Design:** Use interfaces for side effects (filesystem, networking, shell execution) to ensure the logic remains testable.
- **Table-Driven Tests:** Prefer table-driven tests for complex logic to cover multiple scenarios cleanly.
- **Structured Logging:** Use the `slog` package for all logging. Avoid printing directly to stdout/stderr except within the CLI entry points.
- **Context Propagation:** Ensure `context.Context` is passed through for cancellation and timeout support.

## Quality Control

Before submitting a Pull Request, please ensure your changes pass our local quality checks:

```bash
# Run unit tests
make test

# Run the linter
make lint

# Verify that the project builds
make build
```

## Documentation

Urso uses the **README.md** and the **www/** directory as the authoritative sources of documentation. 
- If you change CLI flags or configuration structures, please update the corresponding pages in `www/docs/`.
- To preview your documentation changes locally using Docker:
  ```bash
  make docs-serve
  ```

## Submitting a Pull Request

1.  **Commit your changes:** Write clear, concise commit messages.
2.  **Push to GitHub:**
    ```bash
    git push origin feature/my-new-feature
    ```
3.  **Open a PR:** Submit your Pull Request against the `main` branch of the original Urso repository.
4.  **Review:** Provide a clear description of your changes and reference any related issues.

## Reporting Issues

If you find a bug or have a feature request, please [open an issue](https://github.com/urso-run/urso/issues) on GitHub. For security-related reports, please refer to our [Security Policy](security.md).

For direct inquiries or specific questions, you can reach out to the team at [hello@urso.run](mailto:hello@urso.run) or contact [adam@urso.run](mailto:adam@urso.run) or [petr@urso.run](mailto:petr@urso.run) directly.

Thanks for helping improve Urso!