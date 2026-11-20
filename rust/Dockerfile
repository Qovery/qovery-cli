# Image of the Rust CLI, same runtime as the Go one (../Dockerfile). The binary is the static linux/amd64
# release build of .github/workflows/rust-build.yml; the build context holds it next to the `docker/` folder:
#
#   context/qovery
#   context/docker/exec.sh
#   docker build -f rust/Dockerfile context
FROM public.ecr.aws/r3m4q3r9/pub-mirror-debian:bookworm-slim

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get install -y --no-install-recommends ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists

WORKDIR /app

COPY docker/ docker
RUN chmod +x ./docker/exec.sh

COPY qovery /app/qovery

# Add the /app directory to the PATH environment variable
ENV PATH="/app:${PATH}"

ENTRYPOINT ["sh", "./docker/exec.sh"]
