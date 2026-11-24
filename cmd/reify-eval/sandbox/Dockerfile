# Sandbox image for one agent attempt. The fixture repo is bind-mounted
# at /work at run time; claude edits it; the host reads the git diff.
FROM node:22-slim

RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @anthropic-ai/claude-code

WORKDIR /work
# Run as the HOST uid via `docker run --user` so files the agent writes
# are owned by the host user (clean git diff, no dubious-ownership).
# claude --dangerously-skip-permissions refuses uid 0, so the caller MUST
# pass --user <non-root-host-uid> and -e HOME=/tmp (a writable home).
# ANTHROPIC_API_KEY is passed at run time via `docker run -e`.
