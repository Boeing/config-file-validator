FROM golang:1.25@sha256:d7098379b7da665ab25b99795465ec320b1ca9d4addb9f77409c4827dc904211 AS go-builder
ARG VALIDATOR_VERSION=unknown
COPY . /build/
WORKDIR /build
RUN CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64 \
  go build \
  -ldflags="-w -s -extldflags '-static' -X github.com/Boeing/config-file-validator.version=$VALIDATOR_VERSION" \
  -tags netgo \
  -o validator \
  cmd/validator/validator.go

FROM alpine:3.22@sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412
USER user
COPY --from=go-builder /build/validator /
HEALTHCHECK NONE
ENTRYPOINT [ "/validator" ]
