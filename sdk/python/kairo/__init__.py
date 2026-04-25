"""Kairo Client — Official Python SDK for Kairo Core v6."""

from .client import KairoClient
from .exceptions import KairoError

__all__ = ["KairoClient", "KairoError"]
__version__ = "6.0.0"
