$toolsDir = "C:\Users\ankit\.vault-tools"
$workDir = "C:\Users\ankit\OneDrive\Desktop\hackathon"
$env:PATH = "$toolsDir\go\bin;$toolsDir\etcd;$env:PATH"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "      VAULT DISTRIBUTED STORAGE & WEB CONSOLE LAUNCHER    " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. Stop any existing cluster processes
if (Test-Path "$workDir\cluster_pids.txt") {
    Write-Host "Stopping existing cluster processes..." -ForegroundColor DarkGray
    $pids = (Get-Content "$workDir\cluster_pids.txt").Split(',')
    foreach ($p in $pids) {
        if ($p -match '^\d+$') {
            Stop-Process -Id ([int]$p) -Force -ErrorAction SilentlyContinue
        }
    }
    Remove-Item "$workDir\cluster_pids.txt" -Force -ErrorAction SilentlyContinue
}

# 2. Check & Build Frontend if needed
if (-not (Test-Path "$workDir\frontend\dist\index.html")) {
    Write-Host "Building React Web Console assets..." -ForegroundColor Yellow
    Push-Location "$workDir\frontend"
    npm run build
    Pop-Location
}

# 3. Check & Build Go Binaries if needed
if (-not (Test-Path "$workDir\bin\coordinator.exe")) {
    Write-Host "Compiling Vault Go binaries..." -ForegroundColor Yellow
    New-Item -ItemType Directory -Force -Path "$workDir\bin" | Out-Null
    go build -o "$workDir\bin\storage-node.exe" "$workDir\cmd\storage-node"
    go build -o "$workDir\bin\metadata.exe" "$workDir\cmd\metadata"
    go build -o "$workDir\bin\coordinator.exe" "$workDir\cmd\coordinator"
    go build -o "$workDir\bin\vaultctl.exe" "$workDir\cmd\vaultctl"
}

# 4. Start etcd
Write-Host "Starting etcd consensus store (:2379)..." -ForegroundColor Green
$etcdData = "$workDir\data\etcd"
New-Item -ItemType Directory -Force -Path $etcdData | Out-Null
$etcdProc = Start-Process -FilePath "$toolsDir\etcd\etcd.exe" -ArgumentList "--data-dir=$etcdData", "--listen-client-urls=http://127.0.0.1:2379", "--advertise-client-urls=http://127.0.0.1:2379" -WorkingDirectory $workDir -PassThru -WindowStyle Hidden

Start-Sleep -Seconds 2

# 5. Start metadata
Write-Host "Starting Metadata service (:50050 gRPC / :8080 HTTP)..." -ForegroundColor Green
$metaProc = Start-Process -FilePath "$workDir\bin\metadata.exe" -ArgumentList "-addr=:50050", "-http=8080", "-etcd=127.0.0.1:2379" -WorkingDirectory $workDir -PassThru -WindowStyle Hidden

Start-Sleep -Seconds 1

# 6. Start Storage Nodes
Write-Host "Starting 3 Storage Engine nodes (:50051 - :50053)..." -ForegroundColor Green
$s1Proc = Start-Process -FilePath "$workDir\bin\storage-node.exe" -ArgumentList "-id=storage-01", "-addr=:50051", "-http=8081", "-data=$workDir\data\storage-01" -WorkingDirectory $workDir -PassThru -WindowStyle Hidden
$s2Proc = Start-Process -FilePath "$workDir\bin\storage-node.exe" -ArgumentList "-id=storage-02", "-addr=:50052", "-http=8082", "-data=$workDir\data\storage-02" -WorkingDirectory $workDir -PassThru -WindowStyle Hidden
$s3Proc = Start-Process -FilePath "$workDir\bin\storage-node.exe" -ArgumentList "-id=storage-03", "-addr=:50053", "-http=8083", "-data=$workDir\data\storage-03" -WorkingDirectory $workDir -PassThru -WindowStyle Hidden

Start-Sleep -Seconds 1

# 7. Start Coordinator (with REST & Web UI on :8085)
Write-Host "Starting Coordinator Gateway & Web Console (:8085)..." -ForegroundColor Green
$sNodes = "storage-01=127.0.0.1:50051,storage-02=127.0.0.1:50052,storage-03=127.0.0.1:50053"
$coordProc = Start-Process -FilePath "$workDir\bin\coordinator.exe" -ArgumentList "-addr=:50055", "-http=8085", "-metadata=127.0.0.1:50050", "-storage-nodes=$sNodes" -WorkingDirectory $workDir -PassThru -WindowStyle Hidden

Start-Sleep -Seconds 2

# Save PIDs
"$($etcdProc.Id),$($metaProc.Id),$($s1Proc.Id),$($s2Proc.Id),$($s3Proc.Id),$($coordProc.Id)" | Out-File -FilePath "$workDir\cluster_pids.txt" -Encoding utf8

Write-Host "`nVault Cluster is active and operational!" -ForegroundColor Cyan
Write-Host "Web Console: http://localhost:8085" -ForegroundColor Yellow
Write-Host "Coordinator gRPC: 127.0.0.1:50055" -ForegroundColor DarkCyan
Write-Host "Metadata Service: 127.0.0.1:50050" -ForegroundColor DarkCyan

# Launch Browser
Start-Process "http://localhost:8085"

Write-Host "`nPress Enter or Ctrl+C to gracefully stop the cluster..." -ForegroundColor Gray
[Console]::ReadLine() | Out-Null

Write-Host "`nShutting down cluster processes..." -ForegroundColor DarkYellow
$procs = @($coordProc, $s3Proc, $s2Proc, $s1Proc, $metaProc, $etcdProc)
foreach ($p in $procs) {
    if ($p -and -not $p.HasExited) {
        Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
    }
}
Remove-Item "$workDir\cluster_pids.txt" -Force -ErrorAction SilentlyContinue
Write-Host "Cluster stopped successfully." -ForegroundColor Green
