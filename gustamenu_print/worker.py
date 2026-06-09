"""Loop de polling e impressão automática em background.

Equivalente ao PrintWorker do agente Go. Roda em uma thread, busca jobs
no intervalo configurado, imprime e reporta o resultado. Ao detectar
pedido novo, dispara o alarme sonoro.
"""

from __future__ import annotations

import logging
import threading
from typing import Callable, Optional

from .api import APIError, PrintJob, fetch_jobs, report_job
from .config import Config
from .printer import print_job

log = logging.getLogger(__name__)

StatusCallback = Callable[[str], None]
NewOrderCallback = Callable[[int], None]


class PrintWorker:
    def __init__(
        self,
        cfg: Config,
        on_status: Optional[StatusCallback] = None,
        on_new_order: Optional[NewOrderCallback] = None,
    ) -> None:
        self._cfg = cfg
        self._on_status = on_status
        self._on_new_order = on_new_order
        self._lock = threading.Lock()
        self._stop = threading.Event()
        self._wake = threading.Event()
        self._thread: Optional[threading.Thread] = None

    # --- controle ---------------------------------------------------------
    def start(self) -> None:
        if self._thread and self._thread.is_alive():
            return
        self._stop.clear()
        self._thread = threading.Thread(target=self._run, daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._stop.set()
        self._wake.set()

    def update_config(self, cfg: Config) -> None:
        with self._lock:
            self._cfg = cfg
        self._wake.set()  # acorda o loop para aplicar a nova config já

    def _config(self) -> Config:
        with self._lock:
            return self._cfg

    # --- loop -------------------------------------------------------------
    def _run(self) -> None:
        self._publish("Assistente iniciado.")
        while not self._stop.is_set():
            cfg = self._config()

            if not cfg.is_valid():
                self._publish("Aguardando configuração.")
                self._sleep(10)
                continue

            self._do_poll(cfg)
            self._sleep(cfg.normalized_poll_seconds())

    def _sleep(self, seconds: float) -> None:
        # Espera interrompível: stop ou update_config acordam na hora.
        self._wake.wait(timeout=seconds)
        self._wake.clear()

    def _do_poll(self, cfg: Config) -> None:
        try:
            jobs = fetch_jobs(cfg)
        except APIError as exc:
            self._publish(f"Falha na consulta: {exc}")
            log.warning("poll: %s", exc)
            return

        if not jobs:
            return

        self._publish(f"{len(jobs)} cupom(ns) recebido(s).")
        if self._on_new_order:
            try:
                self._on_new_order(len(jobs))
            except Exception:
                pass

        for job in jobs:
            if self._stop.is_set():
                return
            self._do_job(cfg, job)

    def _do_job(self, cfg: Config, job: PrintJob) -> None:
        copies = max(1, min(job.copies, 5))
        self._publish(f"Imprimindo pedido {job.codigo_pedido}.")

        print_error: Optional[str] = None
        for i in range(copies):
            try:
                print_job(cfg, job)
            except Exception as exc:  # erro de impressora não pode derrubar o loop
                print_error = str(exc)
                log.error("job %d cópia %d: %s", job.id, i + 1, exc)
                break

        if print_error is not None:
            self._publish(f"Falha ao imprimir {job.codigo_pedido}.")
            self._safe_report(cfg, job.id, "falhou", print_error)
            return

        self._publish(f"Pedido {job.codigo_pedido} impresso.")
        self._safe_report(cfg, job.id, "impresso", "")

    def _safe_report(self, cfg: Config, job_id: int, status: str, err: str) -> None:
        try:
            report_job(cfg, job_id, status, err)
        except APIError as exc:
            log.warning("job %d report falhou: %s", job_id, exc)

    def _publish(self, status: str) -> None:
        log.info("status: %s", status)
        if self._on_status:
            try:
                self._on_status(status)
            except Exception:
                pass
