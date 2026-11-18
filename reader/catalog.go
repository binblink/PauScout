package reader

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"time"
)

// CatalogPlayer is a validated save record with a resolved database ID and name.
type CatalogPlayer struct {
	ID          uint32           `json:"uid"`
	Name        string           `json:"name"`
	NameKnown   bool             `json:"name_known"`
	BirthDate   string           `json:"birth_date,omitempty"`
	Value       *uint32          `json:"value,omitempty"`
	Age         *int             `json:"age,omitempty"`
	Position    string           `json:"position,omitempty"`
	Positions   map[string]uint8 `json:"positions_ratings,omitempty"`
	TeamID      uint32           `json:"team_id,omitempty"`
	Club        string           `json:"club,omitempty"`
	ClubID      uint32           `json:"club_id,omitempty"`
	Country     string           `json:"country,omitempty"`
	Nationality string           `json:"nationality,omitempty"`
	League      string           `json:"league,omitempty"`
	Squad       string           `json:"squad,omitempty"`
	TeamSlot    *int             `json:"team_slot,omitempty"`
	Gender      string           `json:"gender"`
	CA          uint16           `json:"ca"`
	PA          int16            `json:"pa"`
	AttrsB64    []byte           `json:"attrs_b64"`
	Personality map[string]uint8 `json:"personality,omitempty"`
	Traits      []int            `json:"traits,omitempty"`
	TraitsKnown bool             `json:"traits_known"`
	Height      uint8            `json:"height"`
	HomeRep     uint16           `json:"home_rep"`
	CurrentRep  uint16           `json:"current_rep"`
	WorldRep    uint16           `json:"world_rep"`
}

type CatalogCoverage struct {
	Indexed    uint32 `json:"indexed"`
	Candidates int    `json:"candidates"`
	WithID     int    `json:"with_id"`
	WithName   int    `json:"with_name"`
	WithClub   int    `json:"with_club"`
	Men        int    `json:"men"`
	Women      int    `json:"women"`
	Markerless int    `json:"markerless"`
}

type PlayerCatalog struct {
	Players  []CatalogPlayer `json:"players"`
	Staff    []CatalogStaff  `json:"staff"`
	Coverage CatalogCoverage `json:"coverage"`
	EuroRate *float64        `json:"euro_rate,omitempty"`
}

func validPlayerStart(data []byte, start int) bool {
	if start < 19 || start+playerDecodeExtent > len(data) {
		return false
	}
	uid := binary.LittleEndian.Uint32(data[start-15:])
	copyUID := binary.LittleEndian.Uint32(data[start-11:])
	kind := data[start-7]
	ca := binary.LittleEndian.Uint16(data[start:])
	pa := int16(binary.LittleEndian.Uint16(data[start+2:]))
	if uid == 0 || uid == ^uint32(0) || copyUID == 0 || copyUID == ^uint32(0) || uid != copyUID && (uid < 2_000_000_000 || kind != 2 && kind != 3) || ca > 200 || pa < -10 || pa > 200 || binary.LittleEndian.Uint32(data[start-19:]) > 1_000_000 {
		return false
	}
	for _, value := range data[start+24 : start+39] {
		if value < 1 || value > 20 {
			return false
		}
	}
	for _, value := range data[start+39 : start+93] {
		if value < 1 || value > 125 {
			return false
		}
	}
	return true
}

func candidateStarts(data []byte) ([]int, int) {
	starts := make(map[int]struct{})
	for pos := 0; pos < len(data); {
		rel := bytes.Index(data[pos:], playerMarker)
		if rel < 0 {
			break
		}
		marker := pos + rel
		pos = marker + 1
		start := marker - playerMarkerOffset
		if validPlayerStart(data, start) {
			starts[start] = struct{}{}
		}
	}
	markerless := 0
	for ratings := 24; ratings+69 < len(data); ratings++ {
		start := ratings - 24
		if _, ok := starts[start]; ok || data[ratings] < 1 || data[ratings] > 20 {
			continue
		}
		if !validPlayerStart(data, start) {
			continue
		}
		starts[start] = struct{}{}
		markerless++
	}
	ordered := make([]int, 0, len(starts))
	for start := range starts {
		ordered = append(ordered, start)
	}
	sort.Ints(ordered)
	return ordered, markerless
}

