package reminder

import (
	"context"
	"log"
	"time"

	"github.com/hctty27/subscription-hub/backend/internal/billing"
	"github.com/hctty27/subscription-hub/backend/internal/notify"
	"github.com/hctty27/subscription-hub/backend/internal/store"
)

// RunOnce records at most one in-app notification per reminder rule and due date.
// A notification is created on the first scan within the configured lead window.
func RunOnce(ctx context.Context,repo *store.Store,loc *time.Location,now time.Time)(int,error){
	if loc==nil{loc=time.UTC}
	today:=now.In(loc)
	rules,err:=repo.ListActiveReminderRules(ctx)
	if err!=nil{return 0,err}
	created:=0
	for _,rule:=range rules{
		if err:=ctx.Err();err!=nil{return created,err}
		item,err:=repo.GetItem(ctx,rule.ItemID)
		if err!=nil{return created,err}
		due,err:=billing.NextDue(item.FirstDueDate,item.PeriodUnit,item.PeriodCount,today)
		if err!=nil{return created,err}
		days,err:=billing.DaysUntil(due,today)
		if err!=nil{return created,err}
		if days<0||days>rule.DaysBefore{continue}
		ok,err:=repo.InsertReminderNotification(ctx,rule,due)
		if err!=nil{return created,err}
		if ok{created++}
	}
	return created,nil
}

// Start executes on boot and subsequently once an hour. Multiple scans are
// harmless because of the unique (rule_id,due_date) database constraint.
func Start(ctx context.Context,repo *store.Store,loc *time.Location,dispatcher *notify.Dispatcher){
	run:=func(){
		if n,err:=RunOnce(ctx,repo,loc,time.Now());err!=nil{
			if ctx.Err()==nil{log.Printf("生成站内到期提醒失败：%v",err)}
		}else if n>0{log.Printf("已生成 %d 条站内到期提醒",n)}
		if dispatcher!=nil {
			if n,err:=dispatcher.Tick(ctx,time.Now());err!=nil{
				if ctx.Err()==nil{log.Printf("外部通知处理失败：%v",err)}
			}else if n>0{log.Printf("已处理 %d 条外部通知任务",n)}
		}
	}
	run()
	ticker:=time.NewTicker(time.Minute)
	defer ticker.Stop()
	for{
		select{
		case <-ctx.Done():return
		case <-ticker.C:run()
		}
	}
}
