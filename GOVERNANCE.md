# Ruptura Governance

This document describes how the Ruptura project is governed, how decisions are made, and how contributors can grow into leadership roles.

---

## Project Roles

### Contributor

Anyone who submits a pull request, opens an issue, improves documentation, or participates in GitHub Discussions is a **Contributor**. Contributors do not need any formal approval — participation is open to everyone under the [Apache 2.0 license](LICENSE) and [DCO](CONTRIBUTING.md#developer-certificate-of-origin-dco).

### Reviewer

A **Reviewer** has demonstrated sustained, quality contributions and has been invited by a Maintainer to review pull requests. Reviewers can approve PRs but cannot merge them without a Maintainer's final approval. Reviewers are listed in `CODEOWNERS` for the areas they know best.

Criteria for becoming a Reviewer:
- At least 5 merged pull requests of meaningful scope
- Familiarity with at least one major subsystem (engine, UI, Helm, operator)
- Demonstrated understanding of the project's design principles

### Maintainer

A **Maintainer** has full commit access and is responsible for the overall health of the project: merging PRs, cutting releases, managing the roadmap, and representing the project in the community.

Criteria for becoming a Maintainer:
- Active as a Reviewer for at least 3 months
- Consistent, high-quality contributions across multiple releases
- Nominated by an existing Maintainer and approved by lazy consensus (see below)

| Name | GitHub | Email | Role |
|------|--------|-------|------|
| Selim Benfradj | @benfradjselim | benfradjselim@gmail.com | Founding Architect & Maintainer |

---

## Maintainer Onboarding

When a new Maintainer is approved:
1. Their GitHub account is added to the `ruptura` GitHub organization with write access.
2. They are added to the Maintainer table above via a PR (signed off by an existing Maintainer).
3. They are added to `CODEOWNERS` as appropriate.
4. They receive access to the `security@ruptura.dev` inbox.

## Maintainer Offboarding

A Maintainer who is inactive for more than 6 months (no PRs, reviews, or community participation) will be moved to **Emeritus Maintainer** status after a private notice and a 30-day grace period. Emeritus Maintainers retain their contributor credit but lose commit access. They may return to active Maintainer status at any time by resuming contributions and requesting re-instatement from an active Maintainer.

A Maintainer may also voluntarily step down at any time by notifying the project via a GitHub issue or email.

---

## Decision Making

**Lazy consensus** is the default decision-making mechanism. A proposal (feature, roadmap item, process change) posted to GitHub Discussions or as a GitHub Issue is considered accepted if no substantive objection is raised within **72 hours**.

**Majority vote** applies when:
- Lazy consensus fails (a substantive objection is raised and cannot be resolved by discussion)
- A breaking API or architecture change is proposed
- A new Maintainer is being confirmed
- The project is making a significant governance or licensing change

For a majority vote, each Maintainer has one vote. A simple majority (>50%) of active Maintainers is required. Votes are cast publicly in the relevant GitHub thread.

---

## Release Process

Ruptura follows [Semantic Versioning](https://semver.org/). Each release is tagged and signed by a Maintainer. A changelog entry is required in `CHANGELOG.md` for every release. Release artifacts are published to `ghcr.io/benfradjselim/ruptura` and the Helm chart to `ghcr.io/benfradjselim/charts/ruptura`.

---

## CNCF Alignment

Ruptura is applying to the [CNCF Sandbox](https://www.cncf.io/sandbox-projects/) program. In preparation, the project maintains:

- **Apache 2.0 license** — confirmed open-source license acceptable to CNCF
- **Open governance** — this document, publicly maintained
- **Security policy** — [`SECURITY.md`](SECURITY.md) with coordinated disclosure and CVE process
- **Public roadmap** — tracked in GitHub Issues and Milestones
- **Contributor ladder** — documented above (Contributor → Reviewer → Maintainer)
- **DCO sign-off** — required for all commits (see [`CONTRIBUTING.md`](CONTRIBUTING.md))
- **Vendor neutrality** — Ruptura is designed to run on any conformant Kubernetes cluster and is not tied to any cloud provider or commercial offering

Once accepted into the CNCF Sandbox, governance will be updated to incorporate any CNCF-specific requirements (e.g., TOC liaison, due diligence report).

---

## Conflict of Interest Policy

Maintainers are expected to act in the best interests of the Ruptura project and its community, not in the interest of any employer or commercial entity. When a Maintainer has a material conflict of interest with respect to a specific decision (for example, a PR that directly benefits their employer), they must disclose that conflict publicly in the relevant GitHub thread and recuse themselves from the vote on that decision. Other Maintainers may request disclosure if they believe a conflict exists. This policy applies to all project decisions, including merging pull requests, setting the roadmap, and evaluating third-party integrations.
