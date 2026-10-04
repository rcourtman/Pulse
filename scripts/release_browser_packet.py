#!/usr/bin/env python3
"""Bind a hosted browser journey to two exact published, signed release assets.

Reads local metadata and archives; performs no network, extraction or execution.
The workflow separately verifies SSH signatures before --archives is supplied.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import tarfile


def release_identity(release, *, tag, commit, prerelease):
    pattern = r'v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)'
    if prerelease:
        pattern += r'-rc\.[1-9][0-9]*'
    if (not re.fullmatch(pattern, tag) or not re.fullmatch(r'[0-9a-f]{40}', commit)
            or release.get('tag_name') != tag or release.get('target_commitish') != commit
            or release.get('draft') is not False or release.get('prerelease') is not prerelease
            or release.get('immutable') is not True or not release.get('published_at')
            or type(release.get('id')) is not int or release['id'] <= 0):
        raise ValueError('Exact published immutable release/tag identity required')
    asset_name = f'pulse-{tag}-linux-amd64.tar.gz'
    assets = []
    for name in (asset_name, asset_name + '.sshsig'):
        matches = [a for a in release.get('assets', []) if a.get('name') == name]
        if len(matches) != 1:
            raise ValueError('Missing or ambiguous release asset')
        asset = matches[0]
        if (asset.get('browser_download_url') != f'https://github.com/rcourtman/Pulse/releases/download/{tag}/{name}'
                or type(asset.get('id')) is not int or asset['id'] <= 0
                or type(asset.get('size')) is not int or not 0 < asset['size'] <= 256 * 1024 * 1024
                or not re.fullmatch(r'sha256:[0-9a-f]{64}', str(asset.get('digest')))):
            raise ValueError('Unsafe or unbound release asset')
        assets.append({k: asset[k] for k in ['name', 'id', 'size', 'digest']})
    return {'tag': tag, 'release_id': release['id'], 'commit': commit, 'assets': assets}


def file_sha256(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def archive_identity(path):
    # Inspect only one bounded regular binary; do not extract the package.
    with tarfile.open(path, 'r:gz') as archive:
        matches = []
        total_size = 0
        for i, member in enumerate(archive):
            total_size += member.size
            if total_size > 1024 * 1024 * 1024:
                raise ValueError('Oversized expanded release archive')
            if i >= 4096:
                raise ValueError('Oversized release archive inventory')
            if member.name in ('./bin/pulse', 'bin/pulse'):
                matches.append(member)
        if len(matches) != 1 or not matches[0].isfile() or not 0 < matches[0].size <= 256 * 1024 * 1024:
            raise ValueError('Missing or ambiguous regular server binary')
        stream = archive.extractfile(matches[0])
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def packet(baseline, candidate, from_commit, to_commit, tag, archives=None):
    stable_tag = baseline.get('tag_name', '')
    releases = [release_identity(baseline, tag=stable_tag, commit=from_commit, prerelease=False),
                release_identity(candidate, tag=tag, commit=to_commit, prerelease=True)]
    if tuple(map(int, stable_tag[1:].split('.'))) >= tuple(map(int, tag[1:].split('-')[0].split('.'))):
        raise ValueError('The candidate must be newer than its published stable baseline')
    if archives is not None:
        for release in releases:
            for asset in release['assets']:
                p = archives / release['tag'] / asset['name']
                if p.is_symlink() or not p.is_file() or p.stat().st_size != asset['size']:
                    raise ValueError('Release asset size/type differs from live identity')
                if file_sha256(p) != asset['digest'].split(':')[1]:
                    raise ValueError('Release asset digest differs from live identity')
            release['binary_sha256'] = archive_identity(archives / release['tag'] / release['assets'][0]['name'])
    return {'schema_version': 1, 'repository': 'rcourtman/Pulse', 'releases': releases}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--baseline', required=True, type=Path)
    parser.add_argument('--candidate', required=True, type=Path)
    parser.add_argument('--from-commit', required=True)
    parser.add_argument('--to-commit', required=True)
    parser.add_argument('--tag', required=True)
    parser.add_argument('--archives', type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    result = packet(json.loads(args.baseline.read_text()), json.loads(args.candidate.read_text()),
                    args.from_commit, args.to_commit, args.tag, args.archives)
    args.output.write_text(json.dumps(result, indent=2) + '\n')


if __name__ == '__main__':
    main()
