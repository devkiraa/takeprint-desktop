package main

import (
	_ "embed"
	"runtime"

	"github.com/energye/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIconData []byte

// initSystemTray starts the system tray icon in a separate goroutine.
// It provides "Show Window" and "Exit" menu items.
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
