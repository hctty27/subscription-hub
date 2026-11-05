package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestRenewalSettlementRetainsOriginalMonthlyAnchor(t *testing.T) {
	repo, err := Open(filepath.Join(t.TempDir(), "renewals.sqlite"))
	if err != nil { t.Fatal(err) }
	defer repo.Close()
	ctx := context.Background()
	item, err := repo.CreateItem(ctx, Item{
		Name: "月末订阅", Kind: "subscription", Status: "active",
		AmountMinor: 2000, Currency: "USD", PeriodUnit: "month",
		PeriodCount: 1, FirstDueDate: "2026-01-31",
	})
	if err != nil { t.Fatal(err) }
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	due, completed, err := repo.CurrentRenewal(ctx, item, now)
	if err != nil || completed || due != "2026-02-28" {
		t.Fatalf("初始化账期错误 due=%s completed=%v err=%v",due,completed,err)
	}
	overdue := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	due, _, err = repo.CurrentRenewal(ctx, item, overdue)
	if err != nil || due != "2026-02-28" {
		t.Fatalf("未支付账期应保留，得到 %s err=%v",due,err)
	}
	p := Payment{ItemID:item.ID, AmountMinor:1999, Currency:"USD", PaidDate:"2026-03-05"}
	renewal, err := repo.SettleRenewal(ctx,p,"2026-02-28",overdue)
	if err != nil { t.Fatal(err) }
	if renewal.NextDueDate != "2026-03-31" {
		t.Fatalf("月末原始锚点丢失：%s",renewal.NextDueDate)
	}
	due, _, err = repo.CurrentRenewal(ctx,item,overdue)
	if err != nil || due != "2026-03-31" { t.Fatalf("续期后的到期日错误 %s, %v",due,err) }
	_, err = repo.SettleRenewal(ctx,p,"2026-02-28",overdue)
	if !errors.Is(err,ErrConflict) { t.Fatalf("重复结算没有拒绝: %v",err) }
	history, err := repo.ListRenewals(ctx,item.ID,10)
	if err != nil || len(history) != 1 || history[0].PaymentID != renewal.PaymentID {
		t.Fatalf("结算历史不一致：%+v err=%v",history,err)
	}
	report,err:=repo.ExpenseReport(ctx,"2026-03")
	if err!=nil || len(report.Totals)!=1 || report.Totals[0].AmountMinor!=1999 {
		t.Fatalf("实际费用报表未包含续期付款: %+v %v",report,err)
	}
	if err:=repo.DeletePayment(ctx,renewal.PaymentID);!errors.Is(err,ErrConflict){
		t.Fatalf("已关联账期的付款不应被直接删除: %v",err)
	}
}

func TestOneTimeResourceCanBeSettledOnce(t *testing.T) {
	repo,err:=Open(filepath.Join(t.TempDir(),"one-time.sqlite"))
	if err!=nil{t.Fatal(err)}
	defer repo.Close()
	ctx:=context.Background()
	item,err:=repo.CreateItem(ctx,Item{Name:"一次性域名资源",Kind:"resource",Status:"active",
		AmountMinor:100,Currency:"CNY",PeriodUnit:"none",PeriodCount:0,FirstDueDate:"2026-01-01"})
	if err!=nil{t.Fatal(err)}
	now:=time.Date(2026,11,5,0,0,0,0,time.UTC)
	due,completed,err:=repo.CurrentRenewal(ctx,item,now)
	if err!=nil||completed||due!="2026-01-01"{t.Fatalf("一次性日期错误: %s %v %v",due,completed,err)}
	result,err:=repo.SettleRenewal(ctx,Payment{ItemID:item.ID,AmountMinor:100,Currency:"CNY",PaidDate:"2026-11-05"},"2026-01-01",now)
	if err!=nil||result.NextDueDate!=""{t.Fatalf("一次性资源结算失败: %+v %v",result,err)}
	_,completed,err=repo.CurrentRenewal(ctx,item,now)
	if err!=nil||!completed{t.Fatalf("一次性资源未结束: %v %v",completed,err)}
}

func TestScheduleEditResetsCurrentDueButKeepsRenewalHistory(t *testing.T) {
	repo,err:=Open(filepath.Join(t.TempDir(),"schedule-edit.sqlite"))
	if err!=nil{t.Fatal(err)}
	defer repo.Close()
	ctx:=context.Background()
	item,err:=repo.CreateItem(ctx,Item{Name:"月订阅",Kind:"subscription",Status:"active",
		AmountMinor:500,Currency:"CNY",PeriodUnit:"month",PeriodCount:1,FirstDueDate:"2026-01-31"})
	if err!=nil{t.Fatal(err)}
	now:=time.Date(2026,2,1,0,0,0,0,time.UTC)
	due,_,err:=repo.CurrentRenewal(ctx,item,now)
	if err!=nil{t.Fatal(err)}
	_,err=repo.SettleRenewal(ctx,Payment{ItemID:item.ID,AmountMinor:500,Currency:"CNY",PaidDate:"2026-02-01"},due,now)
	if err!=nil{t.Fatal(err)}
	item.Name="修改名称不改变账期"
	item,err=repo.UpdateItem(ctx,item)
	if err!=nil{t.Fatal(err)}
	due,_,err=repo.CurrentRenewal(ctx,item,now)
	if err!=nil||due!="2026-03-31"{t.Fatalf("普通编辑应保留账期 %s %v",due,err)}
	item.FirstDueDate="2026-02-12"
	item,err=repo.UpdateItem(ctx,item)
	if err!=nil{t.Fatal(err)}
	due,_,err=repo.CurrentRenewal(ctx,item,now)
	if err!=nil||due!="2026-02-12"{t.Fatalf("重设起始日应重建账期: %s %v",due,err)}
	history,err:=repo.ListRenewals(ctx,item.ID,10)
	if err!=nil||len(history)!=1{t.Fatalf("改账期不能删除付款记录: %+v %v",history,err)}
}
