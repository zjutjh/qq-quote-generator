FROM rust:1.87-alpine3.21 AS resvg-builder

RUN apk add --no-cache git musl-dev
WORKDIR /src
COPY scripts/build-resvg.sh scripts/build-resvg.sh
RUN sh scripts/build-resvg.sh

FROM golang:1.25-alpine AS go-builder

ARG GOPROXY=https://goproxy.cn,direct
RUN apk add --no-cache gcc musl-dev libunwind-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN GOPROXY="$GOPROXY" go mod download
COPY . .
COPY --from=resvg-builder /src/internal/resvg/lib/linux-amd64/libresvg.a internal/resvg/lib/linux-amd64/libresvg.a
COPY --from=resvg-builder /src/internal/resvg/lib/linux-amd64/native-static-libs.txt internal/resvg/lib/linux-amd64/native-static-libs.txt
COPY --from=resvg-builder /src/internal/resvg/resvg.h internal/resvg/resvg.h
RUN CGO_LDFLAGS="$(cat internal/resvg/lib/linux-amd64/native-static-libs.txt)" \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /qq-quote-go .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates fontconfig font-noto-cjk libgcc libunwind
COPY --from=go-builder /qq-quote-go /usr/local/bin/qq-quote-go

EXPOSE 5000
ENV PORT=5000
ENTRYPOINT ["qq-quote-go"]
