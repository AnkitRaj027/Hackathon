$workDir = "C:\Users\ankit\OneDrive\Desktop\hackathon"
if (Test-Path "$workDir\cluster_pids.txt") {
    $pids = (Get-Content "$workDir\cluster_pids.txt").Split(',')
    foreach ($p in $pids) {
        if ($p -match '^\d+$') {
            Stop-Process -Id ([int]$p) -Force -ErrorAction SilentlyContinue
        }
    }
    Remove-Item "$workDir\cluster_pids.txt" -Force -ErrorAction SilentlyContinue
    Write-Host "Cluster stopped."
}
