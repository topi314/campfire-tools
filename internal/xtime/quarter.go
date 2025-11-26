package xtime

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const quarterYearsToShow = 3

var (
	quartersCached      []Quarter
	quartersGeneratedAt time.Time
	quartersMu          sync.Mutex
)

type Quarter struct {
	Name  string
	Value string
}

func GetQuarters() []Quarter {
	now := time.Now().UTC()
	currentYear := now.Year()
	currentMonth := now.Month()

	quartersMu.Lock()
	defer quartersMu.Unlock()
	if quartersGeneratedAt.Year() == currentYear && quartersGeneratedAt.Month() == currentMonth {
		return quartersCached
	}

	var quarters []Quarter
	for year := currentYear; year >= currentYear-quarterYearsToShow; year-- {
		for q := 4; q >= 1; q-- {
			var startMonth time.Month
			switch q {
			case 1:
				startMonth = time.January
			case 2:
				startMonth = time.April
			case 3:
				startMonth = time.July
			case 4:
				startMonth = time.October
			}

			// Only include future quarters for the current year
			if year == currentYear && startMonth > currentMonth {
				continue
			}

			quarterName := fmt.Sprintf("Q%d %d", q, year)
			quarterValue := fmt.Sprintf("q%d-%d", q, year)
			quarters = append(quarters, Quarter{
				Name:  quarterName,
				Value: quarterValue,
			})
		}
	}

	quartersCached = quarters
	quartersGeneratedAt = now

	return quarters
}

func GetRangeFromQuarter(value string) (time.Time, time.Time) {
	value = strings.ToLower(value)

	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return GetCurrentQuarterRange()
	}

	quarter := parts[0]
	yearStr := parts[1]
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return GetCurrentQuarterRange()
	}

	var startMonth time.Month
	switch quarter {
	case "q1":
		startMonth = time.January
	case "q2":
		startMonth = time.April
	case "q3":
		startMonth = time.July
	case "q4":
		startMonth = time.October
	default:
		return GetCurrentQuarterRange()
	}
	start := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 3, -1).Add(time.Hour*23 + time.Minute*59 + time.Second*59) // End of the quarter

	return start, end
}

func GetCurrentQuarterRange() (time.Time, time.Time) {
	now := time.Now().UTC()
	year := now.Year()
	month := now.Month()

	var startMonth time.Month
	switch month {
	case time.January, time.February, time.March:
		startMonth = time.January
	case time.April, time.May, time.June:
		startMonth = time.April
	case time.July, time.August, time.September:
		startMonth = time.July
	default:
		startMonth = time.October
	}

	start := time.Date(year, startMonth, 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 3, -1).Add(time.Hour*23 + time.Minute*59 + time.Second*59) // End of the quarter

	return start, end
}
