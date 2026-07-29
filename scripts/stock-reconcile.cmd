@echo off
setlocal
cd /d %~dp0..
call scripts\env.cmd
if "%ORDER_BASE_URL%"=="" set ORDER_BASE_URL=http://127.0.0.1:8090
if "%ORDER_STOCK_RECONCILE_OUT%"=="" set ORDER_STOCK_RECONCILE_OUT=%ORDER_REPORT_DIR%\baseline-run
node scripts\stock-reconcile-test.js
