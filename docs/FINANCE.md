# 财务账单与站内提醒 API

> 本文档描述开发版接口。所有 API 尚无登录鉴权，只能在可信本地环境访问。

## 数据模型和迁移

SQLite 使用 `PRAGMA user_version=2`。升级时保留旧 `categories`、`items`，新增 `payments`、`reminder_rules`、`reminder_notifications`。

- `items.amount_minor` 是预测续费价格，不代表实际付款。
- `payments.amount_minor` 是真实已支付金额，按支付发生日统计。
- 金额统一使用最小货币单位的整数存储；目前 CNY/USD/HKD/EUR 是分，JPY 是日元整数。
- 多币种分开统计，不直接跨币种相加。

## 支付记录

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/payments?month=YYYY-MM&item_id=1&limit=100&offset=0` | 列表，参数可省略 |
| POST | `/api/v1/payments` | 新增 |
| PUT | `/api/v1/payments/{id}` | 完整更新 |
| DELETE | `/api/v1/payments/{id}` | 删除 |

```json
{
  "item_id": 1,
  "amount_minor": 2000,
  "currency": "USD",
  "paid_date": "2026-11-05",
  "notes": "手动确认已支付"
}
```

`paid_date` 必须是有效且不晚于当前服务端本地日期的 `YYYY-MM-DD`。删除已经存在付款记录的订阅/资源会返回 409，需先处理关联付款记录。

## 费用报表

`GET /api/v1/reports/expenses?month=2026-11`

返回字段：

- `month`：统计月份
- `totals`：按币种分别统计的当月实付金额与笔数
- `trend`：截至该月近 6 个月有实际付款的各币种月度明细

**注意：**当前暂无汇率换算、退款、发票核销和实际付款自动识别功能。

## 站内到期提醒

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/reminder-rules` | 列出所有规则 |
| POST | `/api/v1/reminder-rules` | 新增 |
| PUT | `/api/v1/reminder-rules/{id}` | 修改提前天数和开关 |
| DELETE | `/api/v1/reminder-rules/{id}` | 删除规则和其通知 |
| GET | `/api/v1/notifications?unread=true&limit=100` | 查看站内通知 |
| POST | `/api/v1/notifications/{id}/read` | 标记已读 |
| POST | `/api/v1/notifications/scan` | 手动执行一次扫描 |

创建规则：

```json
{"item_id": 1, "days_before": 7, "enabled": true}
```

到期前 `0–30` 天内，首次扫描会生成一条站内通知（包含提前阈值日期当天和临期窗口）。程序启动后立即扫描，之后每小时执行一次；同一个规则、同一个到期日利用数据库唯一约束去重。

只有 `active` 状态的项目产生提醒。一次性资源已过期不再生成新提醒。订阅按原始账期锚点计算每次到期日期。

当前提醒**不发送邮件、Bark、Telegram 或系统推送**，用户需要打开站内提醒页面查看。正式对外部署前必须补上身份认证、数据备份及访问控制。
