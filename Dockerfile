FROM golang:1.21.5 AS builder

WORKDIR /app

# Disabling cgo as we don't need to interop with C code
# apline doesn't use glibc
ENV CGO_ENABLED=0

# download modules and cache them for future build operations
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

# build app
COPY . /app
RUN go build ./cmd/server

FROM alpine

WORKDIR /app

# Copy binary (only) to new image
COPY --from=builder /app/server /app/bin/server

ENTRYPOINT [ "/app/bin/server" ]
