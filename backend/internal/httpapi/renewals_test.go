package httpapi

import (
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestRenewalEndpointSettlesPaymentOnce(t *testing.T) {
	router, _, _ := testRouter(t)
	today := time.Now().UTC()
	first := today.AddDate(0, 0, 5).Format("2006-01-02")
	item := request(t, router, http.MethodPost, "/api/v1/items", map[string]any{
		"name": "周期结算测试", "kind": "subscription", "status": "active",
		"amount_minor": 2000, "currency": "USD",
		"period_unit": "month", "period_count": 1,
		"first_due_date": first, "notes": "",
	}, http.StatusCreated)
	itemID := int64(item["id"].(float64))
	due := item["next_due_date"].(string)
	if due != first { t.Fatalf("首次待处理账期不正确: %v",item) }

	path := "/api/v1/items/"+strconv.FormatInt(itemID,10)
	body := map[string]any{
		"due_date": due, "amount_minor": 1950,
		"currency": "USD", "paid_date": today.Format("2006-01-02"), "notes": "续期付款",
	}
	created := request(t,router,http.MethodPost,path+"/renew",body,http.StatusCreated)
	if created["due_date"] != due || created["payment_id"] == nil {
		t.Fatalf("结算结果不正确: %+v",created)
	}
	after := request(t,router,http.MethodGet,path,nil,http.StatusOK)
	next := after["next_due_date"].(string)
	if next <= due || after["renewal_completed"] != false {
		t.Fatalf("新账期未生效: %+v",after)
	}
	request(t,router,http.MethodPost,path+"/renew",body,http.StatusConflict)

	history := request(t,router,http.MethodGet,path+"/renewals",nil,http.StatusOK)
	if len(history["items"].([]any)) != 1 {
		t.Fatalf("未记录结算历史: %+v",history)
	}
	paymentID:=int64(created["payment_id"].(float64))
	paymentPath:="/api/v1/payments/"+strconv.FormatInt(paymentID,10)
	request(t,router,http.MethodPut,paymentPath,map[string]any{
		"item_id":itemID,"amount_minor":1950,"currency":"USD",
		"paid_date":today.Format("2006-01-02"),"notes":"修改",
	},http.StatusConflict)
	request(t,router,http.MethodDelete,paymentPath,nil,http.StatusConflict)

	month := today.Format("2006-01")
	report := request(t,router,http.MethodGet,"/api/v1/reports/expenses?month="+month,nil,http.StatusOK)
	if len(report["totals"].([]any))!=1 || report["totals"].([]any)[0].(map[string]any)["amount_minor"]!=float64(1950){
		t.Fatalf("续费未计入实际付款报表: %+v",report)
	}

	// A generic payment must not settle a renewal cycle.
	request(t,router,http.MethodPost,"/api/v1/payments",map[string]any{
		"item_id":itemID,"amount_minor":125,"currency":"USD",
		"paid_date":today.Format("2006-01-02"),"notes":"额外费用",
	},http.StatusCreated)
	got:=request(t,router,http.MethodGet,path,nil,http.StatusOK)
	if got["next_due_date"] != next {
		t.Fatalf("普通付款错误推进账期: %+v",got)
	}
	// Undo restores the pending cycle without erasing the actual payment.
	renewalID:=int64(created["id"].(float64))
	request(t,router,http.MethodDelete,path+"/renewals/"+strconv.FormatInt(renewalID,10),nil,http.StatusNoContent)
	got=request(t,router,http.MethodGet,path,nil,http.StatusOK)
	if got["next_due_date"]!=due {t.Fatalf("撤销后到期日未恢复: %+v",got)}
	request(t,router,http.MethodDelete,paymentPath,nil,http.StatusNoContent)
	history=request(t,router,http.MethodGet,path+"/renewals",nil,http.StatusOK)
	if len(history["items"].([]any))!=0 {t.Fatalf("撤销后历史关联仍存在: %+v",history)}
}

func TestSettlingCycleMarksItsReminderRead(t *testing.T){
	router,_,_:=testRouter(t)
	today:=time.Now().UTC()
	due:=today.AddDate(0,0,2).Format("2006-01-02")
	item:=request(t,router,http.MethodPost,"/api/v1/items",map[string]any{
		"name":"提醒结算联动","kind":"subscription","status":"active",
		"amount_minor":1000,"currency":"CNY","period_unit":"month",
		"period_count":1,"first_due_date":due,
	},http.StatusCreated)
	itemID:=int64(item["id"].(float64))
	request(t,router,http.MethodPost,"/api/v1/reminder-rules",map[string]any{
		"item_id":itemID,"days_before":7,"enabled":true,
	},http.StatusCreated)
	request(t,router,http.MethodPost,"/api/v1/notifications/scan",map[string]any{},http.StatusOK)
	before:=request(t,router,http.MethodGet,"/api/v1/notifications?unread=true",nil,http.StatusOK)
	if before["unread_count"].(float64)!=1 {t.Fatalf("应生成未读提醒: %+v",before)}
	path:="/api/v1/items/"+strconv.FormatInt(itemID,10)+"/renew"
	request(t,router,http.MethodPost,path,map[string]any{
		"due_date":due,"amount_minor":1000,"currency":"CNY",
		"paid_date":today.Format("2006-01-02"),
	},http.StatusCreated)
	after:=request(t,router,http.MethodGet,"/api/v1/notifications?unread=true",nil,http.StatusOK)
	if after["unread_count"].(float64)!=0 {t.Fatalf("已结算账期的提醒未清除: %+v",after)}
}
