import logging
import os
from datetime import datetime

def setup_logging(service_name):
    """Configure le logging pour un service"""
    logging.basicConfig(
        level=logging.INFO,
        format=f'%(asctime)s - {service_name} - %(levelname)s - %(message)s'
    )
    return logging.getLogger(service_name)

def get_db_path():
    """Retourne le chemin de la base de données"""
    return os.environ.get('DB_PATH', '/data/mlops.db')

def now():
    """Retourne le timestamp actuel"""
    return datetime.now().isoformat()
