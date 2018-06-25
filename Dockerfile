FROM golang:1.10 as builder
RUN mkdir -p src/github.com/gridsum/crystal-bridge
COPY . src/github.com/gridsum/crystal-bridge/
WORKDIR src/github.com/gridsum/crystal-bridge
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o crystal-bridge .

FROM alpine:3.7
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/* && mkdir -p /devops/swagger && mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
COPY --from=builder /go/src/github.com/gridsum/crystal-bridge/crystal-bridge /usr/bin
RUN chmod +x /usr/bin/crystal-bridge
EXPOSE 36000 
ENTRYPOINT ["/usr/bin/crystal-bridge"]
