"""Widget flutuante do assistente.

Um círculo sempre-no-topo, arrastável, que mostra status e contador de
pedidos. Pisca (alarme visual) quando chega pedido novo. Usa só tkinter.
"""

from __future__ import annotations

import tkinter as tk
from typing import Callable


class FloatingWidget:
    # paleta
    BRAND = "#FF7A00"      # laranja GustaMenu (ocioso/normal)
    OK = "#0A9D4E"         # verde — operando/imprimiu
    WAIT = "#8A8A8A"       # cinza — aguardando configuração
    ERR = "#C62828"        # vermelho — erro/sem autorização
    ALARM = "#E53935"      # vermelho do alarme visual
    TRANSPARENT = "magenta"  # cor tratada como transparente (Windows)

    def __init__(
        self,
        root: tk.Tk,
        *,
        on_configure: Callable[[], None],
        on_quit: Callable[[], None],
        on_test_alarm: Callable[[], None],
    ) -> None:
        self.root = root
        self.size = 132
        self.count = 0
        self._cur_color = self.BRAND
        self._cur_text = "Iniciando…"
        self._flashing = False

        win = tk.Toplevel(root)
        self.win = win
        win.overrideredirect(True)
        win.attributes("-topmost", True)
        try:
            win.attributes("-transparentcolor", self.TRANSPARENT)
        except tk.TclError:
            pass
        win.configure(bg=self.TRANSPARENT)

        sw = win.winfo_screenwidth()
        win.geometry(f"{self.size}x{self.size}+{sw - self.size - 30}+70")

        self.canvas = tk.Canvas(
            win, width=self.size, height=self.size,
            bg=self.TRANSPARENT, highlightthickness=0,
        )
        self.canvas.pack()
        self._draw(self._cur_color, self._cur_text)

        # arrastar
        for w in (win, self.canvas):
            w.bind("<Button-1>", self._start_move)
            w.bind("<B1-Motion>", self._on_move)
            w.bind("<Button-3>", self._popup)
            w.bind("<Double-Button-1>", lambda e: on_configure())

        # menu de contexto
        self.menu = tk.Menu(win, tearoff=0)
        self.menu.add_command(label="Configurar…", command=on_configure)
        self.menu.add_command(label="Testar alarme", command=on_test_alarm)
        self.menu.add_separator()
        self.menu.add_command(label="Sair", command=on_quit)

    # --- desenho ----------------------------------------------------------
    def _draw(self, color: str, status: str) -> None:
        c = self.canvas
        c.delete("all")
        s = self.size
        m = 8
        c.create_oval(m, m, s - m, s - m, fill=color, outline="white", width=3)
        c.create_text(s / 2, s * 0.27, text="GustaMenu",
                      fill="white", font=("Segoe UI", 9, "bold"))
        c.create_text(s / 2, s * 0.50, text=str(self.count),
                      fill="white", font=("Segoe UI", 26, "bold"))
        c.create_text(s / 2, s * 0.72, text="pedidos",
                      fill="white", font=("Segoe UI", 7))
        c.create_text(s / 2, s * 0.86, text=status, fill="white",
                      font=("Segoe UI", 8), width=s - 26)

    # --- API pública ------------------------------------------------------
    def set_status(self, color: str, text: str) -> None:
        self._cur_color = color
        self._cur_text = text
        if not self._flashing:
            self._draw(color, text)

    def add_count(self, n: int = 1) -> None:
        self.count += n
        if not self._flashing:
            self._draw(self._cur_color, self._cur_text)

    def flash(self, times: int = 8) -> None:
        """Alarme visual: pisca o círculo entre vermelho e a cor atual."""
        if self._flashing:
            return
        self._flashing = True
        self.win.attributes("-topmost", True)
        self._flash_step(times, True)

    def _flash_step(self, remaining: int, on: bool) -> None:
        if remaining <= 0:
            self._flashing = False
            self._draw(self._cur_color, self._cur_text)
            return
        if on:
            self.canvas.delete("all")
            s = self.size
            self.canvas.create_oval(8, 8, s - 8, s - 8, fill=self.ALARM,
                                    outline="white", width=4)
            self.canvas.create_text(s / 2, s * 0.42, text="PEDIDO!",
                                    fill="white", font=("Segoe UI", 13, "bold"))
            self.canvas.create_text(s / 2, s * 0.62, text="NOVO",
                                    fill="white", font=("Segoe UI", 11))
        else:
            self._draw(self.BRAND, self._cur_text)
        self.win.after(230, lambda: self._flash_step(remaining - 1, not on))

    # --- interações -------------------------------------------------------
    def _start_move(self, event: tk.Event) -> None:
        self._ox, self._oy = event.x, event.y

    def _on_move(self, event: tk.Event) -> None:
        x = self.win.winfo_x() + event.x - self._ox
        y = self.win.winfo_y() + event.y - self._oy
        self.win.geometry(f"+{x}+{y}")

    def _popup(self, event: tk.Event) -> None:
        try:
            self.menu.tk_popup(event.x_root, event.y_root)
        finally:
            self.menu.grab_release()
