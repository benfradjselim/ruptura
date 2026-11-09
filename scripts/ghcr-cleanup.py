#!/usr/bin/env python3
"""Delete old per-commit build images (tags ``sha-<hex>``) from GHCR.

Safety rules, applied to every package version:

* A version is deleted only if it has at least one tag, every tag matches
  ``sha-<hex>``, and it is older than ``--min-age-days``.
* Untagged versions are never deleted: on multi-arch images they are the
  per-platform manifests of tagged images.
* Any version carrying a release tag (``v1.2.3``, ``latest``...) is kept.
* Digests listed with ``--protect-digest`` are kept.
* The cosign signature of a deleted version (tag ``sha256-<digest>.sig``) is
  deleted with it, and only when that version is deleted.

Without ``--apply`` nothing is deleted: the script prints what it would do.
"""

import argparse
import datetime as dt
import json
import os
import re
import sys
import urllib.request

API = "https://api.github.com"
SHA_TAG = re.compile(r"^sha-[0-9a-f]{7,40}$")


def parse_time(value):
    return dt.datetime.fromisoformat(value.replace("Z", "+00:00"))


def plan(versions, now, min_age_days, protected_digests=()):
    """Return (to_delete, kept_count). Each item of to_delete is a version dict."""
    cutoff = now - dt.timedelta(days=min_age_days)
    protected = set(protected_digests)
    by_sig_tag = {}
    for v in versions:
        for tag in v["metadata"]["container"]["tags"]:
            if tag.endswith(".sig"):
                by_sig_tag[tag] = v

    to_delete = []
    for v in versions:
        tags = v["metadata"]["container"]["tags"]
        if not tags:
            continue
        if not all(SHA_TAG.match(t) for t in tags):
            continue
        if v["name"] in protected:
            continue
        if parse_time(v["created_at"]) >= cutoff:
            continue
        to_delete.append(v)
        sig = by_sig_tag.get(v["name"].replace(":", "-") + ".sig")
        if sig is not None and len(sig["metadata"]["container"]["tags"]) == 1:
            to_delete.append(sig)
    return to_delete, len(versions) - len(to_delete)


def request(method, url, token):
    req = urllib.request.Request(url, method=method, headers={
        "Authorization": f"Bearer {token}",
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    })
    with urllib.request.urlopen(req) as resp:
        body = resp.read()
        return json.loads(body) if body else None


def list_versions(owner_path, package, token):
    versions, page = [], 1
    while True:
        url = f"{API}/{owner_path}/packages/container/{package}/versions?per_page=100&page={page}"
        batch = request("GET", url, token)
        if not batch:
            return versions
        versions.extend(batch)
        page += 1


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--owner", required=True)
    p.add_argument("--owner-type", choices=["user", "org"], default="user")
    p.add_argument("--package", action="append", required=True)
    p.add_argument("--min-age-days", type=int, default=120)
    p.add_argument("--protect-digest", action="append", default=[])
    p.add_argument("--apply", action="store_true", help="really delete (default: dry run)")
    args = p.parse_args()

    token = os.environ.get("GITHUB_TOKEN")
    if not token:
        sys.exit("GITHUB_TOKEN is not set")
    owner_path = f"users/{args.owner}" if args.owner_type == "user" else f"orgs/{args.owner}"
    now = dt.datetime.now(dt.timezone.utc)

    total = 0
    for package in args.package:
        versions = list_versions(owner_path, package, token)
        to_delete, kept = plan(versions, now, args.min_age_days, args.protect_digest)
        print(f"## {package}: {len(versions)} versions, {len(to_delete)} to delete, {kept} kept")
        for v in to_delete:
            tags = ", ".join(v["metadata"]["container"]["tags"])
            print(f"{'DELETE' if args.apply else 'would delete'} {v['created_at'][:10]} {tags}")
            if args.apply:
                request("DELETE", f"{API}/{owner_path}/packages/container/{package}/versions/{v['id']}", token)
        total += len(to_delete)
    print(f"\nTotal: {total} versions {'deleted' if args.apply else 'would be deleted (dry run)'}")


if __name__ == "__main__":
    main()
