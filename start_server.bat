@echo off
title Serveur téléchargement automatique (Decoloop)
color 0B
echo =======================================
echo     CPT DECOLOOP SERVER
echo =======================================
echo.
echo Ce serveur doit rester ouvert pour recevoir les fichiers PDF.
echo Vous pouvez reduire cette fenetre.
echo.
"%~dp0ptxrid.exe" -server
pause
