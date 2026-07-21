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

ARG APPLE_EMOJI_RELEASE=macos-26-20260613-f1fc560b
ARG APPLE_EMOJI_SHA256=b8c8ed97f642b89ba4a36a3e096619f0805d06cc25ae1116e6953b98142ae20c
RUN apk add --no-cache ca-certificates fontconfig font-noto-cjk libgcc libunwind \
    && apk add --no-cache --virtual .font-download curl \
    && curl -fL --retry 3 --retry-all-errors --connect-timeout 30 \
        -o /usr/share/fonts/AppleColorEmoji.ttf \
        "https://github.com/samuelngs/apple-emoji-ttf/releases/download/${APPLE_EMOJI_RELEASE}/AppleColorEmoji-Linux.ttf" \
    && echo "${APPLE_EMOJI_SHA256}  /usr/share/fonts/AppleColorEmoji.ttf" | sha256sum -c - \
    && apk del .font-download \
    && fc-cache -f
COPY --from=go-builder /qq-quote-go /usr/local/bin/qq-quote-go

EXPOSE 5000
ENV PORT=5000
ENTRYPOINT ["qq-quote-go"]
