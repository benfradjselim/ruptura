#!/bin/bash
# Script de déploiement MLOps v3.0
# Utilise les images depuis Docker Hub

set -e

echo "========================================="
echo "🚀 MLOps v3.0 - Déploiement sur Kubernetes"
echo "========================================="
echo ""

# Couleurs pour l'affichage
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Vérifier que kubectl est disponible
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}❌ kubectl n'est pas installé${NC}"
    exit 1
fi

# Vérifier la connexion au cluster
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}❌ Impossible de se connecter au cluster Kubernetes${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Connexion au cluster établie${NC}"

# Supprimer l'ancien namespace si existe
echo ""
echo "📦 Nettoyage de l'ancien environnement..."
kubectl delete namespace mlops --wait=true --ignore-not-found=true
sleep 5

# Créer le nouveau namespace
echo ""
echo "📦 Création du namespace mlops..."
kubectl create namespace mlops

# Appliquer le manifeste
echo ""
echo "📦 Déploiement des services v2 + v3..."
kubectl apply -f manifests/mlops-v3.0.yaml

# Attendre que les pods démarrent
echo ""
echo "⏳ Attente du démarrage des pods (30 secondes)..."
sleep 30

# Vérifier l'état des pods
echo ""
echo "========================================="
echo "📊 État des services"
echo "========================================="
kubectl get pods -n mlops -o wide

echo ""
echo "========================================="
echo "🔌 Services disponibles"
echo "========================================="
kubectl get svc -n mlops

echo ""
echo "========================================="
echo "📈 Vérification des services v3"
echo "========================================="

# Vérifier metric-predictor
echo ""
echo "🔍 Test metric-predictor (port 8008):"
if kubectl get pod -n mlops -l app=metric-predictor &> /dev/null; then
    METRIC_POD=$(kubectl get pod -n mlops -l app=metric-predictor -o jsonpath="{.items[0].metadata.name}")
    echo -e "   Pod: ${GREEN}$METRIC_POD${NC}"
    
    # Tester le health endpoint via le pod
    kubectl exec -n mlops $METRIC_POD -- curl -s http://localhost:8008/health 2>/dev/null && echo -e "   ${GREEN}✅ metric-predictor health OK${NC}" || echo -e "   ${YELLOW}⚠️ metric-predictor en démarrage${NC}"
else
    echo -e "   ${RED}❌ metric-predictor non trouvé${NC}"
fi

# Vérifier dashboard
echo ""
echo "🔍 Test dashboard (port 8501):"
if kubectl get pod -n mlops -l app=dashboard &> /dev/null; then
    DASHBOARD_POD=$(kubectl get pod -n mlops -l app=dashboard -o jsonpath="{.items[0].metadata.name}")
    echo -e "   Pod: ${GREEN}$DASHBOARD_POD${NC}"
else
    echo -e "   ${RED}❌ dashboard non trouvé${NC}"
fi

echo ""
echo "========================================="
echo "✅ Déploiement v3.0 terminé !"
echo "========================================="
echo ""
echo "📍 Accès au dashboard:"
echo "   ${GREEN}kubectl port-forward -n mlops svc/dashboard 8501:8501${NC}"
echo "   Puis ouvrir http://localhost:8501"
echo ""
echo "📍 Tester l'API metric-predictor:"
echo "   ${GREEN}kubectl port-forward -n mlops svc/metric-predictor 8008:8008${NC}"
echo "   curl http://localhost:8008/health"
echo "   curl http://localhost:8008/forecast"
echo ""
echo "📍 Voir les logs:"
echo "   ${GREEN}kubectl logs -n mlops -l app=metric-predictor --tail=50${NC}"
echo "   ${GREEN}kubectl logs -n mlops -l app=dashboard --tail=50${NC}"
echo ""
echo "📍 Forcer un redémarrage:"
echo "   ${GREEN}kubectl rollout restart deployment -n mlops${NC}"
