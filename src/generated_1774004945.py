import logging
import os
from logging.handlers import RotatingFileHandler
from datetime import datetime

# Configure logging
LOGGING_CONFIG = {
    "version": 1,
    "formatters": {
        "default": {
            "format": "[%(asctime)s] %(name)s - %(levelname)s - %(message)s"
        }
    },
    "handlers": {
        "file": {
            "class": "logging.handlers.RotatingFileHandler",
            "level": "INFO",
            "formatter": "default",
            "filename": "logs/app.log",
            "maxBytes": 5 * 1024 * 1024,  # 5MB
            "backupCount": 5,
            "encoding": "utf-8"
        },
        "console": {
            "class": "logging.StreamHandler",
            "level": "DEBUG",
            "formatter": "default"
        }
    },
    "root": {
        "level": "INFO",
        "handlers": ["file", "console"]
    }
}

# Create logs directory if it doesn't exist
os.makedirs("logs", exist_ok=True)

# Configure logging with the above configuration
logging.config.dictConfig(LOGGING_CONFIG)

# Create a logger instance
logger = logging.getLogger(__name__)

# Example usage
if __name__ == "__main__":
    logger.debug("This is a debug message")
    logger.info("This is an info message")
    logger.warning("This is a warning message")
    logger.error("This is an error message")
    logger.critical("This is a critical message")