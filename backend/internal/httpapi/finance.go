package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hctty27/subscription-hub/backend/internal/reminder"
	"github.com/hctty27/subscription-hub/backend/internal/store"
)

type paymentRequest struct {
	ItemID int64 `json:"item_id"`
	AmountMinor int64 `json:"amount_minor"`
	Currency string `json:"currency"`
	PaidDate string `json:"paid_date"`
	Notes string `json:"notes"`
}

func validMonth(month string) bool {
	date,err:=time.Parse("2006-01",month)
	return err==nil&&date.Format("2006-01")==month
}
func validDate(date string) bool {
	parsed,err:=time.Parse("2006-01-02",date)
	return err==nil&&parsed.Format("2006-01-02")==date
}
func limitOffset(c *gin.Context,defaultLimit int)(int,int,bool){
	limit,offset:=defaultLimit,0
	if raw:=c.Query("limit");raw!=""{
		n,err:=strconv.Atoi(raw)
		if err!=nil||n<1||n>500{fail(c,http.StatusBadRequest,"limit 必须在 1 到 500 之间");return 0,0,false}
		limit=n
	}
	if raw:=c.Query("offset");raw!=""{
		n,err:=strconv.Atoi(raw)
		if err!=nil||n<0{fail(c,http.StatusBadRequest,"offset 不能为负数");return 0,0,false}
		offset=n
	}
	return limit,offset,true
}
func (a *api) readPayment(c *gin.Context)(store.Payment,bool){
	var body paymentRequest
	if err:=c.ShouldBindJSON(&body);err!=nil{fail(c,http.StatusBadRequest,"请求 JSON 格式错误");return store.Payment{},false}
	body.Currency=strings.ToUpper(strings.TrimSpace(body.Currency))
	body.Notes=strings.TrimSpace(body.Notes)
	if body.ItemID<=0{fail(c,http.StatusBadRequest,"项目 ID 无效");return store.Payment{},false}
	if body.AmountMinor<=0||body.AmountMinor>1000000000000{fail(c,http.StatusBadRequest,"实付金额必须大于零且在允许范围内");return store.Payment{},false}
	if !allowedCurrency(body.Currency){fail(c,http.StatusBadRequest,"不支持的币种");return store.Payment{},false}
	if !validDate(body.PaidDate){fail(c,http.StatusBadRequest,"实付日期必须是有效的 YYYY-MM-DD");return store.Payment{},false}
	if body.PaidDate>time.Now().In(a.loc).Format("2006-01-02"){fail(c,http.StatusBadRequest,"实付日期不能晚于今天");return store.Payment{},false}
	if len([]rune(body.Notes))>500{fail(c,http.StatusBadRequest,"备注不能超过 500 个字符");return store.Payment{},false}
	if _,err:=a.repo.GetItem(c.Request.Context(),body.ItemID);err!=nil{
		if errors.Is(err,store.ErrNotFound){fail(c,http.StatusBadRequest,"关联项目不存在")}else{failedStore(c,err)}
		return store.Payment{},false
	}
	return store.Payment{ItemID:body.ItemID,AmountMinor:body.AmountMinor,Currency:body.Currency,PaidDate:body.PaidDate,Notes:body.Notes},true
}
func (a *api) listPayments(c *gin.Context){
	limit,offset,ok:=limitOffset(c,100);if !ok{return}
	var itemID int64
	if raw:=c.Query("item_id");raw!=""{
		id,err:=strconv.ParseInt(raw,10,64);if err!=nil||id<=0{fail(c,http.StatusBadRequest,"item_id 不合法");return}
		itemID=id
	}
	month:=c.Query("month")
	if month!=""&&!validMonth(month){fail(c,http.StatusBadRequest,"month 格式必须为 YYYY-MM");return}
	items,err:=a.repo.ListPayments(c.Request.Context(),store.PaymentFilter{ItemID:itemID,Month:month,Limit:limit,Offset:offset})
	if err!=nil{failedStore(c,err);return}
	c.JSON(http.StatusOK,gin.H{"items":items,"limit":limit,"offset":offset})
}
func (a *api) createPayment(c *gin.Context){
	payment,ok:=a.readPayment(c);if !ok{return}
	created,err:=a.repo.CreatePayment(c.Request.Context(),payment)
	if err!=nil{failedStore(c,err);return}
	c.JSON(http.StatusCreated,created)
}
func (a *api) updatePayment(c *gin.Context){
	id,ok:=parseID(c);if !ok{return}
	payment,ok:=a.readPayment(c);if !ok{return}
	payment.ID=id
	updated,err:=a.repo.UpdatePayment(c.Request.Context(),payment)
	if err!=nil{failedStore(c,err);return}
	c.JSON(http.StatusOK,updated)
}
func (a *api) deletePayment(c *gin.Context){
	id,ok:=parseID(c);if !ok{return}
	if err:=a.repo.DeletePayment(c.Request.Context(),id);err!=nil{failedStore(c,err);return}
	c.Status(http.StatusNoContent)
}
func (a *api) expenseReport(c *gin.Context){
	month:=c.DefaultQuery("month",time.Now().In(a.loc).Format("2006-01"))
	if !validMonth(month){fail(c,http.StatusBadRequest,"month 格式必须为 YYYY-MM");return}
	report,err:=a.repo.ExpenseReport(c.Request.Context(),month)
	if err!=nil{failedStore(c,err);return}
	c.JSON(http.StatusOK,report)
}

