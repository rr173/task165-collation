# task165-collation 评测构建（与 Dockerfile 同源，供 build_benzhi_docker.sh 引用）
FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm AS build

WORKDIR /src
ENV GOTOOLCHAIN=local
ENV CGO_ENABLED=0
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build ./... && go build -o /app/collation ./cmd/collation

FROM docker.m.daocloud.io/library/alpine:3.20
COPY --from=build /app/collation /app/collation
WORKDIR /data
ENTRYPOINT ["/app/collation"]
CMD ["--smoke-test"]
