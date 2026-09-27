# Contributing to Havoc Engine

Thank you for your interest in contributing to Havoc Engine! We welcome all contributions, from bug reports to new features.

## Commit Message Convention

This project adheres to the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) standard. This helps us maintain a readable and automated release history.

When committing your changes, please use one of the following prefixes:
- `feat:` for a new feature (e.g., `feat: add network latency experiment`)
- `fix:` for a bug fix (e.g., `fix: handle missing namespace gracefully`)
- `docs:` for documentation changes only
- `test:` for adding or modifying tests
- `chore:` for maintenance tasks, dependency updates, etc.
- `refactor:` for code changes that neither fix a bug nor add a feature

Example:
```bash
git commit -m "feat: add cron scheduler status command"
```

## Running Tests Locally

Havoc Engine is designed to be highly testable without requiring a live Kubernetes cluster for unit testing. The project uses `k8s.io/client-go/kubernetes/fake` for testing.

### Go Engine
To run the Go tests for the engine:
```bash
# Run all tests
go test -v ./...

# Run tests with coverage
go test -cover -v ./...
```

### Dashboard
To run the Next.js tests/linting (if configured):
```bash
cd havoc-dashboard
npm run lint
# npm run test (if you add jest/vitest in the future)
```

## Pull Request Process

1. Fork the repository and create your feature branch from `main`.
2. Ensure you have tested your changes. If adding a new feature, please add unit tests.
3. Make sure the CI pipeline passes (it runs `go test` and `npm run build` on PRs).
4. Provide a clear and descriptive PR title and description.
