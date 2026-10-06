# GreFlow v0.1.1

Patch release following the validated v0.1.0 initial release.

- Share identical destination SNAT/filter rules across multiple public port mappings.
- Add real-kernel regression tests for shared destinations, automatic failure rollback and sysctl restoration.
- Single release VERSION and a stale-commit publication guard.

Includes static Linux amd64/arm64 binaries and SHA256SUMS. All unit/race/vet, shellcheck and real-kernel network namespace integration checks gate publication. GRE is unencrypted; real VPS/provider/reboot/high-load validation remains separate from CI. See README and the v0.1.0 notes for scope.
