//go:build !windows

package reader

import (
	"errors"
	"os"
)

func moveNewSave(staged, target string) error {
	return os.Link(staged, target)
}

func replaceExistingSave(original, staged string) error {
	return errors.New("l'écriture des sauvegardes n'est prise en charge que sous Windows")
}
