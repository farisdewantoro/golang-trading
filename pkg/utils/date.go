package utils

import (
	"fmt"
	"log"
	"math"
	"time"
)

func GetWibTimeLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		log.Fatal("Failed to load location", err)
	}
	return loc
}

func TimeNowWIB() time.Time {
	return time.Now().In(GetWibTimeLocation())
}
func TimeToWIB(time time.Time) time.Time {
	return time.In(TimeNowWIB().Location())
}

func GetNowWithOnlyHour() time.Time {
	now := TimeNowWIB()
	return time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		now.Hour(),
		0, 0, 0,
		now.Location(),
	)
}

func PrettyDate(date time.Time) string {
	return fmt.Sprintf("%02d %s %d - %02d:%02d WIB",
		date.Day(),
		GetIndonesianMonth(date.Month()),
		date.Year(),
		date.Hour(),
		date.Minute(),
	)
}

func GetIndonesianMonth(month time.Month) string {
	months := map[time.Month]string{
		time.January:   "Januari",
		time.February:  "Februari",
		time.March:     "Maret",
		time.April:     "April",
		time.May:       "Mei",
		time.June:      "Juni",
		time.July:      "Juli",
		time.August:    "Agustus",
		time.September: "September",
		time.October:   "Oktober",
		time.November:  "November",
		time.December:  "Desember",
	}
	return months[month]
}

func RemainingDays(maxHoldingDays int, buyTime time.Time) int {
	// Hitung waktu expired
	expiredTime := buyTime.AddDate(0, 0, maxHoldingDays)

	// Hitung selisih hari dari sekarang
	now := GetNowWithOnlyHour()
	remaining := int(math.Ceil(expiredTime.Sub(now).Hours() / 24))

	return remaining
}

func DaysSince(date time.Time) int {
	// Normalisasi waktu ke 00:00 UTC
	normalizedDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	now := time.Now().UTC()

	duration := now.Sub(normalizedDate)
	return int(duration.Hours() / 24)
}

func MapPeriodeStringToUnix(periode string) (int64, int64) {

	now := TimeNowWIB()
	switch periode {
	case "1d":
		return now.AddDate(0, 0, -1).Unix(), now.Unix()
	case "14d":
		return now.AddDate(0, 0, -14).Unix(), now.Unix()
	case "1w":
		return now.AddDate(0, 0, -7).Unix(), now.Unix()
	case "1m":
		return now.AddDate(0, 0, -30).Unix(), now.Unix()
	case "2m":
		return now.AddDate(0, 0, -60).Unix(), now.Unix()
	case "3m":
		return now.AddDate(0, 0, -90).Unix(), now.Unix()
	case "6m":
		return now.AddDate(0, 0, -180).Unix(), now.Unix()
	case "1y":
		return now.AddDate(0, 0, -365).Unix(), now.Unix()
	default:
		return 0, 0
	}
}

func MapPeriodeStringToUnixMs(periode string) (int64, int64) {
	startTime, endTime := MapPeriodeStringToUnix(periode)
	return startTime * 1000, endTime * 1000
}
