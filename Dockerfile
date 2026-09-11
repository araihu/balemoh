FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125 AS build

ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
COPY client/go.mod client/go.sum ./client/
COPY ui/go.mod ui/go.sum ./ui/
RUN go mod download && cd ui && GOWORK=off go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOWORK=off go build -trimpath -ldflags='-s -w' -o /out/balemoh ./cmd/balemoh && \
    cd ui && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOWORK=off go build -trimpath -ldflags='-s -w' -o /out/balemoh-ui ./cmd/balemoh-ui && \
    mkdir /out/data

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/balemoh /balemoh
COPY --from=build /out/balemoh-ui /balemoh-ui
COPY --from=build --chown=65532:65532 /out/data /data
ENV BALEMOH_DATABASE_PATH=/data/balemoh.db \
    BALEMOH_UI_ICON_CACHE_DIR=/data/icons
USER nonroot:nonroot
EXPOSE 8080 8081
ENTRYPOINT ["/balemoh"]
