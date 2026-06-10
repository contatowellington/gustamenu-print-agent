//go:build windows

package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/lxn/walk"
)

var (
	appWindow *walk.MainWindow
	appTray   *walk.NotifyIcon
	appWorker *PrintWorker
	appWidget *FloatingWidget
	appAlarm  *Alarm
)

// runApp inicializa a janela principal oculta, o ícone na bandeja, o círculo
// flutuante e o worker de impressão.
func runApp() {
	var err error
	appWindow, err = walk.NewMainWindow()
	if err != nil {
		log.Fatalf("mainWindow: %v", err)
	}
	appWindow.SetVisible(false)

	appTray, err = walk.NewNotifyIcon(appWindow)
	if err != nil {
		log.Fatalf("notifyIcon: %v", err)
	}
	defer appTray.Dispose()

	if icon := loadTrayIcon(); icon != nil {
		_ = appTray.SetIcon(icon)
	}
	_ = appTray.SetToolTip("GustaMenu Impressão")
	_ = appTray.SetVisible(true)

	cfg, _ := loadConfig()
	appWorker = NewPrintWorker(cfg)
	appAlarm = NewAlarm()

	// Círculo flutuante (GARNET). Continua sempre-no-topo, ao lado do relógio.
	appWidget, err = NewFloatingWidget(openSettingsDialog, silenceAlarm, testAlarm, quitApp)
	if err != nil {
		log.Printf("widget flutuante indisponível: %v", err)
	}

	buildTrayMenu()

	// Status do worker → tooltip da bandeja + círculo.
	go func() {
		for status := range appWorker.StatusCh() {
			s := status
			appWindow.Synchronize(func() {
				tip := "GustaMenu — " + s
				if len(tip) > 63 {
					tip = tip[:63]
				}
				_ = appTray.SetToolTip(tip)
			})
			if appWidget != nil {
				appWidget.SetStatus(s)
			}
		}
	}()

	// Pedido novo → contador + alarme sonoro/visual.
	go func() {
		for n := range appWorker.NewOrderCh() {
			onNewOrder(n)
		}
	}()

	appWorker.Start()

	// Abre a configuração automaticamente se não estiver configurado — mas só
	// DEPOIS que o loop principal iniciar. Chamar o diálogo modal antes do
	// Run() faz o app fechar ao clicar em Salvar (o fim do loop modal, sendo o
	// único loop ativo, encerra a thread). Via Synchronize ele roda aninhado
	// no Run() e, ao fechar, o app continua residente na bandeja.
	if !cfg.IsValid() {
		appWindow.Synchronize(openSettingsDialog)
	}

	appWindow.Run()
}

// onNewOrder dispara o alarme e o flash ao receber pedidos novos.
func onNewOrder(n int) {
	cfg, _ := loadConfig()
	if appWidget != nil {
		appWidget.Show() // garante que o círculo reaparece se estava escondido
		appWidget.AddCount(n)
		appWidget.StartFlash(cfg.NormalizedAlarmSeconds())
	}
	if cfg.AlarmEnabled {
		appAlarm.Start(cfg.NormalizedAlarmSeconds())
	}
}

// silenceAlarm para o alarme sonoro e o flash visual.
func silenceAlarm() {
	if appAlarm != nil {
		appAlarm.Stop()
	}
	if appWidget != nil {
		appWidget.StopFlash()
	}
}

// showWidget traz o círculo de volta à tela (caso o usuário o tenha fechado).
func showWidget() {
	if appWidget != nil {
		appWidget.Show()
	}
}

// testAlarm dispara o alarme de teste (som + flash).
func testAlarm() {
	cfg, _ := loadConfig()
	if appWidget != nil {
		appWidget.StartFlash(cfg.NormalizedAlarmSeconds())
	}
	appAlarm.Start(cfg.NormalizedAlarmSeconds())
}

// quitApp encerra o assistente por completo.
func quitApp() {
	silenceAlarm()
	if appTray != nil {
		_ = appTray.SetVisible(false)
	}
	if appWorker != nil {
		appWorker.Stop()
	}
	walk.App().Exit(0)
}

func buildTrayMenu() {
	addAction := func(text string, handler func()) {
		a := walk.NewAction()
		_ = a.SetText(text)
		a.Triggered().Attach(handler)
		_ = appTray.ContextMenu().Actions().Add(a)
	}

	addAction("Configurar", openSettingsDialog)
	addAction("Mostrar círculo", showWidget)
	addAction("Imprimir teste", printTestReceipt)
	addAction("Silenciar alarme", silenceAlarm)
	addAction("Abrir log", openLogFile)
	_ = appTray.ContextMenu().Actions().Add(walk.NewSeparatorAction())
	addAction("Sair", quitApp)

	appTray.MouseDown().Attach(func(_, _ int, btn walk.MouseButton) {
		if btn == walk.LeftButton {
			openSettingsDialog()
		}
	})
}

func openSettingsDialog() {
	current, _ := loadConfig()
	ok, newCfg := runSettingsDialog(appWindow, current)
	if !ok {
		return
	}
	if err := saveConfig(newCfg); err != nil {
		walk.MsgBox(appWindow, "GustaMenu", "Erro ao salvar configuração:\n"+err.Error(), walk.MsgBoxIconWarning)
		return
	}
	setStartWithWindows(newCfg.StartWithWindows)
	appWorker.UpdateConfig(newCfg)
	log.Printf("Configuração salva. Impressora: %q", newCfg.Printer)
}

func printTestReceipt() {
	cfg, _ := loadConfig()
	receipt := "GustaMenu\r\nCUPOM NAO FISCAL\r\nTeste de impressao\r\n" +
		time.Now().Format("02/01/2006 15:04") + "\r\n\r\n"

	job := PrintJob{CodigoPedido: "TESTE", PaperWidth: 80, Copies: 1, ReceiptText: receipt}
	if err := printJob(cfg, job); err != nil {
		walk.MsgBox(appWindow, "Falha no teste", err.Error(), walk.MsgBoxIconWarning)
	} else {
		walk.MsgBox(appWindow, "GustaMenu", "Teste enviado para a impressora.", walk.MsgBoxIconInformation)
	}
}

func openLogFile() {
	path := logFilePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		_ = os.WriteFile(path, nil, 0644)
	}
	_ = exec.Command("notepad.exe", path).Start()
}

// loadTrayIcon tenta carregar o ícone do mesmo diretório do executável.
func loadTrayIcon() *walk.Icon {
	exePath, err := os.Executable()
	if err != nil {
		return nil
	}
	icoPath := filepath.Join(filepath.Dir(exePath), "gustamenu.ico")
	icon, err := walk.NewIconFromFile(icoPath)
	if err != nil {
		return nil
	}
	return icon
}
