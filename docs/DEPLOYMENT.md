# 正式部署：Docker Compose + Nginx + Cloudflare Tunnel

本项目使用两套独立的 Compose：

- `compose.yaml`：仅用于本地开发，前端是 Vite。
- `compose.production.yaml`：生产候选方案，构建 Vue 静态文件，用 Nginx 托管前端和反向代理 Docker 内网的 Go API，并可选启动 Cloudflare Tunnel。
- `restic`：可选的加密异地备份 profile。

> 公网 HTTPS 由 Cloudflare 提供；Nginx 只处理 Docker 内网 HTTP。后端 `api:8080` 不映射宿主机端口。该配置仍需实际公网联调、安全审计和恢复演练。

## 一、准备

需要 Linux、Docker Engine + Compose 插件，以及托管在 Cloudflare 的域名。

```bash
cd /home/project/subscription-hub
cp .env.production.example .env.production
chmod 600 .env.production
```

编辑 `.env.production`：

- `ADMIN_PASSWORD`：至少 12 字符，只对首次初始化有效，勿提交 Git。
- `APP_TIMEZONE`：默认 `Asia/Shanghai`。
- `HTTP_BIND`：默认 `127.0.0.1:18080`，仅供宿主机本地访问；不要改为 `0.0.0.0`。
- `CLOUDFLARE_TUNNEL_TOKEN`：只有通过 Compose 启动 Tunnel 时才需要。
- SMTP、Telegram、Bark：可选推送渠道，不填即关闭。

`APP_DOMAIN`、`HTTPS_BIND` 等旧 Caddy 设置已不再使用。Cloudflare 的公网域名由 Tunnel 路由配置负责。

## 二、部署 Nginx 和 API

```bash
docker compose --env-file .env.production -f compose.production.yaml up -d --build
docker compose --env-file .env.production -f compose.production.yaml ps
curl -fsS http://127.0.0.1:18080/api/v1/health
```

默认只在宿主机回环地址开放 HTTP 18080，`api` 只在 Compose 网络暴露端口 8080。生产环境 `SESSION_SECURE=true`，必须通过公网 HTTPS 地址登录；本地 HTTP 只适用于诊断，不作为日常管理入口。

> 服务器若已有 Nginx 占用 80/443，新的部署不会与其争抢端口。

## 三、Cloudflare Tunnel（二选一）

### 方式 A：Tunnel 也通过 Docker Compose 管理（推荐）

1. 在 Cloudflare Zero Trust 控制台创建 remotely-managed Tunnel，并复制 Tunnel Token。
2. 在服务器私有 `.env.production` 配置 `CLOUDFLARE_TUNNEL_TOKEN=...`。
3. 在 Tunnel 中添加 Published application route：公网 hostname 选择你的域名；Service URL 填 **`http://web:80`**（因为 cloudflared 与 Nginx 在同一 Compose 网络）。
4. 启动：

   ```bash
   docker compose --env-file .env.production -f compose.production.yaml \
     --profile tunnel up -d --build
   docker compose --env-file .env.production -f compose.production.yaml \
     --profile tunnel logs -f --tail 100 tunnel
   ```

不要将 Token 写进 Compose 源码或公开仓库。这个 profile 默认关闭，没有设置 Token 时不要启用。无需打开服务器公网入站 TCP 80/443；确保 cloudflared 可以从服务器连接 Cloudflare 的网络。

### 方式 B：已在宿主机安装 cloudflared

不启动 `tunnel` profile，使用前述 `docker compose ... up -d --build` 启动网站。

将 Tunnel 的 Published application Service URL 设置为 **`http://127.0.0.1:18080`**，使用宿主机现有的 cloudflared 服务。注意：宿主机上的 `localhost` 与 Docker 容器中的 `localhost` 不同。

两种方式只选一种，避免重复运行同一 Tunnel 的无意多实例。

### 验证

