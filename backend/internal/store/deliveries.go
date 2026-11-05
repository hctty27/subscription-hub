package store

import (
 "context"
 "database/sql"
 "fmt"
 "strings"
 "time"
)

type NotificationDelivery struct {
 ID int64 `json:"id"`
 NotificationID int64 `json:"notification_id"`
 ItemName string `json:"item_name"`
 DueDate string `json:"due_date"`
 DaysBefore int `json:"days_before"`
 Channel string `json:"channel"`
 Status string `json:"status"`
 Attempts int `json:"attempts"`
 NextAttemptAt int64 `json:"next_attempt_at"`
 LastError string `json:"last_error"`
 SentAt *string `json:"sent_at"`
 CreatedAt string `json:"created_at"`
}

const deliverySelect = `SELECT d.id,d.notification_id,i.name,n.due_date,n.days_before,d.channel,
 d.status,d.attempts,d.next_attempt_at,d.last_error,d.sent_at,d.created_at
 FROM notification_deliveries d JOIN reminder_notifications n ON n.id=d.notification_id
 JOIN items i ON i.id=n.item_id`

func scanDelivery(row scanner)(NotificationDelivery,error) {
 var d NotificationDelivery
 var sentAt sql.NullString
 err:=row.Scan(&d.ID,&d.NotificationID,&d.ItemName,&d.DueDate,&d.DaysBefore,&d.Channel,
  &d.Status,&d.Attempts,&d.NextAttemptAt,&d.LastError,&sentAt,&d.CreatedAt)
 if sentAt.Valid {d.SentAt=&sentAt.String}
 return d,err
}

// EnqueueRecentDeliveries never recreates a delivery for the same notice+channel.
func (s *Store) EnqueueRecentDeliveries(ctx context.Context, channels []string, now time.Time, today string) (int,error) {
 total:=0
 cutoff:=now.UTC().Add(-48*time.Hour).Format("2006-01-02T15:04:05")
 for _,channel:=range channels{
  if channel!="smtp"&&channel!="telegram"&&channel!="bark"{return total,fmt.Errorf("不支持的推送渠道")}
  result,err:=s.db.ExecContext(ctx,`INSERT INTO notification_deliveries(notification_id,channel,next_attempt_at)
   SELECT n.id,?,? FROM reminder_notifications n
   WHERE n.created_at>=? AND n.due_date>=? AND n.read_at IS NULL
   ON CONFLICT(notification_id,channel) DO NOTHING`,
   channel,now.Unix(),cutoff,today)
  if err!=nil{return total,err}
  count,err:=result.RowsAffected()
  if err!=nil{return total,err}
  total+=int(count)
 }
 return total,nil
}

func (s *Store) DueDeliveries(ctx context.Context, now time.Time, limit int)([]NotificationDelivery,error){
 if limit<1||limit>100 {limit=20}
 rows,err:=s.db.QueryContext(ctx,deliverySelect+
  " WHERE d.status IN ('pending','retry') AND d.next_attempt_at<=? ORDER BY d.id LIMIT ?",now.Unix(),limit)
 if err!=nil{return nil,err}
 defer rows.Close()
 out:=make([]NotificationDelivery,0)
 for rows.Next(){
  d,err:=scanDelivery(rows)
  if err!=nil{return nil,err}
  out=append(out,d)
 }
 return out,rows.Err()
}

func (s *Store) FinishDelivery(ctx context.Context,id int64,success bool,attempt int,errMessage string,nextAttempt time.Time)error{
 if len(errMessage)>150 {errMessage=errMessage[:150]}
 if strings.Contains(errMessage,"token="){errMessage="推送请求失败"}
 status:="retry"
 if success {status="sent";errMessage=""}
 if !success&&attempt>=4 {status="failed"}
 var sentAt any
 if success {sentAt=time.Now().UTC().Format(time.RFC3339)}
 _,err:=s.db.ExecContext(ctx,`UPDATE notification_deliveries
  SET status=?,attempts=?,next_attempt_at=?,last_error=?,sent_at=? WHERE id=?`,
  status,attempt,nextAttempt.Unix(),errMessage,sentAt,id)
 return err
}

func (s *Store) ListDeliveryLogs(ctx context.Context,limit int)([]NotificationDelivery,error){
 if limit<1||limit>500{limit=100}
 rows,err:=s.db.QueryContext(ctx,deliverySelect+" ORDER BY d.id DESC LIMIT ?",limit)
 if err!=nil{return nil,err}
 defer rows.Close()
 result:=make([]NotificationDelivery,0)
 for rows.Next(){d,err:=scanDelivery(rows);if err!=nil{return nil,err};result=append(result,d)}
 return result,rows.Err()
}
