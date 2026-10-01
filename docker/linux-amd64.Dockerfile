ARG GO_IMAGE=golang:1.26-bookworm
FROM python:3.12-slim-bookworm AS python-runtime

FROM ${GO_IMAGE} AS toolchain

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates clang git libatomic1 ninja-build python3 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends libclang-rt-dev \
    && rm -rf /var/lib/apt/lists/*

COPY . .

ARG TEST_GOMAXPROCS
ENV GOMAXPROCS=${TEST_GOMAXPROCS}
ENV CGO_ENABLED=1 CC=clang CXX=clang++ GOPROXY=off GOSUMDB=off

FROM toolchain AS source-build
COPY --from=python-runtime /usr/local /usr/local
RUN ldconfig
ARG DEPOT_TOOLS_REVISION
ARG GO_IMAGE
ARG BUILD_JOBS=2
RUN --mount=type=cache,id=v8go-linux-amd64-${GO_IMAGE},target=/native,sharing=locked \
    test -n "$DEPOT_TOOLS_REVISION" \
    && mkdir -p /native/deps \
    && cp go.mod /native/go.mod \
    && cp deps/build.py deps/build_common.py deps/update_cgo.py deps/v8_hash /native/deps/ \
    && cd /native \
    && git init deps/depot_tools \
    && (git -C deps/depot_tools remote get-url origin >/dev/null 2>&1 || \
        git -C deps/depot_tools remote add origin https://chromium.googlesource.com/chromium/tools/depot_tools.git) \
    && git -C deps/depot_tools fetch --depth=1 origin "$DEPOT_TOOLS_REVISION" \
    && git -C deps/depot_tools checkout --detach FETCH_HEAD \
    && git init deps/v8 \
    && (git -C deps/v8 remote get-url origin >/dev/null 2>&1 || \
        git -C deps/v8 remote add origin https://chromium.googlesource.com/v8/v8.git) \
    && git -C deps/v8 fetch --depth=1 origin "$(cat deps/v8_hash)" \
    && git -C deps/v8 checkout --detach FETCH_HEAD \
    && python3 deps/build.py --verbose --clang --jobs "$BUILD_JOBS" --os linux --arch amd64 \
    && rm -rf /src/deps/linux_amd64 /src/deps/include \
    && cp -a deps/linux_amd64 deps/include /src/deps/ \
    && cp cgo_linux_amd64.go /src/

FROM source-build AS validate-source
RUN sh docker/validate-linux-amd64.sh

FROM scratch AS native-artifacts
COPY --from=validate-source /src/deps/linux_amd64 /deps/linux_amd64
COPY --from=validate-source /src/deps/include /deps/include
COPY --from=validate-source /src/cgo_linux_amd64.go /cgo_linux_amd64.go

FROM toolchain AS validate-existing
RUN sh docker/validate-linux-amd64.sh
