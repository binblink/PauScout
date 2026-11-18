package reader

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"pauscout.local/app/reader/zstd"
)

// PlayerEdit changes only fields whose FM27 locations have been verified in
// game. A nil CA or PA leaves that value untouched.
type PlayerEdit struct {
	ID          uint32           `json:"uid"`
	CA          *uint16          `json:"ca,omitempty"`
	PA          *int16           `json:"pa,omitempty"`
	Attrs       map[string]uint8 `json:"attrs,omitempty"`
	Personality map[string]uint8 `json:"personality,omitempty"`
	Positions   map[string]uint8 `json:"positions_ratings,omitempty"`
	Traits      map[int]bool     `json:"traits,omitempty"`
	Height      *uint8           `json:"height,omitempty"`
	HomeRep     *uint16          `json:"home_rep,omitempty"`
	CurrentRep  *uint16          `json:"current_rep,omitempty"`
	WorldRep    *uint16          `json:"world_rep,omitempty"`
	BirthDate   *string          `json:"birth_date,omitempty"`
	Value       *uint32          `json:"value,omitempty"`
}

var ErrNoChanges = errors.New("aucune valeur différente à enregistrer")

type playerLocation struct{ start, closing int }

func editedPlayerOffsets(data []byte, edits []PlayerEdit) (map[uint32]playerLocation, error) {
	wanted := make(map[uint32]bool, len(edits))
	for _, edit := range edits {
		if edit.ID == 0 || wanted[edit.ID] {
			return nil, fmt.Errorf("ID de joueur absent ou répété : %d", edit.ID)
		}
		wanted[edit.ID] = true
	}
	starts, _ := candidateStarts(data)
	found := make(map[uint32]playerLocation, len(edits))
	for i, start := range starts {
		end := len(data)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		closing, id, ok := closingHeader(data, start, end)
		if !ok || !wanted[id] {
			continue
		}
		if _, duplicate := found[id]; duplicate {
			return nil, fmt.Errorf("plusieurs enregistrements pour le joueur %d", id)
		}
		found[id] = playerLocation{start: start, closing: closing}
	}
	for id := range wanted {
		if _, ok := found[id]; !ok {
			return nil, fmt.Errorf("joueur %d introuvable ou non pris en charge", id)
		}
	}
	return found, nil
}

