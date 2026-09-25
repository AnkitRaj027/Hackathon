$toolsDir = "C:\Users\ankit\.vault-tools"
$workDir = "C:\Users\ankit\OneDrive\Desktop\hackathon"

# 1. Start etcd
$etcdData = "$workDir\data\etcd"
New-Item -ItemType Directory -Force -Path $etcdData | Out-Null
$etcdProc = Start-Process -FilePath "$toolsDir\etcd\etcd.exe" -ArgumentList "--data-dir=$etcdData", "--listen-client-urls=http://127.0.0.1:2379", "--advertise-client-urls=http://127.0.0.1:2379" -PassThru -WindowStyle Hidden

Start-Sleep -Seconds 2

# 2. Start metadata
$env:VAULT_METADATA_GRPC_ADDR = "127.0.0.1:50050"
$env:VAULT_METADATA_HTTP_PORT = "8080"
$env:VAULT_ETCD_ENDPOINTS = "127.0.0.1:2379"
$metaProc = Start-Process -FilePath "$workDir\bin\metadata.exe" -PassThru -WindowStyle Hidden

# 3. Start storage-01
$env:VAULT_NODE_ID = "storage-01"
$env:VAULT_DATA_DIR = "$workDir\data\storage-01"
$env:VAULT_GRPC_ADDR = "127.0.0.1:50051"
$env:VAULT_HTTP_PORT = "8081"
$s1Proc = Start-Process -FilePath "$workDir\bin\storage-node.exe" -PassThru -WindowStyle Hidden

# 4. Start storage-02
$env:VAULT_NODE_ID = "storage-02"
$env:VAULT_DATA_DIR = "$workDir\data\storage-02"
$env:VAULT_GRPC_ADDR = "127.0.0.1:50052"
$env:VAULT_HTTP_PORT = "8082"
$s2Proc = Start-Process -FilePath "$workDir\bin\storage-node.exe" -PassThru -WindowStyle Hidden

# 5. Start storage-03
$env:VAULT_NODE_ID = "storage-03"
$env:VAULT_DATA_DIR = "$workDir\data\storage-03"
$env:VAULT_GRPC_ADDR = "127.0.0.1:50053"
$env:VAULT_HTTP_PORT = "8083"
$s3Proc = Start-Process -FilePath "$workDir\bin\storage-node.exe" -PassThru -WindowStyle Hidden

Start-Sleep -Seconds 1

# 6. Start coordinator
$env:VAULT_COORDINATOR_GRPC_ADDR = "127.0.0.1:50055"
$env:VAULT_COORDINATOR_HTTP_PORT = "8085"
$env:VAULT_METADATA_ADDR = "127.0.0.1:50050"
$env:VAULT_STORAGE_NODES = "storage-01=127.0.0.1:50051,storage-02=127.0.0.1:50052,storage-03=127.0.0.1:50053"
$env:VAULT_CHUNK_SIZE = "4194304"
$env:VAULT_REPLICATION_FACTOR = "3"
$env:VAULT_WRITE_QUORUM = "3"
$env:VAULT_READ_QUORUM = "1"
$coordProc = Start-Process -FilePath "$workDir\bin\coordinator.exe" -PassThru -WindowStyle Hidden

Start-Sleep -Seconds 2

# Output PIDs to file for cleanup
"$($etcdProc.Id),$($metaProc.Id),$($s1Proc.Id),$($s2Proc.Id),$($s3Proc.Id),$($coordProc.Id)" | Out-File -FilePath "$workDir\cluster_pids.txt" -Encoding utf8

Write-Host "Cluster started successfully. PIDs: $($etcdProc.Id), $($metaProc.Id), $($s1Proc.Id), $($s2Proc.Id), $($s3Proc.Id), $($coordProc.Id)"
