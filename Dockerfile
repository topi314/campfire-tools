FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build

WORKDIR /build

COPY go.mod ./

RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    go build -o campfire-raffle github.com/topi314/campfire-raffle

FROM alpine

COPY --from=build /build/campfire-raffle /bin/campfire-raffle

ENTRYPOINT ["/bin/campfire-raffle"]
