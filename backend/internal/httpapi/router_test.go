package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/hctty27/subscription-hub/backend/internal/store"
)

func testRouter(t *testing.T) (http.Handler, *store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalog.sqlite")
	repo, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })
	return NewRouter(repo, time.UTC), repo, path
}

func request(t *testing.T, router http.Handler, method, path string, body any, status int) map[string]any {
	t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != status {
		t.Fatalf("%s %s 状态码=%d，期望=%d：%s", method, path, rec.Code, status, rec.Body.String())
	}
	if rec.Body.Len() == 0 {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestHealth(t *testing.T) {
	router, _, _ := testRouter(t)
	result := request(t, router, http.MethodGet, "/api/v1/health", nil, http.StatusOK)
	if result["status"] != "ok" {
		t.Fatalf("非预期健康检查响应：%v", result)
	}
}

func TestCatalogCRUDAndPersistence(t *testing.T) {
	router, repo, path := testRouter(t)
	cat := request(t, router, "POST", "/api/v1/categories", map[string]any{
		"name": "AI 服务", "color": "#5470e8",
	}, http.StatusCreated)
	catID := int64(cat["id"].(float64))

	request(t, router, "POST", "/api/v1/categories", map[string]any{
		"name": "ai 服务", "color": "#5470e8",
	}, http.StatusConflict)

	item := map[string]any{
		"name": "ChatGPT Plus", "kind": "subscription", "category_id": catID,
		"status": "active", "amount_minor": 2000, "currency": "USD",
		"period_unit": "month", "period_count": 1, "first_due_date": "2026-01-31",
		"notes": "个人测试",
	}
	created := request(t, router, "POST", "/api/v1/items", item, http.StatusCreated)
	itemID := int64(created["id"].(float64))
	if created["next_due_date"] == "" {
		t.Fatal("缺少下次续费日期")
	}

	request(t, router, "DELETE", "/api/v1/categories/1", nil, http.StatusConflict)
	list := request(t, router, "GET", "/api/v1/items?kind=subscription", nil, http.StatusOK)
	if len(list["items"].([]any)) != 1 {
		t.Fatalf("列表数量错误：%v", list)
	}
	item["name"] = "ChatGPT"
	request(t, router, "PUT", "/api/v1/items/1", item, http.StatusOK)
	got := request(t, router, "GET", "/api/v1/items/1", nil, http.StatusOK)
	if got["name"] != "ChatGPT" {
		t.Fatalf("更新未生效：%v", got)
	}

	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := reopened.GetItem(t.Context(), itemID)
	if err != nil || loaded.Name != "ChatGPT" {
		t.Fatalf("重启后数据丢失：item=%+v, err=%v", loaded, err)
	}
	if itemID < 1 || catID < 1 {
		t.Fatal("返回的主键不合法")
	}

	request(t, NewRouter(reopened, time.UTC), "DELETE", "/api/v1/items/1", nil, http.StatusNoContent)
	request(t, NewRouter(reopened, time.UTC), "DELETE", "/api/v1/categories/1", nil, http.StatusNoContent)
	request(t, NewRouter(reopened, time.UTC), "GET", "/api/v1/items/1", nil, http.StatusNotFound)
}

func TestItemValidation(t *testing.T) {
	router, _, _ := testRouter(t)
	valid := map[string]any{
		"name": "测试", "kind": "subscription", "status": "active",
		"amount_minor": 100, "currency": "CNY", "period_unit": "month",
		"period_count": 1, "first_due_date": "2026-11-05",
	}
	for _, key := range []string{"name", "kind", "first_due_date"} {
		body := make(map[string]any)
		for k, v := range valid {
			body[k] = v
		}
		delete(body, key)
		request(t, router, "POST", "/api/v1/items", body, http.StatusBadRequest)
	}
	body := make(map[string]any)
	for k, v := range valid {
		body[k] = v
	}
	body["category_id"] = 9999
	request(t, router, "POST", "/api/v1/items", body, http.StatusBadRequest)
	body["category_id"] = nil
	body["first_due_date"] = "2026-02-30"
	request(t, router, "POST", "/api/v1/items", body, http.StatusBadRequest)
	request(t, router, "GET", "/api/v1/items?limit=999999", nil, http.StatusBadRequest)
}
