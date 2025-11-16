@echo off
setlocal enabledelayedexpansion

echo.
echo  Hotel Reservation API - Local Kubernetes Deployment
echo ======================================================
echo.

REM Check if Docker is installed and running
docker info >nul 2>&1
if errorlevel 1 (
    echo  Docker is not running. Please start Docker Desktop.
    echo  Download from: https://www.docker.com/products/docker-desktop
    pause
    exit /b 1
)

REM Check if kubectl is installed
kubectl version --client >nul 2>&1
if errorlevel 1 (
    echo  kubectl is not installed.
    echo  Enable Kubernetes in Docker Desktop settings
    pause
    exit /b 1
)

REM Check if Kubernetes is running
kubectl cluster-info >nul 2>&1
if errorlevel 1 (
    echo  Kubernetes cluster is not accessible.
    echo  Enable Kubernetes in Docker Desktop settings
    pause
    exit /b 1
)

echo  All prerequisites met!
echo.

REM Build Docker image
echo  Building Docker image...
docker build -t hotel-reservation:local .

echo.
echo  Creating namespaces...
kubectl create namespace hotel-reservation-dev --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

echo.
echo  Deploying MongoDB...
kubectl apply -f k8s/base/mongodb.yaml -n hotel-reservation-dev
echo  Waiting for MongoDB to be ready...
kubectl wait --for=condition=ready pod -l app=mongodb --timeout=300s -n hotel-reservation-dev

echo.
echo  Deploying Redis...
kubectl apply -f k8s/base/redis.yaml -n hotel-reservation-dev
echo  Waiting for Redis to be ready...
kubectl wait --for=condition=available --timeout=300s deployment/redis -n hotel-reservation-dev

echo.
echo  Deploying Monitoring Stack...
kubectl apply -f k8s/monitoring/ -n monitoring

echo.
echo  Configuring API deployment...
REM Create modified API yaml with local image
powershell -Command "(Get-Content k8s/base/api.yaml) -replace 'hotel-reservation:latest', 'hotel-reservation:local' | Set-Content api-local-temp.yaml"

echo  Deploying Hotel Reservation API...
kubectl apply -f api-local-temp.yaml -n hotel-reservation-dev
del api-local-temp.yaml

echo  Waiting for API to be ready...
kubectl wait --for=condition=available --timeout=300s deployment/hotel-reservation-api -n hotel-reservation-dev

echo  Waiting for monitoring to be ready...
kubectl wait --for=condition=available --timeout=300s deployment/grafana -n monitoring
kubectl wait --for=condition=available --timeout=300s deployment/prometheus -n monitoring

echo.
echo  Waiting for all services to stabilize...
timeout /t 10 >nul

echo.
echo  DEPLOYMENT COMPLETE!
echo =====================
echo.

echo  Pod Status:
echo Hotel Reservation (Namespace: hotel-reservation-dev):
kubectl get pods -n hotel-reservation-dev

echo.
echo Monitoring (Namespace: monitoring):
kubectl get pods -n monitoring

echo.
echo  ACCESS YOUR SERVICES:
echo ========================
echo.
echo  Starting port forwarding...
echo  Keep this window open to maintain connections
echo.

REM Start port forwarding (will block)
echo Starting API server on http://localhost:8080
start /min cmd /c "kubectl port-forward svc/api-service 8080:80 -n hotel-reservation-dev"

echo Starting Grafana dashboard on http://localhost:3000 (admin/admin123)
start /min cmd /c "kubectl port-forward svc/grafana-service 3000:3000 -n monitoring"

echo Starting Prometheus on http://localhost:9090
start /min cmd /c "kubectl port-forward svc/prometheus-service 9090:9090 -n monitoring"

timeout /t 3 >nul

echo.
echo  API Server:          http://localhost:8080
echo  Grafana Dashboard:   http://localhost:3000 (admin/admin123)
echo  Prometheus Metrics:  http://localhost:9090
echo  API Health:          http://localhost:8080/health
echo  API Metrics:         http://localhost:8080/metrics
echo.

echo  API ENDPOINTS:
echo ==================
echo Authentication:
echo   POST http://localhost:8080/api/v1/auth
echo.
echo Hotels:
echo   GET  http://localhost:8080/api/v1/hotel
echo   GET  http://localhost:8080/api/v1/hotel/:id
echo.
echo Bookings (requires auth):
echo   POST http://localhost:8080/api/v1/room/:id/book
echo   GET  http://localhost:8080/api/v1/booking
echo.

echo  SAMPLE DATA:
echo ===============
echo  To populate with realistic sample data:
echo    go run scripts/comprehensive_seed.go
echo.
echo  This will create:
echo     22 users (including admins)
echo     8 hotels across different cities  
echo     276+ rooms of various types
echo     20+ realistic bookings
echo     JWT tokens for testing
echo.

echo  DEVELOPMENT COMMANDS:
echo =========================
echo View logs:
echo   kubectl logs -f deployment/hotel-reservation-api -n hotel-reservation-dev
echo.
echo Scale replicas:
echo   kubectl scale deployment/hotel-reservation-api --replicas=3 -n hotel-reservation-dev
echo.
echo Clean up:
echo   scripts\local-cleanup.bat
echo.

echo  YOUR ENTERPRISE-GRADE HOTEL RESERVATION SYSTEM IS NOW RUNNING LOCALLY!
echo =========================================================================
echo.
echo  Perfect for portfolio, learning, and development!
echo.
echo Press any key to open the services in your browser...
pause >nul

REM Open services in browser
start http://localhost:8080/health
start http://localhost:3000
start http://localhost:9090

echo.
echo  Services opened in your browser!
echo  Keep this window open to maintain port forwarding
echo Press Ctrl+C to stop all services
pause