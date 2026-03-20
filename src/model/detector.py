# src/model/model_evaluator.py

import os
import numpy as np
import pandas as pd
from sklearn.metrics import (
    accuracy_score,
    precision_score,
    recall_score,
    f1_score,
    confusion_matrix,
    classification_report,
    roc_auc_score
)

class ModelEvaluator:
    def __init__(self, model, X_test, y_test, output_path):
        self.model = model
        self.X_test = X_test
        self.y_test = y_test
        self.output_path = output_path
        self.predictions = None
        self.proba_predictions = None
        
    def run_evaluation(self):
        """Run the evaluation process and save results"""
        self._make_predictions()
        self._calculate_metrics()
        self._save_results()
        
    def _make_predictions(self):
        """Make predictions on test set"""
        self.predictions = self.model.predict(self.X_test)
        self.proba_predictions = self.model.predict_proba(self.X_test)
        
    def _calculate_metrics(self):
        """Calculate various evaluation metrics"""
        # Classification metrics
        self.accuracy = accuracy_score(self.y_test, self.predictions)
        self.precision = precision_score(self.y_test, self.predictions, average='weighted')
        self.recall = recall_score(self.y_test, self.predictions, average='weighted')
        self.f1 = f1_score(self.y_test, self.predictions, average='weighted')
        
        # Additional metrics
        self.confusion_matrix = confusion_matrix(self.y_test, self.predictions)
        self.classification_report = classification_report(self.y_test, self.predictions)
        
        # For probabilistic models
        if len(self.proba_predictions.shape) > 1:
            self.roc_auc = roc_auc_score(self.y_test, self.proba_predictions[:, 1])
            
    def _save_results(self):
        """Save evaluation results to output path"""
        results = {
            'metric': ['Accuracy', 'Precision', 'Recall', 'F1 Score'],
            'value': [self.accuracy, self.precision, self.recall, self.f1]
        }
        
        results_df = pd.DataFrame(results)
        output_file = os.path.join(self.output_path, 'evaluation_metrics.csv')
        results_df.to_csv(output_file, index=False)
        
        # Save confusion matrix
        cm_df = pd.DataFrame(self.confusion_matrix, columns=['Predicted 0', 'Predicted 1'])
        cm_df.index = ['Actual 0', 'Actual 1']
        cm_output = os.path.join(self.output_path, 'confusion_matrix.csv')
        cm_df.to_csv(cm_output)
        
if __name__ == "__main__":
    # Example usage
    from sklearn.datasets import make_classification
    from sklearn.model_selection import train_test_split
    from sklearn.ensemble import RandomForestClassifier
    
    # Generate sample data
    X, y = make_classification(n_samples=1000, n_features=20, n_informative=15, n_redundant=5, random_state=42)
    
    # Split data
    X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=42)
    
    # Train model
    model = RandomForestClassifier(random_state=42)
    model.fit(X_train, y_train)
    
    # Initialize evaluator
    evaluator = ModelEvaluator(model, X_test, y_test, output_path='evaluation_results')
    evaluator.run_evaluation()