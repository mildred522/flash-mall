FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY app /app/app
COPY web /app/web
ENTRYPOINT ["/app/app"]
