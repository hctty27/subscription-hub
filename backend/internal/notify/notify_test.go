package notify

import (
 "context"
 "fmt"
 "path/filepath"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"

 "github.com/hctty27/subscription-hub/backend/internal/store"
)

func TestBarkAndTelegramAdapters(t *testing.T){
 countBark,countTelegram:=0,0
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.Method!=http.MethodPost{t.Errorf("应使用 POST");w.WriteHeader(405);return}
  if strings.HasSuffix(r.URL.Path,"/push"){
   countBark++
   var data map[string]any
   if err:=json.NewDecoder(r.Body).Decode(&data);err!=nil{t.Error(err)}
   if data["device_key"]!="test-bark-key"{t.Errorf("Bark Key 应在请求体中")}
   w.Header().Set("Content-Type","application/json")
   w.Write([]byte(`{"code":200,"message":"success"}`))
   return
  }
  if strings.HasSuffix(r.URL.Path,"/sendMessage"){
   countTelegram++
   var data map[string]any
   if err:=json.NewDecoder(r.Body).Decode(&data);err!=nil{t.Error(err)}
   if data["chat_id"]!="12345"{t.Errorf("Telegram Chat ID 缺失")}
   w.Header().Set("Content-Type","application/json")
   w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
   return
  }
  t.Errorf("非预期请求：%s",r.URL.Path)
  w.WriteHeader(404)
 }))
 defer server.Close()
 sender:=&Sender{TelegramToken:"123:token",TelegramChatID:"12345",BarkDeviceKey:"test-bark-key",
  telegramBase:server.URL,barkBase:server.URL,client:server.Client()}
 item:=store.NotificationDelivery{ItemName:"测试 VPS",DueDate:"2026-12-01",DaysBefore:7}
 if err:=sender.Send(context.Background(),"bark",item);err!=nil{t.Fatal(err)}
 if err:=sender.Send(context.Background(),"telegram",item);err!=nil{t.Fatal(err)}
 if countBark!=1||countTelegram!=1{t.Fatalf("外部发送次数不正确：bark=%d telegram=%d",countBark,countTelegram)}
}

func TestNotificationConfigurationValidation(t *testing.T){
 t.Setenv("NOTIFY_SMTP_HOST","smtp.example.com")
 if _,err:=FromEnv();err==nil{t.Fatal("SMTP 配置不完整应拒绝启动")}
 t.Setenv("NOTIFY_SMTP_HOST","")
 t.Setenv("NOTIFY_TELEGRAM_BOT_TOKEN","invalid")
 if _,err:=FromEnv();err==nil{t.Fatal("非法 Telegram Token 应拒绝启动")}
 t.Setenv("NOTIFY_TELEGRAM_BOT_TOKEN","")
 t.Setenv("NOTIFY_BARK_DEVICE_KEY","test-device-key")
 sender,err:=FromEnv()
 if err!=nil{t.Fatal(err)}
 if len(sender.Channels())!=1||sender.Channels()[0]!="bark"{t.Fatalf("启用渠道错误：%v",sender.Channels())}
}

type flakySender struct {failures int;calls int}
func (f *flakySender) Channels()[]string{return []string{"bark"}}
func (f *flakySender) Send(_ context.Context,_ string,_ store.NotificationDelivery)error{
 f.calls++
 if f.calls<=f.failures{return fmt.Errorf("模拟外部服务不可用")}
 return nil
}

func TestDispatcherRetriesAndDedupes(t *testing.T){
 repo,err:=store.Open(filepath.Join(t.TempDir(),"delivery.db"))
 if err!=nil{t.Fatal(err)}
 defer repo.Close()
 now:=time.Now().UTC()
 due:=now.AddDate(0,0,1).Format("2006-01-02")
 item,err:=repo.CreateItem(context.Background(),store.Item{
  Name:"账单测试",Kind:"subscription",Status:"active",AmountMinor:1000,Currency:"CNY",
  PeriodUnit:"month",PeriodCount:1,FirstDueDate:due,
 })
 if err!=nil{t.Fatal(err)}
 rule,err:=repo.CreateReminderRule(context.Background(),store.ReminderRule{ItemID:item.ID,DaysBefore:1,Enabled:true})
 if err!=nil{t.Fatal(err)}
 ok,err:=repo.InsertReminderNotification(context.Background(),rule,due)
 if err!=nil||!ok{t.Fatalf("生成通知失败：%v",err)}
 sender:=&flakySender{failures:2}
 dispatcher:=&Dispatcher{Repo:repo,Sender:sender,Location:time.UTC}
 testTick:=func(at time.Time,expected int){
  t.Helper()
  processed,err:=dispatcher.Tick(context.Background(),at)
  if err!=nil||processed!=expected{t.Fatalf("处理数量 %d，预期 %d，错误 %v",processed,expected,err)}
 }
 testTick(now,1)
 testTick(now.Add(30*time.Second),0)
 testTick(now.Add(2*time.Minute),1)
 testTick(now.Add(8*time.Minute),1)
 testTick(now.Add(2*time.Hour),0)
 logs,err:=repo.ListDeliveryLogs(context.Background(),10)
 if err!=nil||len(logs)!=1{t.Fatalf("发送任务重复：%v %v",logs,err)}
 if logs[0].Attempts!=3||logs[0].Status!="sent"||sender.calls!=3{
  t.Fatalf("投递状态错误：%+v，次数=%d",logs[0],sender.calls)
 }
}
