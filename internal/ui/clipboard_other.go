//go:build !windows

package ui

import "errors"

func Copy(text string) error {
	return errors.New("area de transferencia so implementada no Windows")
}
