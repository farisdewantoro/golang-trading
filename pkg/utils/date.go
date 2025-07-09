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
