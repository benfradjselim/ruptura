import logging
import sys
from typing import NoReturn

# Configuration du logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

def get_started() -> NoReturn:
    """
    Fonction pour démarrer le projet.

    Retourne:
        None
    """
    try:
        # Installation des dépendances
        logging.info("Installation des dépendances...")
        install_dependencies()

        # Démarrage de l'application
        logging.info("Démarrage de l'application...")
        run_application()

        logging.info("Projet démarré avec succès !")
    except Exception as e:
        logging.error(f"Erreur lors du démarrage du projet : {e}")
        sys.exit(1)

def install_dependencies() -> None:
    """
    Fonction pour installer les dépendances.

    Retourne:
        None
    """
    try:
        # Code d'installation des dépendances
        logging.info("Dépendances installées avec succès !")
    except Exception as e:
        logging.error(f"Erreur lors de l'installation des dépendances : {e}")
        sys.exit(1)

def run_application() -> None:
    """
    Fonction pour démarrer l'application.

    Retourne:
        None
    """
    try:
        # Code de démarrage de l'application
        logging.info("Application démarrée avec succès !")
    except Exception as e:
        logging.error(f"Erreur lors du démarrage de l'application : {e}")
        sys.exit(1)

def main() -> NoReturn:
    """
    Fonction principale du projet.

    Retourne:
        None
    """
    try:
        get_started()
    except Exception as e:
        logging.error(f"Erreur lors de l'exécution du projet : {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()