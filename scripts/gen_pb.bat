@echo off
cd /d "%~dp0\.."

set PROTO_DIR=protocol\proto
set OUT_DIR=protocol\pb

echo === Generating protobuf Go code ===

rem Generate top-level protos
for %%f in (%PROTO_DIR%\*.proto) do (
  echo   %%~nxf
  protoc --proto_path=%PROTO_DIR% --go_out=%OUT_DIR% --go_opt=paths=source_relative --go-grpc_out=%OUT_DIR% --go-grpc_opt=paths=source_relative %%f
  if %errorlevel% neq 0 exit /b %errorlevel%
)

rem Generate subdirectory protos
for /d %%d in (%PROTO_DIR%\*) do (
  for %%f in (%%d\*.proto) do (
    echo   %%~nxd\%%~nxf
    mkdir "%OUT_DIR%\%%~nxd" 2>nul
    protoc --proto_path=%PROTO_DIR% --go_out="%OUT_DIR%\%%~nxd" --go_opt=paths=source_relative --go-grpc_out="%OUT_DIR%\%%~nxd" --go-grpc_opt=paths=source_relative %%f
    if %errorlevel% neq 0 exit /b %errorlevel%
  )
)

echo === Done ===
