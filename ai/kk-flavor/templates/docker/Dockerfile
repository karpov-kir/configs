# The build stage runs natively on the builder and builds for the target (Go: GOOS=$TARGETOS
# GOARCH=$TARGETARCH), so no toolchain runs under emulation.
FROM --platform=$BUILDPLATFORM <build-image>:<major> AS builder
ARG TARGETOS TARGETARCH
WORKDIR /app
# Install dependencies, then build the artifact
# ...

FROM <runtime-image>:<major>
WORKDIR /app
COPY --from=builder /app/<artifact> ./
CMD ["<run-the-artifact>"]
