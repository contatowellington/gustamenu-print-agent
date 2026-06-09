"""Comunicação com a API de fila de impressão do GustaMenu.

Usa apenas a biblioteca padrão (urllib) — sem dependências externas.

Contrato (compatível com o agente Go):
- GET  <endpoint>?device_token=<tok>&limit=10  -> {"ok": true, "jobs": [...]}
       401/403 -> assistente não autorizado
- POST <endpoint>  JSON {device_token, job_id, status, erro?} -> 200
"""

from __future__ import annotations

import json
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from typing import List

from .config import Config

TIMEOUT = 15  # segundos


@dataclass
class PrintJob:
    id: int
    codigo_pedido: str = ""
    paper_width: int = 80
    copies: int = 1
    receipt_text: str = ""
    attempts: int = 0
    created_at: str = ""

    @classmethod
    def from_dict(cls, d: dict) -> "PrintJob":
        return cls(
            id=int(d.get("id", 0)),
            codigo_pedido=str(d.get("codigo_pedido", "")),
            paper_width=int(d.get("paper_width", 80) or 80),
            copies=int(d.get("copies", 1) or 1),
            receipt_text=str(d.get("receipt_text", "")),
            attempts=int(d.get("attempts", 0) or 0),
            created_at=str(d.get("created_at", "")),
        )


class APIError(Exception):
    pass


class NotAuthorizedError(APIError):
    pass


def fetch_jobs(cfg: Config) -> List[PrintJob]:
    """Busca jobs pendentes na fila de impressão."""
    query = urllib.parse.urlencode(
        {"device_token": cfg.device_token, "limit": "10"}
    )
    url = cfg.api_endpoint + ("&" if "?" in cfg.api_endpoint else "?") + query
    req = urllib.request.Request(url, method="GET")

    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT) as resp:
            raw = resp.read()
    except urllib.error.HTTPError as exc:
        if exc.code in (401, 403):
            raise NotAuthorizedError(
                "assistente não autorizado — verifique o código do assistente"
            ) from exc
        raise APIError(f"API retornou status {exc.code}") from exc
    except urllib.error.URLError as exc:
        raise APIError(f"falha na conexão: {exc.reason}") from exc

    try:
        result = json.loads(raw.decode("utf-8"))
    except (ValueError, UnicodeDecodeError) as exc:
        raise APIError("resposta inválida da API") from exc

    if not result.get("ok", False):
        raise APIError("API retornou ok=false")

    return [PrintJob.from_dict(j) for j in result.get("jobs", [])]


def report_job(cfg: Config, job_id: int, status: str, err_msg: str = "") -> None:
    """Notifica a API sobre o resultado da impressão."""
    payload = {
        "device_token": cfg.device_token,
        "job_id": job_id,
        "status": status,
    }
    if err_msg:
        payload["erro"] = err_msg

    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        cfg.api_endpoint,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT) as resp:
            if resp.status != 200:
                raise APIError(f"report retornou status {resp.status}")
    except urllib.error.HTTPError as exc:
        raise APIError(f"report retornou status {exc.code}") from exc
    except urllib.error.URLError as exc:
        raise APIError(f"falha ao reportar: {exc.reason}") from exc
