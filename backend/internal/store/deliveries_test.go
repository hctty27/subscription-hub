package store

import (
 "context"
 "path/filepath"
 "testing"
 "time"
)

func TestDeliveryLogCascadeAfterRuleDeletion(t *testing.T) {
 repo,err:=Open(filepath.Join(t.TempDir(),"queue.db"))
 if err!=nil{t.Fatal(err)}
 defer repo.Close()
 ctx:=context.Background()
 due:=time.Now().UTC().AddDate(0,0,1).Format("2006-01-02")
 item,err:=repo.CreateItem(ctx,Item{Name:"测试订阅",Kind:"subscription",Status:"active",Currency:"CNY",AmountMinor:500,
  PeriodUnit:"month",PeriodCount:1,FirstDueDate:due})
 if err!=nil{t.Fatal(err)}
 rule,err:=repo.CreateReminderRule(ctx,ReminderRule{ItemID:item.ID,DaysBefore:5,Enabled:true})
 if err!=nil{t.Fatal(err)}
 if _,err:=repo.InsertReminderNotification(ctx,rule,due);err!=nil{t.Fatal(err)}
 n,err:=repo.EnqueueRecentDeliveries(ctx,[]string{"smtp","bark"},time.Now().UTC(),time.Now().UTC().Format("2006-01-02"))
 if err!=nil||n!=2{t.Fatalf("应创建两条通道投递任务，实际 %d，错误 %v",n,err)}
 n,err=repo.EnqueueRecentDeliveries(ctx,[]string{"smtp","bark"},time.Now().UTC(),time.Now().UTC().Format("2006-01-02"))
 if err!=nil||n!=0{t.Fatalf("相同通知和渠道不得重复建任务，实际 %d，错误 %v",n,err)}
 if err:=repo.DeleteReminderRule(ctx,rule.ID);err!=nil{t.Fatal(err)}
 entries,err:=repo.ListDeliveryLogs(ctx,100)
 if err!=nil||len(entries)!=0{t.Fatalf("删除规则后队列未清理：%v err=%v",entries,err)}
}
