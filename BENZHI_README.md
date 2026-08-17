# task037-textdiff

这是一个纯 Go 的文本差异对比 HTTP 服务。客户端提交旧文本和新文本后，服务按行计算最小编辑脚本，并返回带上下文的 Unified Diff 文本、结构化 hunk 列表和新增/删除统计。服务只依赖 Go 标准库，不需要数据库或外部服务。

## 标准命令

以下命令均在 `env/` 目录执行：

```bash
go build ./...
go test ./...
go vet ./...
go run . --smoke-test
go run . --addr :8080
```

`--smoke-test` 会在进程内启动 HTTP 服务并完成自检后退出；普通模式监听 `:8080`，可通过 `--addr` 修改地址。

## Benzhi 容器

`build_benzhi_docker.sh` 使用固定的 `benzhi.Dockerfile` 构建评测镜像，参数依次是镜像名和平台，默认值为 `my-project` 与 `linux/amd64`：

```bash
bash build_benzhi_docker.sh textdiff-benzhi linux/amd64
docker run --rm -it textdiff-benzhi:latest
```

容器启动后进入 shell；构建阶段执行 `go build ./...`，不依赖外部业务服务。
