# task165-collation · 古籍异文校勘与定本工作台

古籍整理研究者把同一作品的底本与多个见证本逐段对齐，记录文字、缺佚和段落移动造成的异文，在同行复核后形成**可追溯的定本文本**。系统以段落标识、首尾词和人工确认锚点构建对齐关系，把相邻锚点之间的差异聚合为异文位；编辑在每个异文位提出、复核或驳回读法决定；通过复核的决定被编入候选定本；提交发布时冻结当时的底本、见证本范围、锚点和决定链。

## 业务闭环

创建校勘工程并设定底本 → 导入见证本章节/段落 → 确认锚点 → 区间对齐生成异文位 → 认领异文位并提议读法 → 决定与复核 → 候选定本预览 → 发布冻结 → 追加见证本进入新校勘轮次。

## 核心实体与状态

| 实体 | 状态 |
| --- | --- |
| 校勘工程 | draft → aligning → reviewable → published → archived |
| 文本见证本 | pending_segmentation → pending_alignment → aligned → needs_manual_anchor → frozen |
| 锚点 | candidate → confirmed / conflict / deprecated |
| 异文位 | unhandled → claimed → pending_review → decided → superseded |
| 校勘决定 | proposed → approved / rejected / withdrawn |
| 定本快照 | building → pending_publish → published → superseded |

## 标准命令

```bash
export GO_BIN=$(command -v go)          # go1.26.3, GOTOOLCHAIN=local
CGO_ENABLED=0 GOTOOLCHAIN=local $GO_BIN build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local $GO_BIN vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local $GO_BIN test  ./...
$GO_BIN run ./cmd/collation --smoke-test       # 离线端到端自检（含 DB 关闭重开恢复验证）

# 长驻服务
$GO_BIN run ./cmd/collation --addr=:8080 --db=task165-collation.db
# 浏览器打开 http://localhost:8080 或调用 /api/... 接口
```

## 浏览器校勘工作台

服务根路径 `/` 提供实际可访问的并排校勘工作台：左侧并排显示底本段落与见证本段落，下方展示异文证据卡片（差异类型、底本字符范围、认领人）和校勘决定与定本预览。页面通过同一服务的 `/api` 接口创建/切换工程、加载文本与异文并预览候选定本，不使用模拟数据或独立前后端；静态资源由 Go 二进制通过 `go:embed` 内嵌，单产物部署。

## API 入口（统一 /api 前缀，>30 个）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | /api/stats | 全局统计 |
| POST | /api/projects | 创建校勘工程 |
| GET | /api/projects | 工程列表 |
| GET | /api/projects/{id} | 工程详情 |
| POST | /api/projects/{id}/transition | 工程状态流转（乐观锁） |
| GET | /api/projects/{id}/stats | 工程统计 |
| POST | /api/projects/{id}/witnesses | 添加见证本（is_base 设底本） |
| GET | /api/projects/{id}/witnesses | 见证本列表 |
| GET | /api/witnesses/{id} | 见证本详情 |
| POST | /api/witnesses/{id}/chapters | 导入章节 |
| GET | /api/witnesses/{id}/chapters | 章节列表 |
| POST | /api/witnesses/{id}/passages | 分段导入段落（幂等重试） |
| GET | /api/witnesses/{id}/passages | 段落列表 |
| POST | /api/projects/{id}/anchors | 提议锚点 |
| POST | /api/anchors/{id}/confirm | 确认锚点（乐观锁） |
| POST | /api/anchors/{id}/deprecate | 废弃锚点 |
| GET | /api/projects/{id}/anchors | 锚点列表 |
| POST | /api/projects/{id}/align | 区间对齐生成异文位 |
| POST | /api/projects/{id}/recover | 重启后恢复对齐任务 |
| GET | /api/projects/{id}/variants | 异文位列表（?status=） |
| GET | /api/variants/{id} | 异文位详情 + 读法 |
| POST | /api/variants/{id}/claim | 认领异文位（短租约） |
| POST | /api/variants/{id}/release | 释放认领 |
| POST | /api/variants/{id}/readings | 提议读法 |
| POST | /api/variants/{id}/decide | 提议决定（版本冲突 409） |
| GET | /api/variants/{id}/trace | 异文位追溯链 |
| POST | /api/decisions/{id}/review | 复核决定（通过/退回） |
| POST | /api/decisions/{id}/withdraw | 撤销决定 |
| GET | /api/projects/{id}/snapshot/preview | 候选定本预览 |
| POST | /api/projects/{id}/snapshots | 构建快照（新轮次） |
| GET | /api/projects/{id}/snapshots | 快照列表 |
| GET | /api/snapshots/{id} | 快照详情（冻结视图） |
| POST | /api/snapshots/{id}/publish | 发布冻结定本 |
| POST | /api/projects/{id}/rollback | 回溯最早发布版 |

## 并发与一致性

- **异文位认领**：带版本号的短租约；未过期租约不可抢占，过期可重新认领；同一编辑续租不冲突。
- **决定提交**：base_version 乐观锁，后到决定返回 409 + `ConflictDetail`（含最新决定摘要），绝不静默覆盖。
- **锚点占用**：同一底本段落不能被两个未确认锚点占用。
- **重启恢复**：从已持久化确认锚点恢复未完成对齐，只重算受新增/撤销锚点影响的相邻区间；过期认领可重新认领。
- **快照冻结**：发布后正文、锚点集合、决定链只读；追加见证本只能新建校勘轮次，不改变历史发布版。

## 目录结构

```
cmd/collation/           入口（--addr --db --smoke-test）
internal/model/          实体、状态机、错误
internal/store/          SQLite 持久化（建表迁移 + CRUD + 租约 + 恢复）
internal/text/           文本哈希、段落切分、字符 diff
internal/align/          锚点候选、区间对齐、移动检测、受影响区间重算
internal/collate/        异文位、读法、决定状态、可追溯性校验、定本编译
internal/definitive/     快照冻结、发布、回溯视图
internal/service/        编排层（业务规则）
internal/httpapi/        JSON API（/api 前缀）
internal/demo/           示例工程 + 离线自检
internal/config/         运行时配置
```

## 持久化

SQLite（纯 Go 驱动 `modernc.org/sqlite`，CGO 无关、离线可构建）保存工程、见证本、章节、段落、锚点、异文位、读法、决定、快照及快照链接。重启后从确认锚点恢复未完成对齐；已发布快照保留原始段落哈希、锚点集合和决定链，即使源见证本随后补录也不变。
