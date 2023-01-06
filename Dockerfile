FROM golang:1.19 as go-builder
COPY . /build/
WORKDIR /build
RUN CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64 \
  go build \
  -ldflags='-w -s -extldflags "-static"' \
  -tags netgo \
  -o validator \
  cmd/validator/validator.go

FROM alpine:3.15
COPY --from=go-builder /build/validator /
ENTRYPOINT [ "/validator" ]
