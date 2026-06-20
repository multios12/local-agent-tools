FROM golang:1.26.4-alpine3.24
RUN apk  --no-cache --update add curl unzip go git tzdata python3 && \
    cp /usr/share/zoneinfo/Asia/Tokyo /etc/localtime && \
    apk del tzdata
RUN GOBIN=/usr/local/bin go install github.com/go-delve/delve/cmd/dlv@latest
