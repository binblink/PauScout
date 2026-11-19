package reader

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"
)

var staffAttributeNames = [...]string{
	"authority", "working_with_youngsters", "determination", "goalkeeping",
	"judging_player_ability", "judging_player_potential", "people_management",
	"motivating", "physiotherapy", "tactical_knowledge", "attacking",
	"defending", "fitness", "possession", "technical", "tactical",
	"data_analysis", "sports_science", "negotiating", "judging_staff_ability", "set_pieces",
}

var staffRoleNames = [...]string{
	"manager", "assistant_manager", "coach", "physio", "scout",
	"gk_coach", "fitness_coach", "chairperson", "director_of_football",
	"head_of_youth_development", "loan_manager", "technical_director",
	"data_analyst", "sports_scientist", "set_piece_coach",
}

var staffBlockSlots = [...]int{-1, -1, -1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 18, 21, 23, 24, 25}

type CatalogStaff struct {
	ID          uint32 `json:"uid"`
	Name        string `json:"name"`
	Age         *int   `json:"age,omitempty"`
	BirthDate   string `json:"birth_date,omitempty"`
	Country     string `json:"country,omitempty"`
	Nationality string `json:"nationality,omitempty"`
	League      string `json:"league,omitempty"`
	Club        string `json:"club,omitempty"`
	ClubID      uint32 `json:"club_id,omitempty"`
	TeamID      uint32 `json:"team_id,omitempty"`
	TeamSlot    *int   `json:"team_slot,omitempty"`
	Squad       string `json:"squad,omitempty"`
	Gender      string `json:"gender"`
	Job         string `json:"job,omitempty"`
	CA          uint16 `json:"ca"`
	PA          int16  `json:"pa"`
	AttrsB64    []byte `json:"attrs_b64"`
	RolesB64    []byte `json:"roles_b64,omitempty"`
}

type staffHeader struct {
	at, ability int
	index, id   uint32
}
type staffEmployment struct {
	teamID uint32
	job    byte
	tag    int
}

func staffJobName(code byte) string {
	jobs := map[byte]string{2: "Entraîneur", 4: "Président", 6: "Dirigeant", 8: "Directeur général", 10: "Directeur sportif", 12: "Kinésithérapeute", 14: "Recruteur", 16: "Entraîneur principal", 20: "Entraîneur adjoint", 22: "Entraîneur coups de pied arrêtés", 24: "Directeur général", 26: "Préparateur physique", 34: "Entraîneur des gardiens", 38: "Médecin chef", 40: "Responsable des sciences du sport", 44: "Responsable du recrutement", 46: "Médecin", 48: "Scientifique du sport", 50: "Kinésithérapeute chef", 58: "Analyste de données", 60: "Analyste de performance", 62: "Analyste de données", 64: "Responsable de la formation", 66: "Propriétaire", 70: "Président", 86: "Responsable des prêts", 88: "Directeur technique"}
	return jobs[code]
}

func staffCandidate(data []byte, at int) (staffHeader, bool) {
	if at < 0 || at+87 >= len(data) || data[at+12] != 1 {
		return staffHeader{}, false
	}
	index := binary.LittleEndian.Uint32(data[at:])
	uid := binary.LittleEndian.Uint32(data[at+4:])
	copyUID := binary.LittleEndian.Uint32(data[at+8:])
	if index == 0 || index > 1_000_000 || uid == 0 || uid == ^uint32(0) || copyUID == 0 || copyUID == ^uint32(0) || (uid != copyUID && uid < 2_000_000_000) {
		return staffHeader{}, false
	}
	ability := at + 21 + 7*int(data[at+13])
	if ability+66 > len(data) || data[ability+13] != 12 {
		return staffHeader{}, false
	}
	ca := binary.LittleEndian.Uint16(data[ability:])
	pa := int16(binary.LittleEndian.Uint16(data[ability+2:]))
	if ca < 1 || ca > 200 || pa < 1 || pa > 200 {
		return staffHeader{}, false
	}
	return staffHeader{at: at, ability: ability, index: index}, true
}

