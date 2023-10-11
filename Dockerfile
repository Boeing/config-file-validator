ARG BASE_IMAGE=alpine:3.18
ARG VALIDATOR_VERSION=unknown

FROM golang:1.21 as go-builder
COPY . /build/
WORKDIR /build
RUN CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64 \
  go build \
  -ldflags='-w -s -extldflags "-static" -X github.com/Boeing/config-file-validator.version=$VALIDATOR_VERSION' \
  -tags netgo \
  -o validator \
  cmd/validator/validator.go

FROM $BASE_IMAGE
COPY --from=go-builder /build/validator /
ENTRYPOINT [ "/validator" ]
