import logging
import os
import subprocess
from typing import NoReturn

# Configuration du logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

def build_image() -> None:
    """
    Construit l'image Docker en deux étapes : build et final.

    :return: None
    """
    try:
        # Stage 1: Build
        logging.info("Construisons la première étape de l'image Docker...")
        subprocess.run([
            "docker", "build", "-t", "my-image", "--build-arg", "BUILD_STAGE=1", "."
        ], check=True)

        # Stage 2: Final
        logging.info("Construisons la deuxième étape de l'image Docker...")
        subprocess.run([
            "docker", "build", "-t", "my-image", "--build-arg", "BUILD_STAGE=2", "."
        ], check=True)

        logging.info("L'image Docker a été construite avec succès !")

    except subprocess.CalledProcessError as e:
        logging.error(f"Erreur lors de la construction de l'image Docker : {e}")
        raise

    except Exception as e:
        logging.error(f"Erreur inconnue : {e}")
        raise

def main() -> NoReturn:
    """
    Fonction principale.

    :return: None
    """
    try:
        build_image()
    except Exception as e:
        logging.error(f"Erreur lors de l'exécution de la fonction principale : {e}")
        raise

if __name__ == "__main__":
    main()