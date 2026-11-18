package reader

import (
	"bytes"
	"encoding/binary"
	"unicode/utf8"
)

type clubRecord struct {
	start    int
	name     string
	index    uint32
	nationID uint32
}

type teamInfo struct {
	clubName  string
	clubIndex uint32
	nationID  uint32
	slot      int
}

var clubAnchor = []byte{0xff, 0xff, 0xff, 0xff}
var nullDateTriple = []byte{1, 0, 0x6c, 7, 1, 0, 0x6c, 7, 1, 0, 0x6c, 7}
var floatOne = []byte{0, 0, 0x80, 0x3f}

func clubNameAt(data []byte, at int) (string, bool) {
	if at < 0 || at+4 > len(data) {
		return "", false
	}
	n := int(binary.LittleEndian.Uint32(data[at:]))
	if n < 1 || n > 255 || at+4+n > len(data) || !utf8.Valid(data[at+4:at+4+n]) {
		return "", false
	}
	name := string(data[at+4 : at+4+n])
	for _, r := range name {
		if r < 32 || r == 127 {
			return "", false
		}
	}
	return name, true
}

func clubRecordAt(data []byte, start int) (clubRecord, bool) {
	if start < 0 || start+43 > len(data) || data[start+12] != 0 {
		return clubRecord{}, false
	}
	word := func(off int) uint32 { return binary.LittleEndian.Uint32(data[start+off:]) }
	if word(4) != word(8) || word(0) > 200_000 {
		return clubRecord{}, false
	}
	for _, off := range []int{13, 21, 25} {
		if word(off) > 999 {
			return clubRecord{}, false
		}
	}
	name, ok := clubNameAt(data, start+39)
	if !ok {
		return clubRecord{}, false
	}
	shortAt := start + 43 + len(name)
	if _, ok := clubNameAt(data, shortAt); !ok {
		return clubRecord{}, false
	}
	return clubRecord{start: start, name: name, index: word(0) + 1, nationID: word(13)}, true
}

func scanClubRecords(data []byte) []clubRecord {
	var records []clubRecord
	last := -1
	for pos := 17; pos < len(data); {
		hit := bytes.Index(data[pos:], clubAnchor)
		if hit < 0 {
			break
		}
		at := pos + hit
		pos = at + 1
		start := at - 17
		if last >= 0 && start-last >= 64*1024 {
			break
		}
		if record, ok := clubRecordAt(data, start); ok {
			records = append(records, record)
			last = start
		}
	}
	return records
}

func parseTeamIDs(data []byte, start, end int) ([]uint32, bool) {
	if start < 0 || start+37 >= end {
		return nil, false
	}
	at := start + 37
	for _, entry := range []struct {
		size int
		lead byte
	}{{25, 2}, {12, 1}} {
		if at >= end {
			return nil, false
		}
		count := int(data[at])
		at++
		if count > 64 || at+count*entry.size > end {
			return nil, false
		}
		for i := 0; i < count; i++ {
			if data[at+i*entry.size] != entry.lead {
				return nil, false
			}
		}
		at += count * entry.size
	}
	at += 9
	if at >= end || data[at] < 1 || data[at] > 8 {
		return nil, false
	}
	count := int(data[at])
	at++
	if at+4*count > end {
		return nil, false
	}
	ids := make([]uint32, 0, count)
	seen := make(map[uint32]bool, count)
	for i := 0; i < count; i++ {
		id := binary.LittleEndian.Uint32(data[at+i*4:])
		if id < 1 || id > 3_000_000 || seen[id] {
			return nil, false
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, true
}

func clubTeamIDs(data []byte, start, end int) []uint32 {
	bestAt := end
	var best []uint32
	for at := start; at < end; {
		hit := bytes.Index(data[at:end], nullDateTriple)
		if hit < 0 {
			break
		}
		candidate := at + hit
		at = candidate + 1
		if candidate >= bestAt {
			break
		}
		if ids, ok := parseTeamIDs(data, candidate, end); ok {
			bestAt, best = candidate, ids
		}
	}
	for at := start + 40; at < end; {
		hit := bytes.Index(data[at:end], floatOne)
		if hit < 0 {
			break
		}
		anchor := at + hit
		at = anchor + 1
		candidate := anchor - 33
		if candidate >= bestAt {
			continue
		}
		if ids, ok := parseTeamIDs(data, candidate, end); ok {
			bestAt, best = candidate, ids
		}
	}
	return best
}

// teamClubInfo resolves a registered team ID to its club and its ordered squad slot.
func teamClubInfo(data []byte) map[uint32]teamInfo {
	records := scanClubRecords(data)
	teams := make(map[uint32]teamInfo)
	ambiguous := make(map[uint32]bool)
	for i, record := range records {
		end := len(data)
		if i+1 < len(records) {
			end = records[i+1].start
		} else if record.start+64*1024 < end {
			end = record.start + 64*1024
		}
		for slot, id := range clubTeamIDs(data, record.start, end) {
			info := teamInfo{clubName: record.name, clubIndex: record.index, nationID: record.nationID, slot: slot}
			if previous, ok := teams[id]; ok && previous != info {
				ambiguous[id] = true
			}
			teams[id] = info
		}
	}
	for id := range ambiguous {
		delete(teams, id)
	}
	return teams
}
