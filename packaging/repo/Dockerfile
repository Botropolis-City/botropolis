# Builds the APT and RPM repositories at pkg.botropolis.city; see
# tools/build-package-repos. The same image runs locally and in CI.
FROM debian:trixie-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends apt-utils createrepo-c rpm gnupg ca-certificates \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /work
