# Docker base image policy

The production image uses a versioned and digest-pinned Alpine base image.

Current baseline:

- Alpine: `3.24.1`
- Multi-platform image digest: `sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b`

Both Dockerfile stages reference the same `ALPINE_IMAGE` build argument so the builder and runtime filesystem baseline cannot drift independently.

## Update policy

Base-image updates are isolated dependency-only changes. For each update:

1. select a supported Alpine release;
2. pin the exact multi-platform manifest digest;
3. run the full CI matrix, including amd64/arm64 Go binary builds and the Docker image build;
4. do not combine the base-image update with application protocol or dependency changes.

The digest is intentionally committed even when the version tag is also present. The tag documents the human-readable release; the digest provides reproducible image resolution.
