"""Alarme sonoro ao chegar pedido novo.

Recurso exclusivo da versão Python. Usa winsound (stdlib, Windows).
Toca em uma thread separada para não travar o loop de polling.
"""

from __future__ import annotations

import threading

try:
    import winsound  # stdlib, só existe no Windows
except ImportError:  # pragma: no cover - ambiente não-Windows
    winsound = None

# Padrão de bipes do alarme (frequência Hz, duração ms).
_BEEP_PATTERN = [(880, 180), (1175, 180), (1568, 260)]

_lock = threading.Lock()


def _play(repeat: int) -> None:
    if winsound is None:
        return
    with _lock:  # evita sobreposição de alarmes simultâneos
        for _ in range(max(1, repeat)):
            for freq, dur in _BEEP_PATTERN:
                try:
                    winsound.Beep(freq, dur)
                except RuntimeError:
                    return


def play_new_order_alarm(repeat: int = 3) -> None:
    """Dispara o alarme em background (não bloqueia)."""
    threading.Thread(target=_play, args=(repeat,), daemon=True).start()
