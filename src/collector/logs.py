import logging
import os

def get_error_logs(log_path, log_level='ERROR'):
    """
    Collect error logs from a specified log path.

    Args:
        log_path (str): Path to the log file.
        log_level (str, optional): Log level to filter by. Defaults to 'ERROR'.

    Returns:
        list: List of error logs.
    """
    error_logs = []
    if os.path.exists(log_path):
        with open(log_path, 'r') as f:
            for line in f:
                if log_level in line:
                    error_logs.append(line.strip())
    return error_logs

# Example usage:
log_path = 'path_to_your_log_file.log'
error_logs = get_error_logs(log_path)
for log in error_logs:
    print(log)