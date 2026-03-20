import dash
from dash import html
from dash import dcc
from dash import dash_table
import plotly.express as px
import pandas as pd

# Créer un dataframe exemple
df = pd.DataFrame({
    "Mois": ["Janvier", "Février", "Mars", "Avril", "Mai", "Juin"],
    "Ventes": [2500, 3000, 2800, 3200, 2900, 3100],
    "Depenses": [2000, 2200, 2100, 2300, 2400, 2250],
    "Profit": [500, 800, 700, 900, 500, 850]
})

# Calculer les totaux
total_ventes = df["Ventes"].sum()
total_depenses = df["Depenses"].sum()
total_profit = df["Profit"].sum()

# Créer les figures
fig1 = px.line(df, x="Mois", y="Ventes", title="Ventes mensuelles")
fig2 = px.bar(df, x="Mois", y=["Ventes", "Depenses"], title="Comparaison Ventes/Depenses")
fig3 = px.pie(df, names="Mois", values="Profit", title="Répartition du profit")

# Créer le dashboard
app = dash.Dash(__name__)

app.layout = html.Div(children=[
    html.H1(children='Dashboard de ventes'),
    
    html.Div([
        html.Div([
            html.H3(children='Total Ventes'),
            html.P(children=f'{total_ventes} €', style={'color': 'green'})
        ], style={'border': '1px solid #ccc', 'padding': '10px', 'margin': '5px'}),
        
        html.Div([
            html.H3(children='Total Dépenses'),
            html.P(children=f'{total_depenses} €', style={'color': 'red'})
        ], style={'border': '1px solid #ccc', 'padding': '10px', 'margin': '5px'}),
        
        html.Div([
            html.H3(children='Total Profit'),
            html.P(children=f'{total_profit} €', style={'color': 'blue'})
        ], style={'border': '1px solid #ccc', 'padding': '10px', 'margin': '5px'})
    ], style={'display': 'flex', 'flex-wrap': 'wrap'}),
    
    html.Div([
        html.Div([
            dcc.Graph(figure=fig1)
        ], style={'width': '49%', 'margin': '5px'}),
        
        html.Div([
            dcc.Graph(figure=fig2)
        ], style={'width': '49%', 'margin': '5px'})
    ], style={'display': 'flex', 'flex-wrap': 'wrap'}),
    
    html.Div([
        dcc.Graph(figure=fig3)
    ], style={'margin': '5px'}),
    
    html.Div([
        dash_table.DataTable(
            id='table',
            columns=[{"name": i, "id": i} for i in df.columns],
            data=df.to_dict('records'),
            filter_action="native",
            sort_action="native",
            page_action="native",
            page_current=0,
            page_size=10,
        )
    ], style={'margin': '5px'})
])

if __name__ == '__main__':
    app.run_server(debug=True)