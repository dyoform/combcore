package main

var GUIInfo struct {
	Status string

	StatusLocked bool
}

func combcore_set_status(status string) {
	if !GUIInfo.StatusLocked {
		GUIInfo.Status = status
	}
}

func combcore_lock_status() {
	GUIInfo.StatusLocked = true
}

func combcore_unlock_status() {
	GUIInfo.StatusLocked = false
}
