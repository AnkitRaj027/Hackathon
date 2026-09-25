$workDir = "C:\Users\ankit\OneDrive\Desktop\hackathon"
$toolsDir = "C:\Users\ankit\.vault-tools"
$env:PATH = "$toolsDir\go\bin;$toolsDir\protoc\bin;$env:PATH"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "         STARTING VAULT DISTRIBUTED CLUSTER               " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. Start etcd
New-Item -ItemType Directory -Force -Path "$workDir\data\etcd" | Out-Null
$etcd = Start-Process -FilePath "$toolsDir\etcd\etcd.exe" -ArgumentList "--data-dir=$workDir\data\etcd", "--listen-client-urls=http://127.0.0.1:2379", "--advertise-client-urls=http://127.0.0.1:2379" -PassThru -WindowStyle Hidden

Start-Sleep -Seconds 2

# 2. Start metadata
$env:VAULT_METADATA_GRPC_ADDR = ":50050"
$env:VAULT_METADATA_HTTP_PORT = "8080"
$env:VAULT_ETCD_ENDPOINTS = "127.0.0.1:2379"
$meta = Start-Process -FilePath "$workDir\bin\metadata.exe" -PassThru -WindowStyle Hidden

# 3. Start storage nodes
$env:VAULT_DATA_DIR = "$workDir\data\storage-01"
$env:VAULT_NODE_ID = "storage-01"
$env:VAULT_GRPC_ADDR = ":50051"
$env:VAULT_HTTP_PORT = "8081"
$s1 = Start-Process -FilePath "$workDir\bin\storage-node.exe" -PassThru -WindowStyle Hidden

$env:VAULT_DATA_DIR = "$workDir\data\storage-02"
$env:VAULT_NODE_ID = "storage-02"
$env:VAULT_GRPC_ADDR = ":50052"
$env:VAULT_HTTP_PORT = "8082"
$s2 = Start-Process -FilePath "$workDir\bin\storage-node.exe" -PassThru -WindowStyle Hidden

$env:VAULT_DATA_DIR = "$workDir\data\storage-03"
$env:VAULT_NODE_ID = "storage-03"
$env:VAULT_GRPC_ADDR = ":50053"
$env:VAULT_HTTP_PORT = "8083"
$s3 = Start-Process -FilePath "$workDir\bin\storage-node.exe" -PassThru -WindowStyle Hidden

Start-Sleep -Seconds 1

# 4. Start coordinator
$env:VAULT_COORDINATOR_GRPC_ADDR = ":50055"
$env:VAULT_COORDINATOR_HTTP_PORT = "8085"
$env:VAULT_METADATA_ADDR = "127.0.0.1:50050"
$env:VAULT_STORAGE_NODES = "storage-01=127.0.0.1:50051,storage-02=127.0.0.1:50052,storage-03=127.0.0.1:50053"
$env:VAULT_CHUNK_SIZE = "4194304"
$env:VAULT_REPLICATION_FACTOR = "3"
$env:VAULT_WRITE_QUORUM = "3"
$env:VAULT_READ_QUORUM = "1"
$coord = Start-Process -FilePath "$workDir\bin\coordinator.exe" -PassThru -WindowStyle Hidden

Start-Sleep -Seconds 2

$env:VAULT_COORDINATOR_ADDR = "127.0.0.1:50055"

try {
    Write-Host "`n>>> 1. CHECK CLUSTER STATUS (vaultctl nodes):" -ForegroundColor Yellow
    & "$workDir\bin\vaultctl.exe" nodes

    Write-Host "`n>>> 2. PREPARE TEST PAYLOAD:" -ForegroundColor Yellow
    $testFile = "$workDir\test-data\demo_file.txt"
    New-Item -ItemType Directory -Force -Path "$workDir\test-data" | Out-Null
    Set-Content -Path $testFile -Value "Vault Distributed Storage Engine: Demonstrating correctness under failure with RF=3 replication, SHA-256 sidecars, and etcd Raft consensus." -NoNewline
    Get-Item $testFile | Select-Object Name, Length, LastWriteTime | Format-Table -AutoSize

    Write-Host "`n>>> 3. UPLOAD OBJECT (vaultctl put):" -ForegroundColor Yellow
    & "$workDir\bin\vaultctl.exe" put $testFile "docs/demo.txt"

    Write-Host "`n>>> 4. INSPECT OBJECT METADATA & REPLICA PLACEMENT (vaultctl inspect):" -ForegroundColor Yellow
    & "$workDir\bin\vaultctl.exe" inspect "docs/demo.txt"

    Write-Host "`n>>> 5. VERIFY PHYSICAL REPLICA FILES & CHECKSUM SIDECARS ON DISK:" -ForegroundColor Yellow
    Get-ChildItem -Path "$workDir\data\storage-01", "$workDir\data\storage-02", "$workDir\data\storage-03" -Recurse | Select-Object Directory, Name, Length | Format-Table -AutoSize

    Write-Host "`n>>> 6. DOWNLOAD OBJECT (vaultctl get):" -ForegroundColor Yellow
    $destFile = "$workDir\test-data\downloaded_demo.txt"
    & "$workDir\bin\vaultctl.exe" get "docs/demo.txt" $destFile

    Write-Host "`n>>> 7. COMPARE SOURCE AND DOWNLOADED CHECKSUMS (SHA-256):" -ForegroundColor Yellow
    Get-FileHash -Algorithm SHA256 $testFile, $destFile | Format-Table -AutoSize

    Write-Host "`n>>> 8. DELETE OBJECT (vaultctl delete):" -ForegroundColor Yellow
    & "$workDir\bin\vaultctl.exe" delete "docs/demo.txt"

    Write-Host "`n>>> 9. VERIFY PHYSICAL DELETION ACROSS DISK VOLUMES:" -ForegroundColor Yellow
    $remaining = Get-ChildItem -Path "$workDir\data\storage-01", "$workDir\data\storage-02", "$workDir\data\storage-03" -File -Recurse
    if ($remaining.Count -eq 0) {
        Write-Host "Success: All chunk and sidecar files have been cleanly deleted across all storage nodes." -ForegroundColor Green
    } else {
        $remaining | Format-Table -AutoSize
    }

} finally {
    Write-Host "`n==========================================================" -ForegroundColor Cyan
    Write-Host "         STOPPING VAULT CLUSTER PROCESSES                 " -ForegroundColor Cyan
    Write-Host "==========================================================" -ForegroundColor Cyan
    Stop-Process -Id $coord.Id -Force -ErrorAction SilentlyContinue
    Stop-Process -Id $s3.Id -Force -ErrorAction SilentlyContinue
    Stop-Process -Id $s2.Id -Force -ErrorAction SilentlyContinue
    Stop-Process -Id $s1.Id -Force -ErrorAction SilentlyContinue
    Stop-Process -Id $meta.Id -Force -ErrorAction SilentlyContinue
    Stop-Process -Id $etcd.Id -Force -ErrorAction SilentlyContinue
    Write-Host "All cluster processes stopped cleanly.`n" -ForegroundColor Green
}
