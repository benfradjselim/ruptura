class RupturaError(Exception):
    """Raised when the Ruptura API returns a non-2xx response."""

    def __init__(self, status_code: int, message: str = ""):
        self.status_code = status_code
        self.message = message
        super().__init__(f"kairo {status_code}: {message}")
