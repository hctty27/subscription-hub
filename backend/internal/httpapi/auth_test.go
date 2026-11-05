package httpapi

import (
 "bytes"
 "net/http"
 "net/http/httptest"
 "path/filepath"
 "testing"
 "time"

 "github.com/hctty27/subscription-hub/backend/internal/auth"
 "github.com/hctty27/subscription-hub/backend/internal/store"
)

func TestAuthenticationAndCSRF(t *testing.T){
 repo,err:=store.Open(filepath.Join(t.TempDir(),"auth.db"))
 if err!=nil { t.Fatal(err) }
 defer repo.Close()
 security,err:=auth.New(t.Context(),repo,"a-long-unique-admin-password",false)
 if err!=nil{t.Fatal(err)}
 router:=NewRouter(repo,time.UTC,security,filepath.Join(t.TempDir(),"backups"))
 send:=func(method,path,body string,cookie *http.Cookie,csrf bool)(int,*httptest.ResponseRecorder){
  t.Helper()
  request:=httptest.NewRequest(method,path,bytes.NewBufferString(body))
  request.Header.Set("Content-Type","application/json")
  if csrf{request.Header.Set("X-CSRF-Token","1")}
  if cookie!=nil{request.AddCookie(cookie)}
  response:=httptest.NewRecorder()
  router.ServeHTTP(response,request)
  return response.Code,response
 }
 if code,_:=send("GET","/api/v1/items","",nil,false);code!=http.StatusUnauthorized {
  t.Fatalf("未登录读取账本应当拒绝：%d",code)
 }
 if code,_:=send("POST","/api/v1/items",`{}`,nil,true);code!=http.StatusUnauthorized {
  t.Fatalf("未登录写入账本应当拒绝：%d",code)
 }
 if code,_:=send("POST","/api/v1/auth/login",`{"password":"invalid"}`,nil,true);code!=http.StatusUnauthorized {
  t.Fatalf("错误密码应当被拒绝：%d",code)
 }
 code,login:=send("POST","/api/v1/auth/login",`{"password":"a-long-unique-admin-password"}`,nil,true)
 if code!=http.StatusOK {t.Fatalf("登录失败：%s",login.Body.String())}
 cookies:=login.Result().Cookies()
 if len(cookies)!=1 || !cookies[0].HttpOnly || cookies[0].SameSite!=http.SameSiteStrictMode {
  t.Fatalf("登录 Cookie 属性不安全：%v",cookies)
 }
 session:=cookies[0]
 if code,_:=send("GET","/api/v1/items","",session,false);code!=http.StatusOK{
  t.Fatalf("已登录不能读取账本：%d",code)
 }
 if code,_:=send("POST","/api/v1/categories",`{"name":"测试分类"}`,session,false);code!=http.StatusForbidden {
  t.Fatalf("缺少 CSRF 头仍能写入：%d",code)
 }
 if code,_:=send("POST","/api/v1/categories",`{"name":"测试分类"}`,session,true);code!=http.StatusCreated{
  t.Fatalf("提供 CSRF 头仍无法写入：%d",code)
 }
 if code,_:=send("POST","/api/v1/auth/logout","",session,true);code!=http.StatusNoContent{
  t.Fatalf("退出登录失败：%d",code)
 }
 if code,_:=send("GET","/api/v1/items","",session,false);code!=http.StatusUnauthorized{
  t.Fatalf("退出后会话未失效：%d",code)
 }
}

func TestBootstrapPasswordCannotResetExistingCredential(t *testing.T){
 path:=filepath.Join(t.TempDir(),"bootstrap.db")
 repo,err:=store.Open(path);if err!=nil{t.Fatal(err)}
 if _,err:=auth.New(t.Context(),repo,"",false);err==nil {t.Fatal("空管理员密码不能首次启动")}
 if _,err:=auth.New(t.Context(),repo,"original-password-123",false);err!=nil{t.Fatal(err)}
 repo.Close()
 reopened,err:=store.Open(path);if err!=nil{t.Fatal(err)}
 defer reopened.Close()
 if _,err:=auth.New(t.Context(),reopened,"not-the-original-password",false);err!=nil{t.Fatal(err)}
 salt,hash,ok,err:=reopened.ReadAdminCredential(t.Context())
 if err!=nil||!ok||len(salt)!=16||len(hash)!=32{t.Fatalf("密码存储异常：%v",err)}
}
