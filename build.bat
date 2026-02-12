@echo off
REM Build script for MBS Scanner (Windows)

echo Building MBS Scanner...

REM Create build directory if not exists
if not exist build mkdir build

REM Build the application
go build -o build\mbsscaner.exe main.go

if %ERRORLEVEL% EQU 0 (
    echo Build completed successfully: build\mbsscaner.exe
) else (
    echo Build failed!
    exit /b 1
)

echo.
echo To run the application:
echo   build\mbsscaner.exe -config config.toml
echo.
echo To setup configuration:
echo   copy config.toml.example config.toml
echo.
echo To create logs directory:
echo   mkdir logs