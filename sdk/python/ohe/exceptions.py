class OHEError(Exception):
    """Raised when the OHE API returns a non-2xx response."""

    def __init__(self, status_code: int, code: str = "", message: str = ""):
        self.status_code = status_code
        self.code = code
        self.message = message
        detail = f"{code}: {message}" if code else f"HTTP {status_code}"
        super().__init__(f"ohe {status_code} {detail}")
