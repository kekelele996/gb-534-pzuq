# 发酵动力学偏差分析

```bash
cp .env.example .env
docker compose up -d
```

启动完成后访问 `http://127.0.0.1:18534`。这是面向生物制造工艺团队的离线分析工作台，用于管理发酵罐、培养配方与传感器时序，并通过阶段约束 DTW 解释动力学偏差。

> 安全边界：本系统只提供离线分析和决策支持，不能替代合格工艺人员判断，不连接发酵罐、泵、阀门或加料系统，也不生成或下发设备控制指令。

## 主要功能

- 发酵罐：维护容积、位置、责任团队和传感器通道，查看最近数据质量与偏差摘要。
- 培养配方：管理四阶段边界、参考曲线、通道容差和版本生命周期，支持复制版本。
- 传感器时序：导入多通道 JSON 数据，执行排序去重、缺失率检查、稳健缩放和状态迁移。
- 偏差分析：冻结配方与时序输入，执行阶段约束 DTW，展示阶段证据、对齐曲线和疑似原因。
- 复核与审计：强制发起人与确认人分离，记录 request ID、前后快照、输入哈希、算法版本和耗时。
- 平台保护：JWT、RBAC、统一错误响应、登录/导入/分析限流、幂等键和并发条件更新。

## 演示账号

| 用户名 | 密码 | 角色 | 主要权限 |
| --- | --- | --- | --- |
| `admin` | `admin123` | 管理员 | 全部功能 |
| `scientist` | `scientist123` | 工艺科学家 | 罐体、配方、数据处理与分析复核 |
| `analyst` | `analyst123` | 数据分析师 | 时序导入/处理与分析运行 |
| `reviewer` | `reviewer123` | 独立复核人 | 分析复核、确认与审计读取 |
| `auditor` | `auditor123` | 审计员 | 只读业务数据与审计记录 |

演示密码只适用于本地环境，部署到其他环境前必须替换账号和 `JWT_SECRET`。

## 页面

| 路由 | 页面 | 主要数据 |
| --- | --- | --- |
| `/vessels` | 罐体总览 | `FermentationVessel`、`SensorSeries` |
| `/recipes` | 配方版本 | `CultureRecipe`、`FermentationVessel` |
| `/series` | 时序工作台 | `SensorSeries`、`FermentationVessel`、`CultureRecipe` |
| `/analyses` | 偏差分析 | `DeviationAnalysis`、`SensorSeries`、`CultureRecipe` |
| `/audit` | 审计中心 | 四个实体的变更投影与审计日志 |

## 技术栈

| 层 | 技术 |
| --- | --- |
| 前端 | Vue 3、TypeScript、Vite、Element Plus、Pinia、ECharts |
| 后端 | Go 1.22、Gin、GORM、validator/v10、JWT |
| 正式数据库 | PostgreSQL 16 |
| 自包含验证 | GORM SQLite 内存数据库，仅供单元测试和 runtime smoke |
| 部署 | Docker Compose、Nginx、命名卷、三服务 healthcheck |

## 架构

请求从 Vue 页面进入按实体拆分的 API 和 Pinia store，经 Nginx 原样代理到 `/api/v1`。后端保持 `router -> handler -> service -> repository -> model` 单向依赖；状态迁移、幂等和业务规则位于 service/repository，handler 不直接访问数据库。

```text
.
├── go.work
├── runtime_smoke.json
├── docker-compose.yml
├── .env.example
├── README.md
├── database/
│   └── init.sql
├── backend/
│   ├── cmd/server/main.go
│   ├── go.mod
│   └── internal/
│       ├── algorithm/        # 阶段约束 DTW 与解释
│       ├── timeseries/       # 校验与稳健缩放
│       ├── config/ constants/ dto/ model/
│       ├── repository/ service/ handler/ router/
│       ├── middleware/       # request ID、恢复、认证、RBAC、审计、错误处理
│       └── util/
└── frontend/
    ├── nginx.conf
    └── src/
        ├── api/ stores/ types/
        ├── components/common/
        ├── hooks/ pages/ router/ utils/
        └── styles.css
```

## API

