"""Kairo Client — Official Python SDK for Ruptura v6."""

from .client import RupturaClient
from .exceptions import RupturaError

__all__ = ["RupturaClient", "RupturaError"]
__version__ = "6.0.0"
