#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
This module contains functionality for feature engineering.
It provides methods to process and transform raw data into engineered features.
"""

import pandas as pd
import numpy as np
import logging

class FeatureEngineer:
    """
    A class used to perform feature engineering operations.
    
    Attributes:
    ----------
    logger : logging.Logger
        Logger instance for logging operations.
    config : dict
        Configuration dictionary for feature engineering parameters.
    """
    
    def __init__(self, config_path=None):
        """
        Initializes the FeatureEngineer with configuration.
        
        Parameters:
        ----------
        config_path : str, optional
            Path to the configuration file (default is None).
        """
        self.logger = logging.getLogger(__name__)
        self.config = self._load_config(config_path)
        
    def _load_config(self, config_path):
        """
        Loads configuration from a JSON file.
        
        Parameters:
        ----------
        config_path : str
            Path to the configuration file.
            
        Returns:
        -------
        dict
            Configuration dictionary.
        """
        if config_path:
            try:
                with open(config_path, 'r') as f:
                    return pd.read_json(f)
            except Exception as e:
                self.logger.error(f"Failed to load config: {e}")
                return {}
        return {}
    
    def engineer_features(self, df):
        """
        Performs feature engineering on the input DataFrame.
        
        Parameters:
        ----------
        df : pandas.DataFrame
            Input DataFrame containing raw data.
            
        Returns:
        -------
        pandas.DataFrame
            DataFrame with engineered features.
        """
        # TODO: Implement your feature engineering logic here
        # Example steps:
        # 1. Handle missing values
        # 2. Encode categorical variables
        # 3. Scale/normalize data
        # 4. Create new features
        
        # Example implementation:
        if df is None:
            raise ValueError("Input DataFrame is None")
            
        # Create copy of original DataFrame
        df_engineered = df.copy()
        
        # Example feature engineering steps
        # 1. Handle missing values
        df_engineered.fillna(df_engineered.mean(), inplace=True)
        
        # 2. Encode categorical variables
        categorical_cols = df.select_dtypes(include=['object']).columns
        df_engineered = pd.get_dummies(df_engineered, columns=categorical_cols)
        
        # 3. Scale/normalize data
        from sklearn.preprocessing import StandardScaler
        scaler = StandardScaler()
        numerical_cols = df.select_dtypes(exclude=['object']).columns
        df_engineered[numerical_cols] = scaler.fit_transform(df_engineered[numerical_cols])
        
        # 4. Create new features
        # Example: Create interaction features
        df_engineered['new_feature'] = df_engineered['feature1'] * df_engineered['feature2']
        
        return df_engineered

if __name__ == "__main__":
    # Example usage
    from pathlib import Path
    
    # Initialize feature engineer
    feature_engineer = FeatureEngineer(config_path=Path("config/config.json"))
    
    # Load sample data
    sample_data = pd.DataFrame({
        'A': [1, 2, 3],
        'B': [4, 5, 6]
    })
    
    # Engineer features
    engineered_df = feature_engineer.engineer_features(sample_data)
    
    # Print result
    print(engineered_df)