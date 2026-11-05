# 外部通知配置

> SMTP、Telegram Bot、Bark 是可选通知通道。系统默认只生成站内消息，**不会主动发送外部通知**。

设置服务器环境变量后，启动服务自动启用相应通知通道。所有通道配置都保存在服务器环境变量中，不能通过网页读取密钥。

## SMTP 邮件

在项目根目录的本地 `.env` 中配置（不要提交到 Git）：

```dotenv
NOTIFY_SMTP_HOST=smtp.example.com
NOTIFY_SMTP_PORT=465
NOTIFY_SMTP_USER=your-account@example.com
NOTIFY_SMTP_PASSWORD=your-app-password
NOTIFY_SMTP_FROM=your-account@example.com
NOTIFY_SMTP_TO=receiver@example.com
```

- 端口 465 使用 TLS 直连；其他端口要求服务端支持 STARTTLS，否则拒绝传输。
- 推荐使用邮箱供应商的应用专用密码，不要使用主账户密码。
- 发件人与收件人都应填写合法的邮箱地址；目前每个通道只支持单收件人。

## Telegram Bot

```dotenv
NOTIFY_TELEGRAM_BOT_TOKEN=123456789:YourBotToken
NOTIFY_TELEGRAM_CHAT_ID=123456789
```

使用 Telegram Bot API 的 `sendMessage`。Bot 需要有权限向对应私聊或群组发消息。

## Bark（iOS）

```dotenv
NOTIFY_BARK_DEVICE_KEY=your_bark_device_key
```

通过官方 `https://api.day.app/push` JSON 接口推送，Bark Key 放在请求体中，不放在 URL。当前仅支持官方地址，后续可增加自部署 Bark 服务器。

## 启动

本地 `.env` 中至少应包含首次管理员设置用的 `ADMIN_PASSWORD`（见 [SECURITY.md](SECURITY.md)）。修改通知环境变量后重建/重启服务：

```bash
docker compose up -d --build
```

日志中只显示已启用的渠道类型，不记录 Bot Token、Bark Key 或 SMTP 密码。

## 发送机制

1. 站内提醒规则命中续费日期后生成站内通知。
2. 已配置的外部渠道对新产生的站内提醒建立独立的发送任务。
3. 任务记录状态：`pending` → `sent`；失败变为 `retry`，最多 4 次后成为 `failed`。
4. 失败采用退避重试，1、5、15 分钟后再次尝试；每分钟扫描一次待处理任务。
5. 同一站内通知、同一渠道的投递任务唯一，避免因定时重复扫描而重复建任务。
6. 为避免首次启用渠道时推送多年前的提醒，只补入最近 48 小时创建且仍未读、到期日未过的站内通知。

**语义限制：** 网络调用成功但进程尚未记录发送结果时发生崩溃，可能重试产生重复通知，这是“至少一次投递”，不是“严格恰好一次”。重新启用曾关闭的渠道时，也可能恢复未发送的队列任务。

## 管理与接口

提醒中心会展示启用的通道、最近 100 条任务和失败原因：

- `GET /api/v1/notification-channels` — 仅列出启用通道名。
- `GET /api/v1/notification-deliveries?limit=100` — 查询最近的投递记录，不包含任何密钥。

发送任务由后台调度触发，暂不提供网页中的手动重发或测试推送按钮。

## 注意事项

- 当前仍只适合可信的本地开发环境，并不代表可以直接发布到公网。
- 外部渠道依赖公网连通性、账户权限和供应商限流。
- 默认不会采集你的账户扣费账单；通知只是根据手动录入的到期记录发送。
