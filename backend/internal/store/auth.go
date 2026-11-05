package store

import (
    "context"
    "database/sql"
    "errors"
    "time"
)

func (s *Store) ReadAdminCredential(ctx context.Context) (salt, hash []byte, found bool, err error) {
    err=s.db.QueryRowContext(ctx,"SELECT salt,password_hash FROM admin_credentials WHERE id=1").Scan(&salt,&hash)
    if errors.Is(err,sql.ErrNoRows) {return nil,nil,false,nil}
    if err!=nil {return nil,nil,false,err}
    return salt,hash,true,nil
}

func (s *Store) InstallAdminCredential(ctx context.Context,salt,hash []byte)(bool,error){
    result,err:=s.db.ExecContext(ctx,"INSERT INTO admin_credentials(id,salt,password_hash) VALUES(1,?,?) ON CONFLICT(id) DO NOTHING",salt,hash)
    if err!=nil{return false,err}
    n,err:=result.RowsAffected()
    return n==1,err
}

func (s *Store) SaveAdminSession(ctx context.Context, tokenHash string, expiresAt time.Time)error{
    _,err:=s.db.ExecContext(ctx,"INSERT INTO admin_sessions(token_hash,expires_at) VALUES(?,?)",tokenHash,expiresAt.Unix())
    return err
}
func (s *Store) ValidAdminSession(ctx context.Context,tokenHash string)(bool,error){
    var ok int
    err:=s.db.QueryRowContext(ctx,"SELECT EXISTS(SELECT 1 FROM admin_sessions WHERE token_hash=? AND expires_at>?)",tokenHash,time.Now().Unix()).Scan(&ok)
    return ok==1,err
}
func (s *Store) DeleteAdminSession(ctx context.Context, tokenHash string)error{
    _,err:=s.db.ExecContext(ctx,"DELETE FROM admin_sessions WHERE token_hash=?",tokenHash)
    return err
}
func (s *Store) PurgeExpiredSessions(ctx context.Context)error{
    _,err:=s.db.ExecContext(ctx,"DELETE FROM admin_sessions WHERE expires_at<=?",time.Now().Unix())
    return err
}
