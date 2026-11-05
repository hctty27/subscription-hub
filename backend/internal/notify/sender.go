package notify

import (
 "bytes"
 "context"
 "crypto/tls"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "mime"
 "net"
 "net/http"
 "net/mail"
 "net/smtp"
 "net/url"
 "os"
 "regexp"
 "strconv"
 "strings"
 "time"

 "github.com/hctty27/subscription-hub/backend/internal/store"
)

type ChannelSender interface {
 Channels() []string
 Send(ctx context.Context,channel string,notification store.NotificationDelivery) error
}

type SMTPConfig struct {
 Host string
 Port int
 User string
 Password string
 From string
 To string
}

type Sender struct {
 SMTP *SMTPConfig
 TelegramToken string
 TelegramChatID string
 BarkDeviceKey string
 telegramBase string
 barkBase string
 client *http.Client
}

var telegramTokenPattern=regexp.MustCompile(`^[0-9]+:[a-zA-Z0-9_-]+$`)

func FromEnv() (*Sender,error) {
 s:=&Sender{
  telegramBase:"https://api.telegram.org",
  barkBase:"https://api.day.app",
  client:&http.Client{Timeout:12*time.Second,CheckRedirect:func(_ *http.Request,_ []*http.Request)error{return http.ErrUseLastResponse}},
 }
 smtpHost:=strings.TrimSpace(os.Getenv("NOTIFY_SMTP_HOST"))
 smtpPort:=strings.TrimSpace(os.Getenv("NOTIFY_SMTP_PORT"))
 smtpUser:=os.Getenv("NOTIFY_SMTP_USER")
 smtpPass:=os.Getenv("NOTIFY_SMTP_PASSWORD")
 smtpFrom:=strings.TrimSpace(os.Getenv("NOTIFY_SMTP_FROM"))
 smtpTo:=strings.TrimSpace(os.Getenv("NOTIFY_SMTP_TO"))
 if smtpHost!=""||smtpPort!=""||smtpUser!=""||smtpPass!=""||smtpFrom!=""||smtpTo!="" {
  port,err:=strconv.Atoi(smtpPort)
  if smtpHost==""||err!=nil||port<1||port>65535||smtpUser==""||smtpPass==""||smtpFrom==""||smtpTo=="" {
   return nil,errors.New("SMTP 通知参数不完整或端口无效")
  }
  if _,err:=mail.ParseAddress(smtpFrom);err!=nil{return nil,errors.New("SMTP 发件人邮箱无效")}
  if _,err:=mail.ParseAddress(smtpTo);err!=nil{return nil,errors.New("SMTP 收件人邮箱无效")}
  if strings.ContainsAny(smtpHost,"\r\n/ @"){return nil,errors.New("SMTP 主机地址格式无效")}
  s.SMTP=&SMTPConfig{Host:smtpHost,Port:port,User:smtpUser,Password:smtpPass,From:smtpFrom,To:smtpTo}
 }
 s.TelegramToken=os.Getenv("NOTIFY_TELEGRAM_BOT_TOKEN")
 s.TelegramChatID=os.Getenv("NOTIFY_TELEGRAM_CHAT_ID")
 if s.TelegramToken!=""||s.TelegramChatID!="" {
  if !telegramTokenPattern.MatchString(s.TelegramToken)||s.TelegramChatID=="" {
   return nil,errors.New("Telegram Bot Token 或 Chat ID 配置无效")
  }
  if _,err:=strconv.ParseInt(s.TelegramChatID,10,64);err!=nil &&
   !regexp.MustCompile(`^@[a-zA-Z0-9_]{5,}$`).MatchString(s.TelegramChatID) {
   return nil,errors.New("Telegram Chat ID 格式无效")
  }
 }
 s.BarkDeviceKey=strings.TrimSpace(os.Getenv("NOTIFY_BARK_DEVICE_KEY"))
 if len(s.BarkDeviceKey)>256||strings.ContainsAny(s.BarkDeviceKey,"\r\n") {
  return nil,errors.New("Bark Device Key 格式无效")
 }
 return s,nil
}

func (s *Sender) Channels() []string {
 channels:=make([]string,0,3)
 if s.SMTP!=nil{channels=append(channels,"smtp")}
 if s.TelegramToken!="" {channels=append(channels,"telegram")}
 if s.BarkDeviceKey!="" {channels=append(channels,"bark")}
 return channels
}

func (s *Sender) Send(ctx context.Context,channel string,notice store.NotificationDelivery)error {
 body:=fmt.Sprintf("%s 将于 %s 到期，请确认是否续费。（到期前 %d 天）",
  notice.ItemName,notice.DueDate,notice.DaysBefore)
 title:="Subscription Hub · 到期提醒"
 switch channel {
 case "smtp":
  if s.SMTP==nil{return errors.New("SMTP 通知未配置")}
  return s.sendSMTP(ctx,title,body)
 case "telegram":
  if s.TelegramToken==""{return errors.New("Telegram 未配置")}
  return s.sendTelegram(ctx,title+"\n"+body)
 case "bark":
  if s.BarkDeviceKey==""{return errors.New("Bark 未配置")}
  return s.sendBark(ctx,title,body)
 default:return errors.New("未知通知渠道")
 }
}

