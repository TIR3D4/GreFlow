# Implementation decisions

The supplied archive matches upstream core commit `37b32b8`; its demo is preserved
under `web/console` and never embedded in the production binary. No remote host
is contacted by tests except explicitly configured isolated test endpoints.

The control plane is Go, SQLite, embedded static JavaScript/CSS and local
Vazirmatn. Only the Linux dispatcher imports the kernel adapter. SSH uses fixed commands and a pinned host key. Tunnel operations pass typed JSON
to the dispatcher; preparation uploads bundled verified bytes, and host information,
SSH logs and explicitly confirmed reboot use separate fixed commands.
Discovery never applies configurations. Desired config hashes guard mutations.
Durable jobs become uncertain after interrupted execution, never auto-replayed.

Port updates use rule differences rather than recreating GRE interfaces. Transfers
journal both configs before removing any mapping and recover only owned resources.
Kernel transactions are best effort, not atomic. UDP/application verification is
never inferred from TCP. Old evidence is marked stale by its expiry timestamp.

Release evidence and remaining limits are recorded in `TEST-RESULTS.md`. Console
releases use their own version and are published by CI after Windows/Linux gates
succeed; the original CLI version and release are preserved.

The follow-up product request removes setup codes and TOTP. Migration 4 retires old
TOTP material, while retaining legacy schema columns for backup compatibility.
Onboarding uses explicit first-connection trust and pins the observed key on save.
SSH password/private-key/passphrase envelopes reuse the encrypted vault; old raw
private-key records remain readable. Root bypasses sudo; non-root still uses a
constant restricted `sudo -n` command. The `.greflow` file wraps the authenticated
GFB1 payload in a versioned product-specific envelope; old GFB1 files still restore.
