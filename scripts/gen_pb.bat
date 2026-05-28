@echo off
cd /d "%~dp0\.."

set PROTO_DIR=protocol\proto
set OUT_DIR=protocol\pb

echo === Generating protobuf Go code ===

for %%f in (%PROTO_DIR%\*.proto) do (
  echo   %%~nxf
  protoc --proto_path=%PROTO_DIR% --go_out=%OUT_DIR% --go_opt=paths=source_relative %%f
  if %errorlevel% neq 0 exit /b %errorlevel%
)

echo === Done ===
