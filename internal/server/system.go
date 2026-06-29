package server

import (
	"runtime"
)

// SystemInfo — runtime метрики для админ-панели.
type SystemInfo struct {
	GoVersion       string `json:"go_version"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	Goroutines      int    `json:"goroutines"`
	NumCPU          int    `json:"num_cpu"`
	HeapAllocBytes  uint64 `json:"heap_alloc_bytes"`
	HeapSysBytes    uint64 `json:"heap_sys_bytes"`
	TotalAllocBytes uint64 `json:"total_alloc_bytes"`
	GCRuns          uint32 `json:"gc_runs"`
	ActiveSessions  int    `json:"active_sessions"`
	ActiveUsers     int    `json:"active_users"`
}

func gatherSystem() SystemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	active := GlobalConnTracker.ActiveUsers()
	totalConns := 0
	for _, n := range active {
		totalConns += n
	}
	return SystemInfo{
		GoVersion:       runtime.Version(),
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		Goroutines:      runtime.NumGoroutine(),
		NumCPU:          runtime.NumCPU(),
		HeapAllocBytes:  m.HeapAlloc,
		HeapSysBytes:    m.HeapSys,
		TotalAllocBytes: m.TotalAlloc,
		GCRuns:          m.NumGC,
		ActiveSessions:  totalConns,
		ActiveUsers:     len(active),
	}
}

// бесшумный namespace для возможных future подсчётов
var _ = runtime.GOOS
