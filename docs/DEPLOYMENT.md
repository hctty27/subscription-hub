# 正式部署：Docker Compose + Nginx + Cloudflare Tunnel

本项目使用两套独立的 Compose：

- `compose.yaml`：仅用于本地开发，前端是 Vite。
- `compose.production.yaml`：仓库跟踪的生产基础配置，构建 Vue 静态文件，由 Nginx 代理 Docker 内网的 Go API。
- `compose.production.local.yaml`：服务器私有覆盖配置（可选，Git 忽略），专门放网络、资源限制等机器差异；不要直接改基础配置。
- `.env.production`：服务器私有环境变量与密钥（Git 忽略）。
- `scripts/compose-prod.sh`：生产 Compose 唯一入口，自动加载上述环境文件与可选覆盖文件，并透传 `config/up/ps/logs/--profile` 等参数。
- `restic`：可选的加密异地备份 profile。

> 公网 HTTPS 由 Cloudflare 提供；Nginx 只处理 Docker 内网 HTTP。后端 `api:8080` 不映射宿主机端口。该配置仍需实际公网联调、安全审计和恢复演练。

## 一、准备

需要 Linux、Docker Engine + Compose 插件，以及托管在 Cloudflare 的域名。

```bash
cd /home/project/subscription-hub
# 初次安装时才复制；已有私有配置不要覆盖
test -f .env.production || cp .env.production.example .env.production
chmod 600 .env.production

# 仅在需要服务器个性化 Compose 时创建
# cp compose.production.local.example.yaml compose.production.local.yaml
```

编辑 `.env.production`：

- `ADMIN_PASSWORD`：至少 12 字符，只对首次初始化有效，勿提交 Git。
- `APP_TIMEZONE`：默认 `Asia/Shanghai`。
- `HTTP_BIND`：默认 `127.0.0.1:18080`，仅供宿主机本地访问；不要改为 `0.0.0.0`。
- `CLOUDFLARE_TUNNEL_TOKEN`：只有通过 Compose 启动 Tunnel 时才需要。
- SMTP、Telegram、Bark：可选推送渠道，不填即关闭。

`APP_DOMAIN`、`HTTPS_BIND` 等旧 Caddy 设置已不再使用。Cloudflare 的公网域名由 Tunnel 路由配置负责。

### 公共配置与本地配置如何协作

默认情况下，`sh scripts/compose-prod.sh` 使用仓库的 `compose.production.yaml`；若检测到 `compose.production.local.yaml`，会把它作为第二个 `-f` 文件自动合并。**后面的文件优先，但部分列表（尤其 ports、volumes）遵循 Compose 特定合并规则，不一定是直接替换。** 先执行 `config` 确认最终结果。相对路径始终基于仓库中的第一个 Compose 文件。

适合放在 `.env.production`：`HTTP_BIND`、`APP_TIMEZONE`、`CLOUDFLARE_TUNNEL_TOKEN`、通知和备份密钥；适合放在本地 YAML：特殊网络、容器资源限制等。勿把密钥写进 YAML。

示例（不会更改仓库公共配置）：

```yaml
services:
  api:
    mem_limit: 768m
  web:
    mem_limit: 256m
```

```bash
sh scripts/compose-prod.sh config --quiet
# 以下输出可能包含密钥，请勿复制到日志或公开 Issue
sh scripts/compose-prod.sh config
```

需要通过其他环境文件测试时，可使用 `SUBSCRIPTION_ENV_FILE=.env.production.example sh scripts/compose-prod.sh config --quiet`。不要在生产环境使用示例密码启动服务。

### 已经在服务器上修改过公共 Compose 的迁移方法

1. **先备份当前完整文件**：`cp -p compose.production.yaml "$HOME/subhub-compose.production.before-migration.yaml"`，并用 `git status --short` 检查是否存在本地或暂存修改。
2. 对照备份，将服务器特有配置分别迁入 `.env.production` 和 `compose.production.local.yaml`。不要直接把整个旧 Compose 拷贝为 local 文件，否则会把仓库中的旧服务配置重复覆盖回来。
3. 确认备份无误、私有配置保存完成，再清理旧文件的 Git 本地改动，执行 `git pull --ff-only`。**不要未经核对直接运行 `git restore` 或 `git reset --hard`。** 若 Git 提示冲突，先手动处理。
4. 更新后执行 `sh scripts/compose-prod.sh config --quiet` 并检查 `sh scripts/compose-prod.sh config` 的挂载、端口和网络，再进行部署；旧数据库位于 `data/`，本改造不迁移数据。


## 二、部署 Nginx 和 API

首次安装或从旧命名卷升级时，**先按第四节执行数据目录准备/迁移**，再运行：

