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

## 首次启动

首次运行必须配置 12–128 字符的管理员密码。推荐在本机仓库目录新建不提交版本控制的 `.env`：

```dotenv
ADMIN_PASSWORD=请替换成至少12字符的随机密码
SESSION_SECURE=false
```

然后执行：

```bash
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
- Docker 数据卷 `subscription_data` 同时包含数据库和备份。**删除卷会同时丢失两者**，因此生产使用必须另存一份异地备份。

## 安全说明

本版本新增单管理员登录、服务端会话和 CSRF 请求头检查，但**仍不属于公开部署就绪版本**。Docker 提供的是 Vite 开发服务器，不应当暴露公网。

正式 HTTPS 反向代理部署时必须设置 `SESSION_SECURE=true` 以启用 Secure Cookie，同时配置 TLS、访问限制、静态前端构建、密码恢复机制和异地备份。不要依赖环境变量自动更改已存在的管理员密码。

外部邮件、Telegram、Bark 主动推送、失败重试和发送日志仍在后续阶段。

接口文档见 `docs/API.md` 和 `docs/FINANCE.md`，开源协议 MIT。
