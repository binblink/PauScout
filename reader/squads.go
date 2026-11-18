package reader

import (
	"encoding/binary"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var squadAgeRE = regexp.MustCompile(`(?i)(?:\bU|\bUnder[ _-]*|\bSub[ _-]*)(1[3-9]|2[0-3])\b`)

func squadCompetitionLabel(name string) string {
	if match := squadAgeRE.FindStringSubmatch(name); len(match) == 2 {
		return "U" + match[1]
	}
	lower := strings.ToLower(name)
	if strings.Contains(lower, "primavera") {
		return "U20"
	}
	if strings.Contains(lower, "youth") || strings.Contains(lower, "academy") {
		return "Jeunes"
	}
	if strings.Contains(lower, "reserve") || strings.Contains(lower, "b team") {
		return "Réserve"
	}
	return ""
}

// Resolve a category only when a named competition contains a coherent group
// of secondary teams from the same nation. Teams absent from the calendar are
// left for the roster-age fallback.
func resolveSquadLabels(path string, teams map[uint32]teamInfo) map[uint32]string {
	directory, err := ReadDirectory(path)
	if err != nil {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	secondary := make(map[uint32]teamInfo)
	for id, team := range teams {
		if team.slot > 0 {
			secondary[id] = team
		}
	}
	names := competitionNames()
	type candidate struct {
		label string
		count int
	}
	byTeam := make(map[uint32][]candidate)
	for _, entry := range directory.Entries {
		if entry.Group != "rgman" || entry.Extension != ".dat" || !strings.HasPrefix(entry.Name, "comp_") || entry.Uncompressed > maxSectionBytes {
			continue
		}
		id, err := strconv.ParseUint(strings.TrimPrefix(entry.Name, "comp_"), 10, 32)
		if err != nil {
			continue
		}
		label := squadCompetitionLabel(names[uint32(id)])
		if label == "" {
			continue
		}
		data, err := decompressSection(f, Frame{Offset: entry.Offset, Compressed: entry.Compressed})
		if err != nil || uint64(len(data)) != entry.Uncompressed {
			continue
		}
		counts := make(map[uint32]int)
		for at := 0; at+4 <= len(data); at++ {
			teamID := binary.LittleEndian.Uint32(data[at:])
			if _, ok := secondary[teamID]; ok {
				counts[teamID]++
			}
		}
		for _, hit := range leagueMembers(counts, secondary) {
			byTeam[hit.teamID] = append(byTeam[hit.teamID], candidate{label, hit.count})
		}
	}
	result := make(map[uint32]string)
	for id, candidates := range byTeam {
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].count > candidates[j].count })
		if len(candidates) > 1 && candidates[0].label != candidates[1].label && candidates[0].count*100 < candidates[1].count*115 {
			continue
		}
		result[id] = candidates[0].label
	}
	return result
}

func inferredSquadLabel(ages []int) string {
	if len(ages) < 5 {
		return ""
	}
	young, adults, sum := 0, 0, 0
	for _, age := range ages {
		if age <= 19 {
			young++
		}
		if age >= 20 {
			adults++
		}
		sum += age
	}
	if young*10 >= len(ages)*8 {
		return "Jeunes (âge estimé)"
	}
	if adults*10 >= len(ages)*8 && sum >= len(ages)*21 {
		return "Réserve (âge estimé)"
	}
	return ""
}