```bash
curl -fsS https://你的域名/api/v1/health
```

然后在浏览器打开 `https://你的域名`，测试登录、续费数据和静态资源。确认 API、Cookie、HTTPS 及域名配置全部正常后再正式使用。

## 四、数据持久化与迁移

生产配置沿用 Compose 项目名 `subscription-hub-prod`，数据库位于容器内 `/data/subscription-hub.db`，由 **`subscription_data` 命名卷**持久化；快照位于相同数据卷的 `/data/backups`。

Docker 命名卷由 Docker 管理，**默认不保存在 `/home/project/subscription-hub` 项目目录下**。如果希望改为项目目录绑定挂载，需要另行规划权限与备份，不能直接把运行中的 WAL 数据库文件复制过去。

从旧 Caddy 生产版切换至 Nginx 时，Compose 项目名和 `subscription_data` 卷均保持不变，数据库无需重新初始化。升级前应生成一致性快照并验证。旧 `caddy_data` / `caddy_config` 数据卷不会由本次改造自动删除，请在确认不再回滚 Caddy 后自行决定是否清理。

开发版 `compose.yaml` 使用不同的 Compose 项目和数据卷，**不会自动迁移到生产版**。

**切勿执行 `docker compose down -v`**，否则会删除生产数据库和本地备份。

## 五、加密异地备份（Restic）

应用每天 03:00 自动创建本地 SQLite 一致性快照，保留最近 7 份。建议使用 Restic 加密上传到独立对象存储，例如 S3 或 R2。

在 `.env.production` 配置：

```dotenv
RESTIC_REPOSITORY=s3:https://object-storage.example.com/my-bucket/subscription-hub
RESTIC_PASSWORD=请使用独立强随机备份密码
AWS_ACCESS_KEY_ID=你的对象存储密钥ID
AWS_SECRET_ACCESS_KEY=你的对象存储密钥
AWS_DEFAULT_REGION=auto
```

上面是占位示例，必须替换为服务商的实际地址和密钥。

初始化仓库（首次执行一次）：

```bash
docker compose --env-file .env.production -f compose.production.yaml \
  --profile backup run --rm --no-deps restic init
```

执行备份：

```bash
SUBSCRIPTION_ENV_FILE=.env.production sh scripts/backup-offsite.sh
```

脚本将创建一致性快照、上传加密数据、执行保留策略（7 个日备份、4 个周备份、6 个月备份）并清理旧块。建议通过宿主机 `cron` 或 `systemd timer` 每天 04:00 执行，并对失败告警。

查看快照：

```bash
docker compose --env-file .env.production -f compose.production.yaml \
  --profile backup run --rm --no-deps restic snapshots
```

恢复时必须先恢复到隔离目录，核对 SQLite 完整性后在停止服务的情况下替换数据；不要覆盖正在运行的 WAL 数据库。请妥善保存 Restic 密码，否则无法解密备份。

## 六、维护与安全

- 更新前先备份；更新命令与启动命令相同，使用 `up -d --build`。
- 查看日志：`docker compose --env-file .env.production -f compose.production.yaml logs -f --tail 100`。
- 公网只通过 Cloudflare Tunnel 访问，不将 `HTTP_BIND` 配成公网 IP 或 `0.0.0.0`。
- 保持 API、数据库、Restic 密钥、Tunnel Token 私有；不要暴露 Docker Socket。
- 仅修改 `ADMIN_PASSWORD` 环境变量不会重置已初始化的管理员密码。
- 上线前验证实际 Tunnel 连通、通知投递、异地恢复、漏洞扫描和权限边界。

官方参考：[Cloudflare Tunnel](https://developers.cloudflare.com/tunnel/get-started/)、[Nginx Reverse Proxy](https://docs.nginx.com/nginx/admin-guide/web-server/reverse-proxy/)、[Restic](https://restic.readthedocs.io/en/stable/030_preparing_a_new_repo.html)。
