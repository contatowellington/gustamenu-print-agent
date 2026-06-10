package main

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// PrintWorker gerencia o loop de polling e impressão em background.
type PrintWorker struct {
	mu        sync.RWMutex
	cfg       Config
	status    chan string
	newOrders chan int
	alerts    chan string
	done      chan struct{}
	once      sync.Once
	// alerted: jobs que já contaram/alarmaram nesta sessão (não recontar).
	alerted map[int]bool
	// failedNotified: jobs cuja falha já foi avisada/reportada (não repetir o
	// aviso a cada tentativa). A impressão continua sendo tentada até o job
	// sair da fila — assim, ao corrigir a impressora, imprime sozinho.
	failedNotified map[int]bool
}

// NewPrintWorker cria um novo worker com a configuração inicial.
func NewPrintWorker(cfg Config) *PrintWorker {
	return &PrintWorker{
		cfg:            cfg,
		status:         make(chan string, 16),
		newOrders:      make(chan int, 16),
		alerts:         make(chan string, 8),
		done:           make(chan struct{}),
		alerted:        make(map[int]bool),
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
	jobs, err := fetchJobs(cfg)
	if err != nil {
		w.publish("Falha na consulta: " + err.Error())
		log.Printf("[poll] %v", err)
		return
	}
	if len(jobs) == 0 {
		return
	}

	// Conta/alarma só os pedidos inéditos nesta sessão (de-dupe por ID).
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

	// Tenta imprimir todos os jobs ainda na fila. Os que falham continuam na
	// fila e reentram na próxima consulta — assim, ao corrigir a impressora
	// (papel, conexão, etc.), imprimem sozinhos, sem reiniciar o assistente.
	for _, job := range jobs {
		if w.stopping() {
			return
		}
		w.doJob(cfg, job)
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

func (w *PrintWorker) signalAlert(msg string) {
	select {
	case w.alerts <- msg:
	default:
	}
}
