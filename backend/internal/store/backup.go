package store

import (
  "context"
  "database/sql"
  "crypto/rand"
  "encoding/hex"
  "fmt"
  "os"
  "path/filepath"
  "sort"
  "strings"
  "time"
)

type BackupFile struct {
  File string `json:"file"`
  Size int64 `json:"size"`
  CreatedAt string `json:"created_at"`
}

func (s *Store) Backup(ctx context.Context, destinationDir string, retain int) (BackupFile,error) {
  if retain<1 {retain=7}
  if retain>365 {retain=365}
  if destinationDir=="" {return BackupFile{},fmt.Errorf("备份目录不能为空")}
  if err:=os.MkdirAll(destinationDir,0700);err!=nil{return BackupFile{},err}
  dir,err:=filepath.Abs(destinationDir)
  if err!=nil{return BackupFile{},err}
  nonce:=make([]byte,5)
  if _,err:=rand.Read(nonce);err!=nil{return BackupFile{},err}
  name:="subscription-hub-"+time.Now().UTC().Format("20060102-150405")+"-"+hex.EncodeToString(nonce)+".db"
  path:=filepath.Join(dir,name)
  // SQLite VACUUM INTO creates a consistent snapshot even with WAL enabled.
  if _,err:=s.db.ExecContext(ctx,"VACUUM INTO ?",path);err!=nil{return BackupFile{},fmt.Errorf("SQLite 备份失败：%w",err)}
  if err:=os.Chmod(path,0600);err!=nil {return BackupFile{},err}
  fi,err:=os.Stat(path)
  if err!=nil{return BackupFile{},err}
  copied,err:=sqlOpenBackup(path)
  if err!=nil{return BackupFile{},err}
  defer copied.Close()
  var integrity string
  if err:=copied.QueryRowContext(ctx,"PRAGMA integrity_check").Scan(&integrity);err!=nil{return BackupFile{},err}
  if integrity!="ok"{return BackupFile{},fmt.Errorf("备份文件完整性校验失败")}
  if err:=pruneBackups(dir,retain);err!=nil{return BackupFile{},err}
  return BackupFile{File:name,Size:fi.Size(),CreatedAt:fi.ModTime().UTC().Format(time.RFC3339)},nil
}

func sqlOpenBackup(path string)(*sql.DB,error){return sql.Open("sqlite",path)}
func pruneBackups(dir string,retain int)error{
  entries,err:=os.ReadDir(dir)
  if err!=nil{return err}
  names:=make([]string,0)
  for _,e:=range entries{
    if !e.IsDir()&&strings.HasPrefix(e.Name(),"subscription-hub-")&&strings.HasSuffix(e.Name(),".db"){
      names=append(names,e.Name())
    }
  }
  sort.Sort(sort.Reverse(sort.StringSlice(names)))
  if len(names)<=retain { return nil }
  for _,name:=range names[retain:] {
    if err:=os.Remove(filepath.Join(dir,name));err!=nil{return err}
  }
  return nil
}

func ListBackups(dir string)([]BackupFile,error){
  entries,err:=os.ReadDir(dir)
  if os.IsNotExist(err){return []BackupFile{},nil}
  if err!=nil{return nil,err}
  result:=make([]BackupFile,0)
  for _,e:=range entries{
    if e.IsDir()||!strings.HasPrefix(e.Name(),"subscription-hub-")||!strings.HasSuffix(e.Name(),".db"){continue}
    info,err:=e.Info()
    if err!=nil{return nil,err}
    result=append(result,BackupFile{File:e.Name(),Size:info.Size(),CreatedAt:info.ModTime().UTC().Format(time.RFC3339)})
  }
  sort.Slice(result,func(i,j int)bool{return result[i].File>result[j].File})
  return result,nil
}
