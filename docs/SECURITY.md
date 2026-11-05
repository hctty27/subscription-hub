# 管理员身份认证与备份

## 账户

当前只支持一个管理员账户，不支持注册、多用户和密码重置。

首次启动设置 `ADMIN_PASSWORD`（至少 12 字符），保存 Argon2id 密码哈希及唯一盐值。后续启动从 SQLite 读取哈希，不受 `ADMIN_PASSWORD` 变更影响。

### 接口

`POST /api/v1/auth/login`

```json
{"password":"管理员密码"}
```

登录成功后服务端设置 `HttpOnly; SameSite=Strict; Path=/api/v1` Cookie，有效期 7 天。服务端 SQLite 只保存令牌 SHA-256 摘要和到期时间。正式 HTTPS 环境必须设置 `SESSION_SECURE=true`，防止 Cookie 通过 HTTP 传输。

`GET /api/v1/auth/me`：校验是否登录。

`POST /api/v1/auth/logout`：删除服务端会话并清空 Cookie。

所有账本业务 API 和备份 API 都要求有效登录。除 GET/HEAD/OPTIONS 以外的写操作要求自定义请求头 `X-CSRF-Token: 1`，并拒绝来源域名与请求 Host 不匹配的 Origin。错误密码有限流机制。

### 备份接口

- `GET /api/v1/backups`：返回文件名、字节数及创建时间，不返回数据库文件内容。
- `POST /api/v1/backups`：立即生成快照，并校验数据库完整性。

系统按 APP_TIMEZONE 每天 03:00 自动生成快照，保留最近 7 份。使用 SQLite `VACUUM INTO`，支持 WAL 并发读写下的一致性备份。

**注意**：仅在数据库所在的数据卷备份不能抵御磁盘损坏和误删，必须自行配置外部备份。尚无一键恢复，恢复应在停止服务之后人工进行。

### 部署限制

当前 Compose 暴露的是 Vite 开发服务器，不能直接公开。生产部署仍需要静态资源构建、HTTPS、Secure Cookie、访问控制、数据恢复演练和审计。
