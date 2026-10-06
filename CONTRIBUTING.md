# Contributing

Open an issue describing reproducible behavior, Linux/kernel version, firewall frontend, sanitized configuration and doctor output. Do not include server passwords or private application keys.

Use Go 1.22+ and no new dependency without an explanation. Keep command execution in core/system and firewall rules in the backend; never flush built-in chains. Document ownership and rollback for every host mutation.

Before a PR: gofmt, go test -race ./..., go vet ./..., shellcheck, and tests/integration.sh inside a suitable Linux CI/VM environment. Add behavior tests for validation, ownership, failure handling and actual data paths. Namespace tests require root and kernel capabilities.

Changes to configuration format require migration notes. Native nftables support should implement firewall.Backend and prove cleanup/coexistence in integration tests.
