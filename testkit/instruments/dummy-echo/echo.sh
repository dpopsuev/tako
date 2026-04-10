#!/usr/bin/env bash
# dummy-echo: reads JSON from stdin, wraps it in {"echo": <input>}
set -euo pipefail
input=$(cat)
echo "{\"echo\": ${input}}"
