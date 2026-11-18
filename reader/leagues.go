package reader

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/csv"
	"os"
	"sort"
	"strconv"
	"strings"
)

// currentClubLeagues reads the current competition entry in full club records.
// Competition IDs in these records are save-specific and must first be mapped
// to database IDs; competition section names use database IDs directly.
func currentClubLeagues(data []byte) map[uint32]string {
	names := competitionNames()
	marker := append(bytes.Repeat([]byte{0xff}, 16), 1)
	ids := map[uint32]uint32{}
	conflicts := map[uint32]bool{}
	for pos := 0; pos+len(marker) <= len(data); {
		hit := bytes.Index(data[pos:], marker)
		if hit < 0 {
			break
		}
		at := pos + hit + 30
		pos += hit + 1
		if at < 8 || at+60 >= len(data) {
			continue
		}
		if data[at-8] != 7 || data[at-6] != 0 || data[at-4] != 7 || data[at-1] != 255 || data[at+12] != 0 || data[at+15] != 1 || data[at+22] != 0 || data[at+23] != 0 || data[at+24] != 0 || data[at+25] != 0 || data[at+26] != 1 || data[at+32] != 2 || data[at+40] != 3 || data[at+55] != 6 || data[at+60] != 7 {
			continue
		}
		id := binary.LittleEndian.Uint32(data[at:])
		db := binary.LittleEndian.Uint32(data[at+4:])
		if id == 0 || id >= 200000 || db == 0 || db != binary.LittleEndian.Uint32(data[at+8:]) {
			continue
		}
		if old, ok := ids[id]; ok && old != db {
			conflicts[id] = true
		} else {
			ids[id] = db
		}
	}
	for id := range conflicts {
		delete(ids, id)
	}
	records := scanClubRecords(data)
	result := map[uint32]string{}
	for i, record := range records {
		end := len(data)
		if i+1 < len(records) {
			end = records[i+1].start
		}
		if end-record.start < 520 {
			continue
		}
		found := ""
		ambiguous := false
		for at := record.start + 250; at < record.start+520 && at+5 <= end; at++ {
			if data[at-5] != 1 || data[at+4] != 1 {
				continue
			}
			id := binary.LittleEndian.Uint32(data[at:])
			name := names[ids[id]]
			if !isDomesticLeague(name) {
				continue
			}
			if found != "" && found != name {
				ambiguous = true
				break
			}
			found = name
		}
		if !ambiguous && found != "" {
			result[record.index] = found
		}
	}
	return result
}

//go:embed data/competition-names.csv
var competitionCSV string

func competitionNames() map[uint32]string {
	rows, err := csv.NewReader(strings.NewReader(competitionCSV)).ReadAll()
	if err != nil {
		return nil
	}
	known := make(map[uint32]string, len(rows))
	for _, row := range rows[1:] {
		if len(row) < 2 {
			continue
		}
		id, err := strconv.ParseUint(row[0], 10, 32)
		if err == nil {
			known[uint32(id)] = row[1]
		}
	}
	return known
}

func isDomesticLeague(name string) bool {
	s := strings.ToLower(name)
	for _, excluded := range []string{"cup", "trophy", "shield", "champions league", "qualif", "friendly", "international", "play-off", "playoff", "supercopa", "supercoppa", "u18", "u19", "u20", "u21", "u23", "youth"} {
		if strings.Contains(s, excluded) {
			return false
		}
	}
	for _, marker := range []string{"league", "division", "divisie", "ligue", "liga", "serie", "bundesliga", "championship", "premiership", "superlig", "national", "j1", "j2", "j3"} {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}

type leagueHit struct {
	teamID, nation uint32
	count          int
}

// A league section contains repeated first-team IDs from fixtures and
// standings. Accept only the largest same-nation group with similar counts.
// This rejects byte-pattern artefacts such as 256 and 512, whose counts are
// far above the actual clubs in the same section.
func leagueMembers(counts map[uint32]int, firstTeams map[uint32]teamInfo) []leagueHit {
	byNation := make(map[uint32][]leagueHit)
	for id, count := range counts {
		if count < 25 {
			continue
		}
		team := firstTeams[id]
		byNation[team.nationID] = append(byNation[team.nationID], leagueHit{id, team.nationID, count})
	}
	var best []leagueHit
	secondSize := 0
	for _, rows := range byNation {
		sort.Slice(rows, func(i, j int) bool { return rows[i].count < rows[j].count })
		var nationBest []leagueHit
		for left, right := 0, 0; left < len(rows); left++ {
			if right < left {
				right = left
			}
			for right < len(rows) && rows[right].count*5 <= rows[left].count*8 {
				right++
			}
			width := right - left
			if width > len(nationBest) {
				nationBest = rows[left:right]
			}
		}
		if len(nationBest) > len(best) {
			secondSize = len(best)
			best = nationBest
		} else if len(nationBest) > secondSize {
			secondSize = len(nationBest)
		}
	}
	if len(best) < 8 || secondSize >= len(best) {
		return nil
	}
	return best
}

// resolveClubLeagues uses the save's named competition sections. A club is
// labelled only when one domestic league dominates its competition records.
func resolveClubLeagues(path string, teams map[uint32]teamInfo) map[uint32]string {
	directory, err := ReadDirectory(path)
	if err != nil {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	firstTeams := make(map[uint32]teamInfo)
	for id, team := range teams {
		if team.slot == 0 {
			firstTeams[id] = team
		}
	}
	names := competitionNames()
	type candidate struct {
		name  string
		count int
	}
	byClub := make(map[uint32][]candidate)
	for _, entry := range directory.Entries {
		if entry.Group != "rgman" || entry.Extension != ".dat" || !strings.HasPrefix(entry.Name, "comp_") || entry.Uncompressed > maxSectionBytes {
			continue
		}
		id, err := strconv.ParseUint(strings.TrimPrefix(entry.Name, "comp_"), 10, 32)
		if err != nil {
			continue
		}
		name := names[uint32(id)]
		if !isDomesticLeague(name) {
			continue
		}
		data, err := decompressSection(f, Frame{Offset: entry.Offset, Compressed: entry.Compressed})
		if err != nil || uint64(len(data)) != entry.Uncompressed {
			continue
		}
		counts := make(map[uint32]int)
		for at := 0; at+4 <= len(data); at++ {
			teamID := binary.LittleEndian.Uint32(data[at:])
			if _, ok := firstTeams[teamID]; ok {
				counts[teamID]++
			}
		}
		for _, hit := range leagueMembers(counts, firstTeams) {
			clubID := firstTeams[hit.teamID].clubIndex
			byClub[clubID] = append(byClub[clubID], candidate{name, hit.count})
		}
	}
	result := make(map[uint32]string)
	for clubID, candidates := range byClub {
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].count > candidates[j].count })
		if len(candidates) > 1 && candidates[0].count*100 < candidates[1].count*115 {
			continue
		}
		result[clubID] = candidates[0].name
	}
	return result
}
