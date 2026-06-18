# Security Policy

## Reporting a vulnerability

Please report security issues privately to **opensource@simtabi.com**. Do not
open a public issue for vulnerabilities. You will receive an acknowledgement
within a few business days, and we will keep you informed as we work on a fix.

When possible, include:

- a description of the issue and its impact,
- steps to reproduce or a proof of concept,
- affected version(s) and platform.

## Supported versions

The latest released minor version receives security fixes. Pre-1.0, only the
most recent release is supported.

## Security model

probaci executes third-party container images, runs CI tools over repository
content, and handles tokens. Its defenses:

- **No shell execution.** Commands run via argument vectors only — config values
  are never concatenated into a shell string.
- **Least-privilege containers.** Non-root, all capabilities dropped,
  `no-new-privileges`, read-only root filesystem with a tmpfs scratch, resource
  caps, and offline networking for tools that don't need it. The repository is
  mounted read-only except for the isolated `clean-clone` export.
- **Image pinning.** Images are referenced by digest where pinned; cosign
  verification is advisory by default and can be made strict.
- **Secret redaction.** A central redactor scrubs secret values and token-shaped
  strings from stdout, `--json`, and the log file. Secrets are passed via env
  file, never argv.
- **No socket mount by default.** Nested-Docker runners (act, gitlab-ci-local)
  that need the container socket are opt-in and best run on the rootless/Podman
  backend.

See [docs/security.md](docs/security.md) for the full details.
