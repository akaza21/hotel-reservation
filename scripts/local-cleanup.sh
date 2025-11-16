#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${RED}"
echo "🧹 Hotel Reservation API - Local Cleanup"
echo "========================================"
echo -e "${NC}"

echo -e "${YELLOW}🔌 Stopping port forwarding...${NC}"
# Kill port forwarding processes
pkill -f "kubectl port-forward" 2>/dev/null || true

# Read PIDs if available
if [ -f /tmp/hotel-reservation-pids.txt ]; then
    source /tmp/hotel-reservation-pids.txt
    kill $API_PID $GRAFANA_PID $PROMETHEUS_PID 2>/dev/null || true
    rm -f /tmp/hotel-reservation-pids.txt
fi

echo -e "${BLUE}📦 Cleaning up Kubernetes resources...${NC}"

# Delete namespaces (this will delete everything in them)
echo -e "${YELLOW}Deleting hotel-reservation-dev namespace...${NC}"
kubectl delete namespace hotel-reservation-dev --ignore-not-found=true

echo -e "${YELLOW}Deleting monitoring namespace...${NC}"
kubectl delete namespace monitoring --ignore-not-found=true

# Ingress removed for local-only deployment

echo -e "${BLUE}Cleaning up Docker images...${NC}"
# Remove local Docker image
docker rmi hotel-reservation:local 2>/dev/null || true

# Clean up any dangling images
docker image prune -f > /dev/null 2>&1 || true

echo -e "${GREEN}Cleanup complete.${NC}"

echo -e "${CYAN}"
echo "Resources removed:"
echo "=================="
echo -e "${NC}"
echo "• Kubernetes pods and services"
echo "• MongoDB data and Redis cache"
echo "• Prometheus metrics and Grafana dashboards"
echo "• Port forwarding processes"
echo "• Local Docker images"
echo "• Namespaces: hotel-reservation-dev, monitoring"

echo -e "${YELLOW}"
echo "To redeploy, run ./scripts/local-deploy.sh"
echo -e "${NC}"
