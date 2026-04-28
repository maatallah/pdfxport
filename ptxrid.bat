@echo off

:: Désactiver le blocage de sécurité Windows 7 sur les lecteurs réseau
set SEE_MASK_NOZONECHECKS=1

:: Force le répertoire de travail au dossier actuel (corrige les bugs des lecteurs réseau)
cd /d "%~dp0"

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
echo [Script] Lancement de l'executable...
"%~dp0ptxrid.exe" -dir "%INDIR%" -outdir "%OUTDIR%" -excel -nowait

color 0F
echo.
echo =======================================
echo Exécution treminée
echo =======================================
pause