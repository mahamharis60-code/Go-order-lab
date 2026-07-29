@echo off
setlocal
cd /d %~dp0..
call scripts\env.cmd
if "%ORDER_HTTP_ADDR%"=="" set ORDER_HTTP_ADDR=:8090
if "%ORDER_DB_DRIVER%"=="" set ORDER_DB_DRIVER=sqlite
if "%ORDER_DB_SOURCE%"=="" set ORDER_DB_SOURCE=data/order_lab.db
if "%ORDER_ADMIN_USERNAME%"=="" set ORDER_ADMIN_USERNAME=admin
if "%ORDER_ADMIN_PASSWORD%"=="" set ORDER_ADMIN_PASSWORD=admin123456
go run ./cmd/server
