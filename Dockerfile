# Minimal runtime image for probaci. GoReleaser builds the binary and copies it
# into the build context; this just wraps it. probaci itself brokers other tools
# through the host container runtime, so this image is for running probaci as a
# command, not for nested builds.
FROM gcr.io/distroless/static:nonroot
COPY probaci /usr/bin/probaci
ENTRYPOINT ["/usr/bin/probaci"]
