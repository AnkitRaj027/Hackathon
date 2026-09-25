$toolsDir = "C:\Users\ankit\.vault-tools"
$workDir = "C:\Users\ankit\OneDrive\Desktop\hackathon"
$env:PATH = "$toolsDir\go\bin;$toolsDir\etcd;$env:PATH"

# Stop any previous cluster
if (Test-Path "$workDir\cluster_pids.txt") {
    $pids = (Get-Content "$workDir\cluster_pids.txt").Split(',')
    foreach ($p in $pids) {
        if ($p -match '^\d+$') {
            Stop-Process -Id ([int]$p) -Force -ErrorAction SilentlyContinue
        }
    }
    Remove-Item "$workDir\cluster_pids.txt" -Force -ErrorAction SilentlyContinue
}

New-Item -ItemType Directory -Force -Path "$workDir\logs" | Out-Null

# 1. Start etcd
$etcdData = "$workDir\data\etcd"
New-Item -ItemType Directory -Force -Path $etcdData | Out-Null
$etcdProc = Start-Process -FilePath "$toolsDir\etcd\etcd.exe" `
    -ArgumentList "--data-dir=$etcdData", "--listen-client-urls=http://127.0.0.1:2379", "--advertise-client-urls=http://127.0.0.1:2379" `
    -RedirectStandardOutput "$workDir\logs\etcd.log" -RedirectStandardError "$workDir\logs\etcd_err.log" `
    -WorkingDirectory $workDir -PassThru

Start-Sleep -Seconds 2

# 2. Start metadata
$metaProc = Start-Process -FilePath "$workDir\bin\metadata.exe" `
    -ArgumentList "-addr=:50050", "-http=8080", "-etcd=127.0.0.1:2379" `
    -RedirectStandardOutput "$workDir\logs\metadata.log" -RedirectStandardError "$workDir\logs\metadata_err.log" `
    -WorkingDirectory $workDir -PassThru

Start-Sleep -Seconds 1

# 3. Start storage-01
$s1Proc = Start-Process -FilePath "$workDir\bin\storage-node.exe" `
    -ArgumentList "-id=storage-01", "-addr=:50051", "-http=8081", "-data=$workDir\data\storage-01" `
    -RedirectStandardOutput "$workDir\logs\s1.log" -RedirectStandardError "$workDir\logs\s1_err.log" `
    -WorkingDirectory $workDir -PassThru

# 4. Start storage-02
$s2Proc = Start-Process -FilePath "$workDir\bin\storage-node.exe" `
    -ArgumentList "-id=storage-02", "-addr=:50052", "-http=8082", "-data=$workDir\data\storage-02" `
    -RedirectStandardOutput "$workDir\logs\s2.log" -RedirectStandardError "$workDir\logs\s2_err.log" `
    -WorkingDirectory $workDir -PassThru

# 5. Start storage-03
$s3Proc = Start-Process -FilePath "$workDir\bin\storage-node.exe" `
    -ArgumentList "-id=storage-03", "-addr=:50053", "-http=8083", "-data=$workDir\data\storage-03" `
    -RedirectStandardOutput "$workDir\logs\s3.log" -RedirectStandardError "$workDir\logs\s3_err.log" `
    -WorkingDirectory $workDir -PassThru

Start-Sleep -Seconds 1

# 6. Start coordinator
$sNodes = "storage-01=127.0.0.1:50051,storage-02=127.0.0.1:50052,storage-03=127.0.0.1:50053"
$coordProc = Start-Process -FilePath "$workDir\bin\coordinator.exe" `
    -ArgumentList "-addr=:50055", "-http=8085", "-metadata=127.0.0.1:50050", "-storage-nodes=$sNodes" `
    -RedirectStandardOutput "$workDir\logs\coord.log" -RedirectStandardError "$workDir\logs\coord_err.log" `
    -WorkingDirectory $workDir -PassThru

Start-Sleep -Seconds 2

# Output PIDs to file for cleanup
"$($etcdProc.Id),$($metaProc.Id),$($s1Proc.Id),$($s2Proc.Id),$($s3Proc.Id),$($coordProc.Id)" | Out-File -FilePath "$workDir\cluster_pids.txt" -Encoding utf8

Write-Host "Cluster started successfully. PIDs: $($etcdProc.Id), $($metaProc.Id), $($s1Proc.Id), $($s2Proc.Id), $($s3Proc.Id), $($coordProc.Id)"

# Keep alive to maintain processes
try {
    while ($true) {
        if ($coordProc.HasExited) {
            Write-Host "Coordinator exited with code $($coordProc.ExitCode)"
            break
        }
        Start-Sleep -Seconds 2
    }
} finally {
    Write-Host "Shutting down cluster processes..."
    $pids = @($coordProc.Id, $s3Proc.Id, $s2Proc.Id, $s1Proc.Id, $metaProc.Id, $etcdProc.Id)
    foreach ($p in $pids) {
        Stop-Process -Id $p -Force -ErrorAction SilentlyContinue
    }
    Remove-Item "$workDir\cluster_pids.txt" -Force -ErrorAction SilentlyContinue
}
