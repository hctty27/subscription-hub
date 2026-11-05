package httpapi

import (
	"strconv"
	"net/http"
	"testing"
)

func TestPaymentsAndExpenseReport(t *testing.T) {
	router, _, _ := testRouter(t)
	item := request(t, router, http.MethodPost, "/api/v1/items", map[string]any{
		"name": "支付测试服务", "kind": "subscription", "status": "active",
		"amount_minor": 2000, "currency": "USD", "period_unit": "month",
		"period_count": 1, "first_due_date": "2025-01-31", "notes": "",
	}, http.StatusCreated)
	itemID := int64(item["id"].(float64))
	payment := map[string]any{
		"item_id": itemID, "amount_minor": 1999, "currency": "USD",
		"paid_date": "2025-04-10", "notes": "历史实付",
	}
	created := request(t,router,http.MethodPost,"/api/v1/payments",payment,http.StatusCreated)
	paymentID := int64(created["id"].(float64))
	if created["item_name"] != "支付测试服务" { t.Fatalf("付款项名称丢失：%v",created) }
	list:=request(t,router,http.MethodGet,"/api/v1/payments?month=2025-04",nil,http.StatusOK)
	if len(list["items"].([]any))!=1 { t.Fatalf("付款查询失败：%v",list) }
	report:=request(t,router,http.MethodGet,"/api/v1/reports/expenses?month=2025-04",nil,http.StatusOK)
	totals:=report["totals"].([]any)
	if len(totals)!=1 || totals[0].(map[string]any)["amount_minor"]!=float64(1999) {
		t.Fatalf("实付统计错误：%v",report)
	}
	// Payment is not forecast: even though the configured monthly price is 2000,
	// the report must contain exactly the actual amount 1999.
	itemPath:="/api/v1/items/"+fmtID(itemID)
	request(t,router,http.MethodDelete,itemPath,nil,http.StatusConflict)
	payment["amount_minor"]=3000
	paymentPath:="/api/v1/payments/"+fmtID(paymentID)
	request(t,router,http.MethodPut,paymentPath,payment,http.StatusOK)
	report=request(t,router,http.MethodGet,"/api/v1/reports/expenses?month=2025-04",nil,http.StatusOK)
	if report["totals"].([]any)[0].(map[string]any)["amount_minor"]!=float64(3000){
		t.Fatalf("修改付款后报表未更新：%v",report)
	}
	request(t,router,http.MethodGet,"/api/v1/reports/expenses?month=2025-13",nil,http.StatusBadRequest)
	payment["paid_date"]="2099-01-01"
	request(t,router,http.MethodPost,"/api/v1/payments",payment,http.StatusBadRequest)
	request(t,router,http.MethodDelete,paymentPath,nil,http.StatusNoContent)
	request(t,router,http.MethodDelete,itemPath,nil,http.StatusNoContent)
}

func fmtID(id int64) string { return strconv.FormatInt(id,10) }
