//go:build windows

package ui

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"tailclip/internal/config"
	"tailclip/internal/logging"
	tailruntime "tailclip/internal/runtime"
	"tailclip/internal/startup"
)

type TrayApp struct {
	configPath string

	controller *tailruntime.Controller

	mw               *walk.MainWindow
	notifyIcon       *walk.NotifyIcon
	configPathEdit   *walk.LineEdit
	androidURLEdit   *walk.LineEdit
	authTokenEdit    *walk.LineEdit
	deviceIDEdit     *walk.LineEdit
	enabledCheck     *walk.CheckBox
	startOnLogin     *walk.CheckBox
	statusLabel      *walk.Label
	toggleSyncAction *walk.Action

	currentConfig config.Config
	hasConfig     bool
	initialConfig config.Config
	initialErr    error
}

func Run(configPath string) error {
	if configPath == "" {
		var err error
		configPath, err = config.DefaultPath()
		if err != nil {
			return err
		}
	}

	initialCfg, initialErr := config.Load(configPath)
	logLevel := "info"
	if initialErr == nil {
		logLevel = initialCfg.LogLevel
	}

	logger, closer, err := logging.New(logLevel)
	if err != nil {
		return err
	}
	defer closer.Close()

	app := &TrayApp{
		configPath:    configPath,
		controller:    tailruntime.NewController(logger),
		currentConfig: config.Default(),
		initialConfig: initialCfg,
		initialErr:    initialErr,
	}

	if err := app.createWindow(); err != nil {
		return err
	}

	if err := app.createTrayIcon(); err != nil {
		return err
	}
	defer app.notifyIcon.Dispose()

	app.bindStatus()
	app.loadInitialState()

	app.mw.Run()
	app.controller.Stop()
	return nil
}

func (a *TrayApp) createWindow() error {
	if err := (MainWindow{
		AssignTo: &a.mw,
		Title:    "Tailclip Settings",
		Size:     Size{Width: 440, Height: 280},
		MinSize:  Size{Width: 440, Height: 280},
		Visible:  false,
		Layout:   VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}},
		Children: []Widget{
			Composite{
				Layout: Grid{Columns: 2},
				Children: []Widget{
					Label{Text: "Config path"},
					LineEdit{AssignTo: &a.configPathEdit, ReadOnly: true},
					Label{Text: "Android URL"},
					LineEdit{AssignTo: &a.androidURLEdit},
					Label{Text: "Auth token"},
					LineEdit{AssignTo: &a.authTokenEdit, PasswordMode: true},
					Label{Text: "Device ID"},
					LineEdit{AssignTo: &a.deviceIDEdit},
					CheckBox{AssignTo: &a.enabledCheck, Text: "Enabled", ColumnSpan: 2},
					CheckBox{AssignTo: &a.startOnLogin, Text: "Start on login", ColumnSpan: 2},
				},
			},
			VSpacer{},
			Label{AssignTo: &a.statusLabel, Text: "Loading..."},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					PushButton{
						Text: "Save",
						OnClicked: func() {
							a.save()
						},
					},
					PushButton{
						Text: "Open Config Folder",
						OnClicked: func() {
							a.openConfigFolder()
						},
					},
					HSpacer{},
					PushButton{
						Text: "Hide",
						OnClicked: func() {
							a.mw.Hide()
						},
					},
				},
			},
		},
	}.Create()); err != nil {
		return err
	}

	a.configPathEdit.SetText(a.configPath)
	a.mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if reason == walk.CloseReasonUser {
			*canceled = true
			a.mw.Hide()
		}
	})

	return nil
}

func (a *TrayApp) createTrayIcon() error {
	icon, err := walk.NewIconFromImage(newTrayImage())
	if err != nil {
		return err
	}

	ni, err := walk.NewNotifyIcon(a.mw)
	if err != nil {
		return err
	}

	if err := ni.SetIcon(icon); err != nil {
		return err
	}
	if err := ni.SetToolTip("Tailclip"); err != nil {
		return err
	}

	openAction := walk.NewAction()
	if err := openAction.SetText("Open Settings"); err != nil {
		return err
	}
	openAction.Triggered().Attach(func() {
		a.showWindow()
	})

	a.toggleSyncAction = walk.NewAction()
	if err := a.toggleSyncAction.SetText("Disable Syncing"); err != nil {
		return err
	}
	a.toggleSyncAction.Triggered().Attach(func() {
		a.toggleSyncing()
	})

	quitAction := walk.NewAction()
	if err := quitAction.SetText("Quit"); err != nil {
		return err
	}
	quitAction.Triggered().Attach(func() {
		a.controller.Stop()
		walk.App().Exit(0)
	})

	if err := ni.ContextMenu().Actions().Add(openAction); err != nil {
		return err
	}
	if err := ni.ContextMenu().Actions().Add(a.toggleSyncAction); err != nil {
		return err
	}
	if err := ni.ContextMenu().Actions().Add(quitAction); err != nil {
		return err
	}

	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			a.showWindow()
		}
	})

	if err := ni.SetVisible(true); err != nil {
		return err
	}

	a.notifyIcon = ni
	return nil
}

