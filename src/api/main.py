from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import Optional
from sklearn.ensemble import RandomForestClassifier
from sklearn.model_selection import train_test_split
import pandas as pd
import numpy as np

app = FastAPI()

# Load the model
model = RandomForestClassifier()
model.load('model.pkl')

# Define the endpoint to ingest data
@app.post("/ingest")
async def ingest_data(data: dict):
    # Create a DataFrame from the data
    df = pd.DataFrame(data)
    
    # Save the data to a CSV file
    df.to_csv('data.csv', index=False)
    
    return {"message": "Data ingested successfully"}

# Define the endpoint to make a prediction
@app.post("/predict")
async def make_prediction(data: dict):
    # Load the data from the CSV file
    df = pd.read_csv('data.csv')
    
    # Make a prediction using the model
    prediction = model.predict(df)
    
    return {"prediction": prediction[0]}

# Define the endpoint to check the health of the API
@app.get("/health")
async def check_health():
    return {"status": "healthy"}