#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${GREEN}"
echo " Hotel Reservation API - Local Kubernetes Deployment"
echo "======================================================"
echo -e "${NC}"

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to wait for deployment
wait_for_deployment() {
    local deployment=$1
    local namespace=$2
    echo -e "${YELLOW} Waiting for $deployment to be ready...${NC}"
    kubectl wait --for=condition=available --timeout=300s deployment/$deployment -n $namespace
}

# Function to wait for statefulset
wait_for_statefulset() {
    local statefulset=$1
    local namespace=$2
    echo -e "${YELLOW} Waiting for $statefulset to be ready...${NC}"
    kubectl wait --for=jsonpath='{.status.readyReplicas}'=1 --timeout=300s statefulset/$statefulset -n $namespace
}

echo -e "${BLUE} Checking prerequisites...${NC}"

# Check if Docker is installed and running
if ! command_exists docker; then
    echo -e "${RED} Docker is not installed. Please install Docker Desktop first.${NC}"
    echo -e "${YELLOW} Download from: https://www.docker.com/products/docker-desktop${NC}"
    exit 1
fi

if ! docker info >/dev/null 2>&1; then
    echo -e "${RED} Docker is not running. Please start Docker Desktop.${NC}"
    exit 1
fi

# Check if kubectl is installed
if ! command_exists kubectl; then
    echo -e "${RED} kubectl is not installed.${NC}"
    echo -e "${YELLOW} You can install it via:${NC}"
    echo "   - Docker Desktop (enable Kubernetes)"
    echo "   - Or download from: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi

# Check if Kubernetes is running
if ! kubectl cluster-info >/dev/null 2>&1; then
    echo -e "${RED} Kubernetes cluster is not accessible.${NC}"
    echo -e "${YELLOW} Enable Kubernetes in Docker Desktop settings${NC}"
    echo -e "${YELLOW}   OR install minikube/kind and start a cluster${NC}"
    exit 1
fi

echo -e "${GREEN} All prerequisites met!${NC}"

# Detect Kubernetes environment
K8S_CONTEXT=$(kubectl config current-context)
if [[ "$K8S_CONTEXT" == *"docker-desktop"* ]]; then
    K8S_ENV="Docker Desktop"
elif [[ "$K8S_CONTEXT" == *"minikube"* ]]; then
    K8S_ENV="Minikube"
elif [[ "$K8S_CONTEXT" == *"kind"* ]]; then
    K8S_ENV="Kind"
else
    K8S_ENV="Unknown ($K8S_CONTEXT)"
fi

echo -e "${CYAN} Detected Kubernetes: $K8S_ENV${NC}"

# Build Docker image
echo -e "${PURPLE} Building Docker image...${NC}"
docker build -t hotel-reservation:local .

# Load image to cluster if needed
if [[ "$K8S_CONTEXT" == *"kind"* ]]; then
    echo -e "${YELLOW} Loading image to Kind cluster...${NC}"
    kind load docker-image hotel-reservation:local
elif [[ "$K8S_CONTEXT" == *"minikube"* ]]; then
    echo -e "${YELLOW} Loading image to Minikube...${NC}"
    minikube image load hotel-reservation:local
fi

echo -e "${BLUE} Creating namespaces...${NC}"
# Create namespaces
kubectl create namespace hotel-reservation-dev --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

echo -e "${PURPLE} Deploying MongoDB...${NC}"
# Deploy MongoDB first (it takes the longest)
kubectl apply -f k8s/base/mongodb.yaml -n hotel-reservation-dev
wait_for_statefulset "mongodb" "hotel-reservation-dev"

echo -e "${RED} Deploying Redis...${NC}"
# Deploy Redis
kubectl apply -f k8s/base/redis.yaml -n hotel-reservation-dev
wait_for_deployment "redis" "hotel-reservation-dev"

echo -e "${YELLOW} Deploying Monitoring Stack...${NC}"
# Deploy monitoring stack
kubectl apply -f k8s/monitoring/ -n monitoring
wait_for_deployment "prometheus" "monitoring" &
wait_for_deployment "grafana" "monitoring" &
wait

# Update API image tag for local deployment
echo -e "${CYAN} Configuring API deployment...${NC}"
sed 's/hotel-reservation:latest/hotel-reservation:local/g' k8s/base/api.yaml > /tmp/api-local.yaml

echo -e "${GREEN} Deploying Hotel Reservation API...${NC}"
# Deploy API
kubectl apply -f /tmp/api-local.yaml -n hotel-reservation-dev
wait_for_deployment "hotel-reservation-api" "hotel-reservation-dev"

# Clean up temp file
rm -f /tmp/api-local.yaml

# Ingress removed for local deployment - using port forwarding instead

# Wait a bit for everything to stabilize
echo -e "${YELLOW} Waiting for all services to stabilize...${NC}"
sleep 10

