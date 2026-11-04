#!/usr/bin/env python3
"""Pins every image of an operator bundle CSV to its registry digest, as a
disconnected install requires: mirrors only redirect references by digest.

  python3 scripts/pin-bundle-digests.py bundle/manifests/ruptura-operator.clusterserviceversion.yaml

Run after the images of the release are published. Rewrites `image:` and
`value:` lines that name a tag, then checks that relatedImages lists every
image the CSV uses and that no tag is left.
"""
import json
import re
import sys
import urllib.parse
import urllib.request

REGISTRIES = {
    'docker.io': ('https://registry-1.docker.io', 'https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull'),
    'ghcr.io': ('https://ghcr.io', 'https://ghcr.io/token?scope=repository:%s:pull'),
    'quay.io': ('https://quay.io', 'https://quay.io/v2/auth?service=quay.io&scope=repository:%s:pull'),
}
ACCEPT = ', '.join([
    'application/vnd.oci.image.index.v1+json',
    'application/vnd.docker.distribution.manifest.list.v2+json',
    'application/vnd.oci.image.manifest.v1+json',
    'application/vnd.docker.distribution.manifest.v2+json',
])
REF = re.compile(r'^(\s*-?\s*(?:image|value):\s*)"?((?:%s)/[^\s"@:]+):([^\s"@]+)"?\s*$' % '|'.join(re.escape(r) for r in REGISTRIES))


def digest(registry, repo, tag):
    base, token_url = REGISTRIES[registry]
    with urllib.request.urlopen(token_url % urllib.parse.quote(repo, safe='/'), timeout=30) as resp:
        token = json.load(resp)['token']
    req = urllib.request.Request('%s/v2/%s/manifests/%s' % (base, repo, tag), method='HEAD',
                                 headers={'Authorization': 'Bearer ' + token, 'Accept': ACCEPT})
    with urllib.request.urlopen(req, timeout=30) as resp:
        found = resp.headers.get('Docker-Content-Digest', '')
    if not re.fullmatch(r'sha256:[0-9a-f]{64}', found):
        raise SystemExit('no digest for %s/%s:%s' % (registry, repo, tag))
    return found


def pin(lines):
    cache, out = {}, []
    for line in lines:
        m = REF.match(line)
        if not m:
            out.append(line)
            continue
        prefix, name, tag = m.groups()
        registry, repo = name.split('/', 1)
        if '/' not in repo and registry == 'docker.io':
            repo = 'library/' + repo
        if (name, tag) not in cache:
            cache[(name, tag)] = digest(registry, repo, tag)
            print('%s:%s -> %s' % (name, tag, cache[(name, tag)]), file=sys.stderr)
        out.append('%s%s@%s\n' % (prefix, name, cache[(name, tag)]))
    return out


def check(text):
    used = set(re.findall(r'(?:image|value):\s*"?(\S+@sha256:[0-9a-f]{64})', text))
    related = text.split('relatedImages:', 1)[1].split('\n  icon:', 1)[0] if 'relatedImages:' in text else ''
    missing = sorted(i for i in used if i not in related)
    tags = [m.group(0).strip() for m in map(REF.match, text.splitlines(True)) if m]
    if missing or tags or 'RELATED_IMAGE_RUPTURA' not in text:
        raise SystemExit('not pinned: tags %s, missing from relatedImages %s, RELATED_IMAGE_RUPTURA %s'
                         % (tags, missing, 'RELATED_IMAGE_RUPTURA' in text))


def main(path):
    with open(path) as f:
        lines = f.readlines()
    text = ''.join(pin(lines))
    check(text)
    with open(path, 'w') as f:
        f.write(text)


if __name__ == '__main__':
    main(sys.argv[1])
