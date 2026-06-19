param(
    [Parameter(Mandatory=$true)][string]$InstallRoot,
    [Parameter(Mandatory=$true)][string]$NotifierExe,
    [switch]$SelfTest
)

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$ErrorActionPreference = "Stop"

$configDir = Join-Path $env:USERPROFILE ".claude\claude-notifications-go"
$configPath = Join-Path $configDir "config.json"
$claudeSettingsPath = Join-Path $env:USERPROFILE ".claude\settings.json"
$codexHooksPath = Join-Path $env:USERPROFILE ".codex\hooks.json"

$statuses = @(
    "task_complete",
    "review_complete",
    "question",
    "plan_ready",
    "session_limit_reached",
    "api_error",
    "api_error_overloaded"
)

function New-DefaultConfig {
    $soundsDir = Join-Path $InstallRoot "sounds"
    return [ordered]@{
        notifications = [ordered]@{
            desktop = [ordered]@{
                enabled = $true
                sound = $true
                volume = 1.0
                appIcon = (Join-Path $InstallRoot "claude_icon.png")
                clickToFocus = $true
            }
            webhook = [ordered]@{
                enabled = $false
                preset = "custom"
                url = ""
                chat_id = ""
                format = "json"
                headers = @{}
                payloadFields = @{}
                retry = [ordered]@{
                    enabled = $true
                    maxAttempts = 3
                    initialBackoff = "1s"
                    maxBackoff = "10s"
                }
                circuitBreaker = [ordered]@{
                    enabled = $true
                    failureThreshold = 5
                    timeout = "30s"
                    successThreshold = 2
                }
                rateLimit = [ordered]@{
                    enabled = $true
                    requestsPerMinute = 10
                }
            }
            feishu = [ordered]@{}
            suppressQuestionAfterTaskCompleteSeconds = 12
            suppressQuestionAfterAnyNotificationSeconds = 7
            notifyOnSubagentStop = $false
        }
        statuses = [ordered]@{
            task_complete = [ordered]@{ title = "Completed"; sound = (Join-Path $soundsDir "task-complete.mp3") }
            review_complete = [ordered]@{ title = "Review"; sound = (Join-Path $soundsDir "review-complete.mp3") }
            question = [ordered]@{ title = "Question"; sound = (Join-Path $soundsDir "question.mp3") }
            plan_ready = [ordered]@{ title = "Plan"; sound = (Join-Path $soundsDir "plan-ready.mp3") }
            session_limit_reached = [ordered]@{ title = "Session Limit Reached"; sound = (Join-Path $soundsDir "error.mp3") }
            api_error = [ordered]@{ title = "API Error: 401"; sound = (Join-Path $soundsDir "error.mp3") }
            api_error_overloaded = [ordered]@{ title = "API Error"; sound = (Join-Path $soundsDir "error.mp3") }
        }
    }
}

function Ensure-Object($obj, [string]$name, $value) {
    if ($null -eq $obj.$name) {
        $obj | Add-Member -NotePropertyName $name -NotePropertyValue $value
    }
}

function Load-Config {
    if (Test-Path $configPath) {
        try {
            $cfg = Get-Content -Encoding UTF8 $configPath -Raw | ConvertFrom-Json
        } catch {
            $cfg = New-DefaultConfig | ConvertTo-Json -Depth 20 | ConvertFrom-Json
        }
    } else {
        $cfg = New-DefaultConfig | ConvertTo-Json -Depth 20 | ConvertFrom-Json
    }
    Ensure-Object $cfg "notifications" ([pscustomobject]@{})
    Ensure-Object $cfg.notifications "desktop" ([pscustomobject]@{})
    Ensure-Object $cfg.notifications "webhook" ([pscustomobject]@{})
    Ensure-Object $cfg.notifications "feishu" ([pscustomobject]@{})
    Ensure-Object $cfg "statuses" ([pscustomobject]@{})

    $defaults = New-DefaultConfig | ConvertTo-Json -Depth 20 | ConvertFrom-Json
    foreach ($status in $statuses) {
        if ($null -eq $cfg.statuses.$status) {
            $cfg.statuses | Add-Member -NotePropertyName $status -NotePropertyValue $defaults.statuses.$status
        }
    }
    return $cfg
}

