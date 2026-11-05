package notify

import (
 "context"
 "fmt"
 "log"
 "time"

 "github.com/hctty27/subscription-hub/backend/internal/store"
)

type Dispatcher struct {
 Repo *store.Store
 Sender ChannelSender
 Location *time.Location
}

func retryDelay(attempt int)time.Duration {
 switch attempt {
 case 1:return time.Minute
 case 2:return 5*time.Minute
 case 3:return 15*time.Minute
 default:return time.Hour
 }
}

// Tick executes a bounded delivery batch. Rows are retained for retry after restart.
// Deliveries have at-least-once semantics if the process stops after a send succeeds
// but before it can persist the sent status.
func (d *Dispatcher) Tick(ctx context.Context,now time.Time)(int,error){
 if d==nil||d.Sender==nil||len(d.Sender.Channels())==0{return 0,nil}
 loc:=d.Location
 if loc==nil{loc=time.UTC}
 channels:=d.Sender.Channels()
 if _,err:=d.Repo.EnqueueRecentDeliveries(ctx,channels,now,now.In(loc).Format("2006-01-02"));err!=nil{return 0,err}
 pending,err:=d.Repo.DueDeliveries(ctx,now,20)
 if err!=nil{return 0,err}
 allowed:=make(map[string]bool,len(channels))
 for _,name:=range channels {allowed[name]=true}
 processed:=0
 for _,entry:=range pending {
  if !allowed[entry.Channel] {continue}
  if err:=ctx.Err();err!=nil{return processed,err}
  sendCtx,cancel:=context.WithTimeout(ctx,15*time.Second)
  sendErr:=d.Sender.Send(sendCtx,entry.Channel,entry)
  cancel()
  attempt:=entry.Attempts+1
  reason:=""
  if sendErr!=nil{reason=sendErr.Error()}
  if err:=d.Repo.FinishDelivery(ctx,entry.ID,sendErr==nil,attempt,reason,now.Add(retryDelay(attempt)));err!=nil{return processed,err}
  if sendErr!=nil{log.Printf("到期提醒发送失败：渠道=%s，任务=%d，次数=%d，错误=%s",entry.Channel,entry.ID,attempt,reason)}
  processed++
 }
 return processed,nil
}
func (d *Dispatcher) Configured()[]string{
 if d==nil||d.Sender==nil{return []string{}}
 return d.Sender.Channels()
}

func DeliverySummary(channels []string)string{
 if len(channels)==0{return "未配置外部通知渠道"}
 return fmt.Sprintf("已启用 %d 个外部通知渠道",len(channels))
}
