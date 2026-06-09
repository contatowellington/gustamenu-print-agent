"""Configuração local do assistente.

Lê/grava settings.json em %APPDATA%\\GustaMenu\\PrintAgent\\settings.json.
"""

from __future__ import annotations

import json
import os
from dataclasses import asdict, dataclass

# Endpoint padrão da fila de impressão. É configurável pela tela de
# Configurações; este é apenas o valor inicial sugerido.
DEFAULT_API_ENDPOINT = "https://gustamenu.com.br/api/print_jobs.php"

APP_DIR_NAME = "GustaMenu"
SUB_DIR_NAME = "PrintAgent"


@dataclass
class Config:
    api_endpoint: str = DEFAULT_API_ENDPOINT
    device_token: str = ""
    printer_name: str = ""
    poll_seconds: int = 5
    start_with_windows: bool = True
    # Alarme sonoro ao chegar pedido novo (recurso exclusivo da versão Python).
    alarm_enabled: bool = True
    alarm_repeat: int = 3

    def is_valid(self) -> bool:
        return bool(self.device_token) and bool(self.api_endpoint)

    def normalized_poll_seconds(self) -> int:
        if self.poll_seconds < 3:
            return 5
        if self.poll_seconds > 60:
            return 60
        return self.poll_seconds

    def normalized_alarm_repeat(self) -> int:
        if self.alarm_repeat < 1:
            return 1
        if self.alarm_repeat > 10:
            return 10
        return self.alarm_repeat


def config_dir() -> str:
    base = os.environ.get("APPDATA") or os.path.expanduser("~")
    return os.path.join(base, APP_DIR_NAME, SUB_DIR_NAME)


def config_path() -> str:
    return os.path.join(config_dir(), "settings.json")


def load_config() -> Config:
    """Lê a config do disco. Se não existir, devolve os padrões."""
    cfg = Config()
    path = config_path()
    if not os.path.exists(path):
        return cfg
    try:
        with open(path, "r", encoding="utf-8") as fh:
            data = json.load(fh)
    except (OSError, ValueError):
        return Config()

    if isinstance(data, dict):
        for field in Config().__dict__:
            if field in data and data[field] is not None:
                setattr(cfg, field, data[field])

    if not cfg.api_endpoint:
        cfg.api_endpoint = DEFAULT_API_ENDPOINT
    return cfg


def save_config(cfg: Config) -> None:
    os.makedirs(config_dir(), exist_ok=True)
    tmp = config_path() + ".tmp"
    with open(tmp, "w", encoding="utf-8") as fh:
        json.dump(asdict(cfg), fh, ensure_ascii=False, indent=2)
    os.replace(tmp, config_path())