除健康检查外，接口前缀为 `/api/v1`。受保护接口使用 `Authorization: Bearer <token>`。

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/healthz` | 服务与数据库健康检查 |
| `POST` | `/api/v1/auth/login` | 登录并取得 JWT |
| `GET/POST` | `/api/v1/fermentation-vessels` | 罐体列表/创建 |
| `GET/PUT` | `/api/v1/fermentation-vessels/:id` | 罐体详情/更新 |
| `POST` | `/api/v1/fermentation-vessels/:id/deactivate` | 停用罐体 |
| `GET/POST` | `/api/v1/culture-recipes` | 配方列表/创建 |
| `GET/PUT` | `/api/v1/culture-recipes/:id` | 配方详情/更新 |
| `POST` | `/api/v1/culture-recipes/:id/transition` | 配方状态迁移 |
| `POST` | `/api/v1/culture-recipes/:id/copy` | 复制配方版本 |
| `GET/POST` | `/api/v1/sensor-series` | 时序列表/导入 |
| `GET` | `/api/v1/sensor-series/:id` | 时序详情 |
| `POST` | `/api/v1/sensor-series/:id/transition` | 校验、标准化、就绪或作废 |
| `GET/POST` | `/api/v1/deviation-analyses` | 分析列表/幂等运行 |
| `GET` | `/api/v1/deviation-analyses/:id` | 分析详情与冻结证据 |
| `POST` | `/api/v1/deviation-analyses/:id/transition` | 复核、确认、调查或作废 |
| `POST` | `/api/v1/deviation-analyses/:id/replay` | 冻结输入确定性重放 |
| `GET` | `/api/v1/audit-logs` | 审计筛选 |
| `GET` | `/api/v1/meta/enums` | 共享枚举元数据 |

分析运行必须携带非空且不超过 128 字符的 `Idempotency-Key`。同一键或同一输入哈希与算法版本不会生成重复历史结果。

## 状态与算法

时序状态流：

```text
imported -> validated -> normalized -> ready -> superseded
    |           |             |
    +-----------+-------------+-> rejected
```

分析状态流：

```text
queued -> analyzing -> completed -> reviewed -> confirmed
            |              |          +-----> investigating -> reviewed
            +-> failed     +-----------------> voided
