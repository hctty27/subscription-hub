# Subscription Hub

> 面向个人与开发者的开源订阅及资源到期管理平台（开发中）。

[English](README.md) | [简体中文](README.zh-CN.md)

## 定位

统一管理 AI 服务、软件会员、VPS、域名和其他周期性支出或到期资源。

## 当前状态

**项目初始化阶段。** 当前只有可运行的 API 健康检查与前端基础页面，不具备订阅记录、存储、提醒等业务功能。相关能力将按开发计划逐步实现。

## 技术栈

- 后端：Go + Gin
- 前端：Vue 3 + TypeScript + Vite
- 数据库（计划）：SQLite，后续可扩展 MySQL
- 部署：Docker Compose（当前仅开发环境）

## 本地启动

需要 Go 1.23+、Node.js 22.18+、npm。

启动后端：

```bash
cd backend
go mod tidy
go run ./cmd/server
```

另开终端启动前端：

```bash
cd frontend
npm install
npm run dev
```

访问 http://localhost:5173 。

也可以在仓库根目录执行：

```bash
docker compose up --build
```

当前 Docker Compose 配置仅供开发测试，不适合直接公开部署。

API 检查：`GET http://localhost:8080/api/v1/health`，返回 `{"status":"ok"}`。

## V1 规划

1. 订阅、资源、分类的基础管理
2. 计费周期和到期日计算
3. 支付流水与支出统计
4. SMTP / Telegram / Bark / Webhook 通知适配器
5. SQLite 数据持久化、迁移与备份
6. 登录认证、权限控制、基础安全防护
7. 生产环境 Docker 部署

详见 [开发路线图](docs/ROADMAP.md) 与 [架构设计](docs/ARCHITECTURE.md)。

## 安全提示

当前为基础工程，不含业务数据持久化与用户身份认证。请勿将它直接暴露到公网，也不要向仓库提交密钥。

## 开源协议

MIT，详见 [LICENSE](LICENSE)。
