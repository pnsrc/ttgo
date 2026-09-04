//go:build !windows

package main

type ConflictAdapter struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func (a *App) CheckWintun() bool { return true }

func (a *App) DownloadWintun() error { return nil }

func (a *App) FindConflictAdapters() []ConflictAdapter { return nil }

func (a *App) DisableAdapter(_ string) error { return nil }
