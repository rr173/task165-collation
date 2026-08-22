# BENZHI_README.md — task165-collation 评测说明

## 业务

**古籍异文校勘与定本工作台**：研究者创建校勘工程并设定底本，将见证本按章节和段落导入；系统以段落标识、首尾词和人工确认锚点构建对齐关系，把相邻锚点之间的差异聚合为异文位；编辑在每个异文位提出、复核或驳回读法决定；通过复核的决定被编入候选定本，提交发布时冻结当时的底本、见证本范围、锚点和决定链。追加见证本只能生成新的校勘轮次，不能改写已发布定本。

## 标准命令（必须全部成功）

```bash
export GO_BIN=$(command -v go)   # go1.26.3, GOTOOLCHAIN=local
CGO_ENABLED=0 GOTOOLCHAIN=local $GO_BIN build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local $GO_BIN vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local $GO_BIN test  ./...
$GO_BIN run ./cmd/collation --smoke-test
```

`--smoke-test` 契约：不启动长驻服务；真实执行「建工程 → 建底本/见证本 → 导入分段 → 锚点确认 → 认领异文位 → 提议读法 → 决定 + 复核 → 构建/发布快照 → 版本冲突检查 → **关闭并重新打开数据库**验证持久化与重启恢复」，成功以 0 退出并打印 `smoke test passed` / `collation smoke test OK`。

## 双架构 Docker 验证

```bash
bash build_benzhi_docker.sh task165-collation linux/amd64
docker run --rm task165-collation /app/collation --smoke-test   # 必须打印 smoke test passed

bash build_benzhi_docker.sh task165-collation linux/arm64
docker run --rm task165-collation /app/collation --smoke-test   # 必须打印 smoke test passed
```

镜像入口 `ENTRYPOINT ["/app/collation"]`，默认 `CMD ["--smoke-test"]`。

## API（JSON，/api 前缀，>30 个）

核心闭环接口：

1. `POST /api/projects` 创建工程 → `{data:{id}}`
2. `POST /api/projects/{id}/witnesses` `{"code":"B","title":"底本","is_base":true}` 设底本
3. `POST /api/witnesses/{id}/passages` `{"text":"..."}` 导入分段
4. `POST /api/projects/{id}/anchors` 提议锚点 → `confirm` 确认
5. `POST /api/projects/{id}/align` 区间对齐生成异文位
6. `POST /api/variants/{id}/claim` 认领（`expect_lease` 版本号）
7. `POST /api/variants/{id}/readings` 提议读法
8. `POST /api/variants/{id}/decide` 提议决定（`expected_version` 乐观锁）
9. `POST /api/decisions/{id}/review` `{"approve":true}` 复核通过
10. `GET /api/projects/{id}/snapshot/preview` 候选定本预览
11. `POST /api/projects/{id}/snapshots` 构建快照；`POST /api/snapshots/{id}/publish` 发布冻结
12. `GET /api/variants/{id}/trace` 追溯链（读法 → 决定 → 证据见证本）

完整列表见 `README.md`。

## 并发与错误边界

- 两编辑同时处理同一异文位：后提交返回 409 + `ConflictDetail`（含最新决定摘要）。
- 同一段落不能同时被两个未确认锚点占用（`ErrAnchorOccupied` → 409）。
- 拒绝：不属于工程的见证本（422）、无效字符范围（422）、跨章节锚点（422）、循环移动关系（422）、引用已撤销读法的决定（422）、对已发布定本的直接修改（422）、同轮已有发布快照后再构建（422）。
- 见证本导入幂等重试；文本哈希相同但书目信息冲突时保留冲突（409）而非合并。
- 重启恢复：确认锚点驱动的增量重算；过期认领可重新认领，未过期认领不可抢占。

## 组件版本

- Go 1.26.3（`GOTOOLCHAIN=local`，CGO_ENABLED=0）
- SQLite 3.46.1（纯 Go 驱动 `modernc.org/sqlite` v1.52.0）
- 版本锁见 `component-versions.json`。
