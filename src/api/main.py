from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import Optional
from sklearn.ensemble import RandomForestClassifier
from sklearn.model_selection import train_test_split
import pandas as pd
import numpy as np
import logging

# Initialize the logger
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI()

# Load the model
def load_model(model_path: str) -> RandomForestClassifier:
    """
    Load the trained model from a file.

    Args:
    model_path (str): The path to the model file.

    Returns:
    RandomForestClassifier: The loaded model.

    Raises:
    HTTPException: If the model cannot be loaded.
    """
    try:
        model = RandomForestClassifier()
        model.load(model_path)
        return model
    except Exception as e:
        logger.error(f"Failed to load model: {e}")
        raise HTTPException(status_code=500, detail="Failed to load model")

model = load_model('model.pkl')

# Define the endpoint to ingest data
@app.post("/ingest")
async def ingest_data(data: dict):
    """
    Ingest data from a dictionary and save it to a CSV file.

    Args:
    data (dict): The data to ingest.

    Returns:
    dict: A success message.

    Raises:
    HTTPException: If the data is invalid or cannot be saved.
    """
    try:
        # Input validation
        if not data:
            raise HTTPException(status_code=400, detail="No data provided")

        # Create a DataFrame from the data
        df = pd.DataFrame(data)

        # Save the data to a CSV file
        df.to_csv('data.csv', index=False)

        logger.info("Data ingested successfully")
        return {"message": "Data ingested successfully"}
    except Exception as e:
        logger.error(f"Failed to ingest data: {e}")
        raise HTTPException(status_code=500, detail="Failed to ingest data")

# Define the endpoint to make a prediction
@app.post("/predict")
async def make_prediction(data: dict):
    """
    Make a prediction using the loaded model.

    Args:
    data (dict): The data to make a prediction on.

    Returns:
    dict: The prediction.

    Raises:
    HTTPException: If the data is invalid or the model cannot make a prediction.
    """
    try:
        # Make a prediction using the loaded model
        prediction = model.predict(data)

        return {"prediction": prediction}
    except Exception as e:
        logger.error(f"Failed to make prediction: {e}")
        raise HTTPException(status_code=500, detail="Failed to make prediction")