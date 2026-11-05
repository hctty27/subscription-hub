package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hctty27/subscription-hub/backend/internal/store"
)

type renewalRequest struct {
	DueDate     string `json:"due_date"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	PaidDate    string `json:"paid_date"`
	Notes       string `json:"notes"`
}

func (a *api) renewItem(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input renewalRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, http.StatusBadRequest, "请求 JSON 格式错误")
		return
	}
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Notes = strings.TrimSpace(input.Notes)
	if !validDate(input.DueDate) {
		fail(c, http.StatusBadRequest, "账期到期日无效")
		return
	}
	if input.AmountMinor < 1 || input.AmountMinor > 1000000000000 {
		fail(c, http.StatusBadRequest, "实付金额必须大于零且在允许范围内")
		return
	}
	if !allowedCurrency(input.Currency) {
		fail(c, http.StatusBadRequest, "不支持的币种")
		return
	}
	if !validDate(input.PaidDate) || input.PaidDate > time.Now().In(a.loc).Format("2006-01-02") {
		fail(c, http.StatusBadRequest, "实付日期无效或晚于今天")
		return
	}
	if len([]rune(input.Notes)) > 500 {
		fail(c, http.StatusBadRequest, "备注不能超过 500 个字符")
		return
	}
	renewal, err := a.repo.SettleRenewal(c.Request.Context(), store.Payment{
		ItemID: id, AmountMinor: input.AmountMinor, Currency: input.Currency,
		PaidDate: input.PaidDate, Notes: input.Notes,
	}, input.DueDate, time.Now().In(a.loc))
	if errors.Is(err, store.ErrConflict) {
		fail(c, http.StatusConflict, "账期已结算、状态已变化或币种不匹配，请刷新项目后重试")
		return
	}
	if err != nil {
		failedStore(c, err)
		return
	}
	c.JSON(http.StatusCreated, renewal)
}

func (a *api) listItemRenewals(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if _, err := a.repo.GetItem(c.Request.Context(), id); err != nil {
		failedStore(c, err)
		return
	}
	renewals, err := a.repo.ListRenewals(c.Request.Context(), id, 100)
	if err != nil {
		failedStore(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": renewals})
}


func (a *api) undoItemRenewal(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	renewalID, err := strconv.ParseInt(c.Param("renewalID"), 10, 64)
	if err != nil || renewalID <= 0 {
		fail(c, http.StatusBadRequest, "无效的续期记录 ID")
		return
	}
	err = a.repo.UndoRenewal(c.Request.Context(), id, renewalID)
	if errors.Is(err, store.ErrConflict) {
		fail(c, http.StatusConflict, "只能撤销该项目最近一次续期")
		return
	}
	if err != nil {
		failedStore(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