func closingHeader(data []byte, start, end int) (int, uint32, bool) {
	if end > len(data) {
		end = len(data)
	}
	if start+playerDecodeExtent >= end {
		return 0, 0, false
	}
	searchEnd := end
	if searchEnd-start > maxPersonWindow {
		searchEnd = start + maxPersonWindow
	}
	pindex := binary.LittleEndian.Uint32(data[start-19:])
	if pindex == ^uint32(0) {
		return 0, 0, false
	}
	uidFloor := binary.LittleEndian.Uint32(data[start-15:])
	var needle [4]byte
	binary.LittleEndian.PutUint32(needle[:], pindex+1)
	foundAt, foundID, matches := -1, uint32(0), 0
	exactAt, exactID := -1, uint32(0)
	for pos := start + playerDecodeExtent; pos+12 <= searchEnd; {
		rel := bytes.Index(data[pos:searchEnd], needle[:])
		if rel < 0 {
			break
		}
		at := pos + rel
		pos = at + 1
		if at+12 > searchEnd {
			continue
		}
		id := binary.LittleEndian.Uint32(data[at+4:])
		if id == 0 || id == ^uint32(0) || id != binary.LittleEndian.Uint32(data[at+8:]) || uidFloor < 2_000_000_000 && id <= uidFloor {
			continue
		}
		matches++
		if at == end-19 {
			exactAt, exactID = at, id
		}
		foundAt, foundID = at, id
	}
	if matches > 1 {
		return exactAt, exactID, exactAt >= 0
	}
	return foundAt, foundID, matches == 1
}

