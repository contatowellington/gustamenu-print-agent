//go:build windows

package main

import (
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

// Funções Win32 não expostas pelo lxn/win — carregadas direto.
var (
	procCreateEllipticRgn = syscall.NewLazyDLL("gdi32.dll").NewProc("CreateEllipticRgn")
	procSetWindowRgn      = syscall.NewLazyDLL("user32.dll").NewProc("SetWindowRgn")
)

func createEllipticRgn(x1, y1, x2, y2 int32) uintptr {
	r, _, _ := procCreateEllipticRgn.Call(uintptr(x1), uintptr(y1), uintptr(x2), uintptr(y2))
	return r
}

func setWindowRgn(hwnd win.HWND, hrgn uintptr, redraw bool) {
	var b uintptr
	if redraw {
		b = 1
	}
	procSetWindowRgn.Call(uintptr(hwnd), hrgn, b)
}

// Paleta do widget (espelha a versão Python).
var (
	colBrand = walk.RGB(0xFF, 0x7A, 0x00) // laranja GustaMenu (ocioso/normal)
	colOK    = walk.RGB(0x0A, 0x9D, 0x4E) // verde — operando/imprimiu
	colWait  = walk.RGB(0x8A, 0x8A, 0x8A) // cinza — aguardando configuração
	colErr   = walk.RGB(0xC6, 0x28, 0x28) // vermelho — erro/sem autorização
	colAlarm = walk.RGB(0xE5, 0x39, 0x35) // vermelho do alarme visual
	colWhite = walk.RGB(0xFF, 0xFF, 0xFF)
)

// FloatingWidget é o círculo sempre-no-topo, arrastável, com contador de
// pedidos e status. Pisca (alarme visual) quando chega pedido novo.
type FloatingWidget struct {
	mw   *walk.MainWindow
	cw   *walk.CustomWidget
	size int

	// estado de pintura — sempre alterado na thread de UI (via Synchronize).
	baseColor walk.Color
	status    string
	count     int

	// alarme visual (flash)
	flashing  bool
	flashOn   bool
	flashStop chan struct{}

	// fontes cacheadas
	fontTitle *walk.Font
	fontCount *walk.Font
	fontSmall *walk.Font
	fontStat  *walk.Font

	// arrastar
	dragStartX int
	dragStartY int
	dragged    bool

	onConfigure func()
	onSilence   func()
	onTestAlarm func()
	onQuit      func()
}

// statusColor mapeia um texto de status para a cor do círculo.
func statusColor(text string) walk.Color {
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "falha"), strings.Contains(t, "não autorizado"),
		strings.Contains(t, "nao autorizado"), strings.Contains(t, "erro"):
		return colErr
	case strings.Contains(t, "aguardando configura"):
		return colWait
	case strings.Contains(t, "impresso"), strings.Contains(t, "imprimindo"),
		strings.Contains(t, "recebido"), strings.Contains(t, "iniciado"):
		return colOK
	default:
		return colBrand
	}
}

// NewFloatingWidget cria e exibe o círculo flutuante.
func NewFloatingWidget(onConfigure, onSilence, onTestAlarm, onQuit func()) (*FloatingWidget, error) {
	w := &FloatingWidget{
		size:        132,
		baseColor:   colBrand,
		status:      "Iniciando…",
		onConfigure: onConfigure,
		onSilence:   onSilence,
		onTestAlarm: onTestAlarm,
		onQuit:      onQuit,
	}

	w.fontTitle, _ = walk.NewFont("Segoe UI", 9, walk.FontBold)
	w.fontCount, _ = walk.NewFont("Segoe UI", 22, walk.FontBold)
	w.fontSmall, _ = walk.NewFont("Segoe UI", 7, 0)
	w.fontStat, _ = walk.NewFont("Segoe UI", 8, 0)

	if err := (MainWindow{
		AssignTo: &w.mw,
		Title:    "GustaMenu",
		Size:     Size{Width: w.size, Height: w.size},
		Layout:   VBox{MarginsZero: true, SpacingZero: true},
		Children: []Widget{
			CustomWidget{
				AssignTo:            &w.cw,
				ClearsBackground:    true,
				InvalidatesOnResize: true,
				Paint:               w.paint,
			},
		},
	}).Create(); err != nil {
		return nil, err
	}

	w.applyShape()
	w.wireInput()
	return w, nil
}

// applyShape transforma a janela num círculo sem bordas, sempre-no-topo,
// fora da barra de tarefas, posicionado no canto superior direito.
func (w *FloatingWidget) applyShape() {
	hwnd := w.mw.Handle()
	w.mw.SetVisible(false)

	win.SetWindowLongPtr(hwnd, win.GWL_STYLE, uintptr(win.WS_POPUP|win.WS_VISIBLE))
	win.SetWindowLongPtr(hwnd, win.GWL_EXSTYLE,
		uintptr(win.WS_EX_TOPMOST|win.WS_EX_TOOLWINDOW|win.WS_EX_NOACTIVATE))

	rgn := createEllipticRgn(0, 0, int32(w.size), int32(w.size))
	setWindowRgn(hwnd, rgn, true)

	sw := win.GetSystemMetrics(win.SM_CXSCREEN)
	x := sw - int32(w.size) - 30
	y := int32(70)
	win.SetWindowPos(hwnd, win.HWND_TOPMOST, x, y, int32(w.size), int32(w.size),
		win.SWP_FRAMECHANGED|win.SWP_NOACTIVATE|win.SWP_SHOWWINDOW)
}

