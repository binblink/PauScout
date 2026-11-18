package reader

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteStaffEditsOnCopy(t *testing.T) {
	source := os.Getenv("FM27_SAMPLE")
	if source == "" {
		t.Skip("FM27_SAMPLE non défini")
	}
	in, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	path := filepath.Join(t.TempDir(), "staff.fm")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	if err = out.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := readGameDB(path)
	if err != nil {
		t.Fatal(err)
	}
	locs, err := staffEditLocations(before, []StaffEdit{{ID: 10423}})
	if err != nil {
		t.Fatal(err)
	}
	loc := locs[10423]
	newBirth := "1984-02-29"
	if loc.roles < 0 {
		t.Fatal("postes staff non repérés")
	}
	oldCA := uint16(before[loc.ability]) | uint16(before[loc.ability+1])<<8
	newCA := oldCA + 1
	if newCA > 200 {
		newCA = oldCA - 1
	}
	oldPA := int16(uint16(before[loc.ability+2]) | uint16(before[loc.ability+3])<<8)
	newPA := oldPA + 1
	if newPA > 200 {
		newPA = oldPA - 1
	}
	newAuthority := before[loc.ability+18] + 1
	if newAuthority > 20 {
		newAuthority = 19
	}
	newScout := before[loc.roles+4] + 1
	if newScout > 20 {
		newScout = 19
	}
	count, err := WriteStaffEdits(path, []StaffEdit{{ID: 10423, CA: &newCA, PA: &newPA, Attrs: map[string]uint8{"authority": newAuthority}, Roles: map[string]uint8{"scout": newScout}, BirthDate: &newBirth}})
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("modifications=%d, attendu 5", count)
	}
	if _, err = ReadDirectory(path); err != nil {
		t.Fatal(err)
	}
	after, err := readGameDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if after[loc.ability] != byte(newCA) || after[loc.ability+1] != byte(newCA>>8) || after[loc.ability+2] != byte(newPA) || after[loc.ability+3] != byte(newPA>>8) || after[loc.ability+18] != newAuthority || after[loc.roles+4] != newScout {
		t.Fatal("valeurs staff non enregistrées")
	}
	allowed := map[int]bool{loc.ability: true, loc.ability + 1: true, loc.ability + 2: true, loc.ability + 3: true, loc.ability + 18: true, loc.roles + 4: true}
	for i := 0; i < 4; i++ {
		allowed[loc.birth+i] = true
	}
	for i := range before {
		if before[i] != after[i] && !allowed[i] {
			t.Fatalf("octet inattendu modifié à %d", i)
		}
	}
	catalog, err := ReadPlayerCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, staff := range catalog.Staff {
		if staff.ID == 10423 {
			found = true
			if staff.BirthDate != newBirth {
				t.Fatalf("date relue=%q, attendue %q", staff.BirthDate, newBirth)
			}
			if staff.CA != newCA || staff.PA != newPA || staff.AttrsB64[0] != newAuthority || staff.RolesB64[4] != newScout {
				t.Fatalf("fiche staff relue incohérente : %+v", staff)
			}
			break
		}
	}
	if !found {
		t.Fatal("staff modifié absent du catalogue")
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name() != "staff.fm" {
		t.Fatalf("fichiers après remplacement : %v", files)
	}
}
