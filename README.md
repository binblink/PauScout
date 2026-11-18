# PauScout FM27 — source v0.14

Code source de PauScout FM27 v0.14. Le logiciel lit les sauvegardes locales et peut enregistrer les stats des joueurs et du staff dans le fichier `.fm` existant.

## Stack

- **Go 1.23+** : serveur HTTP local, import `.fm` et `.zip`, extraction des métadonnées FMF.
- **HTML / CSS / JavaScript sans framework** : interface de scouting et import de joueurs CSV / JSON.
- **Zstandard Go intégré** : `reader/zstd/` ; code sous licence BSD, licence incluse dans `reader/zstd/LICENSE`.
- **Compression Zstandard pour l'écriture Windows** : `reader/data/libzstd.dll`, licence incluse dans `reader/data/ZSTD_LICENSE`.
- Le programme compilé fonctionne sans installation de Go ou de Python chez l'utilisateur. **Go est nécessaire pour modifier/recompiler le code.**

## Fichiers principaux

- `main.go` : serveur local, endpoints HTTP, import des fichiers, lancement du navigateur.
- `reader/reader.go` : détection du conteneur `.fm`, trames Zstandard, lecture des métadonnées (version, entraîneur, club et quelques noms du résumé).
- `reader/directory.go` : validation en lecture seule du répertoire et des trames du conteneur FMF.
- `reader/write.go` : écriture des joueurs, réassemblage FMF et validation avant remplacement atomique.
- `reader/staff.go` et `reader/staff_write.go` : lecture et écriture du staff lié à un club, de son CA/PA, de 21 attributs et de 15 aptitudes aux postes.
- `reader/ability.go` : lecture ciblée CA/PA par ID de base de données, avec contrôle de l'en-tête de fermeture du joueur.
- `reader/attributes.go` et `reader/personality.go` : lecture des 52 attributs et des huit valeurs de personnalité sur l'échelle de 1 à 20.
- `reader/names.go` : lecture des tables de noms et rattachement ciblé d'un nom à l'ID affiché.
- `reader/dates.go` et `reader/positions.go` : date en jeu, âge et postes des joueurs.
- `reader/clubs.go` : rattachement des identifiants d'équipes aux clubs et à leurs effectifs distincts.
- `reader/nations.go` et `reader/leagues.go` : résolution du pays et des ligues dont le lien est vérifiable.
- `reader/data/` : noms externes de pays et compétitions issus de FM27 Scout, licence MIT incluse.
- `reader/catalog.go` : catalogue partiel des joueurs vérifiés, avec couverture explicite.
- `reader/reader_test.go` : tests du lecteur, dont un test d'intégration facultatif pour une sauvegarde FM27 locale.
- `reader/zstd/` : décompresseur intégré (conserver la licence).
- `web/index.html` : interface en français, navigation pays → ligue → club → équipe, attributs par catégories, import CSV/JSON, sélection et édition des joueurs.

## Compiler / exécuter

Installer Go 1.23 ou plus récent sur la machine de **développement** (pas sur le PC des utilisateurs).

Depuis le dossier contenant `go.mod` :

```sh
go test ./...
go run .
```

Pour produire le binaire Windows sur Windows :

```powershell
go build -ldflags="-H=windowsgui" -o PauScout_FM27.exe .
```

Ou, depuis Linux/macOS :

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-H=windowsgui" -o PauScout_FM27.exe .
```

L'interface sera servie localement sur `http://127.0.0.1:48727` (ou un port libre si celui-ci est pris) et s'ouvrira dans le navigateur par défaut.

## API locale

