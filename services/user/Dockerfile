FROM golang:1.27.3-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY services/user/go.mod services/user/go.sum ./
RUN go mod download

COPY services/user/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/admin ./cmd/admin \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/keygen ./cmd/keygen

FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 app

WORKDIR /app

COPY --from=build /out/ /app/

USER 10001

EXPOSE 3001

ENTRYPOINT ["/app/api"]
