#!/bin/sh
# pin-digests.sh — pull every probaci tool image and emit a tools.json that pins
# each by sha256 digest, for reproducible/tamper-evident runs.
#
# Run in a network-connected environment with a container runtime:
#
#   ./scripts/pin-digests.sh > tools.pinned.json
#
# Then merge the "tools" object into your probaci.json (or drop it at
# ~/.config/probaci/tools.json). Requires: a built probaci on PATH (or `go run`),
# a container runtime (docker/podman), and python3 for JSON handling.
set -eu

PROBACI="${PROBACI:-probaci}"
RUNTIME="${RUNTIME:-docker}"

command -v "$RUNTIME" >/dev/null 2>&1 || { echo "error: $RUNTIME not found" >&2; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "error: python3 required" >&2; exit 1; }

# Get the registry as JSON: [{name, image, tag, ref, pinned}, ...]
tools_json=$("$PROBACI" tools --json)

printf '%s' "$tools_json" | python3 -c '
import json, subprocess, sys, os

runtime = os.environ.get("RUNTIME", "docker")
tools = json.load(sys.stdin)
pinned = {}
for t in tools:
    ref = t["image"] + ":" + (t["tag"] or "latest")
    sys.stderr.write("pulling %s ...\n" % ref)
    subprocess.run([runtime, "pull", "--quiet", ref], check=True, stdout=subprocess.DEVNULL)
    out = subprocess.check_output(
        [runtime, "inspect", "--format", "{{index .RepoDigests 0}}", ref]
    ).decode().strip()
    # out looks like image@sha256:...
    digest = out.split("@", 1)[1] if "@" in out else ""
    if digest:
        pinned[t["name"]] = {"image": t["image"], "tag": t["tag"], "digest": digest}

print(json.dumps({"tools": pinned}, indent=2))
'
