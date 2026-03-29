param(
    [Parameter(Mandatory = $true)]
    [string]$Url,

    [Parameter(Mandatory = $true)]
    [string]$Token,

    [Parameter(Mandatory = $false)]
    [string]$Content = "hello from test-tailclip-endpoint.ps1",

    [Parameter(Mandatory = $false)]
    [string]$SourceDeviceId = "manual-test"
)

$body = @{
    id               = "evt_manual_test"
    content          = $Content
    content_hash     = ""
    source_device_id = $SourceDeviceId
    created_at       = [DateTime]::UtcNow.ToString("o")
} | ConvertTo-Json -Depth 3

$headers = @{
    Authorization = "Bearer $Token"
}

try {
    $response = Invoke-WebRequest `
        -Method Post `
        -Uri $Url `
        -Headers $headers `
        -ContentType "application/json" `
        -Body $body

    Write-Host ("Status: {0} {1}" -f [int]$response.StatusCode, $response.StatusDescription)
    if ($null -ne $response.Content -and $response.Content -ne "") {
        Write-Host ("Body: {0}" -f $response.Content)
    }
}
catch {
    if ($_.Exception.Response) {
        $statusCode = [int]$_.Exception.Response.StatusCode
        $statusText = [string]$_.Exception.Response.StatusDescription
        Write-Host ("Status: {0} {1}" -f $statusCode, $statusText)
    }

    Write-Error $_
    exit 1
}
