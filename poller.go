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
	done      chan struct{}
	once      sync.Once
	// seen guarda os IDs de jobs já processados nesta sessão, para não
	// recontar/reimprimir os mesmos pedidos a cada consulta (ex.: quando a
	// impressão falha e o job continua pendente na fila). Acessado só pela
	// goroutine de polling.
	seen map[int]bool
}

// NewPrintWorker cria um novo worker com a configuração inicial.
func NewPrintWorker(cfg Config) *PrintWorker {
	return &PrintWorker{
		cfg:       cfg,
		status:    make(chan string, 16),
		newOrders: make(chan int, 16),
		done:      make(chan struct{}),
		seen:      make(map[int]bool),
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

	// Filtra só os pedidos inéditos nesta sessão. Jobs que continuam na fila
	// (ex.: impressão falhou) não são recontados nem reimpressos a cada poll.
	fresh := jobs[:0:0]
	for _, job := range jobs {
		if !w.seen[job.ID] {
			w.seen[job.ID] = true
			fresh = append(fresh, job)
		}
	}
	if len(fresh) == 0 {
		return
	}

	w.publish(fmt.Sprintf("%d cupom(ns) recebido(s).", len(fresh)))
	w.signalNewOrders(len(fresh))
	for _, job := range fresh {
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

	w.publish("Imprimindo pedido " + job.CodigoPedido + ".")
	log.Printf("[job %d] imprimindo %s", job.ID, job.CodigoPedido)

	var printErr error
	for i := 0; i < copies; i++ {
		if err := printJob(cfg, job); err != nil {
			printErr = err
			log.Printf("[job %d] cópia %d: %v", job.ID, i+1, err)
			break
		}
	}

	if printErr != nil {
		w.publish("Falha ao imprimir " + job.CodigoPedido + ".")
		log.Printf("[job %d] falhou: %v", job.ID, printErr)
		if err := reportJob(cfg, job.ID, "falhou", printErr.Error()); err != nil {
			log.Printf("[job %d] report falhou: %v", job.ID, err)
		}
		return
	}

	w.publish("Pedido " + job.CodigoPedido + " impresso.")
	log.Printf("[job %d] impresso (%dx)", job.ID, copies)
	if err := reportJob(cfg, job.ID, "impresso", ""); err != nil {
		log.Printf("[job %d] report: %v", job.ID, err)
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
