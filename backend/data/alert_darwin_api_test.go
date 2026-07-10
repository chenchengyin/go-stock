//go:build darwin
// +build darwin

package data

import (
	"testing"
)

// @Author 2lovecode
// @Date 2025/02/06 17:50
// @Desc
// -----------------------------------------------------------------------------------

func TestAlert(t *testing.T) {
	api := NewAlertWindowsApi("go-stock", "Hello, World!", "This is a macOS notification.", "../../build/appicon.png")
	api.SendNotification()
}
