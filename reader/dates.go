package reader

import (
	"encoding/binary"
	"fmt"
	"time"
)

func writeBirthDate(data []byte, at int, value string) (bool, error) {
	date, err := time.Parse("2006-01-02", value)
	if err != nil || date.Year() < 1901 || date.Year() > 2200 || date.Format("2006-01-02") != value {
		return false, fmt.Errorf("date de naissance invalide : %q (année entre 1901 et 2200)", value)
	}
	if at < 0 || at+4 > len(data) {
		return false, fmt.Errorf("date de naissance introuvable")
	}
	current, ok := fmDate(data, at)
	if !ok {
		return false, fmt.Errorf("date de naissance d'origine invalide")
	}
	if current.Equal(date) {
		return false, nil
	}
	binary.LittleEndian.PutUint16(data[at:], uint16(date.YearDay()))
	binary.LittleEndian.PutUint16(data[at+2:], uint16(date.Year()))
	return true, nil
}

// fmDate decodes the year and day-of-year stored in a four-byte FM date.
func fmDate(data []byte, at int) (time.Time, bool) {
	if at < 0 || at+4 > len(data) {
		return time.Time{}, false
	}
	packed := binary.LittleEndian.Uint16(data[at:])
	year := int(binary.LittleEndian.Uint16(data[at+2:]))
	day := int(packed & 0x1ff)
	if year < 1901 || year > 2200 || day < 1 {
		return time.Time{}, false
	}
	date := time.Date(year, 1, day, 0, 0, 0, 0, time.UTC)
	if date.Year() != year {
		return time.Time{}, false
	}
	return date, true
}

func summaryVersionField(data []byte) (string, int, bool) {
	if len(data) < 12 {
		return "", 0, false
	}
	initial := uint64(binary.LittleEndian.Uint32(data[8:]))
	if initial > 65536 || initial > uint64(len(data)-12) {
		return "", 0, false
	}
	return parseLengthString(data, 12+int(initial))
}

// summaryGameDate follows the FM27 summary's division list and two labels.
func summaryGameDate(data []byte, version string) (time.Time, bool) {
	storedVersion, next, ok := summaryVersionField(data)
	if !ok || storedVersion != version || next+4 > len(data) {
		return time.Time{}, false
	}
	count := binary.LittleEndian.Uint32(data[next:])
	if count > 4096 {
		return time.Time{}, false
	}
	at := next + 4
	for i := uint32(0); i < count; i++ {
		_, at, ok = parseLengthString(data, at)
		if !ok {
			return time.Time{}, false
		}
	}
	_, at, ok = parseLengthString(data, at+8)
	if !ok {
		return time.Time{}, false
	}
	_, at, ok = parseLengthString(data, at)
	if !ok {
		return time.Time{}, false
	}
	return fmDate(data, at+4)
}

func ageAt(birth, clock time.Time) *int {
	if birth.IsZero() || clock.IsZero() || birth.After(clock) {
		return nil
	}
	age := clock.Year() - birth.Year()
	if clock.Month() < birth.Month() || clock.Month() == birth.Month() && clock.Day() < birth.Day() {
		age--
	}
	return &age
}
