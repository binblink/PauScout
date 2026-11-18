package reader

import (
	"bytes"
	"encoding/binary"
	"math"
)

// euroRate returns the game's current EUR-per-GBP rate from the Euro record.
// A unique, fully matching record is required before using the rate.
func euroRate(data []byte) *float64 {
	marker := []byte("\x0d\x00\x00\x00European Euro\x04\x00\x00\x00Euro\x00\x00\x00\x00\x03\x00\x00\x00\xe2\x82\xac")
	at := bytes.Index(data, marker)
	if at < 0 || bytes.Index(data[at+len(marker):], marker) >= 0 || at+len(marker)+4 > len(data) {
		return nil
	}
	rate := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[at+len(marker):])))
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0.1 || rate > 10 {
		return nil
	}
	return &rate
}
