package reader

import (
	"fmt"
	"path/filepath"
	"sort"
)

type recordOffsets struct {
	Closing int `json:"closing_header"`
	Birth   int `json:"birth_date,omitempty"`
	Traits  int `json:"traits,omitempty"`
	Roles   int `json:"staff_roles,omitempty"`
}

type catalogLayout struct {
	players map[uint32]recordOffsets
	staff   map[uint32]recordOffsets
}

type SaveInspection struct {
	File           string           `json:"file"`
	Build          string           `json:"game_build,omitempty"`
	GameDate       string           `json:"game_date,omitempty"`
	Sections       int              `json:"sections"`
	GameDBSchema   *uint16          `json:"game_db_schema,omitempty"`
	GameDBBytes    uint64           `json:"game_db_bytes,omitempty"`
	Coverage       *CatalogCoverage `json:"coverage,omitempty"`
	PlayerRecords  int              `json:"player_records"`
	StaffRecords   int              `json:"staff_records"`
	EditingEnabled bool             `json:"editing_enabled"`
	ReadError      string           `json:"read_error,omitempty"`
	entries        []DirectoryEntry
	layout         *catalogLayout
}

type sectionCount struct {
	Section string `json:"section"`
	Count   int    `json:"count"`
}

type SectionComparison struct {
	Shared  int            `json:"shared"`
	Resized int            `json:"shared_with_different_decoded_size"`
	Added   []sectionCount `json:"added,omitempty"`
	Removed []sectionCount `json:"removed,omitempty"`
}

type offsetChange struct {
	ID     uint32        `json:"id"`
	Before recordOffsets `json:"before"`
	After  recordOffsets `json:"after"`
}

type RecordComparison struct {
	Shared         int            `json:"shared"`
	Changed        int            `json:"records_with_different_offsets"`
	ClosingChanged int            `json:"closing_header_changes"`
	BirthChanged   int            `json:"birth_date_changes"`
	TraitsChanged  int            `json:"traits_changes,omitempty"`
	RolesChanged   int            `json:"staff_role_changes,omitempty"`
	Examples       []offsetChange `json:"examples,omitempty"`
}

type CompatibilityReport struct {
	Before       SaveInspection     `json:"before"`
	After        SaveInspection     `json:"after"`
	SameGameDate *bool              `json:"same_game_date,omitempty"`
	Sections     *SectionComparison `json:"section_comparison,omitempty"`
	Players      *RecordComparison  `json:"player_offsets_from_ca,omitempty"`
	Staff        *RecordComparison  `json:"staff_offsets_from_ca,omitempty"`
	Notes        []string           `json:"notes"`
}

func inspectSave(path string) (result SaveInspection) {
	result.File = filepath.Base(path)
	summary, err := Read(path)
	if err != nil {
		result.ReadError = err.Error()
		return result
	}
	result.Build, result.GameDate = summary.GameVersion, summary.GameDate
	directory, err := ReadDirectory(path)
	if err != nil {
		result.ReadError = err.Error()
		return result
	}
	result.entries, result.Sections = directory.Entries, len(directory.Entries)
	entry, err := gameDBEntry(directory)
	if err != nil {
		result.ReadError = err.Error()
		return result
	}
	result.GameDBBytes = entry.Uncompressed
	data, err := readGameDBSection(path, entry)
	if err != nil {
		result.ReadError = err.Error()
		return result
	}
	schema, ok := gameDBSchema(data)
	if !ok {
		result.ReadError = "signature game_db illisible"
		return result
	}
	result.GameDBSchema = &schema
	if schema != supportedGameDBSchema {
		result.ReadError = fmt.Sprintf("schéma game_db %d non pris en charge", schema)
		return result
	}
	layout := &catalogLayout{players: make(map[uint32]recordOffsets), staff: make(map[uint32]recordOffsets)}
	catalog, err := readCatalog(path, data, summary.GameDate, layout)
	if err != nil {
		result.ReadError = err.Error()
		return result
	}
	result.Coverage = &catalog.Coverage
	result.PlayerRecords, result.StaffRecords = len(catalog.Players), len(catalog.Staff)
	result.EditingEnabled = SupportsWriteVersion(result.Build)
	result.layout = layout
	return result
}