echo -e "${GREEN}"
echo " DEPLOYMENT COMPLETE!"
echo "====================="
echo -e "${NC}"

# Check all pods status
echo -e "${CYAN} Pod Status:${NC}"
echo -e "${BLUE}Hotel Reservation (Namespace: hotel-reservation-dev):${NC}"
kubectl get pods -n hotel-reservation-dev

echo -e "${BLUE}Monitoring (Namespace: monitoring):${NC}"
kubectl get pods -n monitoring

echo -e "${GREEN}"
echo " ACCESS YOUR SERVICES:"
echo "========================"
echo -e "${NC}"

# Start port forwarding in background
echo -e "${YELLOW} Starting port forwarding...${NC}"

# Kill any existing port forwards
pkill -f "kubectl port-forward" 2>/dev/null || true

# Start port forwarding for all services
kubectl port-forward svc/api-service 8080:80 -n hotel-reservation-dev > /dev/null 2>&1 &
API_PID=$!

kubectl port-forward svc/grafana-service 3000:3000 -n monitoring > /dev/null 2>&1 &
GRAFANA_PID=$!

kubectl port-forward svc/prometheus-service 9090:9090 -n monitoring > /dev/null 2>&1 &
PROMETHEUS_PID=$!

# Wait for port forwards to establish
sleep 3

echo -e "${GREEN} API Server:          ${CYAN}http://localhost:8080${NC}"
echo -e "${GREEN} Grafana Dashboard:   ${CYAN}http://localhost:3000${NC} ${YELLOW}(admin/admin123)${NC}"
echo -e "${GREEN} Prometheus Metrics:  ${CYAN}http://localhost:9090${NC}"
echo -e "${GREEN} API Health:          ${CYAN}http://localhost:8080/health${NC}"
echo -e "${GREEN} API Metrics:         ${CYAN}http://localhost:8080/metrics${NC}"

echo -e "${BLUE}"
echo " API ENDPOINTS:"
echo "=================="
echo -e "${NC}"
echo -e "${CYAN}Authentication:${NC}"
echo "  POST http://localhost:8080/api/v1/auth"
echo ""
echo -e "${CYAN}Hotels:${NC}"
echo "  GET  http://localhost:8080/api/v1/hotel"
echo "  GET  http://localhost:8080/api/v1/hotel/:id"
echo ""
echo -e "${CYAN}Bookings (requires auth):${NC}"
echo "  POST http://localhost:8080/api/v1/room/:id/book"
echo "  GET  http://localhost:8080/api/v1/booking"

echo -e "${PURPLE}"
echo " SAMPLE DATA:"
echo "==============="
echo -e "${NC}"
echo -e "${YELLOW} To populate with realistic sample data:${NC}"
echo "   go run scripts/comprehensive_seed.go"
echo ""
echo -e "${YELLOW} This will create:${NC}"
echo "    22 users (including admins)"
echo "    8 hotels across different cities"
echo "    276+ rooms of various types"
echo "    20+ realistic bookings"
echo "    JWT tokens for testing"

echo -e "${GREEN}"
echo " DEVELOPMENT COMMANDS:"
echo "========================="
echo -e "${NC}"
echo -e "${CYAN}View logs:${NC}"
echo "  kubectl logs -f deployment/hotel-reservation-api -n hotel-reservation-dev"
echo ""
echo -e "${CYAN}Scale replicas:${NC}"
echo "  kubectl scale deployment/hotel-reservation-api --replicas=3 -n hotel-reservation-dev"
echo ""
echo -e "${CYAN}Check resources:${NC}"
echo "  kubectl top pods -n hotel-reservation-dev"
echo ""
echo -e "${CYAN}Clean up:${NC}"
echo "  kubectl delete namespace hotel-reservation-dev monitoring"

echo -e "${RED}"
echo "  IMPORTANT:"
echo "============="
echo -e "${NC}"
echo -e "${YELLOW} Port forwarding is running in background${NC}"
echo -e "${YELLOW} To stop: pkill -f 'kubectl port-forward'${NC}"
echo -e "${YELLOW} Services will restart if you restart Docker${NC}"

# Save PIDs for cleanup script
cat > /tmp/hotel-reservation-pids.txt << EOF
API_PID=$API_PID
GRAFANA_PID=$GRAFANA_PID  
PROMETHEUS_PID=$PROMETHEUS_PID
EOF

echo -e "${GREEN}"
echo " YOUR ENTERPRISE-GRADE HOTEL RESERVATION SYSTEM IS NOW RUNNING LOCALLY!"
echo "========================================================================="
echo -e "${NC}"
echo -e "${CYAN}Next steps:${NC}"
echo "1. Visit http://localhost:8080/health to verify the API"
echo "2. Check out Grafana dashboards at http://localhost:3000"
echo "3. Run the seed script to populate sample data"
echo "4. Test the API with the Postman collection"
echo ""
echo -e "${PURPLE} Perfect for portfolio, learning, and development!${NC}"