package reader

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"pauscout.local/app/reader/zstd"
)

const (
	maxGameDBBytes     = 256 * 1024 * 1024
	maxPersonWindow    = 16 * 1024
	playerMarkerOffset = 102
	playerDecodeExtent = 122
)

var playerMarker = []byte{1, 0, 0x6c, 7}

// PlayerAbility contains fields confirmed on two FM27 players and by one
// controlled in-game CA change. The ID is the database ID in the closing
// person header, rather than the identity in the player record's own header.
type PlayerAbility struct {
	ID                uint32
	CA                uint16
	PA                int16
	HeightCM          uint8
	HomeReputation    uint16
	CurrentReputation uint16
	WorldReputation   uint16
	recordOffset      int
	closingOffset     int
}

func readGameDB(path string) ([]byte, error) {
	directory, err := ReadDirectory(path)
	if err != nil {
		return nil, err
	}
	var section *DirectoryEntry
	for i := range directory.Entries {
		e := &directory.Entries[i]
		if e.Group == "" && e.Name == "game_db" && e.Extension == ".dat" {
			if section != nil {
				return nil, errors.New("plusieurs sections game_db")
			}
			section = e
		}
	}
	if section == nil || section.Uncompressed > maxGameDBBytes {
		return nil, errors.New("section game_db absente ou trop grande")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := zstd.NewReader(io.NewSectionReader(f, section.Offset, section.Compressed))
	data, err := io.ReadAll(io.LimitReader(r, maxGameDBBytes+1))
	if err != nil {
		return nil, fmt.Errorf("decompression game_db: %w", err)
	}
	if len(data) > maxGameDBBytes || uint64(len(data)) != section.Uncompressed {
		return nil, errors.New("taille de game_db incoherente")
	}
	if len(data) < 8 || !bytes.Equal(data[:6], []byte("\x03\x01tad.")) || binary.LittleEndian.Uint16(data[6:8]) != 4000 {
		return nil, errors.New("schema game_db FM27 non pris en charge")
	}
	return data, nil
}

func candidateAbility(data []byte, start, closing int, pindex, id uint32) (PlayerAbility, bool) {
	if start < 19 || start+playerDecodeExtent > closing || start+playerDecodeExtent > len(data) {
		return PlayerAbility{}, false
	}
	if binary.LittleEndian.Uint32(data[start-19:]) != pindex {
		return PlayerAbility{}, false
	}
	uid := binary.LittleEndian.Uint32(data[start-15:])
	uidCopy := binary.LittleEndian.Uint32(data[start-11:])
	if uid == 0 || uid == ^uint32(0) || uidCopy == 0 || uidCopy == ^uint32(0) || uid >= id {
		return PlayerAbility{}, false
	}
	kind := data[start-7]
	if uid != uidCopy && (uid < 2_000_000_000 || kind != 2 && kind != 3) {
		return PlayerAbility{}, false
	}
	ca := binary.LittleEndian.Uint16(data[start:])
	pa := int16(binary.LittleEndian.Uint16(data[start+2:]))
	if ca > 255 || pa < -10 || pa > 255 {
		return PlayerAbility{}, false
	}
	for _, rating := range data[start+24 : start+39] {
		if rating < 1 || rating > 20 {
			return PlayerAbility{}, false
		}
	}
	for _, attribute := range data[start+39 : start+93] {
		if attribute < 1 || attribute > 125 {
			return PlayerAbility{}, false
		}
	}
	if !bytes.Equal(data[start+playerMarkerOffset:start+playerMarkerOffset+4], playerMarker) {
		return PlayerAbility{}, false
	}
	return PlayerAbility{
		ID:                id,
		CA:                ca,
		PA:                pa,
		HeightCM:          data[start+121],
		HomeReputation:    binary.LittleEndian.Uint16(data[start-6:]),
		CurrentReputation: binary.LittleEndian.Uint16(data[start-4:]),
		WorldReputation:   binary.LittleEndian.Uint16(data[start-2:]),
		recordOffset:      start,
	}, true
}

func findAbility(data []byte, id uint32) (PlayerAbility, error) {
	if id == 0 || id == ^uint32(0) {
		return PlayerAbility{}, errors.New("ID de joueur invalide")
	}
	var pair [8]byte
	binary.LittleEndian.PutUint32(pair[:4], id)
	binary.LittleEndian.PutUint32(pair[4:], id)
	var found *PlayerAbility
	for at := 0; at < len(data); {
		hit := bytes.Index(data[at:], pair[:])
		if hit < 0 {
			break
		}
		uidAt := at + hit
		at = uidAt + 1
		closing := uidAt - 4
		if closing < 0 {
			continue
		}
		closingPindex := binary.LittleEndian.Uint32(data[closing:])
		if closingPindex == 0 || closingPindex > 1_000_000 {
			continue
		}
		pindex := closingPindex - 1
		windowStart := closing - maxPersonWindow
		if windowStart < 0 {
			windowStart = 0
		}
		for pos := windowStart; pos < closing; {
			index := bytes.Index(data[pos:closing], playerMarker)
			if index < 0 {
				break
			}
			markerAt := pos + index
			pos = markerAt + 1
			ability, ok := candidateAbility(data, markerAt-playerMarkerOffset, closing, pindex, id)
			if !ok {
				continue
			}
			if found != nil {
				return PlayerAbility{}, errors.New("plusieurs enregistrements joueur correspondent a l'ID")
			}
			ability.closingOffset = closing
			found = &ability
		}
	}
	if found == nil {
		return PlayerAbility{}, fmt.Errorf("joueur %d introuvable ou non pris en charge", id)
	}
	return *found, nil
}

// ReadAbilityByID reads the ability and a few cross-check fields for one
// database player ID. It does not claim to enumerate every player in the save.
func ReadAbilityByID(path string, id uint32) (PlayerAbility, error) {
	data, err := readGameDB(path)
	if err != nil {
		return PlayerAbility{}, err
	}
	return findAbility(data, id)
}
