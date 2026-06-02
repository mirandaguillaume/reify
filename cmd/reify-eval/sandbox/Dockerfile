# Sandbox image for one agent attempt. The fixture repo is bind-mounted
# at /work at run time; claude edits it; the host reads the git diff.
FROM node:22-slim

RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @anthropic-ai/claude-code

WORKDIR /work
# ANTHROPIC_API_KEY is passed at run time via `docker run -e`.
# No network egress beyond the Anthropic API is required.