func groupSections(entries []DirectoryEntry) map[string][]uint64 {
	grouped := make(map[string][]uint64)
	for _, entry := range entries {
		name := entry.Name + entry.Extension
		if entry.Group != "" {
			name = entry.Group + "/" + name
		}
		grouped[name] = append(grouped[name], entry.Uncompressed)
	}
	return grouped
}

func compareSections(before, after []DirectoryEntry) *SectionComparison {
	old, new := groupSections(before), groupSections(after)
	result := &SectionComparison{}
	for name, sizes := range old {
		shared := min(len(sizes), len(new[name]))
		result.Shared += shared
		for i := 0; i < shared; i++ {
			if sizes[i] != new[name][i] {
				result.Resized++
			}
		}
		if len(sizes) > shared {
			result.Removed = append(result.Removed, sectionCount{name, len(sizes) - shared})
		}
	}
	for name, sizes := range new {
		if count := len(sizes) - len(old[name]); count > 0 {
			result.Added = append(result.Added, sectionCount{name, count})
		}
	}
	sort.Slice(result.Removed, func(i, j int) bool { return result.Removed[i].Section < result.Removed[j].Section })
	sort.Slice(result.Added, func(i, j int) bool { return result.Added[i].Section < result.Added[j].Section })
	return result
}

func compareRecords(before, after map[uint32]recordOffsets) *RecordComparison {
	result := &RecordComparison{}
	var changedIDs []uint32
	for id, a := range before {
		b, ok := after[id]
		if !ok {
			continue
		}
		result.Shared++
		if a == b {
			continue
		}
		changedIDs = append(changedIDs, id)
		if a.Closing != b.Closing {
			result.ClosingChanged++
		}
		if a.Birth != b.Birth {
			result.BirthChanged++
		}
		if a.Traits != b.Traits {
			result.TraitsChanged++
		}
		if a.Roles != b.Roles {
			result.RolesChanged++
		}
	}
	result.Changed = len(changedIDs)
	sort.Slice(changedIDs, func(i, j int) bool { return changedIDs[i] < changedIDs[j] })
	for _, id := range changedIDs[:min(len(changedIDs), 8)] {
		result.Examples = append(result.Examples, offsetChange{id, before[id], after[id]})
	}
	return result
}

// CompareSaves reports relative structural observations without writing either save.
func CompareSaves(beforePath, afterPath string) CompatibilityReport {
	before, after := inspectSave(beforePath), inspectSave(afterPath)
	report := CompatibilityReport{
		Before: before,
		After:  after,
		Notes: []string{
			"Les tailles et positions internes peuvent varier avec le contenu. Compare si possible la même partie à la même date en jeu.",
			"Les offsets observés sont relatifs au CA. Un champ absent n’a pas pu être localisé dans la fiche.",
			"Le rapport décrit la lecture par PauScout. La validation d’un nouveau build pour l’édition demande une ouverture dans FM et la vérification des valeurs modifiées.",
		},
	}
	if before.GameDate != "" && after.GameDate != "" {
		same := before.GameDate == after.GameDate
		report.SameGameDate = &same
	}
	if before.entries != nil && after.entries != nil {
		report.Sections = compareSections(before.entries, after.entries)
	}
	if before.layout != nil && after.layout != nil {
		report.Players = compareRecords(before.layout.players, after.layout.players)
		report.Staff = compareRecords(before.layout.staff, after.layout.staff)
	}
	report.Before.entries, report.Before.layout = nil, nil
	report.After.entries, report.After.layout = nil, nil
	return report
}
