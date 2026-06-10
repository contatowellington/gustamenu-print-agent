//go:build windows

package main

import (
	"strconv"
	"syscall"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

// Funções Win32 não expostas pelo lxn/win — carregadas direto.
var (
	procCreateEllipticRgn = syscall.NewLazyDLL("gdi32.dll").NewProc("CreateEllipticRgn")
	procCombineRgn        = syscall.NewLazyDLL("gdi32.dll").NewProc("CombineRgn")
	procDeleteObject      = syscall.NewLazyDLL("gdi32.dll").NewProc("DeleteObject")
	procSetWindowRgn      = syscall.NewLazyDLL("user32.dll").NewProc("SetWindowRgn")
)

const rgnOr = 2 // RGN_OR — união de duas regiões

func createEllipticRgn(x1, y1, x2, y2 int32) uintptr {
	r, _, _ := procCreateEllipticRgn.Call(uintptr(x1), uintptr(y1), uintptr(x2), uintptr(y2))
	return r
}

func combineRgn(dest, src1, src2 uintptr, mode int32) {
	procCombineRgn.Call(dest, src1, src2, uintptr(mode))
}

func deleteObject(obj uintptr) {
	procDeleteObject.Call(obj)
}

func setWindowRgn(hwnd win.HWND, hrgn uintptr, redraw bool) {
	var b uintptr
	if redraw {
		b = 1
	}
	procSetWindowRgn.Call(uintptr(hwnd), hrgn, b)
}

// Paleta do widget.
// Círculo principal = estágio do pedido: laranja (repouso), amarelo (pedido
// criado), verde (pago). Badge menor = saúde do assistente: verde (conectado),
// vermelho (problema), cinza (aguardando configuração).
var (
	colBrand  = walk.RGB(0xFF, 0x7A, 0x00) // laranja GustaMenu — repouso
	colYellow = walk.RGB(0xF2, 0xA9, 0x00) // amarelo — pedido criado
	colOK     = walk.RGB(0x0A, 0x9D, 0x4E) // verde — pago / badge conectado
	colWait   = walk.RGB(0x8A, 0x8A, 0x8A) // cinza — badge aguardando configuração
	colErr    = walk.RGB(0xC6, 0x28, 0x28) // vermelho — badge com problema
	colAlarm  = walk.RGB(0xE5, 0x39, 0x35) // vermelho do alarme visual (flash)
	colWhite  = walk.RGB(0xFF, 0xFF, 0xFF)
)

// stageHoldMinutes: tempo que o círculo segura a cor do estágio antes de
// voltar sozinho ao repouso (laranja).
const stageHoldMinutes = 10

// FloatingWidget é o círculo sempre-no-topo, arrastável, com contador de
// pedidos e status. Pisca (alarme visual) quando chega pedido novo.
type FloatingWidget struct {
	mw   *walk.MainWindow
	cw   *walk.CustomWidget
	size int

	// estado de pintura — sempre alterado na thread de UI (via Synchronize).
	baseColor   walk.Color
	healthColor walk.Color
	status      string
	count       int
	stageTimer  *time.Timer

	// alarme visual (flash)
	flashing   bool
	flashOn    bool
	flashStop  chan struct{}
	flashBig   string
	flashSmall string

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

// NewFloatingWidget cria e exibe o círculo flutuante.
func NewFloatingWidget(onConfigure, onSilence, onTestAlarm, onQuit func()) (*FloatingWidget, error) {
	w := &FloatingWidget{
		size:        132,
		baseColor:   colBrand,
		healthColor: colWait,
		status:      "Iniciando…",
		flashBig:    "PEDIDO!",
		flashSmall:  "NOVO",
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

	// Região = círculo principal ∪ bolinha do badge (canto inferior direito,
	// grudada na borda). O SetWindowRgn assume a posse da região combinada;
	// só a região auxiliar do badge precisa ser liberada.
	s := int32(w.size)
	rgn := createEllipticRgn(0, 0, s, s)
	rgnBadge := createEllipticRgn(s-52, s-52, s, s)
	combineRgn(rgn, rgn, rgnBadge, rgnOr)
	deleteObject(rgnBadge)
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
		if button != walk.LeftButton || w.dragged {
			return
		}
		// Clique no ✕ esconde o círculo (app segue na bandeja).
		if pointInRect(x, y, w.closeButtonRect()) {
			w.Hide()
			return
		}
		// Clique no resto do círculo silencia o alarme.
		if w.onSilence != nil {
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
			fill = w.baseColor
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
		w.text(canvas, w.flashBig, w.fontCount, 0.42)
		w.text(canvas, w.flashSmall, w.fontStat, 0.64)
	} else {
		w.text(canvas, "✕", w.fontTitle, 0.13) // botão fechar (esconde o círculo)
		w.text(canvas, "GustaMenu", w.fontTitle, 0.27)
		w.text(canvas, strconv.Itoa(w.count), w.fontCount, 0.49)
		w.text(canvas, "pedidos", w.fontSmall, 0.70)
		w.text(canvas, w.status, w.fontStat, 0.85)
	}

	// Badge de saúde — bolinha menor grudada na borda inferior direita:
	// verde = conectado, vermelho = problema, cinza = aguardando configuração.
	badge := walk.Rectangle{X: s - 42, Y: s - 42, Width: 32, Height: 32}
	badgeBrush, err := walk.NewSolidColorBrush(w.healthColor)
	if err != nil {
		return err
	}
	defer badgeBrush.Dispose()
	if err := canvas.FillEllipsePixels(badgeBrush, badge); err != nil {
		return err
	}
	if pen, err := walk.NewGeometricPen(walk.PenSolid, 3, whiteBrush); err == nil {
		defer pen.Dispose()
		_ = canvas.DrawEllipsePixels(pen, badge)
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

// closeButtonRect é a área clicável do ✕, no topo do círculo.
func (w *FloatingWidget) closeButtonRect() walk.Rectangle {
	cy := int(float64(w.size) * 0.13)
	half := 14
	return walk.Rectangle{X: w.size/2 - half, Y: cy - half, Width: 2 * half, Height: 2 * half}
}

func pointInRect(x, y int, r walk.Rectangle) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// Hide esconde o círculo (o app continua rodando na bandeja).
func (w *FloatingWidget) Hide() {
	win.ShowWindow(w.mw.Handle(), win.SW_HIDE)
}

// Show mostra o círculo de volta, sempre-no-topo, sem roubar o foco.
func (w *FloatingWidget) Show() {
	w.mw.Synchronize(func() {
		hwnd := w.mw.Handle()
		win.ShowWindow(hwnd, win.SW_SHOWNOACTIVATE)
		win.SetWindowPos(hwnd, win.HWND_TOPMOST, 0, 0, 0, 0,
			win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE|win.SWP_SHOWWINDOW)
	})
}

// --- API pública (thread-safe via Synchronize) ---------------------------

// SetStatus atualiza o texto de status exibido no círculo. A cor do círculo
// é dirigida pelo estágio do pedido (SetStage) e a do badge pela saúde
// (SetHealth) — o texto não muda mais cor nenhuma.
func (w *FloatingWidget) SetStatus(text string) {
	w.mw.Synchronize(func() {
		w.status = text
		w.cw.Invalidate()
	})
}

// SetHealth atualiza a cor do badge de saúde: "ok" (verde), "erro"
// (vermelho) ou "config" (cinza).
func (w *FloatingWidget) SetHealth(state string) {
	w.mw.Synchronize(func() {
		switch state {
		case "ok":
			w.healthColor = colOK
		case "erro":
			w.healthColor = colErr
		default:
			w.healthColor = colWait
		}
		w.cw.Invalidate()
	})
}

// SetStage muda a cor do círculo principal conforme o estágio do pedido:
// "criado" (amarelo), "pago" (verde) ou vazio (repouso laranja). A cor de
// estágio volta sozinha ao repouso após stageHoldMinutes.
func (w *FloatingWidget) SetStage(stage string) {
	w.mw.Synchronize(func() {
		switch stage {
		case "criado":
			w.baseColor = colYellow
		case "pago":
			w.baseColor = colOK
		default:
			w.baseColor = colBrand
		}
		if w.stageTimer != nil {
			w.stageTimer.Stop()
			w.stageTimer = nil
		}
		if stage == "criado" || stage == "pago" {
			w.stageTimer = time.AfterFunc(stageHoldMinutes*time.Minute, func() {
				w.SetStage("")
			})
		}
		w.cw.Invalidate()
	})
}

// SetFlashLabels define os textos exibidos durante o alarme visual
// (ex.: "PEDIDO!"/"NOVO" no amarelo, "PAGO!"/"PEDIDO" no verde).
func (w *FloatingWidget) SetFlashLabels(big, small string) {
	w.mw.Synchronize(func() {
		w.flashBig = big
		w.flashSmall = small
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
