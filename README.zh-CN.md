# Subscription Hub

> 面向个人与开发者的开源订阅及资源生命周期管理平台（V1 开发中）。

[English](README.md) | [简体中文](README.zh-CN.md)

## 已实现（V1 第一阶段）

- SQLite 持久化与数据库版本初始化
- 分类管理、订阅及资源的增删改查
- 按天、按月、按年及自定义间隔续费
- 月末日期和闰年日期处理（以首次到期日为锚点）
- 正常、暂停、取消三种状态
- Vue 3 响应式中文管理界面
- Docker 数据卷持久化
- 实付流水录入、修改、删除、按币种分组的月度统计与趋势
- 提前 0–30 天的站内到期提醒，规则幂等去重

**暂未实现：**登录认证、外部通知推送、退款、汇率换算、备份恢复、AI/MCP。当前仅适合可信的本地开发环境，**不要公开部署到公网**。

## 技术栈

- Go 1.26 + Gin + modernc.org/sqlite（纯 Go，无 CGO）
- Vue 3 + TypeScript + Vite
- SQLite（WAL 日志模式）
- Docker Compose（本地开发）

## 快速启动

推荐 Docker：

```bash
docker compose up --build
```

访问 http://localhost:5173。

数据存储在 Docker 命名卷 `subscription_data`。`docker compose down` 不删除数据；执行 `docker compose down -v` **会清空数据卷**。

也可本地运行（需要 Go 1.26+、Node.js 22.18+、npm）：

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

后端默认监听本机 `127.0.0.1:8080`，数据文件默认 `./data/subscription-hub.db`；`APP_TIMEZONE` 默认 `Asia/Shanghai`。Compose 在容器内监听所有网络地址，但向宿主机仅开放 `127.0.0.1`。

## 使用流程

1. 进入分类管理，可先添加 AI、服务器等分类。
2. 进入订阅管理，添加服务名称、金额、周期、首次到期日期。
3. 资源管理中可记录 VPS、域名和一次性到期资源。
4. 付款流水录入已经实际支付的账单，费用统计按币种分别汇总。
5. 在提醒中心配置规则，可查看自动生成的站内提醒；当前不会发送邮件或推送。

货币金额以最小单位存储（例如 20.00 USD = 2000），暂不跨币种汇总。暂停、取消状态不参与下次到期展示。一次性到期资源过期后仍保留原到期日。

## 开发文档

- [API 接口](docs/API.md)
- [开发路线图](docs/ROADMAP.md)
- [架构设计](docs/ARCHITECTURE.md)

## 安全提示

**项目尚无登录鉴权，请勿开放至公网。**升级和卸载前需自行备份数据库，勿将密钥写入备注或提交仓库。

## 许可证

MIT，详见 [LICENSE](LICENSE)。
