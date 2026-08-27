//go:build !darwin

package main

import "golang.design/x/hotkey"

func hotkeyMods() ([]hotkey.Modifier, string) {
	return []hotkey.Modifier{hotkey.ModCtrl, hotkey.ModShift}, "Ctrl+Shift+V"
}
