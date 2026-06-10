package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Config armazena as configurações do assistente de impressão GustaMenu.
type Config struct {
	APIEndpoint      string `json:"api_endpoint"`
	DeviceToken      string `json:"device_token"`
	Printer          string `json:"printer_name"`
	PollSeconds      int    `json:"poll_seconds"`
	StartWithWindows bool   `json:"start_with_windows"`
	// Alarme sonoro + visual ao chegar pedido novo.
	AlarmEnabled bool `json:"alarm_enabled"`
	// Duração máxima do alarme intermitente, em segundos. O alarme toca em
	// loop até ser silenciado (clique no círculo / menu) ou até este limite.
	AlarmSeconds int `json:"alarm_seconds"`
}

// IsValid retorna true se os campos obrigatórios estão preenchidos.
func (c Config) IsValid() bool {
	return c.DeviceToken != "" && c.APIEndpoint != ""
}

// NormalizedPollSeconds retorna o intervalo de polling em segundos (mín 3, máx 60).
func (c Config) NormalizedPollSeconds() int {
	if c.PollSeconds < 3 {
		return 5
	}
	if c.PollSeconds > 60 {
		return 60
	}
	return c.PollSeconds
}

// NormalizedAlarmSeconds retorna a duração do alarme (mín 5, máx 600 segundos).
func (c Config) NormalizedAlarmSeconds() int {
	if c.AlarmSeconds < 5 {
		return 60
	}
	if c.AlarmSeconds > 600 {
		return 600
	}
	return c.AlarmSeconds
}

// configDir retorna o diretório de configuração do assistente.
func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "GustaMenu", "PrintAgent")
}

// configPath retorna o caminho completo do arquivo de configuração.
func configPath() string {
	return filepath.Join(configDir(), "settings.json")
}

// defaultConfig retorna a configuração padrão.
func defaultConfig() Config {
	return Config{
		APIEndpoint:      "https://gustamenu.com.br/api/print_jobs.php",
		PollSeconds:      5,
		StartWithWindows: true,
		AlarmEnabled:     true,
		AlarmSeconds:     60,
	}
}

// loadConfig lê a configuração do disco. Se não existir, retorna os defaults.
func loadConfig() (Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("ler config: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig(), fmt.Errorf("config inválida: %w", err)
	}

	if cfg.APIEndpoint == "" {
		cfg.APIEndpoint = defaultConfig().APIEndpoint
	}
	if cfg.AlarmSeconds == 0 {
		cfg.AlarmSeconds = defaultConfig().AlarmSeconds
	}

	log.Printf("[config] loadConfig token_len=%d endpoint=%q", len(cfg.DeviceToken), cfg.APIEndpoint)
	return cfg, nil
}

// saveConfig persiste a configuração no disco.
func saveConfig(cfg Config) error {
	if err := os.MkdirAll(configDir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(configPath(), data, 0600)
	log.Printf("[config] saveConfig token_len=%d printer=%q path=%q err=%v",
		len(cfg.DeviceToken), cfg.Printer, configPath(), err)
	return err
}
