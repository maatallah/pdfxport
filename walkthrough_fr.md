# PDFXport : Pipeline Automatisé de Decoloop vers Excel

## 1. Résumé Exécutif
**PDFXport** est un pipeline d'automatisation sur mesure conçu pour éliminer la saisie manuelle des données à partir des PDF de commande Decoloop. Il fait le pont entre une interface web lente et à haute latence et les systèmes locaux d'impression d'étiquettes basés sur Excel, avec une optimisation spécifique pour les environnements hérités sous **Windows 7**.

---

## 2. Problèmes et Défis
*   **Friction du flux de travail** : Les utilisateurs devaient cliquer manuellement sur "Imprimer", attendre qu'un serveur lent génère un PDF, le télécharger, et le déplacer manuellement vers un dossier de traitement.
*   **Incohérence des données** : Les parseurs PDF standards avaient du mal avec les commandes "Paire" (séparées) et les formats de commande spécialisés "Tissu".
*   **Goulots d'étranglement réseau** : La lenteur du serveur de Decoloop entraînait le blocage du navigateur à cause de la limite de 6 connexions simultanées de Chrome.
*   **Contraintes héritées** : La solution devait fonctionner sous **Windows 7**, gérer les lecteurs réseau mappés (bugs d'élévation UAC) et supporter les anciens environnements de terminal.

---

## 3. L'Architecture (Couche par Couche)

### Couche 1 : L'Intercepteur (Extension Chrome)
*   **Techno** : JavaScript Vanilla (Manifest V3), API Chrome Extensions (Content Scripts).
*   **Problème résolu** : Capture automatiquement les données PDF dès qu'elles arrivent du serveur.
*   **Logique clé** : Utilise un crochet hybride (XHR + Fetch) et une détection par "Content-Type" pour trouver les PDF quels que soient les changements d'URL. Pousse les données instantanément vers le serveur local via une requête POST sécurisée sur localhost.

### Couche 2 : Le Pont (Serveur HTTP en Go)
*   **Techno** : Go (Golang), bibliothèque standard `net/http`.
*   **Problème résolu** : Élimination des étapes manuelles de téléchargement et de déplacement.
*   **Logique clé** : Écoute exclusivement sur `127.0.0.1` pour la sécurité. Implémente le "handshake" **Private Network Access (PNA)** pour contourner les blocages de sécurité modernes de Chrome.

### Couche 3 : Le Processeur (Moteur d'Extraction)
*   **Techno** : Go, `github.com/ledongthuc/pdf`, `regexp`.
*   **Problème résolu** : Extraction de données de haute précision à partir de mises en page PDF variées.
*   **Logique clé** :
    *   **Règle Paire** : Détecte automatiquement les paires "Gauche/Droite" et divise un enregistrement en deux avec des largeurs divisées par deux.
    *   **Fallback Tissu** : Extrait spécifiquement les dimensions de "Hauteur de coupe" lorsque les étiquettes standard sont absentes.
    *   **Renommage Auto** : Renomme les fichiers génériques `Commandes_timestamp.pdf` en leur numéro réel `NumeroCommande.pdf` pour une traçabilité immédiate.

### Couche 4 : Le Secours (Moteur OCR)
*   **Techno** : NAPS2 (CLI), Tesseract OCR.
*   **Problème résolu** : Gère les PDF "image uniquement" ou les couches de texte brouillées.
*   **Logique clé** : Si le parseur de texte ne trouve aucun résultat, il déclenche automatiquement un scan OCR en arrière-plan via NAPS2 et re-scanne le texte.

### Couche 5 : La Sortie (Excel et Archive)
*   **Techno** : `github.com/xuri/excelize/v2`.
*   **Problème résolu** : Reporting cohérent et traçabilité historique.
*   **Logique clé** : Écrit un fichier maître `output.xlsx` (pour le système d'étiquettes) et crée simultanément une copie horodatée dans un dossier `/archive` dédié.

---

## 4. Percées Techniques Clés
| Défi | Solution |
| :--- | :--- |
| **Lecteurs réseau Windows 7** | Implémentation de la **résolution de chemin UNC** pour contourner le bug des lecteurs mappés lors de l'élévation administrative. |
| **Blocages de sécurité Chrome** | Ajout des **en-têtes PNA** au serveur Go pour que Chrome fasse confiance au transfert de données "Public-vers-Local". |
| **Blocage du navigateur** | Identification du **goulot d'étranglement des 6 connexions** ; établissement d'une stratégie de "Lots de 5" pour garder le canal de notification ouvert. |
| **Formatage des nombres** | Création d'un **helper de normalisation** pour supprimer les décimales `.0` inutiles tout en conservant les fractions réelles (ex: 185 au lieu de 185.0). |

---

## 5. Pile Technique Finale
*   **Langage** : Go 1.20 (Dernière version supportant officiellement Windows 7).
*   **Communication** : HTTP/1.1 (CORS + en-têtes PNA).
*   **Frontend** : JavaScript (Intercepteurs asynchrones).
*   **Infrastructure** : Windows 7 SP1+, Lecteurs réseau mappés (UNC), Scripts Batch.
