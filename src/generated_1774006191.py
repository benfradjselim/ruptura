# src/model/model_trainer.py

import torch
import torch.optim as optim
from torch.utils.data import DataLoader
import torchmetrics

class ModelTrainer:
    def __init__(self, model, train_dataset, val_dataset, 
                 batch_size=32, num_epochs=10, 
                 learning_rate=1e-4, device='cuda'):
        self.model = model
        self.train_dataset = train_dataset
        self.val_dataset = val_dataset
        self.batch_size = batch_size
        self.num_epochs = num_epochs
        self.learning_rate = learning_rate
        self.device = device
        
        # Setup data loaders
        self.train_loader = DataLoader(train_dataset, batch_size=batch_size, shuffle=True)
        self.val_loader = DataLoader(val_dataset, batch_size=batch_size, shuffle=False)
        
    def setup(self):
        # Initialize optimizer and loss function
        self.optimizer = optim.Adam(self.model.parameters(), lr=self.learning_rate)
        self.criterion = torch.nn.CrossEntropyLoss()
        
        # Move model to device
        self.model.to(self.device)
        
    def train_one_epoch(self, epoch):
        self.model.train()
        running_loss = 0.0
        running_corrects = 0
        
        for inputs, labels in self.train_loader:
            inputs = inputs.to(self.device)
            labels = labels.to(self.device)
            
            # Zero the parameter gradients
            self.optimizer.zero_grad()
            
            # Forward + backward + optimize
            outputs = self.model(inputs)
            loss = self.criterion(outputs, labels)
            loss.backward()
            self.optimizer.step()
            
            # Statistics
            running_loss += loss.item() * inputs.size(0)
            _, preds = torch.max(outputs, 1)
            running_corrects += torch.sum(preds == labels.data).item()
            
        epoch_loss = running_loss / len(self.train_loader.dataset)
        epoch_acc = running_corrects / len(self.train_loader.dataset)
        
        print(f'Epoch [{epoch+1}/{self.num_epochs}], Loss: {epoch_loss:.4f}, Acc: {epoch_acc:.4f}')
        
        return epoch_loss, epoch_acc
    
    def validate(self):
        self.model.eval()
        val_loss = 0.0
        val_corrects = 0
        
        with torch.no_grad():
            for inputs, labels in self.val_loader:
                inputs = inputs.to(self.device)
                labels = labels.to(self.device)
                
                outputs = self.model(inputs)
                loss = self.criterion(outputs, labels)
                
                val_loss += loss.item() * inputs.size(0)
                _, preds = torch.max(outputs, 1)
                val_corrects += torch.sum(preds == labels.data).item()
                
        val_loss /= len(self.val_loader.dataset)
        val_acc = val_corrects / len(self.val_loader.dataset)
        
        print(f'Validation Loss: {val_loss:.4f}, Val Acc: {val_acc:.4f}')
        
        return val_loss, val_acc
    
    def train(self):
        self.setup()
        best_acc = 0.0
        
        for epoch in range(self.num_epochs):
            train_loss, train_acc = self.train_one_epoch(epoch)
            val_loss, val_acc = self.validate()
            
            if val_acc > best_acc:
                best_acc = val_acc
                torch.save(self.model.state_dict(), 'best_model.pth')
                print(f'Best model saved with accuracy {best_acc:.4f}')
                
        print('Training complete')
        
    def load_best_model(self):
        self.model.load_state_dict(torch.load('best_model.pth'))
        self.model.eval()
        
def main():
    # Example usage
    from your_dataset import YourDataset
    from your_model import YourModel
    
    # Create datasets
    train_dataset = YourDataset(train=True)
    val_dataset = YourDataset(train=False)
    
    # Initialize model
    model = YourModel()
    
    # Initialize trainer
    trainer = ModelTrainer(model, train_dataset, val_dataset)
    
    # Start training
    trainer.train()

if __name__ == "__main__":
    main()