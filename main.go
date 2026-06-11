package main

import (
	"github.com/lxn/walk"
)

// Versão do Assistente de Impressão GustaMenu.
const appVersion = "1.6.0"

func main() {
	if !acquireSingleInstance() {
		walk.MsgBox(
			nil,
			"GustaMenu",
			"O Assistente de Impressão GustaMenu já está em execução.",
			walk.MsgBoxIconInformation,
		)
		return
	}

	setupLog()
	runApp()
}
