import streamlit as st
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns
import plotly.express as px

# Load data
@st.cache
def load_data():
    data = pd.read_csv('data.csv')
    return data

# Load alerts
@st.cache
def load_alerts():
    alerts = pd.read_csv('alerts.csv')
    return alerts

# Load graphiques
@st.cache
def load_graphiques():
    graphiques = pd.read_csv('graphiques.csv')
    return graphiques

# Main function
def main():
    # Set page title
    st.title('Dashboard')

    # Load data
    data = load_data()

    # Load alerts
    alerts = load_alerts()

    # Load graphiques
    graphiques = load_graphiques()

    # Create sidebar
    st.sidebar.title('Options')
    options = st.sidebar.selectbox('Select an option', ['Graphiques', 'Alertes', 'Data'])

    # Display graphiques
    if options == 'Graphiques':
        st.title('Graphiques')
        fig, ax = plt.subplots()
        sns.heatmap(graphiques, ax=ax)
        st.pyplot(fig)

    # Display alertes
    elif options == 'Alertes':
        st.title('Alertes')
        st.write(alerts)

    # Display data
    elif options == 'Data':
        st.title('Data')
        st.write(data)

if __name__ == '__main__':
    main()