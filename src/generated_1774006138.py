"""Module for validating data against predefined schemas."""

import pathlib
from typing import Any, Dict, Optional

import jsonschema

class DataValidator:
    """Class responsible for validating data against JSON schemas."""
    
    def __init__(self, data_path: pathlib.Path):
        """
        Initialize the DataValidator with a path to the data directory.
        
        Args:
            data_path: Path to the directory containing data files
        """
        self.data_path = data_path
        
        if not self.data_path.exists():
            raise FileNotFoundError(f"The data path {self.data_path} does not exist")
    
    def validate(self, data: Dict[str, Any], schema: Dict[str, Any]) -> bool:
        """
        Validate the given data against the provided schema.
        
        Args:
            data: Dictionary containing the data to validate
            schema: Dictionary containing the JSON schema
            
        Returns:
            bool: True if data is valid according to the schema, False otherwise
        """
        try:
            jsonschema.validate(instance=data, schema=schema)
            return True
        except jsonschema.ValidationError as err:
            print(f"Validation error: {err}")
            return False
    
    def _load_schema(self, schema_path: pathlib.Path) -> Dict[str, Any]:
        """
        Load and return a JSON schema from a file.
        
        Args:
            schema_path: Path to the JSON schema file
            
        Returns:
            Dict[str, Any]: Loaded JSON schema
        """
        with open(schema_path, 'r', encoding='utf-8') as schema_file:
            return json.loads(schema_file.read())
    
    @staticmethod
    def validate_required_fields(data: Dict[str, Any], required_fields: list[str]) -> Dict[str, bool]:
        """
        Validate that all required fields are present in the data.
        
        Args:
            data: Dictionary containing the data to validate
            required_fields: List of required field names
            
        Returns:
            Dict[str, bool]: Dictionary mapping field names to their presence status
        """
        return {field: field in data for field in required_fields}
    
    @staticmethod
    def validate_data_types(data: Dict[str, Any], type_schema: Dict[str, type]) -> Dict[str, bool]:
        """
        Validate that the data types match the expected types.
        
        Args:
            data: Dictionary containing the data to validate
            type_schema: Dictionary mapping field names to expected types
            
        Returns:
            Dict[str, bool]: Dictionary mapping field names to their type validation status
        """
        return {field: isinstance(data[field], expected_type) 
                for field, expected_type in type_schema.items()}

if __name__ == "__main__":
    # Example usage
    from pathlib import Path
    
    # Initialize validator
    data_validator = DataValidator(Path("path/to/your/data"))
    
    # Example schema
    schema = {
        "type": "object",
        "properties": {
            "name": {"type": "string"},
            "age": {"type": "integer"}
        },
        "required": ["name", "age"]
    }
    
    # Example data
    data = {
        "name": "John Doe",
        "age": 30
    }
    
    # Validate data
    is_valid = data_validator.validate(data, schema)
    print(f"Data is valid: {is_valid}")