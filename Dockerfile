# syntax=docker/dockerfile:1

FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm AS builder
WORKDIR /src
COPY . .
ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local \
    GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.org
RUN go build -mod=vendor -o /out/task037-textdiff .

FROM docker.m.daocloud.io/library/alpine:3.20
COPY --from=builder /out/task037-textdiff /usr/local/bin/task037-textdiff
ENTRYPOINT ["task037-textdiff"]
