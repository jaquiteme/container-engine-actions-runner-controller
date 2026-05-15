# Stage 1 : Build
# Compile a fully static binary so it runs on the scratch-based runtime image.
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata file

WORKDIR /build

# Fetch dependencies in a dedicated layer (cached unless go.mod/go.sum change).
COPY go.mod go.sum ./
RUN go mod download -x

COPY server/ ./server/

# CGO_ENABLED=0 + -extldflags=-static = pure static ELF, no libc dependency.
# -w -s   strip DWARF and symbol table = smaller binary.
# -trimpath removes local build paths from the binary (reproducibility + security).
RUN CGO_ENABLED=0 GOOS=linux \
    go build \
      -ldflags="-w -s -extldflags=-static" \
      -trimpath \
      -o /out/ce-arc-server \
      ./server

# Verify the binary is truly static (fails the build if it is not).
RUN file /out/ce-arc-server | grep -q "statically linked" || \
    (echo "ERROR: binary is not statically linked" && exit 1)


# Stage 2 : Runtime
# distroless/static-debian12 ships:
#   • no shell, no package manager, no libc — minimal attack surface
#   • up-to-date CA certificates (needed for GitHub API calls)
#   • tzdata
# :nonroot pre-sets USER to 65532 (nonroot); overridden at runtime with
#   --user $(id -u):$(id -g) so the process UID matches the Podman socket owner.
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="ce-arc-server" \
      org.opencontainers.image.description="Container Engine Actions Runner Controller — GitHub webhook orchestrator" \
      org.opencontainers.image.source="https://github.com/jaquiteme/container-engine-actions-runner-controller" \
      org.opencontainers.image.licenses="MIT"

# Copy the static binary and pre-set ownership to the distroless nonroot UID.
COPY --from=builder --chown=65532:65532 /out/ce-arc-server /usr/local/bin/ce-arc-server

# Port exposed by the webhook server (overridable via PORT env var).
EXPOSE 3000

ENTRYPOINT ["/usr/local/bin/ce-arc-server"]
