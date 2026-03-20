# src/collector/data_processor.py

"""
Data processor module for collecting and processing data.
"""

import pandas as pd
import logging
from typing import Optional, Dict, Any

class DataProcessor:
    """Class responsible for processing collected data."""
    
    def __init__(self, data_dir: str = "./data"):
        """
        Initialize the DataProcessor with a data directory.
        
        Args:
            data_dir: Path to directory where data will be stored
        """
        self.data_dir = data_dir
        self.logger = logging.getLogger(__name__)

    def process_data(self, raw_data: Dict[str, Any]) -> Optional[pd.DataFrame]:
        """
        Process raw collected data.
        
        Args:
            raw_data: Dictionary containing raw data to be processed
            
        Returns:
            Processed data as DataFrame or None if processing failed
        """
        try:
            # Clean the data
            cleaned_data = self._clean_data(raw_data)
            
            # Transform the data
            transformed_data = self._transform_data(cleaned_data)
            
            # Validate the data
            if not self._validate_data(transformed_data):
                self.logger.warning("Data validation failed")
                return None
                
            return transformed_data
            
        except Exception as e:
            self.logger.error(f"Data processing failed: {str(e)}")
            return None
            
    def _clean_data(self, raw_data: Dict[str, Any]) -> pd.DataFrame:
        """
        Clean the raw data by removing invalid entries and duplicates.
        
        Args:
            raw_data: Dictionary containing raw data
            
        Returns:
            Cleaned data as DataFrame
        """
        try:
            df = pd.DataFrame(raw_data)
            df = df.dropna()  # Remove rows with NaN values
            df = df.drop_duplicates()  # Remove duplicate rows
            return df
            
        except Exception as e:
            self.logger.error(f"Data cleaning failed: {str(e)}")
            raise

    def _transform_data(self, df: pd.DataFrame) -> pd.DataFrame:
        """
        Transform the data by applying necessary conversions and calculations.
        
        Args:
            df: DataFrame containing cleaned data
            
        Returns:
            Transformed data as DataFrame
        """
        try:
            # Example transformation: Convert string dates to datetime
            if 'date' in df.columns:
                df['date'] = pd.to_datetime(df['date'])
                
            return df
            
        except Exception as e:
            self.logger.error(f"Data transformation failed: {str(e)}")
            raise

    def _validate_data(self, df: pd.DataFrame) -> bool:
        """
        Validate the processed data.
        
        Args:
            df: DataFrame containing processed data
            
        Returns:
            True if data is valid, False otherwise
        """
        try:
            # Basic validation: Check if DataFrame is empty
            if df.empty:
                return False
                
            # Add custom validation logic here
            # Example: Check required columns
            required_columns = ['id', 'name', 'value']
            if not all(col in df.columns for col in required_columns):
                return False
                
            return True
            
        except Exception as e:
            self.logger.error(f"Data validation failed: {str(e)}")
            return False

def main():
    # Example usage
    if __name__ == "__main__":
        processor = DataProcessor()
        sample_data = {
            'id': [1, 2, 3],
            'name': ['A', 'B', 'C'],
            'value': [10, 20, 30]
        }
        processed_df = processor.process_data(sample_data)
        print(processed_df)

if __name__ == "__main__":
    main()