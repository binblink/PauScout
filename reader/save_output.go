package reader

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type SaveMode string

const (
	SaveCopy    SaveMode = "copy"
	SaveReplace SaveMode = "replace"
)

type SaveResult struct {
	Changed    int    `json:"changed"`
	Path       string `json:"path"`
	BackupPath string `json:"backup_path,omitempty"`
}

func checkUnchanged(path string, before os.FileInfo) error {
	current, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !os.SameFile(before, current) || current.Size() != before.Size() || !current.ModTime().Equal(before.ModTime()) {
		return errors.New("la sauvegarde a changé depuis son ouverture")
	}
	return nil
}

const copySuffix = " - PauScout"

func copyStem(stem string) string {
	if i := strings.LastIndex(stem, copySuffix); i > 0 {
		base, suffix := stem[:i], stem[i+len(copySuffix):]
		if suffix == "" {
			return base
		}
		if strings.HasPrefix(suffix, " (") && strings.HasSuffix(suffix, ")") {
			if n, err := strconv.Atoi(suffix[2 : len(suffix)-1]); err == nil && n > 1 {
				return base
			}
		}
	}
	return stem
}

// publishRelatedSave publishes a complete staged file under a fresh sibling name.
func publishRelatedSave(staged, original, suffix string) (string, error) {
	stem := strings.TrimSuffix(filepath.Base(original), filepath.Ext(original))
	if suffix == copySuffix {
		stem = copyStem(stem)
	}
	base := filepath.Join(filepath.Dir(original), stem+suffix)
	for n := 1; ; n++ {
		target := base + ".fm"
		if n > 1 {
			target = fmt.Sprintf("%s (%d).fm", base, n)
		}
		err := moveNewSave(staged, target)
		if err == nil {
			return target, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
}

func backupOriginal(source *os.File, original string, before os.FileInfo) (string, error) {
	backup, err := os.CreateTemp(filepath.Dir(original), ".pauscout-secours-*.tmp")
	if err != nil {
		return "", err
	}
	staged := backup.Name()
	defer os.Remove(staged)
	defer backup.Close()
	written, err := io.Copy(backup, io.NewSectionReader(source, 0, before.Size()))
	if err != nil {
		return "", err
	}
	if written != before.Size() {
		return "", io.ErrUnexpectedEOF
	}
	if err := backup.Sync(); err != nil {
		return "", err
	}
	if err := backup.Close(); err != nil {
		return "", err
	}
	if err := checkUnchanged(original, before); err != nil {
		return "", err
	}
	return publishRelatedSave(staged, original, " - secours")
}
