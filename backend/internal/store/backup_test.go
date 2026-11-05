package store

import (
 "context"
 "os"
 "path/filepath"
 "testing"
)

func TestBackupAndRetention(t *testing.T){
 repo,err:=Open(filepath.Join(t.TempDir(),"source.db"))
 if err!=nil{t.Fatal(err)}
 defer repo.Close()
 ctx:=context.Background()
 created,err:=repo.CreateCategory(ctx,Category{Name:"备份测试",Color:"#123456"})
 if err!=nil{t.Fatal(err)}
 dir:=filepath.Join(t.TempDir(),"backups")
 first,err:=repo.Backup(ctx,dir,1)
 if err!=nil{t.Fatalf("创建数据库备份失败：%v",err)}
 if first.Size<=0{t.Fatal("备份文件为空")}
 info,err:=os.Stat(filepath.Join(dir,first.File))
 if err!=nil{t.Fatal(err)}
 if info.Mode().Perm()!=0600{t.Fatalf("备份权限不安全：%v",info.Mode())}
 saved,err:=Open(filepath.Join(dir,first.File))
 if err!=nil{t.Fatalf("备份无法作为数据库打开：%v",err)}
 categories,err:=saved.ListCategories(ctx)
 if err!=nil||len(categories)!=1||categories[0].ID!=created.ID {
  t.Fatalf("备份恢复核对失败：%v err=%v",categories,err)
 }
 saved.Close()
 if _,err:=repo.Backup(ctx,dir,1);err!=nil{t.Fatal(err)}
 backups,err:=ListBackups(dir)
 if err!=nil||len(backups)!=1{t.Fatalf("保留策略错误：%v err=%v",backups,err)}
}
