package httpapi

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hctty27/subscription-hub/backend/internal/billing"
	"github.com/hctty27/subscription-hub/backend/internal/store"
)

type api struct {
	repo *store.Store
	loc  *time.Location
}

func NewRouter(repo *store.Store, loc *time.Location) *gin.Engine {
	if loc == nil {
		loc = time.UTC
	}
	a := &api{repo: repo, loc: loc}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	v1 := r.Group("/api/v1")
	v1.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1.GET("/categories", a.listCategories)
	v1.POST("/categories", a.createCategory)
	v1.PUT("/categories/:id", a.updateCategory)
	v1.DELETE("/categories/:id", a.deleteCategory)
	v1.GET("/items", a.listItems)
	v1.GET("/items/:id", a.getItem)
	v1.POST("/items", a.createItem)
	v1.PUT("/items/:id", a.updateItem)
	v1.DELETE("/items/:id", a.deleteItem)
	v1.GET("/payments", a.listPayments)
	v1.POST("/payments", a.createPayment)
	v1.PUT("/payments/:id", a.updatePayment)
	v1.DELETE("/payments/:id", a.deletePayment)
	v1.GET("/reports/expenses", a.expenseReport)
	v1.GET("/reminder-rules", a.listReminderRules)
	v1.POST("/reminder-rules", a.createReminderRule)
	v1.PUT("/reminder-rules/:id", a.updateReminderRule)
	v1.DELETE("/reminder-rules/:id", a.deleteReminderRule)
	v1.GET("/notifications", a.listNotifications)
	v1.POST("/notifications/:id/read", a.markNotificationRead)
	v1.POST("/notifications/scan", a.scanNotifications)
	return r
}

type categoryRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "无效的 ID")
		return 0, false
	}
	return id, true
}

func fail(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": gin.H{"message": message}})
}

func failedStore(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		fail(c, http.StatusNotFound, "记录不存在")
	case errors.Is(err, store.ErrCategory):
		fail(c, http.StatusBadRequest, "分类不存在")
	case errors.Is(err, store.ErrConflict):
		fail(c, http.StatusConflict, "名称重复，或分类仍被项目引用")
	default:
		log.Printf("数据库处理失败：%v", err)
		fail(c, http.StatusInternalServerError, "服务器暂时无法处理请求")
	}
}

func validateCategory(input categoryRequest) (store.Category, string) {
	name := strings.TrimSpace(input.Name)
	if len([]rune(name)) == 0 || len([]rune(name)) > 40 {
		return store.Category{}, "分类名称长度必须为 1 到 40 个字符"
	}
	color := strings.TrimSpace(input.Color)
	if color == "" {
		color = "#5470e8"
	}
	if !colorPattern.MatchString(color) {
		return store.Category{}, "颜色必须是 #RRGGBB 格式"
	}
	return store.Category{Name: name, Color: color}, ""
}

