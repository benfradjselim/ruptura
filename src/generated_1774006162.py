import os
import pandas as pd
import numpy as np
from random import Random
import torch
from torch.utils.data import DataLoader, Dataset

class DataLoader:
    def __init__(self, data_path, batch_size=32, shuffle=True, seed=42):
        self.data_path = data_path
        self.batch_size = batch_size
        self.shuffle = shuffle
        self.seed = seed
        self.random = Random(self.seed)
        
        # Set random seeds for reproducibility
        np.random.seed(self.seed)
        torch.manual_seed(self.seed)
        
    def load_data(self):
        """Load data from CSV file"""
        data = pd.read_csv(self.data_path)
        return data
    
    def split_data(self, data, test_size=0.2):
        """Split data into training and testing sets"""
        # Split into features and target
        X = data.drop('target', axis=1).values
        y = data['target'].values
        
        # Split into train and test sets
        indices = np.arange(X.shape[0])
        self.random.shuffle(indices)
        split_idx = int(test_size * X.shape[0])
        
        train_indices = indices[:split_idx]
        test_indices = indices[split_idx:]
        
        X_train, y_train = X[train_indices], y[train_indices]
        X_test, y_test = X[test_indices], y[test_indices]
        
        return (X_train, y_train), (X_test, y_test)
    
    def to_tensor(self, X, y):
        """Convert numpy arrays to PyTorch tensors"""
        X_tensor = torch.from_numpy(X).float()
        y_tensor = torch.from_numpy(y).long()
        return X_tensor, y_tensor
    
    def get_dataloaders(self):
        """Get training and testing dataloaders"""
        data = self.load_data()
        (X_train, y_train), (X_test, y_test) = self.split_data(data)
        
        # Convert to tensors
        X_train, y_train = self.to_tensor(X_train, y_train)
        X_test, y_test = self.to_tensor(X_test, y_test)
        
        # Create datasets
        train_dataset = torch.utils.data.TensorDataset(X_train, y_train)
        test_dataset = torch.utils.data.TensorDataset(X_test, y_test)
        
        # Create dataloaders
        train_loader = DataLoader(train_dataset, batch_size=self.batch_size, shuffle=self.shuffle)
        test_loader = DataLoader(test_dataset, batch_size=self.batch_size, shuffle=False)
        
        return train_loader, test_loader
    
    def __len__(self):
        """Return the number of samples in the dataset"""
        data = self.load_data()
        return len(data)

if __name__ == "__main__":
    # Example usage
    data_loader = DataLoader(data_path="path/to/your/data.csv")
    train_loader, test_loader = data_loader.get_dataloaders()
    
    # Print first batch
    for batch_features, batch_labels in train_loader:
        print("Batch features shape:", batch_features.shape)
        print("Batch labels shape:", batch_labels.shape)
        break