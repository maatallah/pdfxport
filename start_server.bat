@echo off
title Decoloop Auto-Downloader Server
color 0B
echo =======================================
echo     DECOLOOP BACKGROUND SERVER
echo =======================================
echo.
echo Ce serveur doit rester ouvert pour recevoir les fichiers de l'extension Chrome.
echo Vous pouvez reduire cette fenetre.
echo.
"%~dp0ptxrid.exe" -server
pause
