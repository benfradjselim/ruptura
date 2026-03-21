import logging
import os
import time
from kubernetes import client, config
from typing import Dict

# Set up logging
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

# Create a file handler which logs even debug messages
handler = logging.FileHandler('kubernetes_logs_collector.log')
handler.setLevel(logging.DEBUG)

# Create a console handler with a higher log level
console_handler = logging.StreamHandler()
console_handler.setLevel(logging.INFO)

# Create a logging format
formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
handler.setFormatter(formatter)
console_handler.setFormatter(formatter)

# Add the handlers to the logger
logger.addHandler(handler)
logger.addHandler(console_handler)

def collect_logs(namespace: str, pod_name: str, container_name: str) -> str:
    """
    Collect logs from a Kubernetes pod.

    Args:
        namespace (str): The namespace of the pod.
        pod_name (str): The name of the pod.
        container_name (str): The name of the container.

    Returns:
        str: The collected logs.

    Raises:
        ValueError: If namespace, pod_name or container_name is None or empty.
        client.ApiException: If there is an error collecting logs.
    """
    if not namespace or not pod_name or not container_name:
        raise ValueError("Namespace, pod_name and container_name must not be empty")

    try:
        # Get the pod logs
        v1 = client.CoreV1Api()
        response = v1.read_namespaced_pod_log(
            name=pod_name,
            namespace=namespace,
            container=container_name,
            follow=False
        )
        return response
    except client.ApiException as e:
        logger.error(f"Error collecting logs: {e}")
        raise
    except Exception as e:
        logger.error(f"An error occurred: {e}")
        raise

def collect_all_logs(namespace: str) -> Dict[str, str]:
    """
    Collect logs from all pods in a Kubernetes namespace.

    Args:
        namespace (str): The namespace of the pods.

    Returns:
        Dict[str, str]: A dictionary of pod names and their logs.

    Raises:
        ValueError: If namespace is None or empty.
        client.ApiException: If there is an error collecting logs.
    """
    if not namespace:
        raise ValueError("Namespace must not be empty")

    try:
        # Get the list of pods
        v1 = client.CoreV1Api()
        pods = v1.list_namespaced_pod(namespace=namespace)

        # Collect logs from each pod
        logs = {}
        for pod in pods.items:
            pod_name = pod.metadata.name
            container_name = pod.spec.containers[0].name
            logs[pod_name] = collect_logs(namespace, pod_name, container_name)

        return logs
    except client.ApiException as e:
        logger.error(f"Error collecting logs: {e}")
        raise
    except Exception as e:
        logger.error(f"An error occurred: {e}")
        raise