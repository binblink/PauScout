package reader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishRelatedSaveDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "club - PauScout historique - PauScout.fm")
	stage := filepath.Join(dir, "staged.tmp")
	if err := os.WriteFile(first, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stage, []byte("updated"), 0600); err != nil {
		t.Fatal(err)
	}
	published, err := publishRelatedSave(stage, first, copySuffix)
	if err != nil {
		t.Fatal(err)
	}
	if published != filepath.Join(dir, "club - PauScout historique - PauScout (2).fm") {
		t.Fatalf("copie inattendue : %q", published)
	}
	for name, want := range map[string]string{first: "first", published: "updated"} {
		data, err := os.ReadFile(name)
		if err != nil || string(data) != want {
			t.Fatalf("%s : %q (%v)", name, data, err)
		}
	}
}

func TestBackupFailureLeavesOnlySource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.fm")
	if err := os.WriteFile(path, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if backup, err := backupOriginal(file, path, info); err == nil || backup != "" {
		t.Fatalf("sauvegarde de secours inattendue : %q, %v", backup, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 || files[0].Name() != "source.fm" {
		t.Fatalf("fichiers après échec : %v (%v)", files, err)
	}
}
