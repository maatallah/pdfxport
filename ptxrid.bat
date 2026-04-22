@echo off
chcp 65001 >nul
color 0A
title Analyseur PDF vers Excel

echo =======================================
echo        EXTRACTION PDF VERS EXCEL
echo =======================================
echo.

set /p INDIR="1. Dossier d'entree : "
set /p OUTDIR="2. Dossier de sortie : "

echo.
echo Traitement en cours...
echo.

go run main.go -debug -dir "%INDIR%" -outdir "%OUTDIR%" -excel

echo.
echo =======================================
echo Logs generes a cote des PDF (_debug.txt)
echo =======================================
pause