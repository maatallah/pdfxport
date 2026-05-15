Voici la liste exhaustive des drapeaux (`flags`) disponibles pour le programme principal (Le Parseur) situé dans `m:\dev\cpt\PDFXport` :

### 🚀 Drapeaux de Mode (Action principale)

- **`-server`** : Lance le mode serveur d'orchestration pour recevoir les PDF de l'extension Chrome.
- **`-dump`** : Extrait uniquement le texte brut des PDF et le sauvegarde dans `out/parsed.txt` (sans générer d'Excel). Idéal pour le diagnostic.

### 📁 Drapeaux de Dossiers / Fichiers

- **`-dir`** (par défaut: `"./in"`) : Spécifie le dossier où se trouvent les PDF à traiter.
- **`-outdir`** (par défaut: `"./out"`) : Spécifie le dossier où seront créés l'Excel et les fichiers de log.
- **`-input`** : Permet de traiter un **seul fichier PDF spécifique** au lieu de tout un dossier.
  - Exemple : `go run . -input "./in/commande_123.pdf" -excel`

### 📊 Drapeaux d'Export (Formats)

- **`-excel`** (par défaut: `true`) : Génère le fichier `output.xlsx`.
- **`-json`** : Génère un fichier `output.json` avec les données structurées.
- **`-csv`** : Génère un fichier `output.csv`.

### 🔍 Drapeaux de Diagnostic

- **`-debug`** : Active les logs détaillés (détection des règles, détails de séparation, diagnostics PDF library). Écrit dans la console et dans un fichier de log si configuré.

---

**Exemple de commande complète combinée :**

powershell

go run . -dir "./in" -outdir "./out" -excel -debug

*(Traite le dossier `/in`, sort dans `/out`, génère l'Excel et affiche les logs de debug pour vérifier chaque règle).* 🚜🌾🏛️
