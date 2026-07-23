# syntax=docker/dockerfile:1

FROM golang:1.24 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,id=flash-mall-go-mod,target=/go/pkg/mod \
    go mod download

COPY app/common ./app/common
COPY app/gateway ./app/gateway
COPY app/product/rpc/product ./app/product/rpc/product
COPY app/product/rpc/productclient ./app/product/rpc/productclient
COPY app/order/rpc/order ./app/order/rpc/order
COPY app/order/rpc/orderclient ./app/order/rpc/orderclient
COPY app/inventory/kitex/kitex_gen ./app/inventory/kitex/kitex_gen
COPY artifacts/web ./artifacts/web
RUN --mount=type=cache,id=flash-mall-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=flash-mall-go-build,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -tags timetzdata -o /out/hertz-gateway ./app/gateway/hertz

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/hertz-gateway /app/hertz-gateway
COPY --from=build /src/artifacts/web /app/web

EXPOSE 8889
ENTRYPOINT ["/app/hertz-gateway"]
