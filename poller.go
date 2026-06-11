package main

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// StageEvent sinaliza a mudança de estágio do ciclo do pedido para a UI:
// "criado" (círculo AMARELO) ou "pago" (círculo VERDE), com a quantidade de
// pedidos no estágio e se o alarme sonoro deve tocar (decisão do lojista no
// painel de impressão).
type StageEvent struct {
	Estagio string
	Count   int
	Alarm   bool
}

// PrintWorker gerencia o loop de polling e impressão em background.
type PrintWorker struct {
	mu        sync.RWMutex
	cfg       Config
	status    chan string
	newOrders chan int
	stages    chan StageEvent
	health    chan string
	alerts    chan string
	done      chan struct{}
	once      sync.Once
	// alerted: jobs que já contaram/alarmaram nesta sessão (não recontar).
	alerted map[int]bool
	// seenEvents: eventos de estágio (pedido_id:estagio) já tratados na sessão.
	seenEvents map[string]bool
	// failedNotified: jobs cuja falha já foi avisada/reportada (não repetir o
	// aviso a cada tentativa). A impressão continua sendo tentada até o job
	// sair da fila — assim, ao corrigir a impressora, imprime sozinho.
	failedNotified map[int]bool
	// pollFailed: a última consulta falhou — ao recuperar, publica "Conectado."
	// para o círculo não ficar vermelho para sempre após uma falha transitória.
	pollFailed bool
	// printerBad: a última tentativa de impressão falhou (badge vermelho).
	printerBad bool
	// lastHealth: último estado de saúde enviado (de-dupe do canal).
	lastHealth string
}

// NewPrintWorker cria um novo worker com a configuração inicial.
func NewPrintWorker(cfg Config) *PrintWorker {
	return &PrintWorker{
		cfg:            cfg,
		status:         make(chan string, 16),
		newOrders:      make(chan int, 16),
		stages:         make(chan StageEvent, 16),
		health:         make(chan string, 16),
		alerts:         make(chan string, 8),
		done:           make(chan struct{}),
		alerted:        make(map[int]bool),
		seenEvents:     make(map[string]bool),
		failedNotified: make(map[int]bool),
	}
}

// UpdateConfig atualiza a configuração em uso pelo worker.
func (w *PrintWorker) UpdateConfig(cfg Config) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cfg = cfg
}

// StatusCh retorna o canal de mensagens de status (read-only).
func (w *PrintWorker) StatusCh() <-chan string {
	return w.status
}

// NewOrderCh retorna o canal que sinaliza a chegada de pedidos novos,
// com a quantidade recebida em cada consulta (read-only).
func (w *PrintWorker) NewOrderCh() <-chan int {
	return w.newOrders
}

// StageCh retorna o canal de eventos de estágio do ciclo do pedido
// (AMARELO = criado, VERDE = pago), read-only.
func (w *PrintWorker) StageCh() <-chan StageEvent {
	return w.stages
}

// HealthCh retorna o canal de saúde do assistente para o badge do círculo:
// "ok" (verde), "erro" (vermelho) ou "config" (cinza), read-only.
func (w *PrintWorker) HealthCh() <-chan string {
	return w.health
}

// AlertCh retorna o canal de alertas para o usuário (ex.: falha de
// impressora), exibidos como notificação na bandeja (read-only).
func (w *PrintWorker) AlertCh() <-chan string {
	return w.alerts
}

// Start inicia o loop de polling (idempotente).
func (w *PrintWorker) Start() {
	w.once.Do(func() {
		go w.run()
	})
}

// Stop encerra o loop de polling.
func (w *PrintWorker) Stop() {
	close(w.done)
}

