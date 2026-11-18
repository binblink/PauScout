package reader

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

type namePool struct{ offsets []uint32 }
type namePools struct{ first, last, common namePool }

func (p namePool) get(data []byte, id uint32) string {
	if id == ^uint32(0) || uint64(id) >= uint64(len(p.offsets)) {
		return ""
	}
	at := int(p.offsets[id])
	n := int(binary.LittleEndian.Uint32(data[at:]))
	if n == 0 || at+4+n > len(data) || !utf8.Valid(data[at+4:at+4+n]) {
		return ""
	}
	return string(data[at+4 : at+4+n])
}

func readNamePool(data []byte, at *int) (namePool, bool) {
	if *at+4 > len(data) {
		return namePool{}, false
	}
	count := uint64(binary.LittleEndian.Uint32(data[*at:]))
	*at += 4
	if count == 0 || count > 2_000_000 || count*8 > uint64(len(data)-*at) {
		return namePool{}, false
	}
	pool := namePool{offsets: make([]uint32, 0, int(count))}
	for i := uint32(0); uint64(i) < count; i++ {
		if *at+8 > len(data) {
			return namePool{}, false
		}
		id := binary.LittleEndian.Uint32(data[*at:])
		n := uint64(binary.LittleEndian.Uint32(data[*at+4:]))
		if id != i || n > 1024 || n > uint64(len(data)-*at-8) {
			return namePool{}, false
		}
		pool.offsets = append(pool.offsets, uint32(*at+4))
		*at += 8 + int(n)
	}
	return pool, true
}

func locateNamePools(data []byte) (namePools, error) {
	var prefix [4]byte
	binary.LittleEndian.PutUint32(prefix[:], 46421)
	var found *namePools
	for at := 0; at < len(data); {
		rel := bytes.Index(data[at:], prefix[:])
		if rel < 0 {
			break
		}
		start := at + rel
		at = start + 1
		if start+24 > len(data) {
			continue
		}
		word := func(i int) uint32 { return binary.LittleEndian.Uint32(data[start+i*4:]) }
		if word(5) != 0 || word(3) == 0 || word(3)%256 != 0 {
			continue
		}
		scale := word(3) / 256
		if scale > 1024 || word(2) != 1024*scale || word(4) != 2048*scale || word(1) > 255 {
			continue
		}
		cursor := start + 24
		first, ok := readNamePool(data, &cursor)
		if !ok {
			continue
		}
		last, ok := readNamePool(data, &cursor)
		if !ok {
			continue
		}
		common, ok := readNamePool(data, &cursor)
		if !ok {
			continue
		}
		if found != nil {
			return namePools{}, errors.New("plusieurs tables de noms possibles")
		}
		found = &namePools{first: first, last: last, common: common}
	}
	if found == nil {
		return namePools{}, errors.New("tables de noms FM27 introuvables")
	}
	return *found, nil
}

type PlayerIdentity struct {
	ID                uint32
	Name              string
	FirstName         string
	LastName          string
	BirthDate         time.Time
	NationID          uint16
	Personality       [8]uint8
	personalityOffset int
	traitsOffset      int
}

func findPlayerIdentity(data []byte, ability PlayerAbility, pools namePools) (PlayerIdentity, error) {
	return findPersonIdentity(data, ability.recordOffset+100, ability.closingOffset, pools, ability.ID)
}

func findPersonIdentity(data []byte, start, end int, pools namePools, id uint32) (PlayerIdentity, error) {
	if start < 0 || end > len(data) || start >= end {
		return PlayerIdentity{}, errors.New("fenetre de personne invalide")
	}
	var found *PlayerIdentity
	for at := start; at+23 < end; at++ {
		if data[at+4] != 0 || data[at+9] != 0 || data[at+14] != 0 {
			continue
		}
		firstID := binary.LittleEndian.Uint32(data[at:])
		lastID := binary.LittleEndian.Uint32(data[at+5:])
		commonID := binary.LittleEndian.Uint32(data[at+10:])
		if uint64(firstID) >= uint64(len(pools.first.offsets)) || uint64(lastID) >= uint64(len(pools.last.offsets)) || commonID != ^uint32(0) && uint64(commonID) >= uint64(len(pools.common.offsets)) {
			continue
		}
		legalLen := int(binary.LittleEndian.Uint32(data[at+15:]))
		birth := at + 19 + legalLen
		if legalLen > 80 || birth+40 >= end {
			continue
		}
		birthWord := binary.LittleEndian.Uint32(data[birth:])
		day := birthWord & 0x1ff
		year := birthWord >> 16
		if day < 1 || day > 366 || year < 1901 || year > 2200 || birthWord&0xfe00 != 0 {
			continue
		}
		if !bytes.Equal(data[birth+15:birth+21], []byte{0, 0, 0, 0, 0, 0}) {
			continue
		}
		validPersonality := true
		for _, value := range data[birth+22 : birth+30] {
			if value < 1 || value > 20 {
				validPersonality = false
				break
			}
		}
		if !validPersonality {
			continue
		}
		first := pools.first.get(data, firstID)
		last := pools.last.get(data, lastID)
		if first == "" || last == "" {
			continue
		}
		name := first + " " + last
		if commonID != ^uint32(0) {
			if common := pools.common.get(data, commonID); common != "" {
				name = common
			}
		}
		birthDate, ok := fmDate(data, birth)
		if !ok {
			continue
		}
		identity := PlayerIdentity{ID: id, Name: name, FirstName: first, LastName: last, BirthDate: birthDate, NationID: binary.LittleEndian.Uint16(data[birth+14:]), personalityOffset: birth + 22, traitsOffset: at - 8}
		copy(identity.Personality[:], data[birth+22:birth+30])
		if found != nil {
			return PlayerIdentity{}, fmt.Errorf("nom ambigu pour la personne %d", id)
		}
		found = &identity
	}
	if found == nil {
		return PlayerIdentity{}, fmt.Errorf("nom introuvable pour la personne %d", id)
	}
	return *found, nil
}

// ReadIdentityByID resolves one validated player record to its name.
func ReadIdentityByID(path string, id uint32) (PlayerIdentity, error) {
	data, err := readGameDB(path)
	if err != nil {
		return PlayerIdentity{}, err
	}
	ability, err := findAbility(data, id)
	if err != nil {
		return PlayerIdentity{}, err
	}
	pools, err := locateNamePools(data)
	if err != nil {
		return PlayerIdentity{}, err
	}
	return findPlayerIdentity(data, ability, pools)
}
