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
    """
    try:
        # Get the pod logs
        response = v1.read_namespaced_pod_log(
            name=pod_name,
            namespace=namespace,
            container=container_name,
            follow=False
        )
        return response
    except client.ApiException as e:
        logger.error(f"Error collecting logs: {e}")
        return None

def collect_all_logs(namespace: str) -> Dict[str, str]:
    """
    Collect logs from all pods in a namespace.

    Args:
        namespace (str): The namespace of the pods.

    Returns:
        Dict[str, str]: A dictionary with pod names as keys and logs as values.
    """
    logs = {}
    try:
        pods = v1.list_namespaced_pod(namespace=namespace)
        for pod in pods.items:
            for container in pod.spec.containers:
                logs[f"{pod.metadata.name}-{container.name}"] = collect_logs(namespace, pod.metadata.name, container.name)
    except client.ApiException as e:
        logger.error(f"Error collecting logs: {e}")
    return logs

def main():
    namespace = os.environ.get("NAMESPACE", "default")
    try:
        logs = collect_all_logs(namespace)
        for pod, log in logs.items():
            if log:
                logger.info(f"Logs from {pod}: {log}")
            else:
                logger.warning(f"No logs found for {pod}")
    except Exception as e:
        logger.error(f"Error in main: {e}")

if __name__ == "__main__":
    config.load_kube_config()
    v1 = client.CoreV1Api()
    main()