```

算法按时间戳排序并去重，保留缺失率与长间隔证据；使用中位数和四分位距进行稳健缩放，再在 `lag/growth/production/harvest` 阶段边界内做确定性 DTW。结果包含持续时间、斜率、峰值时刻、曲线距离、多通道加权偏差、对齐点与原因规则命中。冻结输入和算法版本可重放，历史结果不会被覆盖。

## 共享枚举位置

`FermentationPhase = lag | growth | production | harvest`：

- 后端定义与元数据：`backend/internal/constants/fermentation_phase.go`、`backend/cmd/server/main.go`
- DTO/算法/服务输入：`backend/internal/dto/culture_recipe.go`、`backend/internal/algorithm/evaluator.go`、`backend/internal/service/culture_recipe_service.go`
- 模型与种子配置：`backend/internal/model/culture_recipe.go`、`backend/internal/database/database.go`
- 测试：`backend/internal/algorithm/evaluator_test.go`、`backend/internal/service/culture_recipe_service_test.go`
- 前端类型与状态：`frontend/src/types/enums/fermentation-phase.ts`、`frontend/src/types/culture-recipe.ts`、`frontend/src/stores/culture-recipe.ts`
- 组件与页面：`frontend/src/components/common/PhaseBadge.vue`、`frontend/src/pages/RecipesPage.vue`、`frontend/src/pages/SeriesPage.vue`、`frontend/src/pages/AnalysesPage.vue`

`DeviationLevel = normal | watch | major | critical`：

- 后端定义与元数据：`backend/internal/constants/deviation_level.go`、`backend/cmd/server/main.go`
- 模型/DTO/算法/服务：`backend/internal/model/deviation_analysis.go`、`backend/internal/dto/deviation_analysis.go`、`backend/internal/algorithm/evaluator.go`、`backend/internal/service/deviation_analysis_service.go`
- 测试：`backend/internal/algorithm/evaluator_test.go`、`backend/internal/constants/state_machine_test.go`、`backend/internal/service/deviation_analysis_service_test.go`
- 前端类型与状态：`frontend/src/types/enums/deviation-level.ts`、`frontend/src/types/deviation-analysis.ts`、`frontend/src/stores/deviation-analysis.ts`
- 组件与页面：`frontend/src/components/common/DeviationBadge.vue`、`frontend/src/pages/VesselsPage.vue`、`frontend/src/pages/AnalysesPage.vue`

## 环境变量与端口

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `COMPOSE_PROJECT_NAME` | `fermentation-kinetics-deviation-analysis` | 固定英文 Compose 项目名 |
| `FRONTEND_PORT` | `18534` | Web 宿主机端口 |
| `BACKEND_PORT` | `19534` | API 宿主机端口 |
| `DB_PORT` | `57534` | PostgreSQL 宿主机端口 |
| `POSTGRES_DB` | `fermentation_analysis` | 数据库名 |
| `POSTGRES_USER` | `fermentation` | 数据库用户 |
| `POSTGRES_PASSWORD` | `fermentation_dev` | 数据库密码 |
| `DB_DRIVER` | `postgres` | 正式部署固定为 PostgreSQL |
| `DB_DSN` | 见 `.env.example` | GORM 连接字符串 |
| `JWT_SECRET` | 见 `.env.example` | JWT 签名密钥，生产环境必须替换 |
| `JWT_EXPIRY` | `8h` | 登录有效期 |
| `LOGIN_LIMIT_PER_MINUTE` | `30` | 登录限流 |
| `IMPORT_LIMIT_PER_MINUTE` | `30` | 时序导入限流 |
| `ANALYSIS_LIMIT_PER_MINUTE` | `60` | 分析/重放限流 |
| `SHUTDOWN_TIMEOUT` | `10s` | 优雅停机超时 |

Compose 内部端口固定为前端 `80`、后端 `8080`、PostgreSQL `5432`。浏览器代码只请求相对路径 `/api`，不硬编码 localhost API 地址。

## Docker 部署

```bash
cp .env.example .env
docker compose config --quiet
docker compose up -d --build
docker compose ps
```

数据库使用命名卷 `fermentation-kinetics-deviation-analysis-postgres-data`。后端等待数据库健康，前端等待后端健康；Nginx 原样代理 `/api/v1` 并用 `try_files` 支持 SPA 路由。中文或其他任意目录名不会影响 Compose 项目名。

查看日志：

```bash
docker compose logs -f backend frontend
```

## 本地开发

先单独启动数据库：

```bash
cp .env.example .env
docker compose up -d db
```

启动后端：

```bash
cd backend
PORT=19534 \
DB_DRIVER=postgres \
DB_DSN='host=127.0.0.1 user=fermentation password=fermentation_dev dbname=fermentation_analysis port=57534 sslmode=disable TimeZone=Asia/Shanghai' \
JWT_SECRET='local-development-secret-change-me' \
go run ./cmd/server
```

另开终端启动前端，Vite 会把 `/api` 代理到 `19534`：

```bash
npm --prefix frontend ci
npm --prefix frontend run dev
```

访问 `http://127.0.0.1:18534`。

## 构建与测试

```bash
go work sync
go build ./backend/...
go vet ./backend/...
go test ./backend/...

cd backend
go build ./...
go vet ./...
go test ./...
go test -race ./...
cd ..

npm --prefix frontend ci
npm --prefix frontend run typecheck
npm --prefix frontend test
npm --prefix frontend run build
```

自包含启动检查使用 `runtime_smoke.json` 中的 SQLite 内存数据库和 `20534` 端口，不替代正式 PostgreSQL 部署：

```bash

```

## 常见问题

- `project name must not be empty`：确认根目录 `.env` 存在，且 `COMPOSE_PROJECT_NAME` 未被删除；Compose 文件也有固定顶层 `name`。
- 后端连接数据库失败：先执行 `docker compose ps`，确认 `db` 为 `healthy`，并检查 `DB_DSN` 中容器主机名是否为 `db`。
- 页面 API 返回 502：检查 backend healthcheck 和 `docker compose logs backend`；不要把前端 API 地址改成容器内不可达的 localhost。
- 分析返回 409：确认时序已按顺序到达 `ready`，并检查状态是否被其他请求并发修改。
- 分析返回 422：检查缺失率、长间隔、阶段边界、参考曲线覆盖和容差配置；系统不会静默填补大段缺失数据。
- 分析返回 403：按演示角色权限执行操作；分析发起人不能确认自己的结果。

## 停止与清理

保留数据库卷：

```bash
docker compose down --remove-orphans
```

同时删除本项目数据库卷：

```bash
docker compose down -v --remove-orphans
```

## License

MIT License。仅用于软件开发、离线仿真与评审，不构成生物制造工艺或设备控制建议。
