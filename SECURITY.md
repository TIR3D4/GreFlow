# Security policy

v0.2.0 is the current supported release. Please report vulnerabilities using the repository's private vulnerability reporting feature when available; otherwise contact the repository owner through their GitHub profile. Do not publish passwords or sensitive configuration in issues.

GRE is plaintext and has no peer authentication. Source-IP filtering does not prevent every form of spoofing. Use TLS or another authenticated encrypted application protocol. GRE availability depends on network policy.

GreFlow runs as root, modifies forwarding/rp_filter and manages reserved interfaces and chains. Test coexistence with your firewall managers. The installer checks artifact SHA256 from the same release, not a separately trusted signature. Review scripts/source and pin trusted release versions.
