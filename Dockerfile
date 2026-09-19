# syntax=docker/dockerfile:1

# --- Build stage -------------------------------------------------------
# Compiles a small, statically-linked (CGO_ENABLED=0) shint binary for
# whatever platform buildx is targeting (TARGETOS/TARGETARCH are set
# automatically by `docker buildx build --platform ...`).
FROM golang:1.27.1-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -buildvcs=false -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/shint .

# --- Final stage ---------------------------------------------------------
# distroless/static carries CA certificates (needed for the "web" command's
# HTTPS requests) and nothing else - no shell, no package manager - keeping
# the image minimal and reducing attack surface. Runs as the built-in
# "nonroot" user.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/shint /shint

ENTRYPOINT ["/shint"]
CMD ["--help"]