func applyPlayerEdits(data []byte, edits []PlayerEdit) (int, error) {
	if len(edits) == 0 || len(edits) > 100 {
		return 0, errors.New("choisis entre 1 et 100 joueurs à modifier")
	}
	offsets, err := editedPlayerOffsets(data, edits)
	if err != nil {
		return 0, err
	}
	attributeSlots := make(map[string]int, len(attributeNames))
	for i, name := range attributeNames {
		if i >= 24 {
			i += 2
		}
		attributeSlots[name] = i
	}
	changed := 0
	var pools namePools
	for _, edit := range edits {
		if len(edit.Personality) > 0 || len(edit.Traits) > 0 || edit.BirthDate != nil {
			pools, err = locateNamePools(data)
			if err != nil {
				return 0, err
			}
			break
		}
	}
	personalitySlots := make(map[string]int, len(personalityNames))
	for i, key := range personalityNames {
		personalitySlots[key] = i
	}
	for _, edit := range edits {
		loc := offsets[edit.ID]
		start := loc.start
		if edit.Value != nil {
			if *edit.Value == 0 || *edit.Value == 300_000_000 || *edit.Value == ^uint32(0) {
				return 0, fmt.Errorf("valeur indisponible ou réservée pour %d", edit.ID)
			}
			if binary.LittleEndian.Uint32(data[start+93:]) != *edit.Value {
				binary.LittleEndian.PutUint32(data[start+93:], *edit.Value)
				changed++
			}
		}
		if edit.Height != nil {
			if *edit.Height < 100 || *edit.Height > 230 {
				return 0, fmt.Errorf("taille invalide pour %d", edit.ID)
			}
			if data[start+121] != *edit.Height {
				data[start+121] = *edit.Height
				changed++
			}
		}
		for _, rep := range []struct {
			value  *uint16
			offset int
		}{{edit.HomeRep, -6}, {edit.CurrentRep, -4}, {edit.WorldRep, -2}} {
			if rep.value != nil {
				if *rep.value > 10000 {
					return 0, fmt.Errorf("réputation invalide pour %d", edit.ID)
				}
				at := start + rep.offset
				if binary.LittleEndian.Uint16(data[at:]) != *rep.value {
					binary.LittleEndian.PutUint16(data[at:], *rep.value)
					changed++
				}
			}
		}
		for key, value := range edit.Positions {
			slot := -1
			for i, name := range positionNames {
				if name == key {
					slot = i
					break
				}
			}
			if slot < 0 || value < 1 || value > 20 {
				return 0, fmt.Errorf("position ou note invalide : %s", key)
			}
			if data[start+24+slot] != value {
				data[start+24+slot] = value
				changed++
			}
		}
		if edit.CA != nil {
			if *edit.CA > 200 {
				return 0, fmt.Errorf("CA hors limites pour %d", edit.ID)
			}
			old := binary.LittleEndian.Uint16(data[start:])
			if old != *edit.CA {
				binary.LittleEndian.PutUint16(data[start:], *edit.CA)
				changed++
			}
		}
		if edit.PA != nil {
			if *edit.PA < 0 || *edit.PA > 200 {
				return 0, fmt.Errorf("PA hors limites pour %d", edit.ID)
			}
			old := int16(binary.LittleEndian.Uint16(data[start+2:]))
			if old != *edit.PA {
				binary.LittleEndian.PutUint16(data[start+2:], uint16(*edit.PA))
				changed++
			}
		}
		for key, value := range edit.Attrs {
			slot, ok := attributeSlots[key]
			if !ok {
				return 0, fmt.Errorf("attribut inconnu : %s", key)
			}
			if value < 1 || value > 20 {
				return 0, fmt.Errorf("valeur hors limites pour %s", key)
			}
			at := start + 39 + slot
			if displayAttribute(data[at]) != value {
				data[at] = value * 5
				changed++
			}
		}
		if len(edit.Personality) > 0 || len(edit.Traits) > 0 || edit.BirthDate != nil {
			identity, err := findPersonIdentity(data, start+100, loc.closing, pools, edit.ID)
			if err != nil {
				return 0, fmt.Errorf("personnalité du joueur %d introuvable : %w", edit.ID, err)
			}
			if edit.BirthDate != nil {
				updated, err := writeBirthDate(data, identity.personalityOffset-22, *edit.BirthDate)
				if err != nil {
					return 0, fmt.Errorf("joueur %d : %w", edit.ID, err)
				}
				if updated {
					changed++
				}
			}
			if len(edit.Traits) > 0 {
				at := identity.traitsOffset
				if at < start || at+8 > loc.closing {
					return 0, fmt.Errorf("traits introuvables pour %d", edit.ID)
				}
				for bit, enabled := range edit.Traits {
					if bit < 0 || bit > 63 || bit == 53 {
						return 0, fmt.Errorf("trait invalide : %d", bit)
					}
					mask := byte(1 << uint(bit%8))
					p := at + bit/8
					old := data[p]
					if enabled {
						data[p] |= mask
					} else {
						data[p] &^= mask
					}
					if old != data[p] {
						changed++
					}
				}
			}
			for key, value := range edit.Personality {
				slot, ok := personalitySlots[key]
				if !ok {
					return 0, fmt.Errorf("trait de personnalité inconnu : %s", key)
				}
				if value < 1 || value > 20 {
					return 0, fmt.Errorf("valeur hors limites pour %s", key)
				}
				at := identity.personalityOffset + slot
				if data[at] != value {
					data[at] = value
					changed++
				}
			}
		}
	}
	if changed == 0 {
		return 0, ErrNoChanges
	}
	return changed, nil
}

