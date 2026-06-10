package main

import (
	"log"
	"os"
	"path/filepath"
)

// logFilePath retorna o caminho do arquivo de log.
func logFilePath() string {
	return filepath.Join(configDir(), "agent.log")
}

// setupLog configura a saída de log para stdout + arquivo em disco.
func setupLog() {
	_ = os.MkdirAll(configDir(), 0755)

	f, err := os.OpenFile(logFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}

	// App é GUI (-H windowsgui): os.Stdout é inválido, então logamos só no
	// arquivo (senão o MultiWriter aborta no stdout e nada chega ao log).
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags)
}
