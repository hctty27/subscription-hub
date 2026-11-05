# 财务账单与站内提醒 API

> 所有管理接口均要求管理员登录，会话凭据使用受保护的 Cookie；写请求需携带 CSRF 请求头。公网部署仍需按安全指南配置。

## 数据模型和迁移

SQLite 当前使用 `PRAGMA user_version=5`。升级会保留旧 `categories`、`items`、`payments` 等历史数据，并新增续期结算状态与历史记录。部署升级前先备份数据库。

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


## 续期账期与实际付款

单纯的付款流水不代表已续期，系统把 **付款事实** 与 **账期结算** 分开处理：

- `POST /api/v1/payments`：仅新增一笔实际付款，不影响到期日。
- `POST /api/v1/items/{id}/renew`：在一个事务中记录实际付款并确认指定账期已结算，将待处理到期日推进到下一次原始锚点续费日期。
- `GET /api/v1/items/{id}/renewals`：查看最近 100 条账期结算历史，包含关联付款 ID、已结算到期日与下一到期日。
- `DELETE /api/v1/items/{id}/renewals/{renewalID}`：撤销**最近一次续期**，不会删除对应付款流水。账期配置未变更时恢复原来的待处理日期；已调整续费规则时只解除历史关联，不覆盖新规则。

续期请求示例：

```json
{
  "due_date": "2026-11-20",
  "amount_minor": 2000,
  "currency": "USD",
  "paid_date": "2026-11-05",
  "notes": "确认本期续费"
}
```

`due_date` 为客户端先前查询到的**当前待处理到期日**，服务端会校验并发与过时请求；重复结算返回 HTTP 409。金额仍以最小货币单位存储，币种必须匹配订阅当前配置；付款日期不能晚于服务端今天。

项目响应新增：

- `next_due_date`：当前待处理的账期日期。**逾期而未续期不会自动跳过**。
- `days_until`：允许为负数，表示未处理的逾期天数。
- `renewal_completed`：仅一次性资源完成结算后为 `true`，此时无后续到期日。

### 历史数据兼容

V5 迁移不会凭空生成欠费历史：首次访问旧项目时根据当日有效续费计划初始化当前待处理账期；之后这一日期会持久化，逾期仍保留。

更改项目的起始到期日期、计费单位或周期数量时，会重新生成当前待处理账期；既有付款与历史结算记录不会被删除。普通名称、金额、备注修改则不会重置账期。

结算关联的付款不能直接编辑或删除。若误操作，先撤销最近一次续期，再根据实际情况修改或删除付款。需要批量修正历史账期时应先备份数据库。
