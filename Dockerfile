# task165-collation 古籍异文校勘与定本工作台
# 多阶段构建：编译阶段使用 Bookworm Go 镜像（固定工具链），运行阶段使用
# Alpine 精简运行时。CGO 关闭，产出静态二进制；ENTRYPOINT 指向服务入口，
# 默认 CMD 运行离线自检 --smoke-test。
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