func staffClosing(data []byte, h staffHeader) (int, uint32, bool) {
	var needle [4]byte
	binary.LittleEndian.PutUint32(needle[:], h.index+1)
	end := h.at + 32_768
	if end > len(data) {
		end = len(data)
	}
	found, id, matches := -1, uint32(0), 0
	for pos := h.at + 14; pos+12 <= end; {
		rel := bytes.Index(data[pos:end], needle[:])
		if rel < 0 {
			break
		}
		at := pos + rel
		pos = at + 1
		if at+12 > end {
			continue
		}
		uid := binary.LittleEndian.Uint32(data[at+4:])
		if uid == 0 || uid == ^uint32(0) || uid != binary.LittleEndian.Uint32(data[at+8:]) {
			continue
		}
		found, id, matches = at, uid, matches+1
		if matches > 1 {
			return 0, 0, false
		}
	}
	return found, id, matches == 1
}

func staffContract(data []byte, h staffHeader, end int, teams map[uint32]teamInfo) (staffEmployment, bool) {
	var needle [4]byte
	binary.LittleEndian.PutUint32(needle[:], h.index+1)
	var found staffEmployment
	has := false
	for pos := h.at + 5; pos+24 <= end; {
		rel := bytes.Index(data[pos:end], needle[:])
		if rel < 0 {
			break
		}
		hit := pos + rel
		pos = hit + 1
		tag := hit - 5
		if tag < h.at || tag+25 > end || !bytes.Equal(data[tag:tag+4], []byte{1, 0, 0x6c, 7}) {
			continue
		}
		if hit+23 <= end && bytes.Equal(data[hit+19:hit+23], []byte{1, 0, 0x6c, 7}) {
			continue
		}
		teamID := binary.LittleEndian.Uint32(data[tag+9:])
		if _, ok := teams[teamID]; !ok {
			continue
		}
		job := data[hit+16]
		if staffJobName(job) == "" {
			continue
		}
		found = staffEmployment{teamID: teamID, job: job, tag: tag}
		has = true
	}
	if has {
		return found, true
	}
	for pos := h.at + 16; pos+20 <= end; {
		rel := bytes.Index(data[pos:end], needle[:])
		if rel < 0 {
			break
		}
		hit := pos + rel
		pos = hit + 1
		tag := hit - 16
		if tag < h.at || tag+33 > end || !bytes.Equal(data[tag:tag+4], []byte{1, 0, 0x6c, 7}) || !bytes.Equal(data[tag+14:tag+16], []byte{5, 5}) {
			continue
		}
		teamID := binary.LittleEndian.Uint32(data[tag+20:])
		if _, ok := teams[teamID]; !ok {
			continue
		}
		job := data[tag+32]
		if staffJobName(job) == "" {
			continue
		}
		return staffEmployment{teamID: teamID, job: job, tag: tag}, true
	}
	return staffEmployment{}, false
}

func staffAttributes(data []byte, ability int) ([]byte, bool) {
	if ability < 0 || ability+66 > len(data) {
		return nil, false
	}
	values := make([]byte, len(staffAttributeNames))
	pref := ability + 14
	block := ability + 40
	if data[pref+4] < 1 || data[pref+4] > 20 || data[pref+12] < 1 || data[pref+12] > 20 || data[pref+13] < 1 || data[pref+13] > 100 {
		return nil, false
	}
	for _, v := range data[block : block+26] {
		if v < 1 || v > 100 {
			return nil, false
		}
	}
	values[0] = data[pref+4]
	values[1] = data[pref+12]
	values[2] = displayAttribute(data[pref+13])
	for i := 3; i < len(values); i++ {
		values[i] = displayAttribute(data[block+staffBlockSlots[i]])
	}
	return values, true
}

