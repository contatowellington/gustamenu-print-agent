"""Single-instance (mutex via ctypes) e autostart no Windows (winreg).

Sem dependências externas — apenas a biblioteca padrão.
"""

from __future__ import annotations

import ctypes
import logging
import os
import sys

log = logging.getLogger(__name__)

MUTEX_NAME = "Global\\GustaMenuPrintAgent_SingleInstance"
RUN_KEY = r"Software\Microsoft\Windows\CurrentVersion\Run"
RUN_VALUE = "GustaMenuPrintAgent"

_ERROR_ALREADY_EXISTS = 183

# Mantém o handle do mutex vivo durante toda a execução do processo.
_mutex_handle = None


def acquire_single_instance() -> bool:
    """Retorna True se esta é a única instância; False se já há outra."""
    global _mutex_handle
    try:
        kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)
    except OSError:
        return True

    _mutex_handle = kernel32.CreateMutexW(None, False, MUTEX_NAME)
    if ctypes.get_last_error() == _ERROR_ALREADY_EXISTS:
        return False
    return True


def _exe_command() -> str:
    """Comando usado no autostart (executável empacotado ou script)."""
    if getattr(sys, "frozen", False):
        return f'"{sys.executable}"'
    script = os.path.abspath(sys.argv[0])
    return f'"{sys.executable}" "{script}"'


def set_autostart(enabled: bool) -> None:
    try:
        import winreg
    except ImportError:
        return
    try:
        key = winreg.OpenKey(
            winreg.HKEY_CURRENT_USER, RUN_KEY, 0, winreg.KEY_SET_VALUE
        )
    except OSError:
        return
    try:
        if enabled:
            winreg.SetValueEx(key, RUN_VALUE, 0, winreg.REG_SZ, _exe_command())
        else:
            try:
                winreg.DeleteValue(key, RUN_VALUE)
            except FileNotFoundError:
                pass
    except OSError as exc:
        log.warning("autostart: %s", exc)
    finally:
        winreg.CloseKey(key)
