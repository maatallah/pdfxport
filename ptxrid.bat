@echo off
chcp 65001 >nul
color 0F
title Analyseur PDF vers Excel

echo =======================================
echo        EXTRACTION PDF VERS EXCEL
echo =======================================
echo.

set /p INDIR="1. Dossier d'entree : "
set /p OUTDIR="2. Dossier de sortie : "

color 0E
echo.
echo Traitement en cours...
echo.

color 0A
ptxrid.exe -dir "%INDIR%" -outdir "%OUTDIR%" -excel


color 0F
echo.
echo =======================================
echo Exécution treminée
echo =======================================
pause