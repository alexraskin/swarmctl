FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    go build -o swarmctl github.com/alexraskin/swarmctl

FROM alpine

RUN apk --no-cache add ca-certificates

COPY --from=build /build/swarmctl /bin/swarmctl

HEALTHCHECK --timeout=10s --start-period=60s --interval=60s \
  CMD wget --spider -q http://localhost:9000/ping

EXPOSE 9000

ENTRYPOINT ["/bin/swarmctl"]

CMD ["-port", "9000"]