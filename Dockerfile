FROM golang:1.25@sha256:ce63a16e0f7063787ebb4eb28e72d477b00b4726f79874b3205a965ffd797ab2 AS go-builder
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

FROM alpine:3.23@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659
USER user
COPY --from=go-builder /build/validator /
HEALTHCHECK NONE
ENTRYPOINT [ "/validator" ]