// wireInput liga arrastar, clique (silenciar) e menu de contexto.
func (w *FloatingWidget) wireInput() {
	w.cw.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton {
			return
		}
		w.dragStartX, w.dragStartY = x, y
		w.dragged = false
	})

	w.cw.MouseMove().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton {
			return
		}
		if !w.dragged && abs(x-w.dragStartX)+abs(y-w.dragStartY) < 4 {
			return
		}
		w.dragged = true
		var pt win.POINT
		win.GetCursorPos(&pt)
		win.SetWindowPos(w.mw.Handle(), 0,
			pt.X-int32(w.dragStartX), pt.Y-int32(w.dragStartY), 0, 0,
			win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)
	})

	w.cw.MouseUp().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton && !w.dragged && w.onSilence != nil {
			w.onSilence()
		}
	})

	if menu, err := walk.NewMenu(); err == nil {
		add := func(text string, h func()) {
			a := walk.NewAction()
			_ = a.SetText(text)
			if h != nil {
				a.Triggered().Attach(h)
			}
			_ = menu.Actions().Add(a)
		}
		add("Configurar…", w.onConfigure)
		add("Testar alarme", w.onTestAlarm)
		add("Silenciar", w.onSilence)
		_ = menu.Actions().Add(walk.NewSeparatorAction())
		add("Sair", w.onQuit)
		w.cw.SetContextMenu(menu)
	}
}

// --- pintura -------------------------------------------------------------

func (w *FloatingWidget) paint(canvas *walk.Canvas, _ walk.Rectangle) error {
	s := w.size
	m := 8
	rect := walk.Rectangle{X: m, Y: m, Width: s - 2*m, Height: s - 2*m}

	fill := w.baseColor
	if w.flashing {
		if w.flashOn {
			fill = colAlarm
		} else {
			fill = colBrand
		}
	}

	brush, err := walk.NewSolidColorBrush(fill)
	if err != nil {
		return err
	}
	defer brush.Dispose()
	if err := canvas.FillEllipsePixels(brush, rect); err != nil {
		return err
	}

	whiteBrush, err := walk.NewSolidColorBrush(colWhite)
	if err != nil {
		return err
	}
	defer whiteBrush.Dispose()
	if pen, err := walk.NewGeometricPen(walk.PenSolid, 3, whiteBrush); err == nil {
		defer pen.Dispose()
		_ = canvas.DrawEllipsePixels(pen, rect)
	}

	if w.flashing && w.flashOn {
		w.text(canvas, "PEDIDO!", w.fontCount, 0.42)
		w.text(canvas, "NOVO", w.fontStat, 0.64)
	} else {
		w.text(canvas, "GustaMenu", w.fontTitle, 0.25)
		w.text(canvas, strconv.Itoa(w.count), w.fontCount, 0.48)
		w.text(canvas, "pedidos", w.fontSmall, 0.70)
		w.text(canvas, w.status, w.fontStat, 0.85)
	}
	return nil
}

func (w *FloatingWidget) text(c *walk.Canvas, s string, font *walk.Font, yFrac float64) {
	if font == nil {
		return
	}
	h := 22
	y := int(float64(w.size)*yFrac) - h/2
	bounds := walk.Rectangle{X: 6, Y: y, Width: w.size - 12, Height: h}
	_ = c.DrawTextPixels(s, font, colWhite, bounds,
		walk.TextCenter|walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
}

// --- API pública (thread-safe via Synchronize) ---------------------------

// SetStatus atualiza a cor/texto do círculo a partir de uma mensagem de status.
func (w *FloatingWidget) SetStatus(text string) {
	w.mw.Synchronize(func() {
		w.status = text
		w.baseColor = statusColor(text)
		w.cw.Invalidate()
	})
}

// AddCount incrementa o contador de pedidos exibido.
func (w *FloatingWidget) AddCount(n int) {
	w.mw.Synchronize(func() {
		w.count += n
		w.cw.Invalidate()
	})
}

// StartFlash inicia o alarme visual (pisca) por no máximo maxSeconds.
func (w *FloatingWidget) StartFlash(maxSeconds int) {
	w.mw.Synchronize(func() {
		if w.flashing {
			return
		}
		w.flashing = true
		w.flashOn = true
		stop := make(chan struct{})
		w.flashStop = stop
		w.cw.Invalidate()
		go w.flashLoop(stop, maxSeconds)
	})
}

// StopFlash interrompe o alarme visual imediatamente.
func (w *FloatingWidget) StopFlash() {
	w.mw.Synchronize(func() {
		if w.flashStop != nil {
			close(w.flashStop)
			w.flashStop = nil
		}
	})
}

func (w *FloatingWidget) flashLoop(stop chan struct{}, maxSeconds int) {
	var deadline <-chan time.Time
	if maxSeconds > 0 {
		t := time.NewTimer(time.Duration(maxSeconds) * time.Second)
		defer t.Stop()
		deadline = t.C
	}
	ticker := time.NewTicker(230 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			w.endFlash()
			return
		case <-deadline:
			w.endFlash()
			return
		case <-ticker.C:
			w.mw.Synchronize(func() {
				w.flashOn = !w.flashOn
				w.cw.Invalidate()
			})
		}
	}
}

func (w *FloatingWidget) endFlash() {
	w.mw.Synchronize(func() {
		w.flashing = false
		w.flashOn = false
		w.flashStop = nil
		w.cw.Invalidate()
	})
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
