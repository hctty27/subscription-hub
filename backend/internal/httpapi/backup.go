package httpapi

import (
 "net/http"

 "github.com/gin-gonic/gin"
 "github.com/hctty27/subscription-hub/backend/internal/store"
)

func (a *api) createBackup(c *gin.Context){
 created,err:=a.repo.Backup(c.Request.Context(),a.backupDir,7)
 if err!=nil{failedStore(c,err);return}
 c.JSON(http.StatusCreated,created)
}
func (a *api) listBackups(c *gin.Context){
 files,err:=store.ListBackups(a.backupDir)
 if err!=nil{failedStore(c,err);return}
 c.JSON(http.StatusOK,gin.H{"items":files})
}
