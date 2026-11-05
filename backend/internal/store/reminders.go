package store

import (
	"context"
	"database/sql"
	"errors"
)

type ReminderRule struct {
	ID int64 `json:"id"`
	ItemID int64 `json:"item_id"`
	ItemName string `json:"item_name"`
	DaysBefore int `json:"days_before"`
	Enabled bool `json:"enabled"`
}

const ruleSelect="SELECT r.id,r.item_id,i.name,r.days_before,r.enabled FROM reminder_rules r JOIN items i ON r.item_id=i.id"

func scanRule(s scanner)(ReminderRule,error){
	var rule ReminderRule
	err:=s.Scan(&rule.ID,&rule.ItemID,&rule.ItemName,&rule.DaysBefore,&rule.Enabled)
	return rule,err
}
func (s *Store) ListReminderRules(ctx context.Context)([]ReminderRule,error){
	rows,err:=s.db.QueryContext(ctx,ruleSelect+" ORDER BY r.item_id,r.days_before")
	if err!=nil{return nil,err}
	defer rows.Close()
	out:=make([]ReminderRule,0)
	for rows.Next(){r,err:=scanRule(rows);if err!=nil{return nil,err};out=append(out,r)}
	return out,rows.Err()
}
func (s *Store) ListActiveReminderRules(ctx context.Context)([]ReminderRule,error){
	rows,err:=s.db.QueryContext(ctx,ruleSelect+" WHERE r.enabled=1 AND i.status='active' ORDER BY r.id")
	if err!=nil{return nil,err}
	defer rows.Close()
	out:=make([]ReminderRule,0)
	for rows.Next(){r,err:=scanRule(rows);if err!=nil{return nil,err};out=append(out,r)}
	return out,rows.Err()
}
func (s *Store) GetReminderRule(ctx context.Context,id int64)(ReminderRule,error){
	r,err:=scanRule(s.db.QueryRowContext(ctx,ruleSelect+" WHERE r.id=?",id))
	if errors.Is(err,sql.ErrNoRows){return r,ErrNotFound}
	return r,err
}
func (s *Store) CreateReminderRule(ctx context.Context,rule ReminderRule)(ReminderRule,error){
	result,err:=s.db.ExecContext(ctx,"INSERT INTO reminder_rules(item_id,days_before,enabled) VALUES(?,?,?)",rule.ItemID,rule.DaysBefore,rule.Enabled)
	if err!=nil{return ReminderRule{},constraintError(err)}
	id,err:=result.LastInsertId()
	if err!=nil{return ReminderRule{},err}
	return s.GetReminderRule(ctx,id)
}
func (s *Store) UpdateReminderRule(ctx context.Context,rule ReminderRule)(ReminderRule,error){
	result,err:=s.db.ExecContext(ctx,"UPDATE reminder_rules SET days_before=?,enabled=? WHERE id=?",rule.DaysBefore,rule.Enabled,rule.ID)
	if err!=nil{return ReminderRule{},constraintError(err)}
	n,err:=result.RowsAffected()
	if err!=nil{return ReminderRule{},err}
	if n==0{return ReminderRule{},ErrNotFound}
	return s.GetReminderRule(ctx,rule.ID)
}
func (s *Store) DeleteReminderRule(ctx context.Context,id int64)error{
	result,err:=s.db.ExecContext(ctx,"DELETE FROM reminder_rules WHERE id=?",id)
	if err!=nil{return err}
	n,err:=result.RowsAffected()
	if err!=nil{return err}
	if n==0{return ErrNotFound}
	return nil
}

type ReminderNotification struct {
	ID int64 `json:"id"`
	RuleID int64 `json:"rule_id"`
	ItemID int64 `json:"item_id"`
	ItemName string `json:"item_name"`
	DueDate string `json:"due_date"`
	DaysBefore int `json:"days_before"`
	ReadAt *string `json:"read_at"`
	CreatedAt string `json:"created_at"`
}
func (s *Store) InsertReminderNotification(ctx context.Context,rule ReminderRule,dueDate string)(bool,error){
	res,err:=s.db.ExecContext(ctx,`INSERT INTO reminder_notifications(rule_id,item_id,due_date,days_before)
		VALUES(?,?,?,?) ON CONFLICT(rule_id,due_date) DO NOTHING`,rule.ID,rule.ItemID,dueDate,rule.DaysBefore)
	if err!=nil{return false,err}
	n,err:=res.RowsAffected()
	return n>0,err
}
func (s *Store) ListReminderNotifications(ctx context.Context,unreadOnly bool,limit int)([]ReminderNotification,error){
	q:=`SELECT n.id,n.rule_id,n.item_id,i.name,n.due_date,n.days_before,n.read_at,n.created_at
		FROM reminder_notifications n JOIN items i ON i.id=n.item_id`
	if unreadOnly{q+=" WHERE n.read_at IS NULL"}
	q+=" ORDER BY n.id DESC LIMIT ?"
	rows,err:=s.db.QueryContext(ctx,q,limit)
	if err!=nil{return nil,err}
	defer rows.Close()
	out:=make([]ReminderNotification,0)
	for rows.Next(){
		var n ReminderNotification
		var readAt sql.NullString
		if err:=rows.Scan(&n.ID,&n.RuleID,&n.ItemID,&n.ItemName,&n.DueDate,&n.DaysBefore,&readAt,&n.CreatedAt);err!=nil{return nil,err}
		if readAt.Valid{n.ReadAt=&readAt.String}
		out=append(out,n)
	}
	return out,rows.Err()
}
func (s *Store) CountUnreadNotifications(ctx context.Context)(int64,error){
	var count int64
	err:=s.db.QueryRowContext(ctx,"SELECT COUNT(*) FROM reminder_notifications WHERE read_at IS NULL").Scan(&count)
	return count,err
}
func (s *Store) MarkReminderNotificationRead(ctx context.Context,id int64)error{
	res,err:=s.db.ExecContext(ctx,"UPDATE reminder_notifications SET read_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=? AND read_at IS NULL",id)
	if err!=nil{return err}
	n,_:=res.RowsAffected()
	if n>0{return nil}
	var exists int
	if err:=s.db.QueryRowContext(ctx,"SELECT EXISTS(SELECT 1 FROM reminder_notifications WHERE id=?)",id).Scan(&exists);err!=nil{return err}
	if exists==0{return ErrNotFound}
	return nil
}