func patchDirectoryPayload(payload []byte, gameOffset, oldSize, newSize int64) error {
	c := directoryCursor{data: payload}
	if _, err := c.string(); err != nil {
		return err
	}
	top, err := c.word()
	if err != nil {
		return err
	}
	if top == 0 || top > maxFrames {
		return errors.New("répertoire FMF inattendu")
	}
	delta := newSize - oldSize
	gameStored := gameOffset - 26
	seen := 0
	patchEntries := func(count uint32, group string) error {
		for i := uint32(0); i < count; i++ {
			name, err := c.string()
			if err != nil {
				return err
			}
			ext, err := c.string()
			if err != nil {
				return err
			}
			if c.at+40 > len(payload) {
				return io.ErrUnexpectedEOF
			}
			at := c.at
			offset := int64(binary.LittleEndian.Uint64(payload[at:]))
			size := int64(binary.LittleEndian.Uint64(payload[at+8:]))
			if group == "" && name == "game_db" && ext == ".dat" {
				if offset != gameStored || size != oldSize {
					return errors.New("entrée game_db incohérente")
				}
				binary.LittleEndian.PutUint64(payload[at+8:], uint64(newSize))
				seen++
			} else if offset >= gameStored+oldSize {
				if offset+delta < 0 {
					return errors.New("nouvel offset FMF invalide")
				}
				binary.LittleEndian.PutUint64(payload[at:], uint64(offset+delta))
			} else if offset > gameStored {
				return errors.New("section superposée à game_db")
			}
			c.at += 40
		}
		return nil
	}
	if err := patchEntries(top, ""); err != nil {
		return err
	}
	groups, err := c.word()
	if err != nil {
		return err
	}
	if groups > 16 {
		return errors.New("trop de groupes FMF")
	}
	for i := uint32(0); i < groups; i++ {
		group, err := c.string()
		if err != nil {
			return err
		}
		count, err := c.word()
		if err != nil {
			return err
		}
		if count > maxFrames {
			return errors.New("trop de sections FMF")
		}
		if err := patchEntries(count, group); err != nil {
			return err
		}
	}
	end, err := c.word()
	if err != nil || end != 0 || c.at != len(payload) || seen != 1 {
		return errors.New("fin de répertoire FMF invalide")
	}
	return nil
}

