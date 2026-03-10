FROM golang:1.24 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/product-rpc ./app/product/rpc

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/product-rpc /app/product-rpc

EXPOSE 8080 6062 9092
ENTRYPOINT ["/app/product-rpc"]
