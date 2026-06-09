"""Entry point do GustaMenu Assistente de Impressão.

Mostra um widget flutuante (círculo sempre-no-topo) com status e contador,
roda o loop de impressão automática em background e dispara alarme sonoro
+ visual quando chega pedido novo. Apenas biblioteca padrão.
"""

from __future__ import annotations

import logging
import os
import queue
import sys
import tkinter as tk

from . import __app_name__, __version__, alarm
from .config import config_dir, load_config
from .settings_window import open_settings
from .startup import acquire_single_instance, set_autostart
from .widget import FloatingWidget
from .worker import PrintWorker


def _setup_log() -> None:
    log_dir = config_dir()
    try:
        os.makedirs(log_dir, exist_ok=True)
        handler: logging.Handler = logging.FileHandler(
            os.path.join(log_dir, "print-agent.log"), encoding="utf-8"
        )
    except OSError:
        handler = logging.StreamHandler()
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
        handlers=[handler],
    )


def _status_color(text: str) -> str:
    t = text.lower()
    if "falha" in t or "não autorizado" in t or "nao autorizado" in t or "erro" in t:
        return FloatingWidget.ERR
    if "aguardando configura" in t:
        return FloatingWidget.WAIT
    if "impresso" in t or "imprimindo" in t or "recebido" in t or "iniciado" in t:
        return FloatingWidget.OK
    return FloatingWidget.BRAND


class App:
    def __init__(self) -> None:
        self.cfg = load_config()
        set_autostart(self.cfg.start_with_windows)
        # fila de eventos thread-safe: ("status", texto) | ("order", n)
        self.events: "queue.Queue[tuple]" = queue.Queue()
        self.worker = PrintWorker(
            self.cfg,
            on_status=lambda s: self.events.put(("status", s)),
            on_new_order=lambda n: self.events.put(("order", n)),
        )

    # --- ações ------------------------------------------------------------
    def _configure(self) -> None:
        open_settings(self.root, self.cfg, self._apply_config)

    def _apply_config(self, cfg) -> None:
        self.cfg = cfg
        set_autostart(cfg.start_with_windows)
        self.worker.update_config(cfg)

    def _test_alarm(self) -> None:
        if self.cfg.alarm_enabled:
            alarm.play_new_order_alarm(self.cfg.normalized_alarm_repeat())
        self.widget.flash()

    def _quit(self) -> None:
        self.worker.stop()
        self.root.destroy()

    # --- loop de eventos --------------------------------------------------
    def _drain_events(self) -> None:
        try:
            while True:
                kind, payload = self.events.get_nowait()
                if kind == "status":
                    self.widget.set_status(_status_color(payload), payload)
                elif kind == "order":
                    self.widget.add_count(int(payload))
                    if self.cfg.alarm_enabled:
                        alarm.play_new_order_alarm(self.cfg.normalized_alarm_repeat())
                    self.widget.flash()
        except queue.Empty:
            pass
        self.root.after(150, self._drain_events)

    # --- run --------------------------------------------------------------
    def run(self) -> None:
        self.root = tk.Tk()
        self.root.withdraw()  # janela-mãe oculta; só o widget aparece
        self.widget = FloatingWidget(
            self.root,
            on_configure=self._configure,
            on_quit=self._quit,
            on_test_alarm=self._test_alarm,
        )
        self.worker.start()
        self.root.after(150, self._drain_events)
        self.root.mainloop()


def main() -> int:
    if not acquire_single_instance():
        try:
            import ctypes

            ctypes.windll.user32.MessageBoxW(
                0,
                "O Assistente de Impressão GustaMenu já está em execução.",
                "GustaMenu",
                0x40,
            )
        except Exception:
            print("Assistente já está em execução.")
        return 0

    _setup_log()
    logging.getLogger(__name__).info("iniciando %s v%s", __app_name__, __version__)
    App().run()
    return 0


if __name__ == "__main__":
    sys.exit(main())
