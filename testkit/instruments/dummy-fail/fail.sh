#!/usr/bin/env bash
# dummy-fail: always exits with error
echo '{"error": "instrument failed"}' >&2
exit 1
