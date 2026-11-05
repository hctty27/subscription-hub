package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/hctty27/subscription-hub/backend/internal/reminder"
)

func TestReminderIdempotencyAndRead(t *testing.T) {
	router, repo, _ := testRouter(t)
	item := request(t,router,http.MethodPost,"/api/v1/items",map[string]any{
		"name": "提醒测试 VPS", "kind": "resource", "status": "active",
		"amount_minor": 100, "currency": "CNY", "period_unit": "none",
		"period_count": 0, "first_due_date": "2027-01-15", "notes": "",
	},http.StatusCreated)
	itemID:=int64(item["id"].(float64))
	rule:=request(t,router,http.MethodPost,"/api/v1/reminder-rules",map[string]any{
		"item_id": itemID, "days_before": 7, "enabled": true,
	},http.StatusCreated)
	ruleID:=int64(rule["id"].(float64))
	request(t,router,http.MethodPost,"/api/v1/reminder-rules",map[string]any{
		"item_id": itemID, "days_before": 7, "enabled": true,
	},http.StatusConflict)
	when:=time.Date(2027,1,10,12,0,0,0,time.UTC)
	n,err:=reminder.RunOnce(context.Background(),repo,time.UTC,when)
	if err!=nil||n!=1 { t.Fatalf("首次生成提醒失败：n=%d err=%v",n,err) }
	n,err=reminder.RunOnce(context.Background(),repo,time.UTC,when)
	if err!=nil||n!=0 { t.Fatalf("重复提醒未去重：n=%d err=%v",n,err) }
	result:=request(t,router,http.MethodGet,"/api/v1/notifications?unread=true",nil,http.StatusOK)
	if result["unread_count"]!=float64(1) {t.Fatalf("未读数量错误：%v",result)}
	notice:=result["items"].([]any)[0].(map[string]any)
	request(t,router,http.MethodPost,"/api/v1/notifications/"+fmtID(int64(notice["id"].(float64)))+"/read",nil,http.StatusNoContent)
	result=request(t,router,http.MethodGet,"/api/v1/notifications?unread=true",nil,http.StatusOK)
	if result["unread_count"]!=float64(0) {t.Fatalf("标记已读失败：%v",result)}
	request(t,router,http.MethodDelete,"/api/v1/reminder-rules/"+fmtID(ruleID),nil,http.StatusNoContent)
	result=request(t,router,http.MethodGet,"/api/v1/notifications",nil,http.StatusOK)
	if len(result["items"].([]any))!=0 {t.Fatalf("规则删除后通知未级联删除：%v",result)}
}


func TestOverdueUnsettledCycleGeneratesOneReminder(t *testing.T) {
	router,repo,_:=testRouter(t)
	now:=time.Now().UTC()
	due:=now.AddDate(0,0,1).Format("2006-01-02")
	item:=request(t,router,http.MethodPost,"/api/v1/items",map[string]any{
		"name":"逾期未续费","kind":"subscription","status":"active",
		"amount_minor":1000,"currency":"CNY","period_unit":"month",
		"period_count":1,"first_due_date":due,
	},http.StatusCreated)
	itemID:=int64(item["id"].(float64))
	request(t,router,http.MethodPost,"/api/v1/reminder-rules",map[string]any{
		"item_id":itemID,"days_before":1,"enabled":true,
	},http.StatusCreated)
	when:=now.AddDate(0,0,4)
	n,err:=reminder.RunOnce(context.Background(),repo,time.UTC,when)
	if err!=nil||n!=1{t.Fatalf("逾期通知未生成: n=%d err=%v",n,err)}
	n,err=reminder.RunOnce(context.Background(),repo,time.UTC,when)
	if err!=nil||n!=0{t.Fatalf("逾期通知重复生成: n=%d err=%v",n,err)}
}
