#!/bin/sh

AGENT_BIN="${BESZEL_AGENT_BIN:-/usr/local/bin/agent}"

status="200 OK"
body="ok"

if "$AGENT_BIN" health >/dev/null 2>&1; then
    body="ok"
elif "$AGENT_BIN" --help 2>&1 | grep -q '[[:space:]]health'; then
    status="503 Service Unavailable"
    body="agent health failed"
else
    body="ok (health command unavailable)"
fi

printf 'Status: %s\r\n' "$status"
printf 'Content-Type: text/plain\r\n\r\n'
printf '%s\n' "$body"
