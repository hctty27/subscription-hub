# Subscription Hub

> 面向个人与开发者的订阅及资源生命周期管理平台（开发版）。

[English](README.md) | [简体中文](README.zh-CN.md)

## 当前实现

- 订阅、资源、分类管理（SQLite）
- 按日期锚点计算下次续费日期
- 付款流水、按币种统计费用、六个月趋势
- 到期站内提醒、规则和幂等去重
- 单管理员密码登录与 HttpOnly 会话（Argon2id）
- 每天 03:00 自动创建 SQLite 一致性备份，默认保留最近 7 份；支持登录后手动备份
- SMTP、Telegram Bot、Bark 主动通知适配器；SQLite 持久化投递、最多 4 次尝试及发送日志

## 首次启动（开发环境）

首次运行必须配置 12–128 字符的管理员密码。推荐在本机仓库目录新建不提交版本控制的 `.env`：

```dotenv
ADMIN_PASSWORD=请替换成至少12字符的随机密码
SESSION_SECURE=false
```

然后执行（旧版升级也应先迁移命名卷）：

```bash
sh scripts/migrate-data-volumes.sh development
docker compose up -d --build
```

打开 http://127.0.0.1:5173 ，输入管理员密码登录。

首次启动后，密码以带独立盐值的 Argon2id 哈希形式保存在数据库里。后续修改 `ADMIN_PASSWORD` **不会自动重置密码**；不要把密码写入 GitHub、Docker 命令历史或日志。当前尚未实现密码重置界面；丢失密码需要离线维护数据库并执行受控重置。

### 本地 Go + Vue 开发

需要 Go 1.26+ 和 Node.js 22.18+，先设置 `ADMIN_PASSWORD` 环境变量再启动：

```bash
cd backend
go mod tidy
go run ./cmd/server
```

另开终端：

```bash
cd frontend
npm install
npm run dev
```

默认后端只监听 `127.0.0.1:8080`。SQLite 默认位置是 `backend/data/subscription-hub.db`。

## 备份

- `BACKUP_DIR` 默认 `./data/backups`，Docker Compose 中为 `/data/backups`。
- 每天服务器 `APP_TIMEZONE` 的 03:00 自动备份；点击概览中的“立即备份”也可以手动执行。
- 使用 SQLite `VACUUM INTO` 生成一致性快照并通过 `integrity_check` 校验；自动轮换只保留最近 7 份。
- 重装或恢复前，停止应用，保留当前数据库并将备份数据库复制到 `DB_PATH`。恢复操作尚未提供网页界面。
- Docker 开发版数据保存在项目根目录 `data-dev/`，生产版保存在 `data/`；两者隔离。生产数据库是 `data/subscription-hub.db`，备份在 `data/backups/`。
- 老版本使用的 Docker 命名卷需要在升级后、启动新 Compose 前运行 `sh scripts/migrate-data-volumes.sh production`（开发版用 `development`）安全迁移。脚本会保留原卷，不自动删除数据。
- 数据目录不等于异地备份：仍需单独配置远端 Restic 备份。

## 安全说明

本项目提供**两套独立的 Compose**：`compose.yaml` 仅用于开发，`compose.production.yaml` 使用静态前端 + Nginx，配合 Cloudflare Tunnel 提供公网 HTTPS，并在 Docker 内网访问 Go API。

生产配置支持 Cloudflare Tunnel HTTPS、强制 Secure Cookie、只读容器根目录、数据库快照与 Restic 异地加密备份脚本。**配置具备生产部署基础，但正式面向公网使用前仍需完成真实域名/Tunnel/HTTPS 验证、密码恢复流程、异地恢复演练和安全检查。**

详细步骤：[正式部署与异地备份](docs/DEPLOYMENT.md)。

主动推送默认关闭；请在服务端环境变量中配置渠道，详见 [通知部署说明](docs/NOTIFICATIONS.md)。当前不会在网页中保存推送密钥。

生产部署见 [DEPLOYMENT.md](docs/DEPLOYMENT.md)，接口文档见 `docs/API.md` 和 `docs/FINANCE.md`，开源协议 MIT。
