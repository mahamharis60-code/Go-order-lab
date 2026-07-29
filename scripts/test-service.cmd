@echo off
setlocal
cd /d %~dp0..
call scripts\env.cmd
if "%1"=="" (
  go test ./internal/service -count=1 -timeout 30s
) else (
  go test ./internal/service -run %1 -count=1 -timeout 30s
)
