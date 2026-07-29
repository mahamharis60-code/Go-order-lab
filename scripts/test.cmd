@echo off
setlocal
cd /d %~dp0..
call scripts\env.cmd
go test ./... -count=1 -timeout=2m
