import numpy as np
from sklearn.ensemble import RandomForestClassifier
from sklearn.model_selection import train_test_split
from sklearn.metrics import accuracy_score
from src.model.utils import load_data, save_model, load_model
import logging

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class OnlineDetector:
    """
    A class for online detection using a random forest classifier.

    Attributes:
    model_path (str): The path to the model file.
    data_path (str): The path to the data file.
    model (RandomForestClassifier): The trained model.
    """

    def __init__(self, model_path: str, data_path: str):
        """
        Initialize the OnlineDetector instance.

        Args:
        model_path (str): The path to the model file.
        data_path (str): The path to the data file.

        Raises:
        ValueError: If model_path or data_path is None.
        """
        if model_path is None or data_path is None:
            raise ValueError("Model path and data path cannot be None")
        self.model_path = model_path
        self.data_path = data_path
        self.model = None

    def train(self, n_estimators: int = 100, max_depth: int = 10, min_samples_split: int = 2, min_samples_leaf: int = 1):
        """
        Train the model using the data from the data_path.

        Args:
        n_estimators (int): The number of estimators in the random forest.
        max_depth (int): The maximum depth of the trees in the random forest.
        min_samples_split (int): The minimum number of samples required to split an internal node.
        min_samples_leaf (int): The minimum number of samples required to be at a leaf node.

        Returns:
        RandomForestClassifier: The trained model.

        Raises:
        FileNotFoundError: If data_path does not exist.
        ValueError: If n_estimators, max_depth, min_samples_split or min_samples_leaf is invalid.
        """
        try:
            # Load data
            data = load_data(self.data_path)
            X, y = data['X'], data['y']

            # Split data into training and testing sets
            X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=42)

            # Train model
            self.model = RandomForestClassifier(n_estimators=n_estimators, max_depth=max_depth, min_samples_split=min_samples_split, min_samples_leaf=min_samples_leaf)
            self.model.fit(X_train, y_train)

            # Evaluate model
            y_pred = self.model.predict(X_test)
            accuracy = accuracy_score(y_test, y_pred)
            logger.info(f"Model accuracy: {accuracy:.2f}")

            # Save model
            save_model(self.model, self.model_path)

            return self.model

        except FileNotFoundError:
            logger.error(f"Data file not found at {self.data_path}")
            raise
        except ValueError as e:
            logger.error(f"Invalid value: {e}")
            raise
        except Exception as e:
            logger.error(f"An error occurred: {e}")
            raise

    def predict(self, data: np.ndarray):
        """
        Make predictions using the trained model.

        Args:
        data (np.ndarray): The input data.

        Returns:
        np.ndarray: The predicted labels.

        Raises:
        ValueError: If data is None.
        """
        if data is None:
            raise ValueError("Data cannot be None")
        if self.model is None:
            raise ValueError("Model has not been trained")
        return self.model.predict(data)

def main():
    # Example usage
    model_path = "model.pkl"
    data_path = "data.csv"
    detector = OnlineDetector(model_path, data_path)
    detector.train()
    data = np.array([[1, 2, 3], [4, 5, 6]])
    predictions = detector.predict(data)
    print(predictions)

if __name__ == "__main__":
    main()