func directoryPayload(f *os.File, suffix int64) ([]byte, error) {
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	r := zstd.NewReader(io.NewSectionReader(f, suffix+9, stat.Size()-suffix-9))
	payload, err := io.ReadAll(io.LimitReader(r, maxDirectoryBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxDirectoryBytes {
		return nil, errors.New("répertoire FMF trop grand")
	}
	return payload, nil
}

func writeRepackedSave(source, stage *os.File, game DirectoryEntry, gameFrame, footerFrame []byte, oldSuffix int64) error {
	delta := int64(len(gameFrame)) - game.Compressed
	var head [26]byte
	if _, err := source.ReadAt(head[:], 0); err != nil {
		return err
	}
	newSuffix := oldSuffix + delta
	if newSuffix < 26 {
		return errors.New("position FMF invalide")
	}
	binary.LittleEndian.PutUint64(head[9:17], uint64(newSuffix-9))
	if _, err := stage.Write(head[:]); err != nil {
		return err
	}
	if _, err := source.Seek(26, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.CopyN(stage, source, game.Offset-26); err != nil {
		return err
	}
	if _, err := stage.Write(gameFrame); err != nil {
		return err
	}
	if _, err := source.Seek(game.Offset+game.Compressed, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.CopyN(stage, source, oldSuffix-game.Offset-game.Compressed); err != nil {
		return err
	}
	var footerHead [9]byte
	if _, err := source.ReadAt(footerHead[:], oldSuffix); err != nil {
		return err
	}
	if _, err := stage.Write(footerHead[:]); err != nil {
		return err
	}
	if _, err := stage.Write(footerFrame); err != nil {
		return err
	}
	return stage.Sync()
}

// WritePlayerEdits validates a complete staged FMF file before atomically
// replacing the existing save at the same path. No second game save remains.
func WritePlayerEdits(path string, edits []PlayerEdit) (int, error) {
	return writeGameDBEdits(path, func(data []byte) (int, error) { return applyPlayerEdits(data, edits) })
}

func writeGameDBEdits(path string, mutate func([]byte) (int, error)) (int, error) {
	if !strings.EqualFold(filepath.Ext(path), ".fm") {
		return 0, errors.New("sélectionne une sauvegarde .fm")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return 0, err
	}
	summary, err := Read(path)
	if err != nil {
		return 0, err
	}
	if summary.GameVersion != "27.0.1+2503222" {
		return 0, fmt.Errorf("version FM27 non validée : %s", summary.GameVersion)
	}
	directory, err := ReadDirectory(path)
	if err != nil {
		return 0, err
	}
	var game *DirectoryEntry
	for i := range directory.Entries {
		entry := &directory.Entries[i]
		if entry.Group == "" && entry.Name == "game_db" && entry.Extension == ".dat" {
			game = entry
			break
		}
	}
	if game == nil {
		return 0, errors.New("game_db absent")
	}
	source, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer source.Close()
	before, err := source.Stat()
	if err != nil {
		return 0, err
	}
	var head [26]byte
	if _, err := source.ReadAt(head[:], 0); err != nil {
		return 0, err
	}
	suffix := int64(binary.LittleEndian.Uint64(head[9:17])) + 9
	payload, err := directoryPayload(source, suffix)
	if err != nil {
		return 0, err
	}
	data, err := readGameDB(path)
	if err != nil {
		return 0, err
	}
	changed, err := mutate(data)
	if err != nil {
		return 0, err
	}
	gameFrame, err := compressFMFrame(data)
	if err != nil {
		return 0, err
	}
	if len(gameFrame) < 6 || !bytes.Equal(gameFrame[:6], []byte{0x28, 0xb5, 0x2f, 0xfd, 0x04, 0x50}) {
		return 0, errors.New("en-tête Zstandard FM27 inattendu")
	}
	if err := patchDirectoryPayload(payload, game.Offset, game.Compressed, int64(len(gameFrame))); err != nil {
		return 0, err
	}
	footerFrame, err := compressFMFrame(payload)
	if err != nil {
		return 0, err
	}
	stage, err := os.CreateTemp(filepath.Dir(path), ".pauscout-*.tmp")
	if err != nil {
		return 0, err
	}
	stagePath := stage.Name()
	defer os.Remove(stagePath)
	defer stage.Close()
	if err := stage.Chmod(before.Mode()); err != nil {
		return 0, err
	}
	if err := writeRepackedSave(source, stage, *game, gameFrame, footerFrame, suffix); err != nil {
		return 0, err
	}
	if err := stage.Close(); err != nil {
		return 0, err
	}
	if _, err := ReadDirectory(stagePath); err != nil {
		return 0, fmt.Errorf("sauvegarde vérifiée invalide : %w", err)
	}
	decoded, err := readGameDB(stagePath)
	if err != nil {
		return 0, fmt.Errorf("game_db réécrit invalide : %w", err)
	}
	if !bytes.Equal(data, decoded) {
		return 0, errors.New("game_db réécrit diffère des modifications prévues")
	}
	if _, err := Read(stagePath); err != nil {
		return 0, fmt.Errorf("résumé FMF réécrit invalide : %w", err)
	}
	current, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if current.Size() != before.Size() || !current.ModTime().Equal(before.ModTime()) {
		return 0, errors.New("la sauvegarde a changé depuis son ouverture")
	}
	if err := source.Close(); err != nil {
		return 0, err
	}
	if err := replaceExistingSave(path, stagePath); err != nil {
		return 0, err
	}
	return changed, nil
}
