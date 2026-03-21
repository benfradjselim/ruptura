import streamlit as st
import pandas as pd
import matplotlib.pyplot as plt
import plotly.express as px
import yfinance as yf
import requests

# Load data
@st.cache
def load_data():
    data = pd.read_csv('data.csv')
    return data

# Load stock data
@st.cache
def load_stock_data(ticker):
    stock_data = yf.download(tickers=ticker, period='1d')
    return stock_data

# Load alert data
@st.cache
def load_alert_data():
    alert_data = pd.read_csv('alert.csv')
    return alert_data

# Main function
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
            requests.post('https://api.example.com/alert', json={'message': 'Alerte envoyée'})

            # Display success message
            st.success('Alerte envoyée avec succès')

if __name__ == '__main__':
    main()