func (w *PrintWorker) run() {
	w.publish("Assistente iniciado.")

	for {
		w.mu.RLock()
		cfg := w.cfg
		w.mu.RUnlock()

		if !cfg.IsValid() {
			w.signalHealth("config")
			w.publish("Aguardando configuração.")
			select {
			case <-w.done:
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}

		w.doPoll(cfg)

		select {
		case <-w.done:
			return
		case <-time.After(time.Duration(cfg.NormalizedPollSeconds()) * time.Second):
		}
	}
}

func (w *PrintWorker) doPoll(cfg Config) {
	resp, err := fetchJobs(cfg)
	if err != nil {
		w.pollFailed = true
		w.signalHealth("erro")
		w.publish("Falha na consulta: " + err.Error())
		log.Printf("[poll] %v", err)
		return
	}

	// Recuperou de uma falha transitória: avisa que está tudo bem de novo,
	// senão o status de falha fica exibido para sempre.
	if w.pollFailed {
		w.pollFailed = false
		w.publish("Conectado.")
	}
	w.signalHealthAuto()

	// Servidor novo entrega eventos do ciclo do pedido + alarmes por estágio.
	// Servidor antigo (sem settings) cai no comportamento legado por jobs.
	if resp.Settings != nil {
		w.handleEvents(resp.Settings, resp.Events)
	} else {
		w.legacyJobAlerts(resp.Jobs)
	}

	// Tenta imprimir todos os jobs ainda na fila. Os que falham continuam na
	// fila e reentram na próxima consulta — assim, ao corrigir a impressora
	// (papel, conexão, etc.), imprimem sozinhos, sem reiniciar o assistente.
	for _, job := range resp.Jobs {
		if w.stopping() {
			return
		}
		w.doJob(cfg, job)
	}
	w.signalHealthAuto()
}

// handleEvents processa os eventos do ciclo do pedido (de-dupe por
// pedido_id:estagio) e emite os estágios AMARELO/VERDE para a UI. O estágio
// "aceito" silencia o alarme (padrão iFood: toca até a ação do lojista) —
// mas só quando não chegou pedido/pagamento novo no mesmo lote, senão o
// aceite de um pedido antigo calaria o alarme do pedido recém-chegado.
func (w *PrintWorker) handleEvents(s *AgentSettings, events []OrderEvent) {
	criados, pagos, aceitos := 0, 0, 0
	for _, ev := range events {
		key := fmt.Sprintf("%d:%s", ev.PedidoID, ev.Estagio)
		if w.seenEvents[key] {
			continue
		}
		w.seenEvents[key] = true
		switch ev.Estagio {
		case "criado":
			criados++
		case "pago":
			pagos++
		case "aceito":
			aceitos++
		}
	}

	if criados > 0 {
		w.publish(fmt.Sprintf("%d pedido(s) novo(s).", criados))
		w.signalStage(StageEvent{Estagio: "criado", Count: criados, Alarm: s.AlarmePedidoCriado == 1})
	}
	if pagos > 0 {
		w.publish(fmt.Sprintf("%d pagamento(s) confirmado(s).", pagos))
		w.signalStage(StageEvent{Estagio: "pago", Count: pagos, Alarm: s.AlarmePagamento == 1})
	}
	if aceitos > 0 && criados == 0 && pagos == 0 {
		w.signalStage(StageEvent{Estagio: "aceito", Count: aceitos})
	}
}

// legacyJobAlerts mantém o alarme por jobs da fila quando o servidor ainda
// não envia eventos de estágio (de-dupe por ID, como antes).
func (w *PrintWorker) legacyJobAlerts(jobs []PrintJob) {
	novos := 0
	for _, job := range jobs {
		if !w.alerted[job.ID] {
			w.alerted[job.ID] = true
			novos++
		}
	}
	if novos > 0 {
		w.publish(fmt.Sprintf("%d cupom(ns) recebido(s).", novos))
		w.signalNewOrders(novos)
	}
}

func (w *PrintWorker) doJob(cfg Config, job PrintJob) {
	copies := job.Copies
	if copies < 1 {
		copies = 1
	}
	if copies > 5 {
		copies = 5
	}

	var printErr error
	for i := 0; i < copies; i++ {
		if err := printJob(cfg, job); err != nil {
			printErr = err
			break
		}
	}

	if printErr != nil {
		w.printerBad = true
		log.Printf("[job %d] falha de impressao: %v", job.ID, printErr)
		// Avisa o usuário e reporta a falha só UMA vez por job. A impressão
		// continua sendo tentada nas próximas consultas (auto-retry).
		if !w.failedNotified[job.ID] {
			w.failedNotified[job.ID] = true
			w.publish("Impressora indisponível — verifique em Configurar.")
			w.signalAlert("Não consegui imprimir o pedido " + job.CodigoPedido + ".\n" +
				"A impressora pode estar desligada, desconectada, sem papel ou não instalada.\n" +
				"Assim que ela voltar, imprimo sozinho — ou abra Configurar para trocar a impressora.")
			if err := reportJob(cfg, job.ID, "falhou", printErr.Error()); err != nil {
				log.Printf("[job %d] report falhou: %v", job.ID, err)
			}
		}
		return
	}

	// Sucesso — limpa o estado de falha e reporta.
	w.printerBad = false
	delete(w.failedNotified, job.ID)
	w.publish("Pedido " + job.CodigoPedido + " impresso.")
	log.Printf("[job %d] impresso (%dx)", job.ID, copies)
	if err := reportJob(cfg, job.ID, "impresso", ""); err != nil {
		log.Printf("[job %d] report: %v", job.ID, err)
	}
}

func (w *PrintWorker) stopping() bool {
	select {
	case <-w.done:
		return true
	default:
		return false
	}
}

func (w *PrintWorker) publish(status string) {
	log.Printf("[status] %s", status)
	select {
	case w.status <- status:
	default:
	}
}

func (w *PrintWorker) signalNewOrders(n int) {
	select {
	case w.newOrders <- n:
	default:
	}
}

func (w *PrintWorker) signalStage(ev StageEvent) {
	select {
	case w.stages <- ev:
	default:
	}
}

// signalHealth envia o estado de saúde para o badge, com de-dupe para não
// repintar o círculo a cada consulta.
func (w *PrintWorker) signalHealth(state string) {
	if state == w.lastHealth {
		return
	}
	w.lastHealth = state
	select {
	case w.health <- state:
	default:
	}
}

// signalHealthAuto deriva a saúde atual: conexão ok, mas impressora com
// falha ainda conta como problema (badge vermelho).
func (w *PrintWorker) signalHealthAuto() {
	if w.printerBad {
		w.signalHealth("erro")
		return
	}
	w.signalHealth("ok")
}

func (w *PrintWorker) signalAlert(msg string) {
	select {
	case w.alerts <- msg:
	default:
	}
}