func staffRoles(data []byte, h staffHeader, end int) ([]byte, int) {
	if end > h.at+4096 {
		end = h.at + 4096
	}
	var found []byte
	foundAt := -1
	for at := h.at + 1; at+19 <= end; at++ {
		if data[at-1] != 1 || !bytes.Equal(data[at+15:at+19], []byte{0, 0, 0, 0}) {
			continue
		}
		max := byte(0)
		valid := true
		for _, v := range data[at : at+15] {
			if v < 1 || v > 20 {
				valid = false
				break
			}
			if v > max {
				max = v
			}
		}
		if !valid || max < 10 {
			continue
		}
		if foundAt >= 0 {
			return nil, -1
		}
		found = append([]byte(nil), data[at:at+15]...)
		foundAt = at
	}
	return found, foundAt
}

func staffIdentity(data []byte, h staffHeader, closing int, id uint32, employment staffEmployment, pools namePools) (PlayerIdentity, error) {
	if employment.tag >= 0 {
		start := employment.tag + 60
		end := employment.tag + 4096
		if end > closing {
			end = closing
		}
		if start < end {
			if identity, err := findPersonIdentity(data, start, end, pools, id); err == nil {
				return identity, nil
			}
		}
	}
	return findPersonIdentity(data, h.at+13, closing, pools, id)
}

func readStaffCatalogData(data []byte, pools namePools, clock time.Time, teams map[uint32]teamInfo, countries map[uint32]string, leagues map[uint32]string, squads map[uint32]string, layout map[uint32]recordOffsets) []CatalogStaff {
	result := make([]CatalogStaff, 0, 16_000)
	seen := make(map[uint32]bool)
	for pos := 12; pos < len(data); {
		rel := bytes.IndexByte(data[pos:], 1)
		if rel < 0 {
			break
		}
		kindAt := pos + rel
		pos = kindAt + 1
		h, ok := staffCandidate(data, kindAt-12)
		if !ok {
			continue
		}
		closing, id, ok := staffClosing(data, h)
		if !ok || seen[id] {
			continue
		}
		employment, ok := staffContract(data, h, closing, teams)
		if !ok {
			continue
		}
		attrs, ok := staffAttributes(data, h.ability)
		if !ok {
			continue
		}
		identity, err := staffIdentity(data, h, closing, id, employment, pools)
		if err != nil {
			continue
		}
		team := teams[employment.teamID]
		staff := CatalogStaff{ID: id, Name: identity.Name, Age: ageAt(identity.BirthDate, clock), Country: countries[team.nationID], Nationality: countries[uint32(identity.NationID)], League: leagues[team.clubIndex], Club: team.clubName, ClubID: team.clubIndex, TeamID: employment.teamID, Job: staffJobName(employment.job), CA: binary.LittleEndian.Uint16(data[h.ability:]), PA: int16(binary.LittleEndian.Uint16(data[h.ability+2:])), AttrsB64: attrs}
		staff.BirthDate = identity.BirthDate.Format("2006-01-02")
		staff.TeamSlot = &team.slot
		staff.Squad = squads[employment.teamID]
		if staff.Squad == "" {
			if team.slot == 0 {
				staff.Squad = "Équipe première"
			} else {
				staff.Squad = fmt.Sprintf("Équipe %d (ID %d)", team.slot+1, employment.teamID)
			}
		}
		if closing >= 7 && data[closing-7]&0x10 != 0 {
			staff.Gender = "female"
		} else {
			staff.Gender = "male"
		}
		roles, rolesAt := staffRoles(data, h, closing)
		if roles != nil {
			staff.RolesB64 = roles
		}
		if layout != nil {
			offsets := recordOffsets{Closing: closing - h.ability, Birth: identity.personalityOffset - 22 - h.ability}
			if roles != nil {
				offsets.Roles = rolesAt - h.ability
			}
			layout[id] = offsets
		}
		result = append(result, staff)
		seen[id] = true
	}
	return result
}

func StaffAttributeOrder() []string { return append([]string(nil), staffAttributeNames[:]...) }
func StaffRoleOrder() []string      { return append([]string(nil), staffRoleNames[:]...) }
