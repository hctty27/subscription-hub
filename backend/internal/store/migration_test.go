package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateFromVersionOne(t *testing.T){
	path:=filepath.Join(t.TempDir(),"legacy.db")
	old,err:=sql.Open("sqlite",path)
	if err!=nil{t.Fatal(err)}
	for _,stmt:=range []string{
		`CREATE TABLE categories(id INTEGER PRIMARY KEY AUTOINCREMENT,name TEXT NOT NULL COLLATE NOCASE UNIQUE,color TEXT NOT NULL DEFAULT '#5470e8')`,
		`CREATE TABLE items(id INTEGER PRIMARY KEY AUTOINCREMENT,category_id INTEGER REFERENCES categories(id) ON DELETE RESTRICT,name TEXT NOT NULL,kind TEXT NOT NULL,status TEXT NOT NULL,amount_minor INTEGER NOT NULL,currency TEXT NOT NULL,period_unit TEXT NOT NULL,period_count INTEGER NOT NULL,first_due_date TEXT NOT NULL,notes TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL)`,
		`INSERT INTO categories(name,color) VALUES('旧分类','#5470e8')`,
		`INSERT INTO items(name,kind,status,amount_minor,currency,period_unit,period_count,first_due_date,notes,created_at,updated_at) VALUES('旧订阅','subscription','active',2000,'USD','month',1,'2026-01-31','','2026-01-01','2026-01-01')`,
		"PRAGMA user_version = 1",
	}{
		if _,err:=old.Exec(stmt);err!=nil{t.Fatal(err)}
	}
	if err:=old.Close();err!=nil{t.Fatal(err)}
	s,err:=Open(path)
	if err!=nil{t.Fatal(err)}
	defer s.Close()
	item,err:=s.GetItem(context.Background(),1)
	if err!=nil||item.Name!="旧订阅"{t.Fatalf("历史项目丢失：%+v err=%v",item,err)}
	var version int
	if err:=s.db.QueryRow("PRAGMA user_version").Scan(&version);err!=nil||version!=2{
		t.Fatalf("升级后版本错误：version=%d err=%v",version,err)
	}
	if _,err:=s.ExpenseReport(context.Background(),"2026-01");err!=nil{t.Fatalf("新表不可用：%v",err)}
}
