# Run from repo root: .\generate-proto.ps1
# Requires: protoc, protoc-gen-go, protoc-gen-go-grpc

$PROTO_DIR = Resolve-Path "..\tarantula-protocol"
$OUT_DIR   = Resolve-Path "."
$MODULE    = "gameclustering.com"

foreach ($tool in @("protoc", "protoc-gen-go", "protoc-gen-go-grpc")) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
        Write-Error "Missing tool: $tool"
        Write-Host "Install with:"
        Write-Host "  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
        Write-Host "  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
        exit 1
    }
}

$protoFiles = Get-ChildItem -Path $PROTO_DIR -Filter "*.proto" | Select-Object -ExpandProperty FullName

Write-Host "Generating from $PROTO_DIR -> internal/protocol/ ..."

protoc `
    --proto_path="$PROTO_DIR" `
    --go_out="$OUT_DIR" `
    --go_opt="module=$MODULE" `
    --go-grpc_out="$OUT_DIR" `
    --go-grpc_opt="module=$MODULE" `
    @protoFiles

if ($LASTEXITCODE -eq 0) {
    Write-Host "Done. Files written to internal/protocol/"
} else {
    Write-Error "protoc failed with exit code $LASTEXITCODE"
    exit 1
}
