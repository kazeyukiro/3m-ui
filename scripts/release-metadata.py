#!/usr/bin/env python3
"""Validate immutable release tags and emit GitHub Actions metadata."""
import os
import re
import sys


def metadata(tag: str) -> dict[str, str]:
    match = re.fullmatch(
        r"v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)"
        r"(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?"
        r"(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?",
        tag,
    )
    if not match:
        raise ValueError(f"Release tag must be SemVer, for example v1.2.3 or v1.3.0-rc.1: {tag}")
    prerelease = match.group(4)
    if prerelease and any(p.isdecimal() and len(p) > 1 and p[0] == "0" for p in prerelease.split(".")):
        raise ValueError("Numeric prerelease identifiers cannot have leading zeroes")
    # OCI tag syntax excludes '+'. Double underscore cannot occur in SemVer and
    # gives build metadata a distinct spelling without colliding with '-rc'.
    image_tag = tag.replace("+", "__")
    if len(image_tag) > 128:
        raise ValueError("Release tag is too long for an OCI image tag")
    return {
        "tag": tag,
        "prerelease": str(bool(prerelease)).lower(),
        "latest": str(not prerelease).lower(),
        "image_tag": image_tag,
    }


if __name__ == "__main__":
    try:
        values = metadata(sys.argv[1])
    except (IndexError, ValueError) as exc:
        sys.exit(str(exc))
    output = "".join(f"{key}={value}\n" for key, value in values.items())
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as stream:
            stream.write(output)
    else:
        print(output, end="")
