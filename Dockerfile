FROM golang:1.24@sha256:d9db32125db0c3a680cfb7a1afcaefb89c898a075ec148fdc2f0f646cc2ed509 AS go-builder
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

FROM alpine:3.21@sha256:a8560b36e8b8210634f77d9f7f9efd7ffa463e380b75e2e74aff4511df3ef88c
USER user
COPY --from=go-builder /build/validator /
HEALTHCHECK NONE
ENTRYPOINT [ "/validator" ]
