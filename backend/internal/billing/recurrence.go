package billing

import (
	"fmt"
	"time"
)

const DateLayout = "2006-01-02"

// NextDue computes the next scheduled date on or after today.
// Dates are calendar-only UTC values; monthly/yearly dates are anchored to
// the original day, with invalid days clamped to the end of the month.
func NextDue(firstDate, unit string, interval int, today time.Time) (string, error) {
	anchor, err := time.Parse(DateLayout, firstDate)
	if err != nil || anchor.Format(DateLayout) != firstDate {
		return "", fmt.Errorf("无效的首次到期日期")
	}
	if unit == "none" {
		return firstDate, nil
	}
	if interval < 1 || interval > 365 {
		return "", fmt.Errorf("计费周期必须在 1 到 365 之间")
	}
	if unit != "day" && unit != "month" && unit != "year" {
		return "", fmt.Errorf("不支持的计费周期")
	}
	now, err := time.Parse(DateLayout, today.Format(DateLayout))
	if err != nil {
		return "", err
	}
	if !anchor.Before(now) {
		return firstDate, nil
	}

	var steps int
	switch unit {
	case "day":
		steps = int(now.Sub(anchor).Hours()/24) / interval
	case "month":
		months := (now.Year()-anchor.Year())*12 + int(now.Month()) - int(anchor.Month())
		steps = months / interval
	case "year":
		steps = (now.Year() - anchor.Year()) / interval
	}
	if steps < 0 {
		steps = 0
	}
	for {
		next := dateAt(anchor, unit, interval*steps)
		if !next.Before(now) {
			return next.Format(DateLayout), nil
		}
		steps++
	}
}

func dateAt(anchor time.Time, unit string, offset int) time.Time {
	switch unit {
	case "day":
		return anchor.AddDate(0, 0, offset)
	case "month":
		monthIndex := anchor.Year()*12 + int(anchor.Month()) - 1 + offset
		year := monthIndex / 12
		month := time.Month(monthIndex%12 + 1)
		return clamped(year, month, anchor.Day())
	default:
		return clamped(anchor.Year()+offset, anchor.Month(), anchor.Day())
	}
}

func clamped(year int, month time.Month, day int) time.Time {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func DaysUntil(date string, today time.Time) (int, error) {
	due, err := time.Parse(DateLayout, date)
	if err != nil {
		return 0, err
	}
	current, err := time.Parse(DateLayout, today.Format(DateLayout))
	if err != nil {
		return 0, err
	}
	return int(due.Sub(current).Hours() / 24), nil
}
