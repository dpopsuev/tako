#!/usr/bin/env bash
# dummy-upper: reads JSON from stdin, returns {"upper": "UPPERCASED"}
# Simple proof of dispatch — doesn't parse input, just returns a known value.
set -euo pipefail
cat > /dev/null  # consume stdin
echo '{"upper": "TRANSFORMED"}'
