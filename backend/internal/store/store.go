package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound      = errors.New("记录不存在")
	ErrConflict      = errors.New("名称重复或记录仍被使用")
	ErrCategory      = errors.New("分类不存在")
)

type Store struct {
	db *sql.DB
}

type Category struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type Item struct {
	ID           int64  `json:"id"`
	CategoryID   *int64 `json:"category_id"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	AmountMinor  int64  `json:"amount_minor"`
	Currency     string `json:"currency"`
	PeriodUnit   string `json:"period_unit"`
	PeriodCount  int    `json:"period_count"`
	FirstDueDate string `json:"first_due_date"`
	Notes        string `json:"notes"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	NextDueDate  string `json:"next_due_date"`
	DaysUntil    int    `json:"days_until"`
}

const itemColumns = "id, category_id, name, kind, status, amount_minor, currency, period_unit, period_count, first_due_date, notes, created_at, updated_at"

func Open(path string) (*Store, error) {
	if path == "" {
		path = "./data/subscription-hub.db"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("创建数据目录失败：%w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Keep connection-scoped PRAGMAs consistent and serialize SQLite writes.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx := context.Background()
	for _, statement := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("初始化数据库失败：%w", err)
		}
	}
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > 2 {
		return fmt.Errorf("数据库版本 %d 高于当前程序支持的版本", version)
	}
	if version == 2 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL COLLATE NOCASE UNIQUE,
			color TEXT NOT NULL DEFAULT '#5470e8'
		)`,
		`CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category_id INTEGER REFERENCES categories(id) ON DELETE RESTRICT,
			name TEXT NOT NULL,
			kind TEXT NOT NULL CHECK(kind IN ('subscription','resource')),
			status TEXT NOT NULL CHECK(status IN ('active','paused','canceled')),
			amount_minor INTEGER NOT NULL DEFAULT 0 CHECK(amount_minor >= 0),
			currency TEXT NOT NULL,
			period_unit TEXT NOT NULL CHECK(period_unit IN ('none','day','month','year')),
			period_count INTEGER NOT NULL DEFAULT 0 CHECK(period_count BETWEEN 0 AND 365),
			first_due_date TEXT NOT NULL,
			notes TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
			updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		)`,
		"CREATE INDEX IF NOT EXISTS idx_items_kind_status ON items(kind, status)",
		"CREATE INDEX IF NOT EXISTS idx_items_category_id ON items(category_id)",
		"PRAGMA user_version = 1",
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("创建数据库表失败：%w", err)
		}
	}
	if version < 2 {
		upgrade := []string{
			`CREATE TABLE IF NOT EXISTS payments (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
				amount_minor INTEGER NOT NULL CHECK(amount_minor > 0 AND amount_minor <= 1000000000000),
				currency TEXT NOT NULL,
				paid_date TEXT NOT NULL,
				notes TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
			)`,
			"CREATE INDEX IF NOT EXISTS idx_payments_paid_date ON payments(paid_date)",
			"CREATE INDEX IF NOT EXISTS idx_payments_item_id ON payments(item_id)",
			`CREATE TABLE IF NOT EXISTS reminder_rules (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
				days_before INTEGER NOT NULL CHECK(days_before BETWEEN 0 AND 30),
				enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
				UNIQUE(item_id, days_before)
			)`,
			`CREATE TABLE IF NOT EXISTS reminder_notifications (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				rule_id INTEGER NOT NULL REFERENCES reminder_rules(id) ON DELETE CASCADE,
				item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
				due_date TEXT NOT NULL,
				days_before INTEGER NOT NULL,
				read_at TEXT,
				created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
				UNIQUE(rule_id, due_date)
			)`,
			"CREATE INDEX IF NOT EXISTS idx_notifications_read ON reminder_notifications(read_at, id)",
			"PRAGMA user_version = 2",
		}
		for _, stmt := range upgrade {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("升级数据库到 V2 失败：%w", err)
			}
		}
	}
	return tx.Commit()
}

func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, color FROM categories ORDER BY name, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Category, 0)
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Color); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCategory(ctx context.Context, id int64) (Category, error) {
	var c Category
	err := s.db.QueryRowContext(ctx, "SELECT id, name, color FROM categories WHERE id = ?", id).Scan(&c.ID, &c.Name, &c.Color)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

func (s *Store) CreateCategory(ctx context.Context, c Category) (Category, error) {
	res, err := s.db.ExecContext(ctx, "INSERT INTO categories(name, color) VALUES(?, ?)", c.Name, c.Color)
	if err != nil {
		return Category{}, constraintError(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Category{}, err
	}
	return s.GetCategory(ctx, id)
}

func (s *Store) UpdateCategory(ctx context.Context, c Category) (Category, error) {
	res, err := s.db.ExecContext(ctx, "UPDATE categories SET name = ?, color = ? WHERE id = ?", c.Name, c.Color, c.ID)
	if err != nil {
		return Category{}, constraintError(err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Category{}, err
	}
	if affected == 0 {
		return Category{}, ErrNotFound
	}
	return s.GetCategory(ctx, c.ID)
}

func (s *Store) DeleteCategory(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM categories WHERE id = ?", id)
	if err != nil {
		return constraintError(err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CategoryExists(ctx context.Context, categoryID *int64) (bool, error) {
	if categoryID == nil {
		return true, nil
	}
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM categories WHERE id = ?)", *categoryID).Scan(&exists)
	return exists == 1, err
}

type ItemFilter struct {
	Kind   string
	Status string
	Query  string
	Limit  int
	Offset int
}

func (s *Store) ListItems(ctx context.Context, filter ItemFilter) ([]Item, error) {
	q := "SELECT " + itemColumns + " FROM items WHERE 1 = 1"
	args := make([]any, 0, 5)
	if filter.Kind != "" {
		q += " AND kind = ?"
		args = append(args, filter.Kind)
	}
	if filter.Status != "" {
		q += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.Query != "" {
		q += " AND name LIKE ?"
		escaped := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(filter.Query)
		q += " ESCAPE '\\'"
		args = append(args, "%"+escaped+"%")
	}
	q += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, filter.Limit, filter.Offset)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Item, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetItem(ctx context.Context, id int64) (Item, error) {
	item, err := scanItem(s.db.QueryRowContext(ctx, "SELECT "+itemColumns+" FROM items WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	return item, err
}

func (s *Store) CreateItem(ctx context.Context, item Item) (Item, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO items
		(category_id, name, kind, status, amount_minor, currency, period_unit, period_count, first_due_date, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.CategoryID, item.Name, item.Kind, item.Status, item.AmountMinor, item.Currency,
		item.PeriodUnit, item.PeriodCount, item.FirstDueDate, item.Notes)
	if err != nil {
		return Item{}, constraintError(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Item{}, err
	}
	return s.GetItem(ctx, id)
}

func (s *Store) UpdateItem(ctx context.Context, item Item) (Item, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE items SET
		category_id=?, name=?, kind=?, status=?, amount_minor=?, currency=?, period_unit=?,
		period_count=?, first_due_date=?, notes=?, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id=?`,
		item.CategoryID, item.Name, item.Kind, item.Status, item.AmountMinor, item.Currency,
		item.PeriodUnit, item.PeriodCount, item.FirstDueDate, item.Notes, item.ID)
	if err != nil {
		return Item{}, constraintError(err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Item{}, err
	}
	if affected == 0 {
		return Item{}, ErrNotFound
	}
	return s.GetItem(ctx, item.ID)
}

func (s *Store) DeleteItem(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM items WHERE id = ?", id)
	if err != nil {
		return constraintError(err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanItem(row scanner) (Item, error) {
	var item Item
	var categoryID sql.NullInt64
	err := row.Scan(&item.ID, &categoryID, &item.Name, &item.Kind, &item.Status,
		&item.AmountMinor, &item.Currency, &item.PeriodUnit, &item.PeriodCount,
		&item.FirstDueDate, &item.Notes, &item.CreatedAt, &item.UpdatedAt)
	if categoryID.Valid {
		item.CategoryID = &categoryID.Int64
	}
	return item, err
}

func constraintError(err error) error {
	if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		return ErrConflict
	}
	return err
}