```bash
sh scripts/compose-prod.sh up -d --build
sh scripts/compose-prod.sh ps
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
   sh scripts/compose-prod.sh \
     --profile tunnel up -d --build
   sh scripts/compose-prod.sh \
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

## 四、数据目录与旧命名卷迁移

所有 Docker Compose 数据均使用**项目目录绑定挂载**，开发和生产完全隔离：

```text
/home/project/subscription-hub/
├── data/                         # 生产数据，勿提交 Git
│   ├── subscription-hub.db
│   └── backups/
├── data-dev/                     # 开发数据，勿提交 Git
│   ├── subscription-hub.db
│   └── backups/
├── compose.yaml
├── compose.production.yaml
├── compose.production.local.example.yaml # 跟踪的示例
├── compose.production.local.yaml         # 服务器私有，非必需
├── .env.production                       # 服务器私有，必需
└── scripts/compose-prod.sh
```

`api` 和 `backend` 的容器内数据库路径仍然是 `/data/subscription-hub.db`，备份路径是 `/data/backups`。Restic 的生产备份服务只读取 `./data`，不会触碰 `./data-dev`。

**首次启动和从旧版升级都要先执行目录准备脚本，之后再启动新 Compose**：

```bash
cd /home/project/subscription-hub
# 生产：要求已配置 .env.production；若有旧数据，先进行一致性快照后停止 API 并复制
sh scripts/migrate-data-volumes.sh production
# 开发：按需执行，与生产数据独立
sh scripts/migrate-data-volumes.sh development
```

脚本执行规则：

1. 生产默认查找 `subscription-hub-prod_subscription_data`；开发默认查找 `subscription-hub_subscription_data`。如果旧 Compose 使用了自定义项目名，可以通过 `OLD_VOLUME=旧卷名称` 显式指定；可用 `docker volume ls` 检查。
2. 发现旧命名卷时，若 API 正在运行，先运行 `/app/backup` 创建 SQLite 一致性快照；随后停止相关 API，确认旧卷没有其他运行中的容器使用，再完整复制数据库、WAL 文件和备份。
3. 目标目录**必须为空**。只要已有文件，脚本就会拒绝覆盖，避免误删或数据互相覆盖。如果项目原本已有 `data/` 文件，先核对内容并人工规划迁移。
4. Docker 容器中的服务以 UID 10001 运行；脚本为目录设置正确的属主权限。新安装且没有旧卷时，脚本只准备空目录。
5. 旧命名卷不会被删除；迁移后请检查业务数据与备份，确认无误再决定是否清理旧卷。

脚本不会自动启动或重启服务。准备完成后，按照第二、三节的命令启动生产环境；开发环境使用 `docker compose up -d --build`。如果迁移意外中断，**不要启动新服务，也不要删除旧数据卷**；先检查目录内容，再决定如何重试。容器停机是保证 WAL 数据一致性的重要前提。

`sh scripts/compose-prod.sh down -v` 不会删除项目绑定挂载目录，但**仍可能删除 Restic 等其他命名卷**；不要将其当作清理命令。由于生产数据已保存在项目目录下，宿主机磁盘损坏或误删仍会导致丢失，请务必配置异地备份。

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
sh scripts/compose-prod.sh \
  --profile backup run --rm --no-deps restic init
```

执行备份：

```bash
SUBSCRIPTION_ENV_FILE=.env.production sh scripts/backup-offsite.sh
```

脚本将创建一致性快照、上传加密数据、执行保留策略（7 个日备份、4 个周备份、6 个月备份）并清理旧块。建议通过宿主机 `cron` 或 `systemd timer` 每天 04:00 执行，并对失败告警。

查看快照：

```bash
sh scripts/compose-prod.sh \
  --profile backup run --rm --no-deps restic snapshots
```

恢复时必须先恢复到隔离目录，核对 SQLite 完整性后在停止服务的情况下替换数据；不要覆盖正在运行的 WAL 数据库。请妥善保存 Restic 密码，否则无法解密备份。

## 六、维护与安全

- 更新前先备份；执行 `git pull --ff-only`（确认公共文件没有未处理的本地修改），再执行 `sh scripts/compose-prod.sh up -d --build`。
- 查看日志：`sh scripts/compose-prod.sh logs -f --tail 100`。
- 公网只通过 Cloudflare Tunnel 访问，不将 `HTTP_BIND` 配成公网 IP 或 `0.0.0.0`。
- 保持 API、数据库、Restic 密钥、Tunnel Token 私有；不要暴露 Docker Socket。
- 仅修改 `ADMIN_PASSWORD` 环境变量不会重置已初始化的管理员密码。
- 上线前验证实际 Tunnel 连通、通知投递、异地恢复、漏洞扫描和权限边界。

官方参考：[Cloudflare Tunnel](https://developers.cloudflare.com/tunnel/get-started/)、[Nginx Reverse Proxy](https://docs.nginx.com/nginx/admin-guide/web-server/reverse-proxy/)、[Restic](https://restic.readthedocs.io/en/stable/030_preparing_a_new_repo.html)。
