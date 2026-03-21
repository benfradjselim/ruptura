import numpy as np
from sklearn.ensemble import RandomForestClassifier
from sklearn.model_selection import train_test_split
from sklearn.metrics import accuracy_score
from src.model.utils import load_data, save_model, load_model

class OnlineDetector:
    def __init__(self, model_path, data_path):
        self.model_path = model_path
        self.data_path = data_path
        self.model = None

    def train(self, n_estimators=100, max_depth=10, min_samples_split=2, min_samples_leaf=1):
        X, y = load_data(self.data_path)
        X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=42)

        self.model = RandomForestClassifier(n_estimators=n_estimators, max_depth=max_depth, 
                                             min_samples_split=min_samples_split, min_samples_leaf=min_samples_leaf)
        self.model.fit(X_train, y_train)

        y_pred = self.model.predict(X_test)
        accuracy = accuracy_score(y_test, y_pred)
        print(f"Model accuracy: {accuracy:.3f}")

        save_model(self.model, self.model_path)

    def predict(self, data):
        if self.model is None:
            self.model = load_model(self.model_path)
        return self.model.predict(data)

    def update(self, new_data):
        if self.model is None:
            self.model = load_model(self.model_path)
        self.model.fit(new_data, np.zeros(len(new_data)))
        save_model(self.model, self.model_path)