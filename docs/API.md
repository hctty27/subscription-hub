# V1 API（开发版）

统一前缀：`/api/v1`。响应是 JSON，失败格式：`{"error":{"message":"中文错误信息"}}`。

## 分类

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/categories` | 返回 `{"items":[]}` |
| POST | `/categories` | 新增 |
| PUT | `/categories/{id}` | 完整更新 |
| DELETE | `/categories/{id}` | 删除（被项目引用时 409） |

创建示例：

```bash
curl -X POST http://localhost:8080/api/v1/categories \
  -H 'Content-Type: application/json' \
  -d '{"name":"AI 服务","color":"#5470e8"}'
```

## 项目（订阅 / 资源）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/items` | 查询，支持 kind/status/q/limit/offset |
| GET | `/items/{id}` | 单条 |
| POST | `/items` | 新增 |
| PUT | `/items/{id}` | 完整更新 |
| DELETE | `/items/{id}` | 删除 |

新增示例（省略分类关联）：

```bash
curl -X POST http://localhost:8080/api/v1/items \
  -H 'Content-Type: application/json' \
  -d '{"name":"ChatGPT Plus","kind":"subscription","status":"active","amount_minor":2000,"currency":"USD","period_unit":"month","period_count":1,"first_due_date":"2026-11-20","notes":""}'
```

字段：

| 字段 | 规则 |
|---|---|
| `kind` | `subscription` / `resource` |
| `category_id` | 数字 ID 或 null |
| `status` | `active` / `paused` / `canceled` |
| `amount_minor` | 非负整数，最大 10^12；如 20 USD 为 2000 |
| `currency` | CNY、USD、HKD、EUR、JPY |
| `period_unit` | day、month、year；resource 可用 none |
| `period_count` | 1-365，none 时归零 |
| `first_due_date` | YYYY-MM-DD |
| `notes` | 最长 500 个字符 |

`next_due_date` 和 `days_until` 为服务端计算字段：正常状态会填充；暂停、取消状态不会产生续费日期。对于 `period_unit=none` 的资源，即使到期日已过去仍返回该日期，`days_until` 为负数，便于展示到期状态。所有日期基于服务端 `APP_TIMEZONE` 的当前自然日计算。

列表默认返回最多 200 条，可通过 `limit`（1-500）和 `offset` 分页；前端当前仅载入前 500 条。

## 当前限制

暂不支持鉴权、付款历史、自动提醒、币种换算和多用户。禁止将此开发接口开放至公网。
