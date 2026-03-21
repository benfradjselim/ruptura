import streamlit as st
import pandas as pd
import matplotlib.pyplot as plt
import plotly.express as px
import yfinance as yf
import requests
import logging

# Configuration du logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def load_data() -> pd.DataFrame:
    """
    Charge les données à partir du fichier CSV.

    Returns:
        pd.DataFrame: Les données chargées.
    """
    try:
        data = pd.read_csv('data.csv')
        return data
    except FileNotFoundError:
        logger.error("Le fichier 'data.csv' n'existe pas.")
        raise
    except pd.errors.EmptyDataError:
        logger.error("Le fichier 'data.csv' est vide.")
        raise

def load_stock_data(ticker: str) -> pd.DataFrame:
    """
    Charge les données de la bourse pour le ticker spécifié.

    Args:
        ticker (str): Le ticker de la bourse.

    Returns:
        pd.DataFrame: Les données de la bourse.
    """
    try:
        stock_data = yf.download(tickers=ticker, period='1d')
        return stock_data
    except yf.TickerDataUnavailable:
        logger.error(f"Les données pour le ticker '{ticker}' ne sont pas disponibles.")
        raise

def load_alert_data() -> pd.DataFrame:
    """
    Charge les données d'alerte à partir du fichier CSV.

    Returns:
        pd.DataFrame: Les données d'alerte chargées.
    """
    try:
        alert_data = pd.read_csv('alert.csv')
        return alert_data
    except FileNotFoundError:
        logger.error("Le fichier 'alert.csv' n'existe pas.")
        raise
    except pd.errors.EmptyDataError:
        logger.error("Le fichier 'alert.csv' est vide.")
        raise

def send_alert() -> None:
    """
    Envoie une alerte via l'API.
    """
    try:
        requests.post('https://api.example.com/alert', json={'message': 'Alerte envoyée'})
        logger.info("L'alerte a été envoyée avec succès.")
    except requests.exceptions.RequestException as e:
        logger.error(f"Erreur lors de l'envoi de l'alerte : {e}")

def main():
    # Set page title
    st.title('Dashboard')

    # Create sidebar
    st.sidebar.title('Menu')
    menu = ['Graphiques', 'Alertes']
    choice = st.sidebar.selectbox('Choisir une option', menu)

    # Graphiques
    if choice == 'Graphiques':
        # Load data
        data = load_data()

        # Create line chart
        fig, ax = plt.subplots()
        ax.plot(data['Date'], data['Value'])
        st.pyplot(fig)

        # Create bar chart
        fig, ax = plt.subplots()
        ax.bar(data['Date'], data['Value'])
        st.pyplot(fig)

    # Alertes
    elif choice == 'Alertes':
        # Load alert data
        alert_data = load_alert_data()

        # Create table
        st.write(alert_data)

        # Create alert message
        if st.button('Envoyer une alerte'):
            # Send alert via API
            send_alert()

            # Display success message
            st.success('Alerte envoyée avec succès.')

if __name__ == '__main__':
    main()