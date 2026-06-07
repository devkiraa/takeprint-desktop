package main

import (
	_ "embed"
	"fmt"
	"runtime"

	"github.com/energye/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIconData []byte

// initSystemTray starts the system tray icon in a separate goroutine.
// It provides "Show Window", "Check for Updates", and "Exit" menu items.
func (a *App) initSystemTray() {
	go systray.Run(a.onTrayReady, a.onTrayExit)
}

// onTrayReady is called when the system tray is initialized and ready.
func (a *App) onTrayReady() {
	// Lock this goroutine to the current OS thread to prevent
	// Windows threading issues with TrackPopupMenu.
	runtime.LockOSThread()

	systray.SetIcon(trayIconData)
	systray.SetTitle("TakePrint")
	systray.SetTooltip("TakePrint — Network Print Server")

	// "Show Window" menu item
	mShow := systray.AddMenuItem("Show Window", "Show the TakePrint window")

	// "Check for Updates" menu item
	mCheckUpdate := systray.AddMenuItem("Check for Updates", "Check if a new version of TakePrint is available")

	systray.AddSeparator()

	// "Exit" menu item
	mExit := systray.AddMenuItem("Exit", "Quit TakePrint completely")

	// Handle tray click (left-click on the tray icon shows the window)
	systray.SetOnClick(func(menu systray.IMenu) {
		if a.ctx != nil {
			a.windowHidden = false
			wailsRuntime.WindowShow(a.ctx)
		}
	})

	systray.SetOnDClick(func(menu systray.IMenu) {
		if a.ctx != nil {
			a.windowHidden = false
			wailsRuntime.WindowShow(a.ctx)
		}
	})

	// Handle menu item clicks via callbacks
	mShow.Click(func() {
		if a.ctx != nil {
			a.windowHidden = false
			wailsRuntime.WindowShow(a.ctx)
		}
	})

	mCheckUpdate.Click(func() {
		go a.checkUpdateFromTray()
	})

	mExit.Click(func() {
		a.shouldQuit = true
		systray.Quit()
		if a.ctx != nil {
			wailsRuntime.Quit(a.ctx)
		}
	})
}

// onTrayExit is called when the system tray is being shut down.
func (a *App) onTrayExit() {
	// Cleanup is handled by the Wails OnShutdown callback.
}

// checkUpdateFromTray queries the update status and prompts the user.
func (a *App) checkUpdateFromTray() {
	res, err := a.CheckForUpdate()
	if err != nil {
		if a.ctx != nil {
			_, _ = wailsRuntime.MessageDialog(a.ctx, wailsRuntime.MessageDialogOptions{
				Type:          wailsRuntime.ErrorDialog,
				Title:         "Update Check Failed",
				Message:       fmt.Sprintf("Failed to check for updates:\n\n%v", err),
				Buttons:       []string{"OK"},
				DefaultButton: "OK",
			})
		}
		return
	}

	if !res.UpdateAvailable {
		if a.ctx != nil {
			_, _ = wailsRuntime.MessageDialog(a.ctx, wailsRuntime.MessageDialogOptions{
				Type:          wailsRuntime.InfoDialog,
				Title:         "Up to Date",
				Message:       fmt.Sprintf("TakePrint is up to date.\n\nYou are running the latest version (v%s).", AppVersion),
				Buttons:       []string{"OK"},
				DefaultButton: "OK",
			})
		}
		return
	}

	if a.ctx != nil {
		selection, err := wailsRuntime.MessageDialog(a.ctx, wailsRuntime.MessageDialogOptions{
			Type:          wailsRuntime.QuestionDialog,
			Title:         "Update Available",
			Message:       fmt.Sprintf("A new version (%s) of TakePrint is available.\n\nWould you like to open settings and install it?", res.LatestVersion),
			Buttons:       []string{"Yes", "No"},
			DefaultButton: "Yes",
			CancelButton:  "No",
		})
		if err == nil && selection == "Yes" {
			a.windowHidden = false
			wailsRuntime.WindowShow(a.ctx)
			wailsRuntime.EventsEmit(a.ctx, "open-settings-update", res)
		}
	}
}

