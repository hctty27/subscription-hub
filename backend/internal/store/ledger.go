package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Payment struct {
	ID          int64  `json:"id"`
	ItemID      int64  `json:"item_id"`
	ItemName    string `json:"item_name"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	PaidDate    string `json:"paid_date"`
	Notes       string `json:"notes"`
	CreatedAt   string `json:"created_at"`
}

type PaymentFilter struct {
	ItemID int64
	Month string
	Limit int
	Offset int
}

const paymentSelect = "SELECT p.id,p.item_id,i.name,p.amount_minor,p.currency,p.paid_date,p.notes,p.created_at FROM payments p JOIN items i ON i.id=p.item_id"

func scanPayment(s scanner) (Payment, error) {
	var p Payment
	err := s.Scan(&p.ID,&p.ItemID,&p.ItemName,&p.AmountMinor,&p.Currency,&p.PaidDate,&p.Notes,&p.CreatedAt)
	return p,err
}

func (s *Store) GetPayment(ctx context.Context, id int64) (Payment, error) {
	p,err := scanPayment(s.db.QueryRowContext(ctx, paymentSelect+" WHERE p.id=?",id))
	if errors.Is(err,sql.ErrNoRows) { return p,ErrNotFound }
	return p,err
}

func (s *Store) ListPayments(ctx context.Context, f PaymentFilter) ([]Payment,error) {
	q := paymentSelect+" WHERE 1=1"
	var args []any
	if f.ItemID>0 { q+=" AND p.item_id=?";args=append(args,f.ItemID) }
	if f.Month!="" { q+=" AND substr(p.paid_date,1,7)=?";args=append(args,f.Month) }
	q+=" ORDER BY p.paid_date DESC,p.id DESC LIMIT ? OFFSET ?"
	args=append(args,f.Limit,f.Offset)
	rows,err:=s.db.QueryContext(ctx,q,args...)
	if err!=nil{return nil,err}
	defer rows.Close()
	out:=make([]Payment,0)
	for rows.Next(){p,err:=scanPayment(rows);if err!=nil{return nil,err};out=append(out,p)}
	return out,rows.Err()
}

func (s *Store) CreatePayment(ctx context.Context,p Payment)(Payment,error){
	result,err:=s.db.ExecContext(ctx,"INSERT INTO payments(item_id,amount_minor,currency,paid_date,notes) VALUES(?,?,?,?,?)",p.ItemID,p.AmountMinor,p.Currency,p.PaidDate,p.Notes)
	if err!=nil{return Payment{},constraintError(err)}
	id,err:=result.LastInsertId()
	if err!=nil{return Payment{},err}
	return s.GetPayment(ctx,id)
}

func (s *Store) UpdatePayment(ctx context.Context,p Payment)(Payment,error){
	result,err:=s.db.ExecContext(ctx,"UPDATE payments SET item_id=?,amount_minor=?,currency=?,paid_date=?,notes=? WHERE id=?",p.ItemID,p.AmountMinor,p.Currency,p.PaidDate,p.Notes,p.ID)
	if err!=nil{return Payment{},constraintError(err)}
	n,err:=result.RowsAffected()
	if err!=nil{return Payment{},err}
	if n==0{return Payment{},ErrNotFound}
	return s.GetPayment(ctx,p.ID)
}

func (s *Store) DeletePayment(ctx context.Context,id int64)error{
	result,err:=s.db.ExecContext(ctx,"DELETE FROM payments WHERE id=?",id)
	if err!=nil{return err}
	n,err:=result.RowsAffected()
	if err!=nil{return err}
	if n==0{return ErrNotFound}
	return nil
}

type CurrencyTotal struct {
	Currency string `json:"currency"`
	AmountMinor int64 `json:"amount_minor"`
	Count int64 `json:"count"`
}
type MonthlyPoint struct {
	Month string `json:"month"`
	Currency string `json:"currency"`
	AmountMinor int64 `json:"amount_minor"`
	Count int64 `json:"count"`
}
type ExpenseReport struct {
	Month string `json:"month"`
	Totals []CurrencyTotal `json:"totals"`
	Trend []MonthlyPoint `json:"trend"`
}

func (s *Store) ExpenseReport(ctx context.Context, month string)(ExpenseReport,error){
	report:=ExpenseReport{Month:month,Totals:make([]CurrencyTotal,0),Trend:make([]MonthlyPoint,0)}
	rows,err:=s.db.QueryContext(ctx,"SELECT currency,COALESCE(SUM(amount_minor),0),COUNT(*) FROM payments WHERE substr(paid_date,1,7)=? GROUP BY currency ORDER BY currency",month)
	if err!=nil{return report,err}
	for rows.Next(){
		var t CurrencyTotal
		if err:=rows.Scan(&t.Currency,&t.AmountMinor,&t.Count);err!=nil{rows.Close();return report,err}
		report.Totals=append(report.Totals,t)
	}
	if err:=rows.Err();err!=nil{rows.Close();return report,err}
	rows.Close()
	start,err:=time.Parse("2006-01",month)
	if err!=nil{return report,err}
	since:=start.AddDate(0,-5,0).Format("2006-01")
	trendRows,err:=s.db.QueryContext(ctx,"SELECT substr(paid_date,1,7),currency,COALESCE(SUM(amount_minor),0),COUNT(*) FROM payments WHERE substr(paid_date,1,7)>=? AND substr(paid_date,1,7)<=? GROUP BY substr(paid_date,1,7),currency ORDER BY 1,2",since,month)
	if err!=nil{return report,err}
	defer trendRows.Close()
	for trendRows.Next(){
		var t MonthlyPoint
		if err:=trendRows.Scan(&t.Month,&t.Currency,&t.AmountMinor,&t.Count);err!=nil{return report,err}
		report.Trend=append(report.Trend,t)
	}
	return report,trendRows.Err()
}
