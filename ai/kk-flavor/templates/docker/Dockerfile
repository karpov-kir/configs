# The build stage runs natively on the builder and builds for the target (Go: GOOS=$TARGETOS
# GOARCH=$TARGETARCH), so no toolchain runs under emulation.
FROM --platform=$BUILDPLATFORM <build-base> AS builder
ARG TARGETOS TARGETARCH
WORKDIR /app
# Install dependencies, then build the artifact
# ...

FROM <runtime-base>
WORKDIR /app
COPY --from=builder /app/<artifact> ./
EXPOSE 4040
CMD ["<run-the-artifact>"]
