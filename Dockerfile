FROM public.ecr.aws/r3m4q3r9/pub-mirror-go:1.25.1 as builder

ARG APP_VERSION=unknown

# Set the working directory within the container
WORKDIR /app

# Copy go.mod and go.sum files to the container's working directory
COPY go.mod go.sum ./

# Download dependencies
# Retried: proxy.golang.org intermittently resets HTTP/2 streams mid-download.
# Only transport failures are worth retrying. If the proxy or VCS returned a
# definitive answer, or the error is in local files, another attempt cannot
# change it, so fail fast instead of sleeping through the backoff.
RUN --mount=type=cache,target=/go/pkg/mod \
    for attempt in 1 2 3 4 5; do \
        if go mod download >/tmp/godl.log 2>&1; then exit 0; fi; \
        cat /tmp/godl.log; \
        if grep -qE 'SECURITY ERROR|checksum mismatch|missing go.sum entry|errors parsing go.mod|invalid version|unknown revision|malformed module path|module lookup disabled' /tmp/godl.log; then \
            echo "go mod download failed with a non-transient error; not retrying"; \
            exit 1; \
        fi; \
        echo "go mod download failed (attempt ${attempt}/5)"; \
        [ "${attempt}" = 5 ] && break; \
        sleep $((attempt * 5)); \
    done; \
    exit 1

# Copy the source code to the container's working directory
COPY . .

# Build the Go application
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o qovery -ldflags "-X github.com/qovery/qovery-cli/utils.Version=$APP_VERSION"

FROM public.ecr.aws/r3m4q3r9/pub-mirror-debian:bookworm-slim as runner

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get install -y --no-install-recommends ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists

WORKDIR /app

# make the exec.sh file executable
COPY docker/ docker
RUN chmod +x ./docker/exec.sh

COPY --from=builder /app/qovery /app/qovery

# Add the /app directory to the PATH environment variable
ENV PATH="/app:${PATH}"

ENTRYPOINT ["sh", "./docker/exec.sh"]