func (a *TrayApp) bindStatus() {
	statuses := a.controller.Subscribe()
	go func() {
		for status := range statuses {
			a.mw.Synchronize(func() {
				a.statusLabel.SetText(status.Message)
				a.notifyIcon.SetToolTip(fmt.Sprintf("Tailclip: %s", status.Message))
			})
		}
	}()
}

func (a *TrayApp) loadInitialState() {
	startOnLogin, err := startup.Enabled()
	if err == nil {
		a.startOnLogin.SetChecked(startOnLogin)
	} else {
		a.statusLabel.SetText(err.Error())
	}

	if a.initialErr != nil {
		a.hasConfig = false
		a.populateForm(config.Default())
		a.controller.SetNeedsConfig(fmt.Sprintf("Config not loaded: %v", a.initialErr))
		a.refreshToggleAction()
		return
	}

	a.hasConfig = true
	a.currentConfig = a.initialConfig
	a.populateForm(a.initialConfig)
	a.controller.Apply(a.initialConfig)
	a.refreshToggleAction()
}

func (a *TrayApp) populateForm(cfg config.Config) {
	a.androidURLEdit.SetText(cfg.AndroidURL)
	a.authTokenEdit.SetText(cfg.AuthToken)
	a.deviceIDEdit.SetText(cfg.DeviceID)
	a.enabledCheck.SetChecked(cfg.Enabled)
}

func (a *TrayApp) save() {
	cfg := config.Default()
	if a.hasConfig {
		cfg = a.currentConfig
	}

	cfg.AndroidURL = strings.TrimSpace(a.androidURLEdit.Text())
	cfg.AuthToken = strings.TrimSpace(a.authTokenEdit.Text())
	cfg.DeviceID = strings.TrimSpace(a.deviceIDEdit.Text())
	cfg.Enabled = a.enabledCheck.Checked()

	if err := config.Save(a.configPath, cfg); err != nil {
		a.setError(err)
		return
	}

	if err := startup.SetEnabled(a.startOnLogin.Checked(), a.configPath); err != nil {
		a.setError(err)
		return
	}

	savedCfg, err := config.Load(a.configPath)
	if err != nil {
		a.setError(err)
		return
	}

	a.currentConfig = savedCfg
	a.hasConfig = true
	a.populateForm(savedCfg)
	a.controller.Apply(savedCfg)
	a.refreshToggleAction()
}

func (a *TrayApp) toggleSyncing() {
	if !a.hasConfig {
		a.showWindow()
		return
	}

	cfg := a.currentConfig
	cfg.Enabled = !cfg.Enabled
	if err := config.Save(a.configPath, cfg); err != nil {
		a.setError(err)
		return
	}

	savedCfg, err := config.Load(a.configPath)
	if err != nil {
		a.setError(err)
		return
	}

	a.currentConfig = savedCfg
	a.populateForm(savedCfg)
	a.controller.Apply(savedCfg)
	a.refreshToggleAction()
}

func (a *TrayApp) refreshToggleAction() {
	if !a.hasConfig {
		a.toggleSyncAction.SetEnabled(false)
		a.toggleSyncAction.SetText("Enable Syncing")
		return
	}

	a.toggleSyncAction.SetEnabled(true)
	if a.currentConfig.Enabled {
		a.toggleSyncAction.SetText("Disable Syncing")
		return
	}

	a.toggleSyncAction.SetText("Enable Syncing")
}

func (a *TrayApp) showWindow() {
	a.mw.Show()
	a.mw.SetFocus()
}

func (a *TrayApp) openConfigFolder() {
	dir := filepath.Dir(a.configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.setError(err)
		return
	}

	if err := exec.Command("explorer.exe", dir).Start(); err != nil {
		a.setError(err)
	}
}

func (a *TrayApp) setError(err error) {
	a.statusLabel.SetText(err.Error())
	if a.notifyIcon != nil {
		a.notifyIcon.ShowError("Tailclip", err.Error())
	}
}

func newTrayImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	bg := color.RGBA{R: 29, G: 78, B: 216, A: 255}
	fg := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, bg)
		}
	}

	for y := 3; y < 13; y++ {
		img.Set(4, y, fg)
	}
	for x := 4; x < 12; x++ {
		img.Set(x, 3, fg)
		img.Set(x, 8, fg)
	}

	return img
}
