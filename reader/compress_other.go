//go:build !windows

package reader

import "fmt"

func compressFMFrame([]byte) ([]byte, error) {
	return nil, fmt.Errorf("l'enregistrement FM27 est disponible dans la version Windows")
}