func (s *Sender) postJSON(ctx context.Context,endpoint string,content any)(int,[]byte,error){
 data,err:=json.Marshal(content)
 if err!=nil{return 0,nil,err}
 req,err:=http.NewRequestWithContext(ctx,http.MethodPost,endpoint,bytes.NewReader(data))
 if err!=nil{return 0,nil,errors.New("创建通知请求失败")}
 req.Header.Set("Content-Type","application/json; charset=utf-8")
 resp,err:=s.client.Do(req)
 if err!=nil{return 0,nil,errors.New("通知网络请求失败")}
 defer resp.Body.Close()
 body,err:=io.ReadAll(io.LimitReader(resp.Body,2048))
 if err!=nil{return resp.StatusCode,nil,errors.New("读取通知响应失败")}
 return resp.StatusCode,body,nil
}
func (s *Sender) sendTelegram(ctx context.Context,message string)error{
 endpoint:=strings.TrimSuffix(s.telegramBase,"/")+"/bot"+s.TelegramToken+"/sendMessage"
 code,body,err:=s.postJSON(ctx,endpoint,map[string]any{"chat_id":s.TelegramChatID,"text":message})
 if err!=nil{return fmt.Errorf("Telegram %w",err)}
 if code!=http.StatusOK{return fmt.Errorf("Telegram HTTP 状态码 %d",code)}
 var result struct { Ok bool `json:"ok"` }
 if err:=json.Unmarshal(body,&result);err!=nil||!result.Ok{return errors.New("Telegram API 返回失败")}
 return nil
}
func (s *Sender) sendBark(ctx context.Context,title,body string)error{
 endpoint:=strings.TrimSuffix(s.barkBase,"/")+"/push"
 code,data,err:=s.postJSON(ctx,endpoint,map[string]any{
  "device_key":s.BarkDeviceKey,"title":title,"body":body,"group":"Subscription Hub",
 })
 if err!=nil{return fmt.Errorf("Bark %w",err)}
 if code<200||code>=300{return fmt.Errorf("Bark HTTP 状态码 %d",code)}
 var result struct{Code int `json:"code"`}
 if err:=json.Unmarshal(data,&result);err!=nil||result.Code!=200{return errors.New("Bark API 返回失败")}
 return nil
}

func (s *Sender) sendSMTP(ctx context.Context,title,body string)error{
 cfg:=s.SMTP
 if cfg==nil{return errors.New("SMTP 未配置")}
 from,err:=mail.ParseAddress(cfg.From)
 if err!=nil{return errors.New("SMTP 发件人地址无效")}
 to,err:=mail.ParseAddress(cfg.To)
 if err!=nil{return errors.New("SMTP 收件人地址无效")}
 dialer:=net.Dialer{Timeout:10*time.Second}
 connection,err:=dialer.DialContext(ctx,"tcp",net.JoinHostPort(cfg.Host,strconv.Itoa(cfg.Port)))
 if err!=nil{return errors.New("SMTP 连接失败")}
 defer connection.Close()
 if err:=connection.SetDeadline(time.Now().Add(12*time.Second));err!=nil{return errors.New("SMTP 设置超时失败")}
 if cfg.Port==465 {
  secure:=tls.Client(connection,&tls.Config{ServerName:cfg.Host,MinVersion:tls.VersionTLS12})
  if err:=secure.HandshakeContext(ctx);err!=nil{return errors.New("SMTP TLS 握手失败")}
  connection=secure
 }
 client,err:=smtp.NewClient(connection,cfg.Host)
 if err!=nil{return errors.New("SMTP 协议握手失败")}
 defer client.Close()
 if cfg.Port!=465 {
  if ok,_:=client.Extension("STARTTLS");!ok{return errors.New("SMTP 服务器不支持 STARTTLS")}
  if err:=client.StartTLS(&tls.Config{ServerName:cfg.Host,MinVersion:tls.VersionTLS12});err!=nil{
   return errors.New("SMTP STARTTLS 失败")
  }
 }
 auth:=smtp.PlainAuth("",cfg.User,cfg.Password,cfg.Host)
 if err:=client.Auth(auth);err!=nil{return errors.New("SMTP 身份认证失败")}
 if err:=client.Mail(from.Address);err!=nil{return errors.New("SMTP 发件人被拒绝")}
 if err:=client.Rcpt(to.Address);err!=nil{return errors.New("SMTP 收件人被拒绝")}
 writer,err:=client.Data()
 if err!=nil{return errors.New("SMTP 正文写入失败")}
 subject:=mime.QEncoding.Encode("utf-8",title)
 message:=fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
  from.Address,to.Address,subject,strings.ReplaceAll(body,"\n","\r\n"))
 if _,err:=io.WriteString(writer,message);err!=nil{writer.Close();return errors.New("SMTP 邮件发送失败")}
 if err:=writer.Close();err!=nil{return errors.New("SMTP 邮件提交失败")}
 return nil
}

// Validate fixed endpoints to avoid redirecting secrets if the sender gains external configuration.
func validateHTTPS(endpoint string) bool{
 parsed,err:=url.Parse(endpoint)
 return err==nil&&parsed.Scheme=="https"&&parsed.Hostname()!=""
}
