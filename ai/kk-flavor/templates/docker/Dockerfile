FROM <build-base> AS builder
WORKDIR /app
# Install dependencies, then build the artifact
# ...

FROM <runtime-base>
WORKDIR /app
COPY --from=builder /app/<artifact> ./
EXPOSE 4040
CMD ["<run-the-artifact>"]
