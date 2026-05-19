package pricing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func ts(hour, min int) time.Time {
	return time.Date(2026, 5, 17, hour, min, 0, 0, time.Local)
}

func TestCalculate_OneHour(t *testing.T) {
	res := Calculate(ts(10, 0), ts(11, 0))

	assert.Equal(t, int64(5000), res.ParkingFeeIDR)
	assert.Equal(t, int64(0), res.OvernightFeeIDR)
	assert.Equal(t, int64(10000), res.TotalIDR) // 5000 booking + 5000 parking
	assert.Equal(t, 1, res.DurationHours)
	assert.False(t, res.IsOvernight)
}

func TestCalculate_TwoHours(t *testing.T) {
	res := Calculate(ts(10, 0), ts(12, 0))

	assert.Equal(t, int64(10000), res.ParkingFeeIDR)
	assert.Equal(t, int64(15000), res.TotalIDR) // 5000 + 10000 + 0
	assert.Equal(t, 2, res.DurationHours)
}

func TestCalculate_PartialHourRoundsUp(t *testing.T) {
	// 1h30m → ceil = 2 hours
	res := Calculate(ts(10, 0), ts(11, 30))

	assert.Equal(t, int64(10000), res.ParkingFeeIDR)
	assert.Equal(t, 2, res.DurationHours)
}

func TestCalculate_Overnight(t *testing.T) {
	// 23:00 → 01:00 next day — crosses midnight
	checkedIn := time.Date(2026, 5, 17, 23, 0, 0, 0, time.Local)
	checkedOut := time.Date(2026, 5, 18, 1, 0, 0, 0, time.Local)

	res := Calculate(checkedIn, checkedOut)

	assert.True(t, res.IsOvernight)
	assert.Equal(t, int64(20000), res.OvernightFeeIDR)
	assert.Equal(t, int64(2), int64(res.DurationHours))
	// total = 5000 booking + 10000 parking + 20000 overnight
	assert.Equal(t, int64(35000), res.TotalIDR)
}

func TestCalculate_SameDay_NoOvernight(t *testing.T) {
	res := Calculate(ts(8, 0), ts(22, 0))

	assert.False(t, res.IsOvernight)
	assert.Equal(t, int64(0), res.OvernightFeeIDR)
	assert.Equal(t, 14, res.DurationHours)
}

func TestCalculate_MinimumOneHour(t *testing.T) {
	// 10 minutes → rounds up to 1 hour minimum
	res := Calculate(ts(10, 0), ts(10, 10))

	assert.Equal(t, 1, res.DurationHours)
	assert.Equal(t, int64(5000), res.ParkingFeeIDR)
}

func TestCalculate_ZeroDuration_MinimumOneHour(t *testing.T) {
	// same time in/out → 0 minutes → 1 hour minimum
	res := Calculate(ts(10, 0), ts(10, 0))

	assert.Equal(t, 1, res.DurationHours)
	assert.Equal(t, int64(5000), res.ParkingFeeIDR)
}

func TestCalculate_NegativeDuration_MinimumOneHour(t *testing.T) {
	// checkout before checkin (defensive)
	res := Calculate(ts(12, 0), ts(10, 0))

	assert.Equal(t, 1, res.DurationHours)
	assert.Equal(t, int64(5000), res.ParkingFeeIDR)
}

func TestCalculate_BookingFeeAlwaysIncluded(t *testing.T) {
	res := Calculate(ts(10, 0), ts(11, 0))

	assert.Equal(t, BookingFeeIDR, int64(5000))
	assert.Equal(t, res.TotalIDR, BookingFeeIDR+res.ParkingFeeIDR+res.OvernightFeeIDR)
}

func TestCalculate_MultiNight_ThreeMidnightsCrossed(t *testing.T) {
	// Mon 10:00 → Thu 10:00 = 72 hours, crosses 3 midnights
	checkedIn := time.Date(2026, 5, 18, 10, 0, 0, 0, time.Local) // Monday
	checkedOut := time.Date(2026, 5, 21, 10, 0, 0, 0, time.Local) // Thursday

	res := Calculate(checkedIn, checkedOut)

	assert.True(t, res.IsOvernight)
	assert.Equal(t, 72, res.DurationHours)
	assert.Equal(t, int64(360000), res.ParkingFeeIDR)  // 72 * 5000
	assert.Equal(t, int64(60000), res.OvernightFeeIDR) // 3 * 20000
	// total = 5000 booking + 360000 parking + 60000 overnight
	assert.Equal(t, int64(425000), res.TotalIDR)
}
