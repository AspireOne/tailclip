//go:build !windows

package clipboard

import "errors"

func SetText(text string) error {
	return errors.New("clipboard writes are only implemented on Windows")
}
