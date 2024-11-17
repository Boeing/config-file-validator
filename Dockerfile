ARG BASE_IMAGE=alpine:3.21@sha256:21dc6063fd678b478f57c0e13f47560d0ea4eeba26dfc947b2a4f81f686b9f45

FROM golang:1.23@sha256:70031844b8c225351d0bb63e2c383f80db85d92ba894e3da7e13bcf80efa9a37 AS go-builder
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

FROM $BASE_IMAGE as base
USER user
COPY --from=go-builder /build/validator /
ENTRYPOINT [ "/validator" ]
