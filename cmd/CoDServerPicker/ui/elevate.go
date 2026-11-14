package ui

import "golang.org/x/sys/windows"

// isAdmin reports whether the app runs with administrator rights.
func isAdmin() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
