import torch
import torchvision
from torch import nn

class Detector(nn.Module):
    def __init__(self, num_classes=80):
        super(Detector, self).__init__()
        # Initialize the backbone network
        self.backbone = torchvision.models.resnet50(pretrained=True)
        # Freeze the backbone weights
        for param in self.backbone.parameters():
            param.requires_grad = False
        
        # Build the detector model
        self.model = torchvision.models.detection.fasterrcnn_resnet50_fpn(pretrained=True)
        self.model.roi_heads.num_classes = num_classes
        
        # Initialize the detection head
        self.detection_head = DetectionHead()
        
    def forward(self, x):
        # Forward pass through the backbone
        features = self.backbone(x)
        # Get detection predictions
        predictions = self.detection_head(features)
        return predictions
    
    def load_pretrained(self, path):
        # Load pretrained weights
        self.load_state_dict(torch.load(path))
    
    @classmethod
    def get_model(cls):
        return cls()
    
    def inference(self, images, image_sizes=None):
        # Convert images to device
        images = list(img.to(next(self.parameters()).device) for img in images)
        
        # Perform detection
        with torch.no_grad():
            outputs = self.model(images, image_sizes)
        return outputs
    
    def __repr__(self):
        return f"Detector(backbone={self.backbone.__class__.__name__}, num_classes={self.model.roi_heads.num_classes})"