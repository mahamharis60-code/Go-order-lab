@echo off
if "%ORDER_WORK_DIR%"=="" set ORDER_WORK_DIR=%CD%
if "%ORDER_REPORT_DIR%"=="" set ORDER_REPORT_DIR=%ORDER_WORK_DIR%\reports
if "%ORDER_CACHE_DIR%"=="" set ORDER_CACHE_DIR=%ORDER_WORK_DIR%\.cache

if "%GOPROXY%"=="" set GOPROXY=https://goproxy.cn,direct
if "%GOPATH%"=="" set GOPATH=%ORDER_CACHE_DIR%\go
if "%GOMODCACHE%"=="" set GOMODCACHE=%ORDER_CACHE_DIR%\go\pkg\mod
if "%GOCACHE%"=="" set GOCACHE=%ORDER_CACHE_DIR%\go-build
if "%GOTMPDIR%"=="" set GOTMPDIR=%ORDER_CACHE_DIR%\go-tmp
if "%TMP%"=="" set TMP=%ORDER_CACHE_DIR%\go-tmp
if "%TEMP%"=="" set TEMP=%ORDER_CACHE_DIR%\go-tmp

if not exist "%ORDER_REPORT_DIR%" mkdir "%ORDER_REPORT_DIR%"
if not exist "%GOTMPDIR%" mkdir "%GOTMPDIR%"