function Save-Config($cfg) {
    New-Item -ItemType Directory -Force $configDir | Out-Null
    $json = $cfg | ConvertTo-Json -Depth 30
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($configPath, $json + [Environment]::NewLine, $utf8NoBom)
}

function Set-Prop($obj, [string]$name, $value) {
    if ($null -eq $obj.PSObject.Properties[$name]) {
        $obj | Add-Member -NotePropertyName $name -NotePropertyValue $value
    } else {
        $obj.$name = $value
    }
}

function Set-Channel($statusObj, [string]$channel, [bool]$enabled) {
    if ($null -eq $statusObj.$channel) {
        $statusObj | Add-Member -NotePropertyName $channel -NotePropertyValue ([pscustomobject]@{ enabled = $enabled })
    } elseif ($null -eq $statusObj.$channel.PSObject.Properties["enabled"]) {
        $statusObj.$channel | Add-Member -NotePropertyName "enabled" -NotePropertyValue $enabled
    } else {
        $statusObj.$channel.enabled = $enabled
    }
}

function Is-Enabled($value, [bool]$default) {
    if ($null -eq $value) { return $default }
    return [bool]$value
}

function Has-FeishuAppBinding($cfg) {
    return $cfg.notifications.webhook.preset -eq "feishu_app" -and
        -not [string]::IsNullOrWhiteSpace($cfg.notifications.feishu.app_id) -and
        (-not [string]::IsNullOrWhiteSpace($cfg.notifications.feishu.app_secret) -or -not [string]::IsNullOrWhiteSpace($cfg.notifications.feishu.app_secret_env)) -and
        -not [string]::IsNullOrWhiteSpace($cfg.notifications.feishu.receive_id_type) -and
        -not [string]::IsNullOrWhiteSpace($cfg.notifications.feishu.receive_id)
}

function Has-CustomFeishuWebhook($cfg) {
    return $cfg.notifications.webhook.preset -eq "lark" -and
        -not [string]::IsNullOrWhiteSpace($cfg.notifications.webhook.url)
}

