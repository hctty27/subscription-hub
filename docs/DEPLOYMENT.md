# 正式部署与异地备份

本项目提供两个**互相独立**的 Docker Compose 配置：

- `compose.yaml`：本地开发，Node/Vite 开发服务器监听 `127.0.0.1:5173`。
- `compose.production.yaml`：生产候选方案，Node 仅在镜像构建阶段运行，Caddy 提供静态文件及 HTTPS，并反向代理内网 Go API。

> 生产配置通过容器集成测试，但这不等于已经获得上线认证。公网发布前仍需完成安全审计、真实 DNS/HTTPS 验证和恢复演练。

## 一、首次部署

需要 Linux 服务器、Docker Engine + Compose 插件、可控制的域名。

1. 为域名配置 A/AAAA DNS 记录，指向部署机器。开放 TCP 80/443；如果服务器已有 Nginx、Caddy 或其他服务占用这些端口，请先规划反向代理与端口分配，**不要直接停掉既有网站**。
2. 复制示例环境文件并填入私密值：

   ```bash
   cp .env.production.example .env.production
   chmod 600 .env.production
   ```

3. 修改 `.env.production`：

   - `APP_DOMAIN`：例如 `subscription.example.com`。
   - `ADMIN_PASSWORD`：首次启动使用的强随机密码，至少 12 字符；不得提交 Git。
   - `APP_TIMEZONE`：默认 `Asia/Shanghai`。
   - 需要外部通知时，配置 SMTP/Telegram/Bark；未配置不会主动推送。

4. 构建并启动：

   ```bash
   docker compose --env-file .env.production -f compose.production.yaml up -d --build
   ```

5. 验证：

   ```bash
   docker compose --env-file .env.production -f compose.production.yaml ps
   curl -fsS https://subscription.example.com/api/v1/health
   ```

   用自己的域名替换示例。浏览器打开该 HTTPS 域名进入管理员登录界面。

Caddy 在具备域名、可用 DNS、80/443 TCP 连通性的条件下负责获取、续期证书及 HTTP → HTTPS 跳转。证书保存在 `caddy_data` 数据卷中，请勿随意清空。

**安全边界：** 生产环境仅由 `web` 发布 80/443，后端 `api` 使用 Compose 内网 `expose: 8080`，没有主机端口映射。生产容器默认开启只读根文件系统、限制额外权限和必要的持久化卷；`SESSION_SECURE` 强制为 `true`。

### 数据卷与开发环境区别

生产 Compose 项目名是 `subscription-hub-prod`，与开发 Compose 使用不同的数据库卷。**不会自动读取开发版的数据库**。

需要迁移已有数据时：

1. 保留并停止开发版写入。
2. 在开发版通过数据库备份命令生成最新一致性快照（可使用管理界面手动备份）。
3. 先在新环境完成备份、再停止生产服务，将快照复制到生产 `DB_PATH=/data/subscription-hub.db` 所在卷。不要直接复制正在运行的 WAL 数据库。
4. 重启生产服务，并核对分类、订阅、实付流水和提醒记录。

注意：生产 Compose 的 `down -v` 会同时删除 SQLite 数据和本地备份，不要执行。

## 二、远端加密备份（Restic）

应用内每天 03:00 创建本地 SQLite 一致性快照。为防止宿主机磁盘损坏或误删，建议使用 Restic 将快照加密上传到**独立的 S3 或 S3 兼容存储**。

Restic 默认不随应用启动，不会在没有配置远程仓库时自动上传。

### 准备仓库

例如使用已创建的 S3 兼容 Bucket，修改 `.env.production`：

```dotenv
RESTIC_REPOSITORY=s3:https://object-storage.example.com/my-bucket/subscription-hub
RESTIC_PASSWORD=请使用独立强随机备份密码
AWS_ACCESS_KEY_ID=你的对象存储密钥ID
AWS_SECRET_ACCESS_KEY=你的对象存储密钥
AWS_DEFAULT_REGION=auto
```

填写供应商要求的实际存储端点与区域；不要用本例地址。为 Restic 配置权限最小化的独立对象存储凭据，切勿将仓库密码和访问密钥提交到 GitHub。

首次使用，只需初始化仓库一次：

```bash
docker compose --env-file .env.production -f compose.production.yaml \
  --profile backup run --rm --no-deps restic init
```

### 执行完整备份

```bash
SUBSCRIPTION_ENV_FILE=.env.production sh scripts/backup-offsite.sh
```

脚本会执行：

1. 调用生产 API 容器内的 `/app/backup`，创建 SQLite 一致性快照。
2. 将 `/data/backups` 通过 Restic 加密传输到已配置仓库。
3. 对 `subscription-hub` 标签执行保留策略：最近 7 个有备份的日、4 个周、6 个月，并运行 `prune`。

失败时立即退出，建议由宿主机的 systemd timer 或 cron 调用，并接入执行失败告警。示例 cron（每天 04:00）：

```cron
0 4 * * * cd /path/to/subscription-hub && SUBSCRIPTION_ENV_FILE=.env.production /bin/sh scripts/backup-offsite.sh >> /var/log/subscription-hub-backup.log 2>&1
```

不要把备份仓库的加密密码丢失，否则将无法解密还原。对象存储若启用版本锁定/不可变策略，要评估 `forget --prune` 与策略的兼容性。

### 检查和恢复演练

查看远端快照：

```bash
docker compose --env-file .env.production -f compose.production.yaml \
  --profile backup run --rm --no-deps restic snapshots
```

Restic 可以使用 `restore` 将远端快照提取到指定目录；**必须先恢复到隔离目录检查 SQLite 完整性**，不能在应用正在写入数据库时直接覆盖生产数据库。

建议每月至少在临时机器或隔离 Docker 卷验证一次恢复数据，确认分类、订阅、付款记录都可打开。

## 三、运行维护

- 更新：备份后执行 `docker compose --env-file .env.production -f compose.production.yaml up -d --build`。
- 查看日志：`docker compose --env-file .env.production -f compose.production.yaml logs -f --tail 100`。
- 保留 Caddy 的 `caddy_data` 和数据库 `subscription_data` 数据卷。
- 管理员密码只在首次初始化时设置，修改环境文件不会自动改动已经保存的密码。
- 不要通过公网暴露 Docker Socket、数据库文件、备份目录、`8080` API 或 Vite 开发端口。

## 四、当前仍需人工验证

- 真正域名与证书签发；公网网络与浏览器访问情况。
- 邮箱/Telegram/Bark 使用真实凭据的投递。
- 独立 S3 Bucket 的权限、加密、上传、离线恢复。
- 漏洞扫描、密码恢复流程、持续备份告警与最终发布审批。

参考：<https://caddyserver.com/docs/automatic-https>、<https://restic.readthedocs.io/en/stable/030_preparing_a_new_repo.html>。
