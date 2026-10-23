# Cross-compiles on the build machine's platform, so multi-arch images need no emulation.
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bot /bot
ENTRYPOINT ["/bot"]
