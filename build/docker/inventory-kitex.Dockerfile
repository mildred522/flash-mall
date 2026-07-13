# syntax=docker/dockerfile:1

FROM golang:1.24 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,id=flash-mall-go-mod,target=/go/pkg/mod \
    go mod download

COPY app/common ./app/common
COPY app/inventory ./app/inventory
RUN --mount=type=cache,id=flash-mall-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=flash-mall-go-build,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -tags timetzdata -o /out/inventory-kitex ./app/inventory/kitex

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/inventory-kitex /app/inventory-kitex

EXPOSE 8093
ENTRYPOINT ["/app/inventory-kitex"]
