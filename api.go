package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// PrintJob representa um job retornado pela API GustaMenu.
type PrintJob struct {
	ID           int    `json:"id"`
	CodigoPedido string `json:"codigo_pedido"`
	PaperWidth   int    `json:"paper_width"` // 58 ou 80 mm
	Copies       int    `json:"copies"`
	ReceiptText  string `json:"receipt_text"`
	Attempts     int    `json:"attempts"`
	CreatedAt    string `json:"created_at"`
}

// AgentSettings são as configurações de alarme por estágio definidas pelo
// lojista no painel (admin/impressao.php), entregues pela API a cada consulta.
type AgentSettings struct {
	AlarmePedidoCriado int    `json:"alarme_pedido_criado"`
	AlarmePagamento    int    `json:"alarme_pagamento"`
	ImprimirQuando     string `json:"imprimir_quando"`
}

// OrderEvent é um evento do ciclo do pedido: estágio "criado" (círculo
// AMARELO) ou "pago" (círculo VERDE).
type OrderEvent struct {
	PedidoID     int    `json:"pedido_id"`
	CodigoPedido string `json:"codigo_pedido"`
	Estagio      string `json:"estagio"`
}

type jobsResponse struct {
	OK   bool       `json:"ok"`
	Jobs []PrintJob `json:"jobs"`
	// Events/Settings ausentes (nil) indicam servidor antigo — o assistente
	// cai no comportamento legado de alarmar pelos jobs da fila.
	Events   []OrderEvent   `json:"events"`
	Settings *AgentSettings `json:"settings"`
}

type reportRequest struct {
	DeviceToken string `json:"device_token"`
	JobID       int    `json:"job_id"`
	Status      string `json:"status"`
	Error       string `json:"erro,omitempty"`
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

// fetchJobs busca jobs pendentes na fila de impressão, junto com os eventos
// do ciclo do pedido e as configurações de alarme da loja.
func fetchJobs(cfg Config) (*jobsResponse, error) {
	params := url.Values{}
	params.Set("device_token", cfg.DeviceToken)
	params.Set("limit", "10")

	resp, err := httpClient.Get(cfg.APIEndpoint + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 401, 403:
		return nil, fmt.Errorf("assistente não autorizado — verifique o código do assistente")
	case 200:
		// ok
	default:
		return nil, fmt.Errorf("API retornou status %d", resp.StatusCode)
	}

	var result jobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("resposta inválida da API: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API retornou ok=false")
	}

	return &result, nil
}

// reportJob notifica a API sobre o resultado da impressão.
func reportJob(cfg Config, jobID int, status, errMsg string) error {
	payload := reportRequest{
		DeviceToken: cfg.DeviceToken,
		JobID:       jobID,
		Status:      status,
		Error:       errMsg,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := httpClient.Post(cfg.APIEndpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("report retornou status %d", resp.StatusCode)
	}

	return nil
}
