package reader

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

type StaffEdit struct {
	ID        uint32           `json:"uid"`
	CA        *uint16          `json:"ca,omitempty"`
	PA        *int16           `json:"pa,omitempty"`
	Attrs     map[string]uint8 `json:"attrs,omitempty"`
	Roles     map[string]uint8 `json:"roles,omitempty"`
	BirthDate *string          `json:"birth_date,omitempty"`
}

type staffLocation struct{ ability, roles, birth int }

func staffEditLocations(data []byte, edits []StaffEdit) (map[uint32]staffLocation, error) {
	wanted := make(map[uint32]bool, len(edits))
	for _, edit := range edits {
		if edit.ID == 0 || wanted[edit.ID] {
			return nil, fmt.Errorf("ID staff absent ou répété : %d", edit.ID)
		}
		wanted[edit.ID] = true
	}
	teams := teamClubInfo(data)
	pools, err := locateNamePools(data)
	if err != nil {
		return nil, err
	}
	found := make(map[uint32]staffLocation, len(edits))
	for pos := 12; pos < len(data); {
		rel := bytes.IndexByte(data[pos:], 1)
		if rel < 0 {
			break
		}
		at := pos + rel
		pos = at + 1
		h, ok := staffCandidate(data, at-12)
		if !ok {
			continue
		}
		closing, id, ok := staffClosing(data, h)
		if !ok || !wanted[id] {
			continue
		}
		if _, duplicate := found[id]; duplicate {
			return nil, fmt.Errorf("plusieurs fiches staff pour %d", id)
		}
		employment, ok := staffContract(data, h, closing, teams)
		if !ok {
			continue
		}
		if _, ok = staffAttributes(data, h.ability); !ok {
			continue
		}
		identity, err := staffIdentity(data, h, closing, id, employment, pools)
		if err != nil {
			continue
		}
		_, rolesAt := staffRoles(data, h, closing)
		found[id] = staffLocation{ability: h.ability, roles: rolesAt, birth: identity.personalityOffset - 22}
	}
	for id := range wanted {
		if _, ok := found[id]; !ok {
			return nil, fmt.Errorf("membre du staff %d introuvable ou non pris en charge", id)
		}
	}
	return found, nil
}

func applyStaffEdits(data []byte, edits []StaffEdit) (int, error) {
	if len(edits) == 0 || len(edits) > 100 {
		return 0, errors.New("choisis entre 1 et 100 membres du staff")
	}
	locations, err := staffEditLocations(data, edits)
	if err != nil {
		return 0, err
	}
	attrs := make(map[string]int, len(staffAttributeNames))
	for i, key := range staffAttributeNames {
		attrs[key] = i
	}
	roles := make(map[string]int, len(staffRoleNames))
	for i, key := range staffRoleNames {
		roles[key] = i
	}
	changed := 0
	for _, edit := range edits {
		loc := locations[edit.ID]
		if edit.BirthDate != nil {
			updated, err := writeBirthDate(data, loc.birth, *edit.BirthDate)
			if err != nil {
				return 0, fmt.Errorf("staff %d : %w", edit.ID, err)
			}
			if updated {
				changed++
			}
		}
		if edit.CA != nil {
			if *edit.CA < 1 || *edit.CA > 200 {
				return 0, fmt.Errorf("CA hors limites pour %d", edit.ID)
			}
			old := binary.LittleEndian.Uint16(data[loc.ability:])
			if old != *edit.CA {
				binary.LittleEndian.PutUint16(data[loc.ability:], *edit.CA)
				changed++
			}
		}
		if edit.PA != nil {
			if *edit.PA < 1 || *edit.PA > 200 {
				return 0, fmt.Errorf("PA hors limites pour %d", edit.ID)
			}
			old := int16(binary.LittleEndian.Uint16(data[loc.ability+2:]))
			if old != *edit.PA {
				binary.LittleEndian.PutUint16(data[loc.ability+2:], uint16(*edit.PA))
				changed++
			}
		}
		for key, value := range edit.Attrs {
			i, ok := attrs[key]
			if !ok {
				return 0, fmt.Errorf("attribut staff inconnu : %s", key)
			}
			if value < 1 || value > 20 {
				return 0, fmt.Errorf("valeur hors limites pour %s", key)
			}
			at, raw := 0, byte(value*5)
			switch i {
			case 0:
				at = loc.ability + 14 + 4
				raw = value
			case 1:
				at = loc.ability + 14 + 12
				raw = value
			case 2:
				at = loc.ability + 14 + 13
			default:
				at = loc.ability + 40 + staffBlockSlots[i]
			}
			if (i < 2 && data[at] != value) || (i >= 2 && displayAttribute(data[at]) != value) {
				data[at] = raw
				changed++
			}
		}
		for key, value := range edit.Roles {
			i, ok := roles[key]
			if !ok {
				return 0, fmt.Errorf("poste staff inconnu : %s", key)
			}
			if value < 1 || value > 20 {
				return 0, fmt.Errorf("aptitude hors limites pour %s", key)
			}
			if loc.roles < 0 {
				return 0, fmt.Errorf("aptitudes aux postes ambiguës pour %d", edit.ID)
			}
			at := loc.roles + i
			if data[at] != value {
				data[at] = value
				changed++
			}
		}
	}
	if changed == 0 {
		return 0, ErrNoChanges
	}
	return changed, nil
}

// WriteStaffEdits replaces the save after publishing a backup of its source.
func WriteStaffEdits(path string, edits []StaffEdit) (int, error) {
	result, err := SaveStaffEdits(path, edits, SaveReplace)
	return result.Changed, err
}

func SaveStaffEdits(path string, edits []StaffEdit, mode SaveMode) (SaveResult, error) {
	return writeGameDBEdits(path, mode, func(data []byte) (int, error) { return applyStaffEdits(data, edits) })
}
