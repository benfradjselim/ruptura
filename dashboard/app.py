import streamlit as st
import pandas as pd
import plotly.express as px
import logging
from typing import Optional

# Configuration du logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def load_data(file_path: str) -> pd.DataFrame:
    """
    Charge les données à partir du fichier CSV.

    Args:
        file_path (str): Chemin du fichier CSV.

    Returns:
        pd.DataFrame: Les données chargées.

    Raises:
        FileNotFoundError: Si le fichier n'existe pas.
        pd.errors.EmptyDataError: Si le fichier est vide.
    """
    try:
        return pd.read_csv(file_path)
    except FileNotFoundError as e:
        logger.error(f"Le fichier '{file_path}' n'existe pas.")
        raise e
    except pd.errors.EmptyDataError as e:
        logger.error(f"Le fichier '{file_path}' est vide.")
        raise e

def load_stock_data(ticker: str) -> Optional[pd.DataFrame]:
    """
    Charge les données de la bourse pour le ticker spécifié.

    Args:
        ticker (str): Le ticker de la bourse.

    Returns:
        pd.DataFrame: Les données de la bourse.

    Raises:
        yf.TickerDataUnavailable: Si les données ne sont pas disponibles.
    """
    try:
        return yf.download(tickers=ticker, period='1d')
    except yf.TickerDataUnavailable as e:
        logger.error(f"Les données pour le ticker '{ticker}' ne sont pas disponibles.")
        return None

def load_alert_data(file_path: str) -> pd.DataFrame:
    """
    Charge les données d'alerte à partir du fichier CSV.

    Args:
        file_path (str): Chemin du fichier CSV.

    Returns:
        pd.DataFrame: Les données d'alerte chargées.

    Raises:
        FileNotFoundError: Si le fichier n'existe pas.
        pd.errors.EmptyDataError: Si le fichier est vide.
    """
    try:
        return pd.read_csv(file_path)
    except FileNotFoundError as e:
        logger.error(f"Le fichier '{file_path}' n'existe pas.")
        raise e
    except pd.errors.EmptyDataError as e:
        logger.error(f"Le fichier '{file_path}' est vide.")
        raise e

def display_data(data: pd.DataFrame) -> None:
    """
    Affiche les données dans un tableau.

    Args:
        data (pd.DataFrame): Les données à afficher.
    """
    st.write(data)

def display_graph(data: pd.DataFrame) -> None:
    """
    Affiche un graphique à partir des données.

    Args:
        data (pd.DataFrame): Les données à afficher.
    """
    fig = px.line(data, x='date', y='value')
    st.plotly_chart(fig)

def main() -> None:
    """
    Fonction principale du script.
    """
    st.title("Dashboard")

    # Chargement des données
    file_path = "data.csv"
    data = load_data(file_path)

    # Affichage des données
    display_data(data)

    # Affichage du graphique
    display_graph(data)

if __name__ == "__main__":
    main()