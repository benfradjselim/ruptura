import psutil
import prometheus_client
import logging
import time
import requests
from typing import Optional

# Set up logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

# Define metrics
cpu_usage = prometheus_client.Gauge('cpu_usage', 'CPU usage')
memory_usage = prometheus_client.Gauge('memory_usage', 'Memory usage')
latency = prometheus_client.Gauge('latency', 'Latency')

def collect_metrics() -> None:
    """
    Collect CPU, memory, and latency metrics.

    Raises:
        Exception: If an error occurs while collecting metrics.
    """
    try:
        # Collect CPU usage
        cpu_usage.set(psutil.cpu_percent())

        # Collect memory usage
        memory_usage.set(psutil.virtual_memory().percent)

        # Collect latency (for example, using a simple HTTP request)
        start_time = time.time()
        response = requests.get('http://example.com', timeout=5)
        response.raise_for_status()  # Raise an exception for HTTP errors
        latency.set((time.time() - start_time) * 1000)
    except requests.RequestException as e:
        logging.error(f"Error collecting metrics: {e}")
        raise
    except Exception as e:
        logging.error(f"Unexpected error collecting metrics: {e}")
        raise

def start_metrics_server(port: int = 8000) -> None:
    """
    Start the Prometheus metrics server.

    Args:
        port (int): The port to listen on. Defaults to 8000.

    Raises:
        Exception: If an error occurs while starting the metrics server.
    """
    try:
        # Start the Prometheus metrics server
        prometheus_client.start_http_server(port)
        logging.info(f"Metrics server started on port {port}")
    except Exception as e:
        logging.error(f"Error starting metrics server: {e}")
        raise

def main() -> None:
    """
    Main function.

    Raises:
        Exception: If an error occurs.
    """
    try:
        # Collect metrics
        collect_metrics()

        # Start the metrics server
        start_metrics_server()
    except Exception as e:
        logging.error(f"Error in main function: {e}")
        raise