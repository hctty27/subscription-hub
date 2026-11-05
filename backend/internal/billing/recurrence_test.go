package billing

import (
	"testing"
	"time"
)

func TestNextDue(t *testing.T) {
	cases := []struct {
		name, first, unit, today, want string
		interval int
	}{
		{"当日到期", "2026-11-05", "month", "2026-11-05", "2026-11-05", 1},
		{"月底二月", "2025-01-31", "month", "2025-02-01", "2025-02-28", 1},
		{"月底不漂移", "2025-01-31", "month", "2025-03-01", "2025-03-31", 1},
		{"隔月", "2025-01-31", "month", "2025-04-01", "2025-05-31", 2},
		{"闰年年度", "2024-02-29", "year", "2025-02-01", "2025-02-28", 1},
		{"闰年还原", "2024-02-29", "year", "2028-02-01", "2028-02-29", 1},
		{"按日", "2026-11-01", "day", "2026-11-05", "2026-11-07", 3},
		{"未来开始", "2027-01-01", "year", "2026-11-05", "2027-01-01", 1},
		{"一次性过期仍保留日期", "2025-01-01", "none", "2026-11-05", "2025-01-01", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			today, _ := time.Parse(DateLayout, tc.today)
			got, err := NextDue(tc.first, tc.unit, tc.interval, today)
			if err != nil || got != tc.want {
				t.Fatalf("实际 %q, 期望 %q, 错误 %v", got, tc.want, err)
			}
		})
	}
}

func TestNextDueValidation(t *testing.T) {
	today := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	for _, first := range []string{"2026-02-30", "2026-1-1", "invalid"} {
		if _, err := NextDue(first, "month", 1, today); err == nil {
			t.Fatalf("日期 %q 应当报错", first)
		}
	}
	if _, err := NextDue("2026-01-01", "month", 0, today); err == nil {
		t.Fatal("周期为 0 应当报错")
	}
}
