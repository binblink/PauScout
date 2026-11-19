package reader

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestWritePlayerEditsOnCopy(t *testing.T) {
	source := os.Getenv("FM27_SAMPLE")
	if source == "" {
		t.Skip("FM27_SAMPLE non défini")
	}
	const briceID uint32 = 34001981
	playerID := briceID
	if value := os.Getenv("FM27_SAMPLE_PLAYER_ID"); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil || parsed == 0 {
			t.Fatalf("FM27_SAMPLE_PLAYER_ID invalide : %q", value)
		}
		playerID = uint32(parsed)
	}
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	target := filepath.Join(t.TempDir(), "test.fm")
	copyFile, err := os.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(copyFile, input); err != nil {
		t.Fatal(err)
	}
	if err = copyFile.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := readGameDB(target)
	if err != nil {
		t.Fatal(err)
	}
	offsets, err := editedPlayerOffsets(before, []PlayerEdit{{ID: playerID}})
	if err != nil {
		t.Fatal(err)
	}
	loc := offsets[playerID]
	start := loc.start
	pools, err := locateNamePools(before)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := findPersonIdentity(before, start+100, loc.closing, pools, playerID)
	if err != nil {
		t.Fatal(err)
	}
	if playerID == briceID {
		for key, expected := range map[string]uint8{
			"adaptability": 20, "ambition": 12, "controversy": 7, "loyalty": 16,
			"pressure": 11, "professionalism": 15, "sportsmanship": 15, "temperament": 15,
		} {
			if got := personalityValues(identity)[key]; got != expected {
				t.Fatalf("%s=%d, attendu %d selon la capture de Brice", key, got, expected)
			}
		}
	}
	newProfessionalism := before[identity.personalityOffset+4] + 1
	newBirth := "1988-02-29"
	newValue := uint32(9876543)
	birthOffset := identity.personalityOffset - 22
	newTrait := before[identity.traitsOffset]&1 == 0
	newHeight := before[start+121] + 1
	newRep := uint16(before[start-4]) | uint16(before[start-3])<<8
	newRep++
	if newProfessionalism > 20 {
		newProfessionalism = 19
	}
	oldCA := uint16(before[start]) | uint16(before[start+1])<<8
	newCA := oldCA + 1
	if newCA > 200 {
		newCA = oldCA - 1
	}
	oldPA := int16(uint16(before[start+2]) | uint16(before[start+3])<<8)
	newPA := oldPA + 1
	if newPA > 200 {
		newPA = oldPA - 1
	}
	newHandling := displayAttribute(before[start+39+11]) + 1
	if newHandling > 20 {
		newHandling = 19
	}
	count, err := WritePlayerEdits(target, []PlayerEdit{{ID: playerID, CA: &newCA, PA: &newPA, Attrs: map[string]uint8{"handling": newHandling}, Personality: map[string]uint8{"professionalism": newProfessionalism}, Traits: map[int]bool{0: newTrait}, Height: &newHeight, CurrentRep: &newRep, BirthDate: &newBirth, Value: &newValue}})
	if err != nil {
		t.Fatal(err)
	}
	if count < 1 {
		t.Fatalf("aucune valeur écrite : %d", count)
	}
	if _, err = ReadDirectory(target); err != nil {
		t.Fatal(err)
	}
	after, err := readGameDB(target)
	if err != nil {
		t.Fatal(err)
	}
	if after[start] != byte(newCA) || after[start+1] != byte(newCA>>8) {
		t.Fatalf("CA non modifié : %d", newCA)
	}
	if after[start+2] != byte(newPA) || after[start+3] != byte(newPA>>8) {
		t.Fatalf("PA non modifié : %d", newPA)
	}
	if displayAttribute(after[start+39+11]) != newHandling {
		t.Fatalf("attribut non modifié : %d", newHandling)
	}
	if after[identity.personalityOffset+4] != newProfessionalism {
		t.Fatalf("personnalité non modifiée : %d", newProfessionalism)
	}
	if (after[identity.traitsOffset]&1 != 0) != newTrait || after[start+121] != newHeight || (uint16(after[start-4])|uint16(after[start-3])<<8) != newRep {
		t.Fatal("trait, taille ou réputation non enregistré")
	}
	for i := range before {
		if before[i] != after[i] && i != start && i != start+1 && i != start+2 && i != start+3 && i != start+39+11 && i != identity.personalityOffset+4 && i != identity.traitsOffset && i != start+121 && i != start-4 && i != start-3 && (i < birthOffset || i >= birthOffset+4) && (i < start+93 || i >= start+97) {
			t.Fatalf("octet inattendu modifié à %d", i)
		}
	}
	catalog, err := ReadPlayerCatalog(target)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, player := range catalog.Players {
		if player.ID == playerID {
			found = true
			if player.Value == nil || *player.Value != newValue {
				t.Fatalf("valeur incorrecte : %v", player.Value)
			}
			if player.BirthDate != newBirth {
				t.Fatalf("date relue=%q, attendue %q", player.BirthDate, newBirth)
			}
			if playerID == briceID && player.Nationality != "France" {
				t.Fatalf("nationalité de Brice=%q, attendue France", player.Nationality)
			}
			if player.Personality["professionalism"] != newProfessionalism {
				t.Fatalf("personnalité relue=%d, attendue=%d", player.Personality["professionalism"], newProfessionalism)
			}
			break
		}
	}
	if !found {
		t.Fatal("joueur modifié absent du catalogue")
	}
	files, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name() != "test.fm" {
		t.Fatalf("fichiers après remplacement : %v", files)
	}
}
