# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25.5

FROM golang:${GO_VERSION}-bookworm AS build

WORKDIR /src

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ARG TARGETOS
ARG TARGETARCH

ENV CGO_ENABLED=0

COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /out/opencompat .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/opencompat /app/opencompat

ENV OPENCOMPAT_HOST=0.0.0.0

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/opencompat"]
CMD ["serve"]
