package main

import (
	"io"
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

	log.SetOutput(io.MultiWriter(os.Stdout, f))
	log.SetFlags(log.LstdFlags)
}
