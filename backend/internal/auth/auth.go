package auth

import (
 "context"
 "crypto/rand"
 "crypto/sha256"
 "crypto/subtle"
 "encoding/hex"
 "errors"
 "fmt"
 "net/http"
 "net/url"
 "os"
 "strings"
 "sync"
 "time"

 "github.com/gin-gonic/gin"
 "github.com/hctty27/subscription-hub/backend/internal/store"
 "golang.org/x/crypto/argon2"
)

const CookieName = "subscription_hub_session"
const sessionTTL = 7 * 24 * time.Hour
var ErrBootstrapPassword = errors.New("首次启动必须通过 ADMIN_PASSWORD 设置 12–128 字符的管理员密码")

type attempt struct { Count int; Start time.Time }
type Service struct {
 repo *store.Store
 salt []byte
 hash []byte
 secure bool
 limiterMu sync.Mutex
 attempts map[string]attempt
}

func New(ctx context.Context, repo *store.Store, bootstrapPassword string, secureCookie bool) (*Service,error) {
 salt,hash,found,err:=repo.ReadAdminCredential(ctx)
 if err!=nil{return nil,err}
 if !found {
  if len([]rune(bootstrapPassword))<12 || len([]byte(bootstrapPassword))>128 {return nil,ErrBootstrapPassword}
  salt=make([]byte,16)
  if _,err:=rand.Read(salt);err!=nil{return nil,err}
  hash=argon2.IDKey([]byte(bootstrapPassword),salt,2,19456,1,32)
  created,err:=repo.InstallAdminCredential(ctx,salt,hash)
  if err!=nil{return nil,err}
  if !created{
   salt,hash,found,err=repo.ReadAdminCredential(ctx)
   if err!=nil||!found{return nil,fmt.Errorf("读取管理员密码失败：%w",err)}
  }
 }
 if err:=repo.PurgeExpiredSessions(ctx);err!=nil{return nil,err}
 return &Service{repo:repo,salt:salt,hash:hash,secure:secureCookie,attempts:make(map[string]attempt)},nil
}

func tokenHash(token string) string {
 h:=sha256.Sum256([]byte(token))
 return hex.EncodeToString(h[:])
}

func (s *Service) limited(ip string) bool {
 s.limiterMu.Lock()
 defer s.limiterMu.Unlock()
 now:=time.Now()
 entry:=s.attempts[ip]
 if now.Sub(entry.Start)>15*time.Minute {delete(s.attempts,ip);return false}
 return entry.Count>=5
}
func (s *Service) failure(ip string) {
 s.limiterMu.Lock()
 defer s.limiterMu.Unlock()
 now:=time.Now()
 if len(s.attempts)>2000{
  for k,v:=range s.attempts {if now.Sub(v.Start)>15*time.Minute {delete(s.attempts,k)}}
 }
 if len(s.attempts)>4000 {s.attempts=make(map[string]attempt)}
 a:=s.attempts[ip]
 if now.Sub(a.Start)>15*time.Minute {a=attempt{Start:now}}
 a.Count++
 s.attempts[ip]=a
}
func (s *Service) clearFailure(ip string) {
 s.limiterMu.Lock()
 defer s.limiterMu.Unlock()
 delete(s.attempts,ip)
}

func (s *Service) Login(c *gin.Context) {
 if !allowedMutation(c) {c.JSON(http.StatusForbidden,gin.H{"error":gin.H{"message":"请求安全校验失败"}});return}
 if s.limited(c.ClientIP()) {c.JSON(http.StatusTooManyRequests,gin.H{"error":gin.H{"message":"登录失败次数过多，请 15 分钟后重试"}});return}
 c.Request.Body=http.MaxBytesReader(c.Writer,c.Request.Body,4096)
 var req struct { Password string `json:"password"` }
 if err:=c.ShouldBindJSON(&req);err!=nil||len(req.Password)>128{
  c.JSON(http.StatusBadRequest,gin.H{"error":gin.H{"message":"密码格式不正确"}});return
 }
 computed:=argon2.IDKey([]byte(req.Password),s.salt,2,19456,1,32)
 if subtle.ConstantTimeCompare(computed,s.hash)!=1 {
  s.failure(c.ClientIP())
  c.JSON(http.StatusUnauthorized,gin.H{"error":gin.H{"message":"密码错误"}})
  return
 }
 s.clearFailure(c.ClientIP())
 raw:=make([]byte,32)
 if _,err:=rand.Read(raw);err!=nil{c.Status(http.StatusInternalServerError);return}
 token:=hex.EncodeToString(raw)
 if err:=s.repo.SaveAdminSession(c.Request.Context(),tokenHash(token),time.Now().Add(sessionTTL));err!=nil{
  c.JSON(http.StatusInternalServerError,gin.H{"error":gin.H{"message":"创建登录会话失败"}});return
 }
 http.SetCookie(c.Writer,&http.Cookie{
  Name:CookieName,Value:token,Path:"/api/v1",HttpOnly:true,SameSite:http.SameSiteStrictMode,
  Secure:s.secure||c.Request.TLS!=nil,MaxAge:int(sessionTTL.Seconds()),
 })
 c.JSON(http.StatusOK,gin.H{"authenticated":true,"username":"admin"})
}

func (s *Service) sessionValid(c *gin.Context) bool {
 cookie,err:=c.Request.Cookie(CookieName)
 if err!=nil||len(cookie.Value)!=64 {return false}
 _,err=hex.DecodeString(cookie.Value)
 if err!=nil{return false}
 ok,err:=s.repo.ValidAdminSession(c.Request.Context(),tokenHash(cookie.Value))
 return err==nil&&ok
}

func allowedMutation(c *gin.Context) bool {
 if c.GetHeader("X-CSRF-Token")!="1" {return false}
 if origin:=c.Request.Header.Get("Origin");origin!="" {
  u,err:=url.Parse(origin)
  if err!=nil||u.Host!=c.Request.Host||(u.Scheme!="http"&&u.Scheme!="https") {return false}
 }
 return true
}

func (s *Service) Require(c *gin.Context) {
 if !s.sessionValid(c) {
  c.JSON(http.StatusUnauthorized,gin.H{"error":gin.H{"message":"请先登录"}})
  c.Abort()
  return
 }
 if c.Request.Method!=http.MethodGet&&c.Request.Method!=http.MethodHead&&
  c.Request.Method!=http.MethodOptions&&!allowedMutation(c) {
  c.JSON(http.StatusForbidden,gin.H{"error":gin.H{"message":"请求安全校验失败"}})
  c.Abort()
 }
}

func (s *Service) Me(c *gin.Context) {c.JSON(http.StatusOK,gin.H{"authenticated":true,"username":"admin"})}

func (s *Service) Logout(c *gin.Context) {
 cookie,err:=c.Request.Cookie(CookieName)
 if err==nil {
  if err:=s.repo.DeleteAdminSession(c.Request.Context(),tokenHash(cookie.Value));err!=nil{
   c.JSON(http.StatusInternalServerError,gin.H{"error":gin.H{"message":"退出登录失败"}});return
  }
 }
 http.SetCookie(c.Writer,&http.Cookie{
  Name:CookieName,Value:"",Path:"/api/v1",HttpOnly:true,SameSite:http.SameSiteStrictMode,
  Secure:s.secure||c.Request.TLS!=nil,MaxAge:-1,
 })
 c.Status(http.StatusNoContent)
}

func SecureCookieFromEnvironment() bool {
 return strings.EqualFold(os.Getenv("SESSION_SECURE"),"true")
}
