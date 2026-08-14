#!/usr/bin/env bash
# Canned local "agent" for the README demo. It stands in for a real agent CLI
# (OpenClaw / Claude Code) wired via `type: command`: read the prompt on stdin,
# print a JSON reply the assistant parses into subject/body.
cat >/dev/null
cat <<'JSON'
{"subject":"Re: Welcome to Postero","body":"Thanks for the warm welcome! Excited to try the vim-style keys — I'll set up my account today and reach out if I hit anything."}
JSON
