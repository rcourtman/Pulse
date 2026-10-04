#!/usr/bin/env python3
"""Validate only the joined CI node's identity before exposing its loopback UI.

Reads local `tailscale status --json`; makes no network request and never emits
peer inventory, raw status or credentials. OAuth/ACL scope is configured by the
Tailnet owner, not inferred from a DNS name or a tag observation.
"""

import json
import re
import sys

MAX_STATUS_BYTES = 1024 * 1024
ERROR = "Release browser requires a running Self node with only tag:ci-deploy and a business-tailnet DNS name"


def self_origin(status):
    if not isinstance(status, dict) or status.get("BackendState") != "Running":
        raise ValueError(ERROR)
    node = status.get("Self")
    if (not isinstance(node, dict) or node.get("Online") is not True
            or node.get("Tags") != ["tag:ci-deploy"]):
        raise ValueError(ERROR)
    name = node.get("DNSName")
    if not isinstance(name, str):
        raise ValueError(ERROR)
    name = name.removesuffix(".")
    if not re.fullmatch(r"[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.tawny-powan\.ts\.net", name):
        raise ValueError(ERROR)
    return "https://" + name


def main():
    try:
        raw = sys.stdin.buffer.read(MAX_STATUS_BYTES + 1)
        if len(raw) > MAX_STATUS_BYTES:
            raise ValueError(ERROR)
        origin = self_origin(json.loads(raw))
    except (ValueError, UnicodeError):
        print(ERROR, file=sys.stderr)
        return 2
    print(origin)
    return 0


if __name__ == "__main__":
    sys.exit(main())
