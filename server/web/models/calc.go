package models

import (
	"math"
	"time"
)

func CalcCheckInRate(accepted int, checkIns int) float64 {
	if checkIns == 0 {
		return 0
	}
	return math.RoundToEven(float64(checkIns) / float64(accepted) * 100)
}

func CalcQuarterProgress(days int, daysRemaining int) float64 {
	if days == 0 {
		return 0
	}
	if daysRemaining <= 0 {
		return 100
	}
	return math.RoundToEven(float64(days-daysRemaining) / float64(days) * 100)
}

func CalcCheckInProgress(goal int, checkIns int) float64 {
	if goal == 0 {
		return 0
	}
	if checkIns >= goal {
		return 100
	}
	return math.RoundToEven(float64(checkIns) / float64(goal) * 100)
}

func CalcCAProjectedCheckIns(from time.Time, to time.Time, totalCheckIns int) (int, int, int) {
	duration := to.Sub(from)
	if duration <= 0 {
		return 0, 0, 0 // No projection if the duration is zero or negative
	}

	days := int(duration.Hours() / 24)
	if days == 0 {
		return totalCheckIns, 0, 0 // No projection if the duration is less than a day
	}

	now := time.Now()
	nowDuration := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Sub(from)

	daysElapsed := int(nowDuration.Hours() / 24)

	daysRemaining := int(max(float64(days-daysElapsed), 0))

	// project for the remaining days in the quarter
	projectedCheckIns := totalCheckIns
	if daysRemaining > 0 {
		projectedCheckIns = int(math.Round(float64(totalCheckIns) / float64(daysElapsed) * float64(days)))
	}

	return projectedCheckIns, days, daysRemaining
}
