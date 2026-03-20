import torch
import torch.nn as nn

class Autoencoder(nn.Module):
    def __init__(self, input_size=784, hidden_size=256, latent_size=20):
        super(Autoencoder, self).__init__()
        
        # Encoder
        self.encoder = nn.Sequential(
            nn.Linear(input_size, hidden_size),
            nn.ReLU(),
            nn.Linear(hidden_size, latent_size)
        )
        
        # Decoder
        self.decoder = nn.Sequential(
            nn.Linear(latent_size, hidden_size),
            nn.ReLU(),
            nn.Linear(hidden_size, input_size),
            nn.Sigmoid()
        )
        
    def forward(self, x):
        # Encoding
        z = self.encoder(x)
        
        # Decoding
        reconstructed = self.decoder(z)
        
        return reconstructed
    
    def get_latent(self, x):
        return self.encoder(x)