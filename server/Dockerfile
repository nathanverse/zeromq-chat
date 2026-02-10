FROM ubuntu:22.04 AS deps

ENV DEBIAN_FRONTEND=noninteractive
ARG TARGETARCH

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        curl \
        git \
        cmake \
        ninja-build \
        pkg-config \
        zip \
        unzip \
        tar \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY scripts/bootstrap_vcpkg.sh scripts/bootstrap_vcpkg.sh
COPY server/vcpkg.json server/vcpkg.json
COPY server/ports server/ports

RUN if [ "$TARGETARCH" = "arm64" ]; then export VCPKG_DEFAULT_TRIPLET=arm64-linux; \
    elif [ "$TARGETARCH" = "amd64" ]; then export VCPKG_DEFAULT_TRIPLET=x64-linux; \
    else export VCPKG_DEFAULT_TRIPLET=x64-linux; fi; \
    echo "$VCPKG_DEFAULT_TRIPLET" > /tmp/vcpkg_triplet; \
    bash scripts/bootstrap_vcpkg.sh

FROM deps AS build

WORKDIR /app
COPY --from=deps /tmp/vcpkg_triplet /tmp/vcpkg_triplet
COPY --from=deps /app/server/vcpkg_installed /app/server/vcpkg_installed
COPY . .
RUN TRIPLET="$(cat /tmp/vcpkg_triplet)" \
    VCPKG_INSTALLED_DIR=/app/server/vcpkg_installed \
    VCPKG_TRIPLET="$TRIPLET" \
    make -C server all

FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /app/server/build/node_server /app/node_server

EXPOSE 9002
CMD ["/app/node_server"]