type ruleRequest struct {
	ItemID int64 `json:"item_id"`
	DaysBefore int `json:"days_before"`
	Enabled *bool `json:"enabled"`
}
func (a *api) readRule(c *gin.Context,updating bool)(store.ReminderRule,bool){
	var body ruleRequest
	if err:=c.ShouldBindJSON(&body);err!=nil{fail(c,http.StatusBadRequest,"请求 JSON 格式错误");return store.ReminderRule{},false}
	if body.DaysBefore<0||body.DaysBefore>30{fail(c,http.StatusBadRequest,"提前提醒天数只能在 0 到 30 之间");return store.ReminderRule{},false}
	if !updating {
		if body.ItemID<=0{fail(c,http.StatusBadRequest,"项目 ID 无效");return store.ReminderRule{},false}
		if _,err:=a.repo.GetItem(c.Request.Context(),body.ItemID);err!=nil{
			if errors.Is(err,store.ErrNotFound){fail(c,http.StatusBadRequest,"关联项目不存在")}else{failedStore(c,err)}
			return store.ReminderRule{},false
		}
	}
	enabled:=true
	if body.Enabled!=nil{enabled=*body.Enabled}
	return store.ReminderRule{ItemID:body.ItemID,DaysBefore:body.DaysBefore,Enabled:enabled},true
}
func (a *api) listReminderRules(c *gin.Context){
	rules,err:=a.repo.ListReminderRules(c.Request.Context())
	if err!=nil{failedStore(c,err);return}
	c.JSON(http.StatusOK,gin.H{"items":rules})
}
func (a *api) createReminderRule(c *gin.Context){
	rule,ok:=a.readRule(c,false);if !ok{return}
	created,err:=a.repo.CreateReminderRule(c.Request.Context(),rule)
	if err!=nil{failedStore(c,err);return}
	c.JSON(http.StatusCreated,created)
}
func (a *api) updateReminderRule(c *gin.Context){
	id,ok:=parseID(c);if !ok{return}
	rule,ok:=a.readRule(c,true);if !ok{return}
	rule.ID=id
	updated,err:=a.repo.UpdateReminderRule(c.Request.Context(),rule)
	if err!=nil{failedStore(c,err);return}
	c.JSON(http.StatusOK,updated)
}
func (a *api) deleteReminderRule(c *gin.Context){
	id,ok:=parseID(c);if !ok{return}
	if err:=a.repo.DeleteReminderRule(c.Request.Context(),id);err!=nil{failedStore(c,err);return}
	c.Status(http.StatusNoContent)
}
func (a *api) listNotifications(c *gin.Context){
	unread:=c.Query("unread")=="true"
	limit,_,ok:=limitOffset(c,100);if !ok{return}
	items,err:=a.repo.ListReminderNotifications(c.Request.Context(),unread,limit)
	if err!=nil{failedStore(c,err);return}
	count,err:=a.repo.CountUnreadNotifications(c.Request.Context())
	if err!=nil{failedStore(c,err);return}
	c.JSON(http.StatusOK,gin.H{"items":items,"unread_count":count})
}
func (a *api) markNotificationRead(c *gin.Context){
	id,ok:=parseID(c);if !ok{return}
	if err:=a.repo.MarkReminderNotificationRead(c.Request.Context(),id);err!=nil{failedStore(c,err);return}
	c.Status(http.StatusNoContent)
}
func (a *api) scanNotifications(c *gin.Context){
	created,err:=reminder.RunOnce(c.Request.Context(),a.repo,a.loc,time.Now())
	if err!=nil{failedStore(c,err);return}
	c.JSON(http.StatusOK,gin.H{"created":created})
}
