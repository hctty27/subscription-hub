package httpapi

import (
 "net/http"
 "strconv"

 "github.com/gin-gonic/gin"
)

func (a *api) listNotificationChannels(c *gin.Context){
 channels:=[]string{}
 if a.dispatcher!=nil{channels=a.dispatcher.Configured()}
 c.JSON(http.StatusOK,gin.H{"items":channels})
}
func (a *api) listNotificationDeliveries(c *gin.Context){
 limit:=100
 if raw:=c.Query("limit");raw!=""{
  n,err:=strconv.Atoi(raw)
  if err!=nil||n<1||n>500{fail(c,http.StatusBadRequest,"limit 必须在 1 到 500 之间");return}
  limit=n
 }
 items,err:=a.repo.ListDeliveryLogs(c.Request.Context(),limit)
 if err!=nil{failedStore(c,err);return}
 c.JSON(http.StatusOK,gin.H{"items":items})
}