// ReadPlayerCatalog returns validated records and explicit coverage. It does
// not claim that every ACTUAL_PLAYER entry has been decoded.
func ReadPlayerCatalog(path string) (*PlayerCatalog, error) {
	data, err := readGameDB(path)
	if err != nil {
		return nil, err
	}
	pools, err := locateNamePools(data)
	if err != nil {
		return nil, err
	}
	index, err := ReadIndex(path)
	if err != nil {
		return nil, err
	}
	summary, err := Read(path)
	if err != nil {
		return nil, err
	}
	clock, _ := time.Parse("2006-01-02", summary.GameDate)
	clubsByTeam := teamClubInfo(data)
	countries := nationNames(data)
	leagues := resolveClubLeagues(path, clubsByTeam)
	if leagues == nil {
		leagues = map[uint32]string{}
	}
	for id, name := range currentClubLeagues(data) {
		leagues[id] = name
	}
	squadsByTeam := resolveSquadLabels(path, clubsByTeam)
	result := &PlayerCatalog{EuroRate: euroRate(data)}
	for _, entry := range index {
		if entry.Name == "ACTUAL_PLAYER" {
			result.Coverage.Indexed = entry.Count
		}
	}
	starts, markerless := candidateStarts(data)
	result.Coverage.Candidates = len(starts)
	result.Coverage.Markerless = markerless
	seenIDs := make(map[uint32]struct{})
	for i, start := range starts {
		end := len(data)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		closing, id, ok := closingHeader(data, start, end)
		if !ok {
			continue
		}
		result.Coverage.WithID++
		if _, duplicate := seenIDs[id]; duplicate {
			continue
		}
		ability := PlayerAbility{ID: id, CA: binary.LittleEndian.Uint16(data[start:]), PA: int16(binary.LittleEndian.Uint16(data[start+2:])), recordOffset: start, closingOffset: closing}
		identity, err := findPlayerIdentity(data, ability, pools)
		seenIDs[id] = struct{}{}
		attrs := make([]byte, len(attributeNames))
		for key := range attributeNames {
			slot := key
			if key >= 24 {
				slot += 2
			}
			attrs[key] = displayAttribute(data[start+39+slot])
		}
		name, known := identity.Name, err == nil
		if !known {
			name = fmt.Sprintf("Joueur ID %d", id)
		} else {
			result.Coverage.WithName++
		}
		player := CatalogPlayer{ID: id, Name: name, NameKnown: known, CA: ability.CA, PA: ability.PA, AttrsB64: attrs, Position: displayPositions(data[start+24 : start+39])}
		value := binary.LittleEndian.Uint32(data[start+93:])
		if value != 0 && value != 300_000_000 && value != ^uint32(0) {
			player.Value = &value
		}
		player.Height = data[start+121]
		player.HomeRep = binary.LittleEndian.Uint16(data[start-6:])
		player.CurrentRep = binary.LittleEndian.Uint16(data[start-4:])
		player.WorldRep = binary.LittleEndian.Uint16(data[start-2:])
		player.Positions = make(map[string]uint8, len(positionNames))
		for i, name := range positionNames {
			player.Positions[name] = data[start+24+i]
		}
		if data[closing-7]&0x10 != 0 {
			player.Gender = "female"
			result.Coverage.Women++
		} else {
			player.Gender = "male"
			result.Coverage.Men++
		}
		teamID := binary.LittleEndian.Uint32(data[start+16:])
		if teamID != ^uint32(0) {
			player.TeamID = teamID
			if team, ok := clubsByTeam[teamID]; ok {
				player.Club = team.clubName
				player.ClubID = team.clubIndex
				player.Country = countries[team.nationID]
				player.League = leagues[team.clubIndex]
				player.TeamSlot = &team.slot
				if team.slot == 0 {
					player.Squad = "Équipe première"
					// Pau's B/U19 IDs are supported by the roster ages in the
					// observed save. Other clubs retain their numbered team slots.
				} else if team.clubName == "Pau FC" && teamID == 14871 {
					player.Squad = "Équipe B"
				} else if team.clubName == "Pau FC" && teamID == 28964 {
					player.Squad = "U19"
				} else if label := squadsByTeam[teamID]; label != "" {
					player.Squad = label
				} else {
					player.Squad = fmt.Sprintf("Équipe %d (ID %d)", team.slot+1, teamID)
				}
				result.Coverage.WithClub++
			}
		}
		if known {
			if identity.traitsOffset >= start && identity.traitsOffset+8 <= closing {
				player.TraitsKnown = true
				bits := binary.LittleEndian.Uint64(data[identity.traitsOffset:])
				for i := 0; i < 64; i++ {
					if bits&(uint64(1)<<i) != 0 {
						player.Traits = append(player.Traits, i)
					}
				}
			}
			player.BirthDate = identity.BirthDate.Format("2006-01-02")
			player.Age = ageAt(identity.BirthDate, clock)
			player.Nationality = countries[uint32(identity.NationID)]
			player.Personality = personalityValues(identity)
		}
		result.Players = append(result.Players, player)
	}
	agesByTeam := make(map[uint32][]int)
	for _, player := range result.Players {
		if player.TeamSlot != nil && *player.TeamSlot > 0 && player.Age != nil {
			agesByTeam[player.TeamID] = append(agesByTeam[player.TeamID], *player.Age)
		}
	}
	for i := range result.Players {
		player := &result.Players[i]
		if player.TeamSlot == nil || *player.TeamSlot == 0 || player.Squad != fmt.Sprintf("Équipe %d (ID %d)", *player.TeamSlot+1, player.TeamID) {
			continue
		}
		if label := inferredSquadLabel(agesByTeam[player.TeamID]); label != "" {
			player.Squad = label
		}
	}
	if result.Coverage.Indexed == 0 {
		return nil, fmt.Errorf("ACTUAL_PLAYER absent de l'index")
	}
	staffSquads := make(map[uint32]string)
	for _, player := range result.Players {
		if player.TeamID != 0 && player.Squad != "" {
			staffSquads[player.TeamID] = player.Squad
		}
	}
	result.Staff = readStaffCatalogData(data, pools, clock, clubsByTeam, countries, leagues, staffSquads)
	return result, nil
}