func (a *api) listCategories(c *gin.Context) {
	items, err := a.repo.ListCategories(c.Request.Context())
	if err != nil {
		failedStore(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *api) createCategory(c *gin.Context) {
	var input categoryRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, http.StatusBadRequest, "请求 JSON 格式错误")
		return
	}
	item, message := validateCategory(input)
	if message != "" {
		fail(c, http.StatusBadRequest, message)
		return
	}
	created, err := a.repo.CreateCategory(c.Request.Context(), item)
	if err != nil {
		failedStore(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (a *api) updateCategory(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input categoryRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, http.StatusBadRequest, "请求 JSON 格式错误")
		return
	}
	item, message := validateCategory(input)
	if message != "" {
		fail(c, http.StatusBadRequest, message)
		return
	}
	item.ID = id
	updated, err := a.repo.UpdateCategory(c.Request.Context(), item)
	if err != nil {
		failedStore(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (a *api) deleteCategory(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := a.repo.DeleteCategory(c.Request.Context(), id); err != nil {
		failedStore(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type itemRequest struct {
	CategoryID   *int64 `json:"category_id"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	AmountMinor  int64  `json:"amount_minor"`
	Currency     string `json:"currency"`
	PeriodUnit   string `json:"period_unit"`
	PeriodCount  int    `json:"period_count"`
	FirstDueDate string `json:"first_due_date"`
	Notes        string `json:"notes"`
}

func validItem(input itemRequest) (store.Item, string) {
	item := store.Item{
		CategoryID: input.CategoryID, Name: strings.TrimSpace(input.Name),
		Kind: input.Kind, Status: input.Status, AmountMinor: input.AmountMinor,
		Currency: strings.ToUpper(strings.TrimSpace(input.Currency)),
		PeriodUnit: input.PeriodUnit, PeriodCount: input.PeriodCount,
		FirstDueDate: input.FirstDueDate, Notes: strings.TrimSpace(input.Notes),
	}
	if len([]rune(item.Name)) < 1 || len([]rune(item.Name)) > 100 {
		return item, "名称长度必须为 1 到 100 个字符"
	}
	if item.Kind != "subscription" && item.Kind != "resource" {
		return item, "类型必须为订阅或资源"
	}
	if item.Status != "active" && item.Status != "paused" && item.Status != "canceled" {
		return item, "状态不合法"
	}
	if item.AmountMinor < 0 || item.AmountMinor > 1000000000000 {
		return item, "金额必须在允许范围内"
	}
	if !allowedCurrency(item.Currency) {
		return item, "暂不支持该货币代码"
	}
	if item.CategoryID != nil && *item.CategoryID <= 0 {
		return item, "分类 ID 不合法"
	}
	if len([]rune(item.Notes)) > 500 {
		return item, "备注不能超过 500 个字符"
	}
	if item.PeriodUnit == "none" {
		if item.Kind != "resource" {
			return item, "订阅必须设置续费周期"
		}
		item.PeriodCount = 0
	} else if item.PeriodUnit != "day" && item.PeriodUnit != "month" && item.PeriodUnit != "year" {
		return item, "不支持的续费周期"
	} else if item.PeriodCount < 1 || item.PeriodCount > 365 {
		return item, "续费周期数值必须在 1 到 365 之间"
	}
	if _, err := billing.NextDue(item.FirstDueDate, item.PeriodUnit, item.PeriodCount, time.Now()); err != nil {
		return item, "到期日期格式必须为 YYYY-MM-DD 且日期有效"
	}
	return item, ""
}

func allowedCurrency(value string) bool {
	switch value {
	case "CNY", "USD", "HKD", "EUR", "JPY":
		return true
	default:
		return false
	}
}

func (a *api) fillSchedule(item *store.Item) {
	if item.Status != "active" {
		return
	}
	today := time.Now().In(a.loc)
	date, err := billing.NextDue(item.FirstDueDate, item.PeriodUnit, item.PeriodCount, today)
	if err != nil {
		return
	}
	item.NextDueDate = date
	item.DaysUntil, _ = billing.DaysUntil(date, today)
}

func (a *api) listItems(c *gin.Context) {
	limit := 200
	offset := 0
	if value := c.Query("limit"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 500 {
			fail(c, http.StatusBadRequest, "limit 必须在 1 到 500 之间")
			return
		}
		limit = n
	}
	if value := c.Query("offset"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			fail(c, http.StatusBadRequest, "offset 不能为负数")
			return
		}
		offset = n
	}
	filter := store.ItemFilter{Kind: c.Query("kind"), Status: c.Query("status"),
		Query: strings.TrimSpace(c.Query("q")), Limit: limit, Offset: offset}
	if filter.Kind != "" && filter.Kind != "subscription" && filter.Kind != "resource" {
		fail(c, http.StatusBadRequest, "kind 参数不合法")
		return
	}
	if filter.Status != "" && filter.Status != "active" && filter.Status != "paused" && filter.Status != "canceled" {
		fail(c, http.StatusBadRequest, "status 参数不合法")
		return
	}
	if len([]rune(filter.Query)) > 100 {
		fail(c, http.StatusBadRequest, "搜索内容太长")
		return
	}
	items, err := a.repo.ListItems(c.Request.Context(), filter)
	if err != nil {
		failedStore(c, err)
		return
	}
	for i := range items {
		a.fillSchedule(&items[i])
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "limit": limit, "offset": offset})
}

func (a *api) getItem(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := a.repo.GetItem(c.Request.Context(), id)
	if err != nil {
		failedStore(c, err)
		return
	}
	a.fillSchedule(&item)
	c.JSON(http.StatusOK, item)
}

func (a *api) readItem(c *gin.Context) (store.Item, bool) {
	var input itemRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, http.StatusBadRequest, "请求 JSON 格式错误")
		return store.Item{}, false
	}
	item, message := validItem(input)
	if message != "" {
		fail(c, http.StatusBadRequest, message)
		return store.Item{}, false
	}
	exists, err := a.repo.CategoryExists(c.Request.Context(), item.CategoryID)
	if err != nil {
		failedStore(c, err)
		return store.Item{}, false
	}
	if !exists {
		fail(c, http.StatusBadRequest, "分类不存在")
		return store.Item{}, false
	}
	return item, true
}

func (a *api) createItem(c *gin.Context) {
	item, ok := a.readItem(c)
	if !ok {
		return
	}
	created, err := a.repo.CreateItem(c.Request.Context(), item)
	if err != nil {
		failedStore(c, err)
		return
	}
	a.fillSchedule(&created)
	c.JSON(http.StatusCreated, created)
}

func (a *api) updateItem(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, ok := a.readItem(c)
	if !ok {
		return
	}
	item.ID = id
	updated, err := a.repo.UpdateItem(c.Request.Context(), item)
	if err != nil {
		failedStore(c, err)
		return
	}
	a.fillSchedule(&updated)
	c.JSON(http.StatusOK, updated)
}

func (a *api) deleteItem(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := a.repo.DeleteItem(c.Request.Context(), id); err != nil {
		failedStore(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
