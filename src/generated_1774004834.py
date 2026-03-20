from flask import Flask, jsonify
import random
from faker import Faker

app = Flask(__name__)

# Initialize Faker for generating fake data
fake = Faker()

@app.route('/')
def home():
    """Home route that returns a simple welcome message."""
    return "Welcome to the API!"

@app.route('/random-user')
def get_random_user():
    """Returns a random user data in JSON format."""
    user_data = {
        "name": fake.name(),
        "email": fake.email(),
        "address": fake.address(),
        "phone_number": fake.phone_number()
    }
    return jsonify(user_data)

@app.route('/health')
def health_check():
    """Simple health check endpoint."""
    return jsonify({"status": "ok", "timestamp": fake.date_time().isoformat()})

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=5000)