//go:build !windows

package reader

import "errors"

func replaceExistingSave(original, staged string) error {
	return errors.New("l'écriture des sauvegardes n'est prise en charge que sous Windows")
}
