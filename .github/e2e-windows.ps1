# Connects sshr to the OpenSSH Server of the runner on 127.0.0.1:22, once
# directly and once through a saved jump host, which runs sshr itself as
# ProxyCommand
$ErrorActionPreference = 'Stop'
$sshr = Join-Path $PWD 'cli\sshr.exe'
$w = Join-Path $env:RUNNER_TEMP 'sshr-e2e-win'
New-Item -ItemType Directory -Force -Path $w | Out-Null

# the capability may need installing first, the service answers on 22
$cap = Get-WindowsCapability -Online | Where-Object Name -like 'OpenSSH.Server*'
if ($cap.State -ne 'Installed') {
    Add-WindowsCapability -Online -Name $cap.Name | Out-Null
}
Start-Service sshd

# the runner's user is an administrator, so sshd reads its key from the
# administrators file, and its ACL must admit only system and admins
ssh-keygen -q -t ed25519 -N '""' -f "$w\userkey"
$aak = Join-Path $env:ProgramData 'ssh\administrators_authorized_keys'
Get-Content "$w\userkey.pub" | Add-Content $aak
icacls $aak /inheritance:r /grant '*S-1-5-32-544:F' /grant '*S-1-5-18:F' | Out-Null

# seed known_hosts so ssh never prompts for the host key
$known = Join-Path $env:USERPROFILE '.ssh\known_hosts'
New-Item -ItemType Directory -Force -Path (Split-Path $known) | Out-Null
foreach ($i in 1..10) {
    ssh-keyscan -t ed25519 127.0.0.1 2>$null | Out-File -Encoding ascii $known
    if (Select-String -Path $known -Pattern 'ssh-ed25519' -Quiet) { break }
    Start-Sleep 1
}

function Invoke-Sshr {
    & $sshr @args
    if ($LASTEXITCODE -ne 0) { throw "sshr $($args -join ' ') exited $LASTEXITCODE" }
}

Invoke-Sshr add 127.0.0.1 -u $env:USERNAME -n winbase -p 22 -k "$w\userkey"
Invoke-Sshr add 127.0.0.1 -u $env:USERNAME -n winjump -p 22 -k "$w\userkey" -J winbase

Invoke-Sshr run winbase -- 'echo direct-ok'
$out = & $sshr run winjump -- 'echo via-jump-ok'
if ($LASTEXITCODE -ne 0) { throw "sshr run winjump exited $LASTEXITCODE" }
if ("$out" -notmatch 'via-jump-ok') { throw "the command through the jump host printed: $out" }

Write-Output 'windows jump checks passed'
