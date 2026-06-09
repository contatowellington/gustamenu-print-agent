"""Tela de configuração (tkinter Toplevel)."""

from __future__ import annotations

import tkinter as tk
from tkinter import messagebox, ttk
from typing import Callable, Optional

from .config import Config, save_config
from .printer import default_printer_name, installed_printers

OnSave = Callable[[Config], None]

_window: Optional[tk.Toplevel] = None


def open_settings(parent: tk.Misc, cfg: Config, on_save: OnSave) -> None:
    """Abre a janela de configuração (reaproveita se já estiver aberta)."""
    global _window
    if _window is not None:
        try:
            _window.deiconify()
            _window.lift()
            _window.focus_force()
            return
        except tk.TclError:
            _window = None

    win = tk.Toplevel(parent)
    _window = win
    win.title("GustaMenu — Configurações")
    win.resizable(False, False)
    win.transient(parent)
    try:
        win.attributes("-topmost", True)
    except tk.TclError:
        pass

    pad = {"padx": 10, "pady": 6}
    frm = ttk.Frame(win, padding=16)
    frm.grid(row=0, column=0, sticky="nsew")

    row = 0
    ttk.Label(frm, text="Código do assistente").grid(row=row, column=0, sticky="w", **pad)
    token_var = tk.StringVar(value=cfg.device_token)
    ttk.Entry(frm, textvariable=token_var, width=44).grid(row=row, column=1, **pad)

    row += 1
    ttk.Label(frm, text="Endpoint da API").grid(row=row, column=0, sticky="w", **pad)
    endpoint_var = tk.StringVar(value=cfg.api_endpoint)
    ttk.Entry(frm, textvariable=endpoint_var, width=44).grid(row=row, column=1, **pad)

    row += 1
    ttk.Label(frm, text="Impressora").grid(row=row, column=0, sticky="w", **pad)
    printers = installed_printers()
    current = cfg.printer_name or default_printer_name()
    if current and current not in printers:
        printers.insert(0, current)
    printer_var = tk.StringVar(value=current)
    ttk.Combobox(
        frm, textvariable=printer_var, values=printers, width=42, state="readonly"
    ).grid(row=row, column=1, **pad)

    row += 1
    ttk.Label(frm, text="Intervalo (segundos)").grid(row=row, column=0, sticky="w", **pad)
    poll_var = tk.IntVar(value=cfg.poll_seconds)
    ttk.Spinbox(frm, from_=3, to=60, textvariable=poll_var, width=8).grid(
        row=row, column=1, sticky="w", **pad
    )

    row += 1
    start_var = tk.BooleanVar(value=cfg.start_with_windows)
    ttk.Checkbutton(
        frm, text="Iniciar com o Windows", variable=start_var
    ).grid(row=row, column=1, sticky="w", **pad)

    row += 1
    alarm_var = tk.BooleanVar(value=cfg.alarm_enabled)
    ttk.Checkbutton(
        frm, text="Alarme sonoro ao chegar pedido", variable=alarm_var
    ).grid(row=row, column=1, sticky="w", **pad)

    row += 1
    ttk.Label(frm, text="Repetições do alarme").grid(row=row, column=0, sticky="w", **pad)
    repeat_var = tk.IntVar(value=cfg.alarm_repeat)
    ttk.Spinbox(frm, from_=1, to=10, textvariable=repeat_var, width=8).grid(
        row=row, column=1, sticky="w", **pad
    )

    row += 1
    btns = ttk.Frame(frm)
    btns.grid(row=row, column=0, columnspan=2, pady=(12, 0))

    def do_save() -> None:
        try:
            poll = int(poll_var.get())
            repeat = int(repeat_var.get())
        except (tk.TclError, ValueError):
            messagebox.showerror("GustaMenu", "Valores numéricos inválidos.")
            return

        new_cfg = Config(
            api_endpoint=endpoint_var.get().strip(),
            device_token=token_var.get().strip(),
            printer_name=printer_var.get().strip(),
            poll_seconds=poll,
            start_with_windows=bool(start_var.get()),
            alarm_enabled=bool(alarm_var.get()),
            alarm_repeat=repeat,
        )
        if not new_cfg.device_token:
            messagebox.showwarning("GustaMenu", "Informe o código do assistente.")
            return

        save_config(new_cfg)
        on_save(new_cfg)
        messagebox.showinfo("GustaMenu", "Configurações salvas.")
        close()

    def close() -> None:
        global _window
        _window = None
        win.destroy()

    ttk.Button(btns, text="Salvar", command=do_save).grid(row=0, column=0, padx=6)
    ttk.Button(btns, text="Cancelar", command=close).grid(row=0, column=1, padx=6)

    win.protocol("WM_DELETE_WINDOW", close)
