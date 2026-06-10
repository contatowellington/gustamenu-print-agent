//go:build windows

package main

import (
	"log"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// runSettingsDialog exibe o formulário de configuração modal.
// Retorna (true, newCfg) se o usuário salvou, ou (false, current) se cancelou.
func runSettingsDialog(owner walk.Form, current Config) (bool, Config) {
	result := current
	saved := false

	var dlg *walk.Dialog
	var endpointEdit *walk.LineEdit
	var tokenEdit *walk.LineEdit
	var printerCombo *walk.ComboBox
	var pollEdit *walk.NumberEdit
	var startupCheck *walk.CheckBox
	var alarmCheck *walk.CheckBox
	var alarmSecondsEdit *walk.NumberEdit

	printers := installedPrinters()

	// Índice inicial da impressora no combo
	printerIndex := 0
	for i, p := range printers {
		if p == current.Printer {
			printerIndex = i
			break
		}
	}

	err := (Dialog{
		AssignTo:  &dlg,
		Title:     "GustaMenu — Configurar Assistente",
		MinSize:   Size{Width: 520, Height: 520},
		MaxSize:   Size{Width: 520, Height: 520},
		Layout:    VBox{Margins: Margins{Left: 16, Right: 16, Top: 16, Bottom: 16}},
		Children: []Widget{

			// Cabeçalho
			Composite{
				Layout: VBox{MarginsZero: true, SpacingZero: true},
				Children: []Widget{
					Label{
						Text: "Assistente de Impressão GustaMenu",
						Font: Font{Bold: true, PointSize: 14},
					},
					Label{
						Text: "Cole o código gerado no painel e selecione a impressora térmica.",
					},
				},
			},

			// Campos do formulário (grid 2 colunas: label | input)
			Composite{
				Layout: Grid{Columns: 2, Spacing: 6},
				Children: []Widget{
					Label{Text: "Endpoint da API:"},
					LineEdit{AssignTo: &endpointEdit, Text: current.APIEndpoint},

					Label{Text: "Código do Assistente Windows:"},
					LineEdit{
						AssignTo:     &tokenEdit,
						Text:         current.DeviceToken,
						PasswordMode: true,
					},

					Label{Text: "Impressora:"},
					ComboBox{
						AssignTo:     &printerCombo,
						Model:        printers,
						CurrentIndex: printerIndex,
					},

					Label{Text: "Intervalo de consulta (segundos):"},
					NumberEdit{
						AssignTo: &pollEdit,
						Value:    float64(current.NormalizedPollSeconds()),
						Decimals: 0,
						MinValue: 3,
						MaxValue: 60,
					},

					Label{Text: "Duração do alarme (segundos):"},
					NumberEdit{
						AssignTo: &alarmSecondsEdit,
						Value:    float64(current.NormalizedAlarmSeconds()),
						Decimals: 0,
						MinValue: 5,
						MaxValue: 600,
					},
				},
			},

			CheckBox{
				AssignTo: &startupCheck,
				Text:     "Abrir junto com o Windows",
				Checked:  current.StartWithWindows,
			},

			CheckBox{
				AssignTo: &alarmCheck,
				Text:     "Alarme sonoro ao chegar pedido novo",
				Checked:  current.AlarmEnabled,
			},

			VSpacer{},

			// Botões de ação
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					PushButton{
						Text: "Atualizar impressoras",
						OnClicked: func() {
							refreshPrinters(printerCombo)
						},
					},
					PushButton{
						Text: "Cancelar",
						OnClicked: func() {
							dlg.Cancel()
						},
					},
					PushButton{
						Text: "Salvar",
						OnClicked: func() {
							newCfg := Config{
								APIEndpoint:      endpointEdit.Text(),
								DeviceToken:      tokenEdit.Text(),
								Printer:          printerCombo.Text(),
								PollSeconds:      int(pollEdit.Value()),
								StartWithWindows: startupCheck.Checked(),
								AlarmEnabled:     alarmCheck.Checked(),
								AlarmSeconds:     int(alarmSecondsEdit.Value()),
							}
							log.Printf("[settings] Salvar clicado: token_len=%d endpoint=%q printer=%q valid=%v",
								len(newCfg.DeviceToken), newCfg.APIEndpoint, newCfg.Printer, newCfg.IsValid())
							if !newCfg.IsValid() {
								walk.MsgBox(
									dlg,
									"GustaMenu",
									"Preencha o código do assistente.",
									walk.MsgBoxIconWarning,
								)
								return
							}
							result = newCfg
							saved = true
							dlg.Accept()
						},
					},
				},
			},
		},
	}).Create(owner)

	if err != nil {
		walk.MsgBox(owner, "GustaMenu", "Erro ao abrir configurações:\n"+err.Error(), walk.MsgBoxIconWarning)
		return false, current
	}

	dlg.Run()
	return saved, result
}

// refreshPrinters atualiza a lista de impressoras no ComboBox.
func refreshPrinters(combo *walk.ComboBox) {
	current := combo.Text()
	printers := installedPrinters()
	_ = combo.SetModel(printers)
	for i, p := range printers {
		if p == current {
			_ = combo.SetCurrentIndex(i)
			return
		}
	}
	if len(printers) > 0 {
		_ = combo.SetCurrentIndex(0)
	}
}
