#!/bin/bash
# Script de déploiement MLOps v3.0 avec images Docker Hub

set -e

echo "🚀 Déploiement MLOps v3.0"
echo "========================"

# Vérifier kubectl
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl n'est pas installé"
    exit 1
fi

# Vérifier Kind
if ! command -v kind &> /dev/null; then
    echo "❌ kind n'est pas installé"
    exit 1
fi

# Supprimer l'ancien namespace
echo "📦 Nettoyage..."
kubectl delete namespace mlops --wait=true --ignore-not-found=true

# Créer le nouveau namespace
kubectl create namespace mlops

# Appliquer le manifeste
echo "📦 Déploiement des services..."
kubectl apply -f manifests/mlops-v3.0.yaml

# Attendre
sleep 30

# Vérifier les pods
echo ""
echo "📊 État des services:"
kubectl get pods -n mlops

# Vérifier les services
echo ""
echo "🔌 Services disponibles:"
kubectl get svc -n mlops

echo ""
echo "✅ Déploiement terminé !"
echo ""
echo "📍 Accès au dashboard:"
echo "   kubectl port-forward -n mlops svc/dashboard 8501:8501"