- `GET /api/status` : état et capacités du lecteur.
- `POST /api/charger` : corps binaire `.fm` ou `.zip`, avec en-tête `X-File-Name` (nom du fichier encodé dans l'URL).
- `GET /api/sauvegardes` : liste des `.fm` du dossier `games` local.
- `POST /api/ouvrir-local` : lecture d'un `.fm` sur le disque et création d'une session d'édition.
- `POST /api/enregistrer` : application des modifications de joueurs dans le fichier ouvert, après validation d'une copie temporaire.
- `POST /api/enregistrer-staff` : même écriture pour les membres du staff.
- `POST /api/quit` : fermeture du serveur.

## État réel v0.14

**Fonctionne :** ouverture locale, import `.fm`/`.zip`, détection FMF/Zstandard, validation du répertoire et lecture des métadonnées de la sauvegarde testée (FM27 `27.0.1+2503222`). Le lecteur retrouve les ID, CA/PA, 52 attributs, huit valeurs de personnalité, postes et genre des fiches validées ; il rattache l'âge et le nom lorsque le bloc de personne passe ses contrôles. Les clubs sont reliés à partir des ID d'équipes enregistrés dans `game_db`. L'interface affiche les hommes par défaut et sépare les équipes. Pour modifier les stats, ouvrir la sauvegarde locale dans le menu dédié, changer les valeurs d'une fiche et cliquer sur **Enregistrer les stats**. L'application construit un fichier temporaire complet, valide sa structure et ses données, puis remplace le fichier `.fm` au même chemin. Un test automatisé vérifie cette écriture sur une copie temporaire de la sauvegarde fournie. Les imports par glisser-déposer restent consultables sans écriture.

**Limites :** l'énumération de la sauvegarde actuelle reste partielle : 79 980 fiches liées à un ID, dont 65 964 hommes et 14 016 femmes, 79 303 avec un nom et 75 660 avec un club, pour 80 217 entrées `ACTUAL_PLAYER` annoncées dans l'index. Les 677 fiches sans nom sont étiquetées `Joueur ID …` ; aucun nom n'est inventé. L'âge est absent quand le bloc de personne ne peut pas être validé. L'écriture est activée uniquement pour le build `27.0.1+2503222` et les champs de joueur validés. L'utilisateur a confirmé le chargement dans FM27 après modification. Fermer la partie dans FM27 avant d'enregistrer. Aucun éditeur en direct ni modification des finances pour le moment.

**Navigation :** filtres en cascade pays → ligue → club → équipe. Le menu du club comprend les clubs de la ligue choisie. Sélectionner un club affiche d'abord son équipe première ; les équipes B et jeunes restent distinctes par ID. Sur la sauvegarde Pau FC étudiée, les ID 1651, 14871 et 28964 correspondent respectivement à l'équipe première, l'équipe B et les U19. Pour les autres clubs, le lecteur reconnaît U18/U19/U20/U21 quand les compétitions le permettent ; il indique une estimation fondée sur les âges si elle est solide, et numérote les autres équipes. Le lecteur rattache désormais 485 clubs à 29 compétitions à partir des sections de championnat de la sauvegarde. Les 18 clubs de Ligue 2 et les 20 de Premier League correspondent exactement aux classements fournis depuis l'éditeur. Les ligues sans correspondance fiable restent sous « Ligue non identifiée » ; leurs clubs restent accessibles.

**Interface v0.12 :** le choix du club demande maintenant une ligue, et le chemin choisi apparaît au-dessus de la liste des joueurs. Les attributs sont nommés en français et regroupés en technique, coups de pied arrêtés, mental, physique, technique du gardien et attributs cachés. Les pays sont affichés en français. Les lignes et valeurs modifiées en attente sont signalées visuellement. Les noms officiels des compétitions et des clubs restent ceux des données du jeu.

**Staff v0.13 :** un onglet séparé reprend la navigation pays → ligue → club → équipe. Sur la sauvegarde de test, 24 136 membres du staff liés à un club sont identifiés. La fiche affiche en français le poste, le CA/PA, 21 attributs regroupés et, quand ils sont présents sans ambiguïté, 15 aptitudes aux postes. Ces valeurs sont modifiables dans la sauvegarde existante. Un test vérifie l'écriture et la relecture sur une copie temporaire ; le chargement dans FM27 après une modification du staff reste à confirmer dans le jeu. Les autres données de la capture (réputations, licence, tendances tactiques et relations avec le comité) ne sont pas encore décodées.

**Personnalité des joueurs v0.13 :** adaptabilité, ambition, controverse, loyauté, pression, professionnalisme, esprit sportif et tempérament sont affichés et modifiables quand le bloc de personne est identifié. Le libellé synthétique de personnalité de l'éditeur FM n'est pas encore calculé.

**Date de naissance :** l'onglet Infos des joueurs et des membres du staff identifiés permet de modifier leur date de naissance. L'âge est recalculé à la date de la partie. Les tests sur copies vérifient l'écriture et la relecture, notamment pour le 29 février ; le chargement après modification reste à vérifier dans FM27.

**Recherche avancée :** les joueurs et le staff peuvent être recherchés dans toute la base chargée, puis affinés par pays, ligue, club et équipe. Les filtres comprennent nom ou ID, nationalité, genre, poste ou fonction, âge, CA et PA avec bornes minimum et maximum. Plusieurs seuils d'attributs peuvent être cumulés. La valeur des joueurs est lisible, filtrable et modifiable en livres ou en euros : le taux provient de la sauvegarde et les montants sont arrondis à l'unité. Les deux valeurs fournies par l'utilisateur pour Yoon Sungjun et Kosei Tani concordent avec la conversion. Les valeurs réservées sont affichées comme inconnues. Le salaire est filtrable pour les imports qui le fournissent.

La sauvegarde de test personnelle n'est pas incluse dans ce paquet. Garder une copie de tout `.fm` avant la première écriture.

## Suite suggérée dans Codex

Voir `CODEX_START.md` pour le suivi. Prochaine étape : étudier l'écriture des données financières du club.

Les observations vérifiées sur le conteneur et les limites de l'analyse sont
consignées dans `FMF_FORMAT.md`.
