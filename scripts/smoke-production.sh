#!/bin/sh
set -eu

root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$root"

base="http://127.0.0.1:18080"
for i in $(seq 1 60); do
  if curl -fsS "$base/api/v1/health" >/dev/null && curl -fsS "$base/" >/dev/null; then
    break
  fi
  sleep 2
done

curl -fsS "$base/" | grep -q 'Subscription Hub'
curl -fsS "$base/api/v1/health" | grep -q '"status":"ok"'

code="$(curl -s -o /dev/null -w '%{http_code}' "$base/api/v1/items")"
test "$code" = "401"

curl -fsS -D /tmp/sh-prod-login-headers -X POST "$base/api/v1/auth/login" \
  -H 'Content-Type: application/json' -H 'X-CSRF-Token: 1' \
  -d '{"password":"ci-prod-long-password-123"}' >/tmp/sh-prod-login.json

grep -q '"authenticated":true' /tmp/sh-prod-login.json
grep -qi '^set-cookie:.*secure' /tmp/sh-prod-login-headers
cookie="$(grep -i '^set-cookie:' /tmp/sh-prod-login-headers | head -1 | cut -d' ' -f2 | cut -d';' -f1)"
test -n "$cookie"

curl -fsS "$base/api/v1/auth/me" -H "Cookie: $cookie" | grep -q '"authenticated":true'

code="$(curl -s -o /dev/null -w '%{http_code}' \
  -H "Cookie: $cookie" -X POST "$base/api/v1/categories" \
  -H 'Content-Type: application/json' -d '{"name":"缺少 CSRF"}')"
test "$code" = "403"

curl -fsS "$base/api/v1/categories" -H "Cookie: $cookie" | grep -q '"items":'
echo "生产 HTTP 和安全访问测试通过"
