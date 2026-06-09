//go:build windows

package main

import (
	"sync"
	"time"
)

// procBeep reutiliza o kernel32 já carregado em startup_windows.go.
var procBeep = kernel32.NewProc("Beep")

// Padrão ascendente de bipes do alarme (frequência Hz, duração ms).
var alarmPattern = []struct{ freq, dur int }{
	{880, 180},
	{1175, 180},
	{1568, 260},
}

// Pausa entre cada repetição do padrão (deixa o alarme "intermitente").
const alarmGap = 500 * time.Millisecond

// Alarm toca um alarme sonoro intermitente em background.
//
// Diferente da versão Python (que repetia um número fixo de vezes), aqui o
// alarme toca em loop contínuo até ser silenciado (Stop) ou até o limite de
// segundos informado em Start expirar.
type Alarm struct {
	mu      sync.Mutex
	running bool
	stop    chan struct{}
}

// NewAlarm cria um controlador de alarme ocioso.
func NewAlarm() *Alarm { return &Alarm{} }

// Start dispara o alarme intermitente. Se já estiver tocando, não reinicia.
// maxSeconds limita a duração total; <= 0 significa sem limite de tempo.
func (a *Alarm) Start(maxSeconds int) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return
	}
	a.running = true
	stop := make(chan struct{})
	a.stop = stop
	a.mu.Unlock()

	go a.loop(stop, maxSeconds)
}

// Stop silencia o alarme imediatamente.
func (a *Alarm) Stop() {
	a.mu.Lock()
	if a.running && a.stop != nil {
		close(a.stop)
		a.stop = nil
	}
	a.mu.Unlock()
}

func (a *Alarm) loop(stop chan struct{}, maxSeconds int) {
	defer func() {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
	}()

	var deadline <-chan time.Time
	if maxSeconds > 0 {
		t := time.NewTimer(time.Duration(maxSeconds) * time.Second)
		defer t.Stop()
		deadline = t.C
	}

	for {
		for _, b := range alarmPattern {
			select {
			case <-stop:
				return
			case <-deadline:
				return
			default:
			}
			beep(b.freq, b.dur)
		}

		select {
		case <-stop:
			return
		case <-deadline:
			return
		case <-time.After(alarmGap):
		}
	}
}

// beep emite um bip síncrono via kernel32.Beep (bloqueia durationMs).
func beep(freq, durationMs int) {
	procBeep.Call(uintptr(freq), uintptr(durationMs))
}
