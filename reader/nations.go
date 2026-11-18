package reader

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/csv"
	"strconv"
	"strings"
)

//go:embed data/nation-names.csv
var nationCSV string

func nationNames(data []byte) map[uint32]string {
	known := make(map[uint32]string)
	rows, err := csv.NewReader(strings.NewReader(nationCSV)).ReadAll()
	if err != nil {
		return nil
	}
	for _, row := range rows[1:] {
		if len(row) < 3 {
			continue
		}
		id, err := strconv.ParseUint(row[0], 10, 32)
		if err == nil && row[2] != "" {
			known[uint32(id)] = row[2]
		}
	}
	head := []byte{0xfb, 0, 0, 0, 0}
	for at := 0; at < len(data); {
		rel := bytes.Index(data[at:], head)
		if rel < 0 {
			break
		}
		start := at + rel
		at = start + 1
		cursor := start + len(head)
		result := make(map[uint32]string, 251)
		valid := true
		for i := uint32(0); i < 251; i++ {
			if cursor >= len(data) || data[cursor] != 0 {
				valid = false
				break
			}
			cursor++
			var ok bool
			cursor, ok = walkNationRecord(data, cursor)
			if !ok || cursor+12 > len(data) {
				valid = false
				break
			}
			index := binary.LittleEndian.Uint32(data[cursor:])
			id := binary.LittleEndian.Uint32(data[cursor+4:])
			copyID := binary.LittleEndian.Uint32(data[cursor+8:])
			if index != i || id == 0 || id != copyID {
				valid = false
				break
			}
			if name := known[id]; name != "" {
				result[i] = name
			}
			cursor += 12
		}
		if valid {
			return result
		}
	}
	return nil
}

func skipNationCount(data []byte, at, countBytes, entryBytes int) (int, bool) {
	if at < 0 || at+countBytes > len(data) {
		return 0, false
	}
	var count uint32
	switch countBytes {
	case 1:
		count = uint32(data[at])
	case 2:
		count = uint32(binary.LittleEndian.Uint16(data[at:]))
	case 4:
		count = binary.LittleEndian.Uint32(data[at:])
	default:
		return 0, false
	}
	end := at + countBytes + int(count)*entryBytes
	return end, count <= 100_000 && end <= len(data)
}

func walkNationRecord(data []byte, at int) (int, bool) {
	if at+4 > len(data) {
		return 0, false
	}
	at += 4
	var ok bool
	for _, spec := range [][2]int{{2, 4}, {4, 4}} {
		at, ok = skipNationCount(data, at, spec[0], spec[1])
		if !ok {
			return 0, false
		}
	}
	if at+2 > len(data) {
		return 0, false
	}
	count := int(binary.LittleEndian.Uint16(data[at:]))
	at += 2
	for i := 0; i < count; i++ {
		if at >= len(data) {
			return 0, false
		}
		at++
		for j := 0; j < 4; j++ {
			if at+4 > len(data) {
				return 0, false
			}
			n := int(binary.LittleEndian.Uint32(data[at:]))
			if n > 1024 || at+4+n > len(data) {
				return 0, false
			}
			at += 4 + n
		}
		if at+20 > len(data) {
			return 0, false
		}
		at += 20
	}
	if at+48 > len(data) {
		return 0, false
	}
	at += 48
	for _, spec := range [][2]int{{4, 25}, {4, 12}, {4, 14}} {
		at, ok = skipNationCount(data, at, spec[0], spec[1])
		if !ok {
			return 0, false
		}
	}
	if at+24 > len(data) {
		return 0, false
	}
	at += 24
	at, ok = skipNationCount(data, at, 1, 5)
	if !ok {
		return 0, false
	}
	for _, spec := range [][2]int{{4, 28}, {4, 28}, {4, 12}} {
		at, ok = skipNationCount(data, at, spec[0], spec[1])
		if !ok {
			return 0, false
		}
	}
	if at+54 > len(data) {
		return 0, false
	}
	return at + 54, true
}
