# Security Policy

## Supported Versions

| Version | Support status |
|---------|---------------|
| v7.1.x  | Fully supported — all security fixes |
| v7.0.x  | Security fixes only |
| < v7.0  | Unsupported — please upgrade |

---

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Email the security team at **security@ruptura.dev** with:

- A description of the vulnerability and its potential impact
- Steps to reproduce (proof-of-concept if possible)
- Affected versions
- Any suggested mitigations

### Response SLA

| Milestone | Target |
|-----------|--------|
| Acknowledgement | 48 hours |
| Triage and severity assessment | 5 business days |
| Fix for Critical / High severity | 14 days |
| Fix for Medium / Low severity | 90 days |

We will keep you informed of progress throughout the process.

---

## Coordinated Disclosure / Embargo

Ruptura follows a coordinated disclosure model:

1. You report the vulnerability to security@ruptura.dev.
2. We confirm receipt within 48 hours and begin triage.
3. We develop and test a fix privately.
4. We request a CVE identifier via [GitHub Security Advisories](https://github.com/benfradjselim/ruptura/security/advisories) once the fix is ready.
5. We release the patched version and publish the GitHub Security Advisory.
6. **Embargo period:** We ask reporters to hold public disclosure for **7 days after the patched release** ships, to give users time to upgrade.

If you need to disclose sooner for any reason, please discuss with us so we can coordinate.

---

## GitHub Security Advisories

We use [GitHub Security Advisories](https://github.com/benfradjselim/ruptura/security/advisories) to track and disclose security issues. Once a fix is released, the advisory is published with full details, affected versions, and CVE reference.

---

## CVE Numbering

Ruptura will request CVE identifiers through the GitHub Security Advisory process for any confirmed vulnerability with a CVSS score of Medium or higher (CVSS >= 4.0).

---

## Scope

The following are in scope:

- The Ruptura engine binary (`workdir/`)
- The Helm chart (`helm/`)
- The Ruptura operator (`operator/`)
- The Svelte dashboard (`ui/`)
- The `ruptura-ctl` CLI

Out of scope: third-party dependencies (report those to their respective projects), the public demo instance, and issues requiring physical access.

---

## Contact

security@ruptura.dev

Maintainer: Selim Benfradj — [@benfradjselim](https://github.com/benfradjselim)