function New-HookCommand([string]$eventName, [bool]$codex) {
    if ($codex) {
        return [ordered]@{
            type = "command"
            command = "`"$NotifierExe`" handle-codex-hook $eventName"
            timeout = 30
            statusMessage = "Sending Codex notification"
        }
    }
    return [ordered]@{
        type = "command"
        command = $NotifierExe
        args = @("handle-hook", $eventName)
        timeout = 30
    }
}

function Is-ManagedGroup($group) {
    foreach ($hook in @($group.hooks)) {
        if (($hook.command -as [string]) -like "*claude-notifications*" -or ($hook.command -as [string]) -like "*notification-installer*") {
            return $true
        }
        foreach ($arg in @($hook.args)) {
            if (($arg -as [string]) -like "*handle-hook*" -or ($arg -as [string]) -like "*handle-codex-hook*") {
                return $true
            }
        }
    }
    return $false
}

function Read-HookFile([string]$path) {
    if (Test-Path $path) {
        try {
            return Get-Content -Encoding UTF8 $path -Raw | ConvertFrom-Json
        } catch {
            throw "Cannot parse hook file: $path"
        }
    }
    return ([pscustomobject]@{ hooks = [pscustomobject]@{} })
}

function Write-HookFile([string]$path, $settings) {
    New-Item -ItemType Directory -Force (Split-Path -Parent $path) | Out-Null
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($path, (($settings | ConvertTo-Json -Depth 30) + [Environment]::NewLine), $utf8NoBom)
}

function Merge-Hooks($settings, [hashtable]$generated) {
    if ($null -eq $settings.hooks) {
        $settings | Add-Member -NotePropertyName "hooks" -NotePropertyValue ([pscustomobject]@{})
    }
    foreach ($name in $generated.Keys) {
        $kept = @()
        if ($null -ne $settings.hooks.$name) {
            foreach ($group in @($settings.hooks.$name)) {
                if (-not (Is-ManagedGroup $group)) {
                    $kept += $group
                }
            }
        }
        $merged = @($kept) + @($generated[$name])
        if ($null -eq $settings.hooks.PSObject.Properties[$name]) {
            $settings.hooks | Add-Member -NotePropertyName $name -NotePropertyValue $merged
        } else {
            $settings.hooks.$name = $merged
        }
    }
    return $settings
}

function Install-ClaudeHooks {
    $generated = @{
        PreToolUse = @([ordered]@{ matcher = "ExitPlanMode|AskUserQuestion"; hooks = @((New-HookCommand "PreToolUse" $false)) })
        Notification = @([ordered]@{ matcher = "permission_prompt"; hooks = @((New-HookCommand "Notification" $false)) })
        Stop = @([ordered]@{ hooks = @((New-HookCommand "Stop" $false)) })
        SubagentStop = @([ordered]@{ hooks = @((New-HookCommand "SubagentStop" $false)) })
        TeammateIdle = @([ordered]@{ hooks = @((New-HookCommand "TeammateIdle" $false)) })
    }
    $settings = Read-HookFile $claudeSettingsPath
    Write-HookFile $claudeSettingsPath (Merge-Hooks $settings $generated)
}

function Install-CodexHooks {
    $generated = @{
        Stop = @([ordered]@{ hooks = @((New-HookCommand "Stop" $true)) })
        PermissionRequest = @([ordered]@{ hooks = @((New-HookCommand "PermissionRequest" $true)) })
        SubagentStop = @([ordered]@{ hooks = @((New-HookCommand "SubagentStop" $true)) })
    }
    $settings = Read-HookFile $codexHooksPath
    Write-HookFile $codexHooksPath (Merge-Hooks $settings $generated)
}

function Run-Notifier([string[]]$notifierArgs) {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $NotifierExe
    $psi.UseShellExecute = $false
    $psi.RedirectStandardError = $true
    $psi.RedirectStandardOutput = $true
    $psi.Arguments = (($notifierArgs | ForEach-Object { Quote-Arg $_ }) -join " ")
    $p = [System.Diagnostics.Process]::Start($psi)
    $stdout = $p.StandardOutput.ReadToEnd()
    $stderr = $p.StandardError.ReadToEnd()
    $p.WaitForExit()
    if ($p.ExitCode -ne 0) {
        throw (($stderr + "`r`n" + $stdout).Trim())
    }
    return (($stdout + "`r`n" + $stderr).Trim())
}

function Quote-Arg([string]$value) {
    if ($null -eq $value) { return '""' }
    if ($value -notmatch '[\s"]') { return $value }
    return '"' + ($value -replace '"', '\"') + '"'
}

function Run-TestNotification {
    $testMessage = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String("6aOe5Lmm5Lit5paH6YCa55+l5rWL6K+V44CC5pS25Yiw6L+Z5Y+l6K+d6K+05piOIFVURi04IOe8lueggeato+W4uO+8jOmjnuS5puS8muaYvuekuuWujOaVtOaooeWei+WbnuWkjeOAgg=="))
    $payload = @{
        session_id = "installer-test"
        turn_id = "turn1"
        cwd = $InstallRoot
        hook_event_name = "Stop"
        model = "installer"
        permission_mode = "never"
        last_assistant_message = $testMessage
    } | ConvertTo-Json -Compress
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $NotifierExe
    $psi.Arguments = "handle-codex-hook Stop"
    $psi.UseShellExecute = $false
    $psi.RedirectStandardInput = $true
    $psi.RedirectStandardError = $true
    $psi.RedirectStandardOutput = $true
    $p = [System.Diagnostics.Process]::Start($psi)
    $payloadBytes = [System.Text.Encoding]::UTF8.GetBytes($payload + [Environment]::NewLine)
    $p.StandardInput.BaseStream.Write($payloadBytes, 0, $payloadBytes.Length)
    $p.StandardInput.BaseStream.Flush()
    $p.StandardInput.Close()
    $stdout = $p.StandardOutput.ReadToEnd()
    $stderr = $p.StandardError.ReadToEnd()
    $p.WaitForExit()
    if ($p.ExitCode -ne 0) {
        throw "Test failed with exit code $($p.ExitCode). $stderr $stdout"
    }
}

function Start-Notifier([string[]]$notifierArgs) {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $NotifierExe
    $psi.UseShellExecute = $false
    $psi.RedirectStandardError = $true
    $psi.RedirectStandardOutput = $true
    $psi.Arguments = (($notifierArgs | ForEach-Object { Quote-Arg $_ }) -join " ")
    return [System.Diagnostics.Process]::Start($psi)
}

if ($SelfTest) {
    Run-Notifier @("version") | Out-Null
    Write-Output "GUI script self-test OK"
    exit 0
}

$cfg = Load-Config

$form = New-Object System.Windows.Forms.Form
$form.Text = "Claude/Codex Notifications"
$form.Size = New-Object System.Drawing.Size(720, 720)
$form.StartPosition = "CenterScreen"
$form.FormBorderStyle = "FixedDialog"
$form.MaximizeBox = $false

$tabs = New-Object System.Windows.Forms.TabControl
$tabs.Location = New-Object System.Drawing.Point(12, 12)
$tabs.Size = New-Object System.Drawing.Size(680, 610)
$form.Controls.Add($tabs)

$tabGeneral = New-Object System.Windows.Forms.TabPage
$tabGeneral.Text = "General"
$tabs.TabPages.Add($tabGeneral)

$desktopBox = New-Object System.Windows.Forms.CheckBox
$desktopBox.Text = "Desktop notifications"
$desktopBox.Location = New-Object System.Drawing.Point(24, 24)
$desktopBox.Size = New-Object System.Drawing.Size(260, 28)
$desktopBox.Checked = Is-Enabled $cfg.notifications.desktop.enabled $true
$tabGeneral.Controls.Add($desktopBox)

$soundBox = New-Object System.Windows.Forms.CheckBox
$soundBox.Text = "Sound"
$soundBox.Location = New-Object System.Drawing.Point(24, 58)
$soundBox.Size = New-Object System.Drawing.Size(260, 28)
$soundBox.Checked = Is-Enabled $cfg.notifications.desktop.sound $true
$tabGeneral.Controls.Add($soundBox)

$webhookBox = New-Object System.Windows.Forms.CheckBox
$webhookBox.Text = "Feishu/Lark mobile notifications"
$webhookBox.Location = New-Object System.Drawing.Point(24, 92)
$webhookBox.Size = New-Object System.Drawing.Size(310, 28)
$webhookBox.Checked = Is-Enabled $cfg.notifications.webhook.enabled $false
$tabGeneral.Controls.Add($webhookBox)

$claudeBox = New-Object System.Windows.Forms.CheckBox
$claudeBox.Text = "Install Claude Code hooks"
$claudeBox.Location = New-Object System.Drawing.Point(24, 140)
$claudeBox.Size = New-Object System.Drawing.Size(260, 28)
$claudeBox.Checked = $true
$tabGeneral.Controls.Add($claudeBox)

$codexBox = New-Object System.Windows.Forms.CheckBox
$codexBox.Text = "Install Codex hooks"
$codexBox.Location = New-Object System.Drawing.Point(24, 174)
$codexBox.Size = New-Object System.Drawing.Size(260, 28)
$codexBox.Checked = $true
$tabGeneral.Controls.Add($codexBox)

$pathLabel = New-Object System.Windows.Forms.Label
$pathLabel.Location = New-Object System.Drawing.Point(24, 230)
$pathLabel.Size = New-Object System.Drawing.Size(620, 90)
$pathLabel.Text = "Install root:`r`n$InstallRoot`r`n`r`nHooks point to:`r`n$NotifierExe"
$tabGeneral.Controls.Add($pathLabel)

$tabStatuses = New-Object System.Windows.Forms.TabPage
$tabStatuses.Text = "Statuses"
$tabs.TabPages.Add($tabStatuses)

$statusControls = @{}
$y = 20
foreach ($status in $statuses) {
    $statusObj = $cfg.statuses.$status
    $enabled = New-Object System.Windows.Forms.CheckBox
    $enabled.Text = $status
    $enabled.Location = New-Object System.Drawing.Point(20, $y)
    $enabled.Size = New-Object System.Drawing.Size(210, 24)
    $enabled.Checked = Is-Enabled $statusObj.enabled $true
    $tabStatuses.Controls.Add($enabled)

    $desk = New-Object System.Windows.Forms.CheckBox
    $desk.Text = "Desktop"
    $desk.Location = New-Object System.Drawing.Point(260, $y)
    $desk.Size = New-Object System.Drawing.Size(100, 24)
    $desk.Checked = Is-Enabled $statusObj.desktop.enabled $true
    $tabStatuses.Controls.Add($desk)

    $web = New-Object System.Windows.Forms.CheckBox
    $web.Text = "Webhook"
    $web.Location = New-Object System.Drawing.Point(390, $y)
    $web.Size = New-Object System.Drawing.Size(100, 24)
    $web.Checked = Is-Enabled $statusObj.webhook.enabled $true
    $tabStatuses.Controls.Add($web)

    $statusControls[$status] = @{ enabled = $enabled; desktop = $desk; webhook = $web }
    $y += 38
}

$tabFeishu = New-Object System.Windows.Forms.TabPage
$tabFeishu.Text = "Feishu"
$tabs.TabPages.Add($tabFeishu)

$feishuStatus = New-Object System.Windows.Forms.Label
$feishuStatus.Location = New-Object System.Drawing.Point(24, 24)
$feishuStatus.Size = New-Object System.Drawing.Size(620, 86)
$tabFeishu.Controls.Add($feishuStatus)

$qrImageBox = New-Object System.Windows.Forms.PictureBox
$qrImageBox.Location = New-Object System.Drawing.Point(24, 180)
$qrImageBox.Size = New-Object System.Drawing.Size(300, 300)
$qrImageBox.SizeMode = [System.Windows.Forms.PictureBoxSizeMode]::Zoom
$qrImageBox.BorderStyle = [System.Windows.Forms.BorderStyle]::FixedSingle
$qrImageBox.Visible = $false
$tabFeishu.Controls.Add($qrImageBox)

function Refresh-FeishuStatus {
    $script:cfg = Load-Config
    if (Has-FeishuAppBinding $script:cfg) {
        $webhookBox.Checked = Is-Enabled $script:cfg.notifications.webhook.enabled $true
        $platform = [string]$script:cfg.notifications.feishu.platform
        if ([string]::IsNullOrWhiteSpace($platform)) { $platform = "feishu" }
        $feishuStatus.Text = "Connected by QR binding.`r`nPlatform = $platform`r`nreceive_id_type = $($script:cfg.notifications.feishu.receive_id_type)"
    } elseif (Has-CustomFeishuWebhook $script:cfg) {
        $webhookBox.Checked = Is-Enabled $script:cfg.notifications.webhook.enabled $false
        $feishuStatus.Text = "Custom bot webhook exists, but QR binding is not active.`r`nClick Bind Feishu/Lark to switch to automatic QR binding."
    } else {
        $feishuStatus.Text = "Not connected.`r`nClick Bind Feishu/Lark, scan the QR code in the browser, then approve."
    }
}
Refresh-FeishuStatus

$bindButton = New-Object System.Windows.Forms.Button
$bindButton.Text = "Bind Feishu/Lark"
$bindButton.Location = New-Object System.Drawing.Point(24, 130)
$bindButton.Size = New-Object System.Drawing.Size(160, 34)
$bindButton.Add_Click({
    $process = $null
    $qrPath = Join-Path $env:TEMP ("claude-notifications-feishu-" + [Guid]::NewGuid().ToString("N") + ".png")
    try {
        $bindButton.Enabled = $false
        $testButton.Enabled = $false
        if ($qrImageBox.Image -ne $null) {
            $oldImage = $qrImageBox.Image
            $qrImageBox.Image = $null
            $oldImage.Dispose()
        }
        $qrImageBox.Visible = $false
        $feishuStatus.Text = "Creating Feishu/Lark QR code...`r`nScan and approve in the mobile app."
        $form.Refresh()
        $process = Start-Notifier @("feishu", "bind", "--timeout", "600", "--qr-image", $qrPath, "--no-browser")

        $qrDeadline = [DateTime]::Now.AddSeconds(20)
        while (-not $process.HasExited -and -not (Test-Path $qrPath) -and [DateTime]::Now -lt $qrDeadline) {
            [System.Windows.Forms.Application]::DoEvents()
            Start-Sleep -Milliseconds 200
        }
        if (Test-Path $qrPath) {
            $bytes = [System.IO.File]::ReadAllBytes($qrPath)
            $stream = New-Object System.IO.MemoryStream(,$bytes)
            $qrImageBox.Image = [System.Drawing.Image]::FromStream($stream)
            $qrImageBox.Visible = $true
            $feishuStatus.Text = "Scan this QR code with Feishu/Lark mobile app.`r`nWaiting for approval..."
            $form.Refresh()
        }

        while (-not $process.HasExited) {
            [System.Windows.Forms.Application]::DoEvents()
            Start-Sleep -Milliseconds 200
        }
        $stdout = $process.StandardOutput.ReadToEnd()
        $stderr = $process.StandardError.ReadToEnd()
        if ($process.ExitCode -ne 0) {
            throw (($stderr + "`r`n" + $stdout).Trim())
        }
        Refresh-FeishuStatus
        if (Has-FeishuAppBinding $script:cfg) {
            $webhookBox.Checked = $true
            [System.Windows.Forms.MessageBox]::Show("Feishu/Lark connected by QR binding.", "Connected")
        }
    } catch {
        [System.Windows.Forms.MessageBox]::Show($_.Exception.Message, "Feishu bind failed", "OK", "Error")
    } finally {
        $bindButton.Enabled = $true
        $testButton.Enabled = $true
    }
})
$tabFeishu.Controls.Add($bindButton)

$testButton = New-Object System.Windows.Forms.Button
$testButton.Text = "Send Test"
$testButton.Location = New-Object System.Drawing.Point(200, 130)
$testButton.Size = New-Object System.Drawing.Size(120, 34)
$testButton.Add_Click({
    try {
        $script:cfg = Load-Config
        if (-not (Has-FeishuAppBinding $script:cfg)) {
            throw "Feishu/Lark is not connected. Click Bind Feishu/Lark first."
        }
        $webhookBox.Checked = $true
        Apply-FormToConfig
        Save-Config $script:cfg
        Run-TestNotification
        [System.Windows.Forms.MessageBox]::Show("Test notification sent.", "Test")
    } catch {
        [System.Windows.Forms.MessageBox]::Show($_.Exception.Message, "Test failed", "OK", "Error")
    }
})
$tabFeishu.Controls.Add($testButton)

function Apply-FormToConfig {
    Set-Prop $script:cfg.notifications.desktop "enabled" ([bool]$desktopBox.Checked)
    Set-Prop $script:cfg.notifications.desktop "sound" ([bool]$soundBox.Checked)
    Set-Prop $script:cfg.notifications.desktop "appIcon" (Join-Path $InstallRoot "claude_icon.png")
    Set-Prop $script:cfg.notifications.webhook "enabled" ([bool]$webhookBox.Checked)

    foreach ($status in $statuses) {
        $statusObj = $script:cfg.statuses.$status
        Set-Prop $statusObj "enabled" ([bool]$statusControls[$status].enabled.Checked)
        Set-Channel $statusObj "desktop" ([bool]$statusControls[$status].desktop.Checked)
        Set-Channel $statusObj "webhook" ([bool]$statusControls[$status].webhook.Checked)
    }
}

$saveButton = New-Object System.Windows.Forms.Button
$saveButton.Text = "Save / Install"
$saveButton.Location = New-Object System.Drawing.Point(430, 635)
$saveButton.Size = New-Object System.Drawing.Size(120, 34)
$saveButton.Add_Click({
    try {
        Apply-FormToConfig
        if ($script:cfg.notifications.webhook.enabled -and -not (Has-FeishuAppBinding $script:cfg)) {
            throw "Feishu/Lark mobile notifications are enabled but QR binding is not connected. Click Bind Feishu/Lark first."
        }
        Save-Config $script:cfg
        if ($claudeBox.Checked) { Install-ClaudeHooks }
        if ($codexBox.Checked) { Install-CodexHooks }
        [System.Windows.Forms.MessageBox]::Show("Installed.`r`nRestart Claude Code/Codex and trust hooks.", "Done")
    } catch {
        [System.Windows.Forms.MessageBox]::Show($_.Exception.Message, "Install failed", "OK", "Error")
    }
})
$form.Controls.Add($saveButton)

$cancelButton = New-Object System.Windows.Forms.Button
$cancelButton.Text = "Close"
$cancelButton.Location = New-Object System.Drawing.Point(570, 635)
$cancelButton.Size = New-Object System.Drawing.Size(120, 34)
$cancelButton.Add_Click({ $form.Close() })
$form.Controls.Add($cancelButton)

[void]$form.ShowDialog()
