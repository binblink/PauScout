# Notes sur le conteneur FM27

Ces observations proviennent d'une lecture seule de deux sauvegardes locales de
Pau FC, version de jeu `27.0.1+2503222`. Les sauvegardes ne sont pas incluses
dans ce dépôt. Les offsets sont des positions en octets depuis le début du
fichier et les indices de trame commencent à zéro.

Les copies expérimentales v6 à v10 décrites ci-dessous ont été supprimées du
projet après validation, à la demande de l'utilisateur. Leurs résultats sont
conservés ici ; les tests courants utilisent une sauvegarde locale fournie par
`FM27_SAMPLE`.

## Structure observée

- L'en-tête du fichier commence par `02 01 66 6d 66 2e` (`fmf.` précédé de
  deux octets). La première trame Zstandard commence à l'offset 26.
- Les octets 9 à 16 de l'en-tête forment un entier non signé sur 64 bits,
  petit-boutiste. Dans l'original, sa valeur 133 525 364 est exactement
  l'offset de fin de la dernière trame reconnue (133 525 373) moins 9.
  Cette valeur délimite donc le flux principal avant le suffixe FMF.
- Les trames Zstandard sont consécutives. Leur contenu décompressé commence
  par `03 01 74 61 64 2e` (`tad.` précédé de deux octets).
- La trame 0 contient notamment une chaîne de version de format
  `27.0.0+0` et des identifiants internes.
- La trame 1 contient une table : après l'en-tête `03 01 tad.` et deux octets
  encore non interprétés, un entier non signé sur 16 bits donne le nombre
  d'entrées. Chaque entrée contient une longueur sur 32 bits, le libellé ASCII
  de cette longueur, puis une valeur sur 32 bits. La table occupe exactement
  le reste de la trame dans les deux sauvegardes examinées.
- La trame 2 contient le résumé de partie. Elle permet de lire la version
  du jeu, l'entraîneur, le club et 35 noms affichés dans le résumé. Cette
  liste n'est pas une table de joueurs.
- La trame 3 est `game_db.dat` d'après le répertoire de sections décrit
  plus bas. Elle contient des données binaires et des chaînes de jeu, mais
  sa structure d'enregistrements n'est pas encore décodée.

| Sauvegarde | Taille fichier | Trame 0 | Trame 1 | Trame 2 | Trame 3 | Trames reconnues |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `Binbin Carpenter - Pau FC (v02).fm` | 129 921 398 | 26 / 2 654 | 2 680 / 5 024 | 7 704 / 1 005 | 8 709 / 62 725 118 | 1 278 |
| `Binbin Carpenter - Pau FC.fm` | 133 544 049 | 26 / 2 664 | 2 690 / 4 542 | 7 232 / 997 | 8 229 / 62 990 572 | 1 287 |

Chaque cellule de trame indique `offset / taille compressée`. Dans la
sauvegarde récente, la trame 3 se décompresse en 196 019 133 octets. Après
la dernière trame reconnue, le lecteur rencontre 18 676 octets qui ne
commencent pas par une signature Zstandard. Leur fonction reste inconnue.

Le suffixe de 18 676 octets est un second objet FMF : en-tête de 9 octets,
puis une trame Zstandard de 18 667 octets. Son contenu décompressé mesure
83 589 octets. Il commence par le titre de la partie et un répertoire de
136 sections. Chaque entrée comprend deux chaînes (nom et extension), puis
cinq entiers de 64 bits. Les trois premiers indiquent l'offset de la trame
depuis l'octet 26 du fichier, sa taille compressée et sa taille décompressée.
Les 136 offsets pointent tous vers des débuts de trames vérifiés ; les deux
derniers entiers n'ont pas encore été interprétés. `game_db.dat` est
l'entrée 3, à l'offset relatif 8 203, avec une taille compressée de
62 990 572 et une taille décompressée de 196 019 133 octets dans la
sauvegarde récente. Le premier groupe occupe 9 268 octets du contenu
décompressé du suffixe. Il est suivi d'un groupe nommé `rgman` de 1 151
autres entrées, de même structure. Les 136 + 1 151 entrées couvrent
exactement les 1 287 trames de la sauvegarde ; chaque offset et chaque
taille compressée correspondent à une trame vérifiée. Le suffixe se termine
par quatre octets nuls.

La table de la sauvegarde récente contient 485 entrées, contre 537 dans
`(v02)`. Les valeurs suivantes suivent les libellés indiqués :

| Libellé | Sauvegarde récente | `(v02)` |
| --- | ---: | ---: |
| `PERSON_NAME` | 1 067 038 | 1 067 004 |
| `ACTUAL_PLAYER` | 76 150 | 77 616 |
| `ACTUAL_NON_PLAYER` | 42 792 | 42 577 |
| `PLAYER_ATTRIBUTE_SNAPSHOT` | 416 | 364 |

Ces valeurs ressemblent à des nombres d'objets, mais leur sens exact et leur
correspondance avec les données de la trame 3 ne sont pas encore prouvés.
`reader.ReadIndex` expose cette table en lecture seule, sans décompresser la
grande trame.

Deux autres sauvegardes ont été lues pour comparer l'index : `(v03)` contient
`ACTUAL_PLAYER = 77 639` et `PERSON_NAME = 1 067 035` ;
`last save overwrite backup.fm` contient respectivement `77 742` et
`1 067 038`. Ces valeurs varient entre sauvegardes, sans donner à elles
seules les frontières des objets dans la trame 3.

Un balayage complet de la trame 3 décompressée trouve `Maubleu` une fois et
`Brice` 67 fois dans chaque sauvegarde. Les premières occurrences de
`Maubleu` se situent aux offsets décompressés 84 559 914 (récente) et
84 230 897 (`v02`). Les libellés `ACTUAL_PLAYER` et `PERSON_NAME` n'y sont
pas présents en texte brut. Ces chaînes montrent la présence de noms dans
la section, mais pas la position ni la structure d'un enregistrement joueur.

Autour de `Maubleu`, les deux sauvegardes présentent exactement la même
séquence locale : identifiant `e3 4d 07 00`, longueur `07 00 00 00`, puis
`Maubleu` en UTF-8. Les noms voisins suivent la même forme. Cela indique
une table de chaînes avec identifiants et longueurs ; le lien de l'identifiant
avec un objet `ACTUAL_PLAYER` n'est pas encore établi. Une occurrence de
`Brice` examinée correspond à un nom de club (`Saint-Brice FC`), ce qui
montre qu'une recherche textuelle seule ne peut pas identifier un joueur.

Une autre entrée de la table de noms contient `Brice` avec l'identifiant
`35 84 03 00`. `Mathys` a l'identifiant `88 dd 00 00` (56 712) et
`Tourraine` l'identifiant `02 37 02 00` (145 154). Les deux paires de noms
du résumé réapparaissent avec la même disposition : identifiant du prénom
sur 4 octets, un octet nul, identifiant du nom sur 4 octets, un octet nul.

| Sauvegarde | `Brice Maubleu` | `Mathys Tourraine` |
| --- | ---: | ---: |
| `(v02)` | 103 514 657 | 110 585 757 |
| `(v03)` | 103 470 231 | 110 557 805 |
| Récente | 103 830 639 | 110 894 084 |
| `last save overwrite backup.fm` | 103 824 733 | 110 888 534 |

Ces offsets sont ceux du premier identifiant dans la trame 3 décompressée.
La répétition sur quatre sauvegardes établit un lien entre la table des
chaînes et des blocs de données de personne. Elle ne suffit pas à délimiter
ces blocs, à les relier à `ACTUAL_PLAYER` ni à identifier des attributs.
Les 256 octets autour de chaque paire sont identiques entre la sauvegarde
récente et `last save overwrite backup.fm` ; ils diffèrent plus loin dans
les versions `(v02)` et `(v03)`. Les 80 premiers octets du contexte étudié
sont identiques dans les quatre fichiers pour chaque paire.

## Recherche CA/PA

Une comparaison des 2 048 octets centrés sur chaque paire de noms montre que
les deux fichiers récents ont des contextes identiques. `(v02)` et `(v03)`
diffèrent, notamment aux offsets relatifs +131 pour Brice et +83/+99 pour
Mathys, où un octet passe de 156 à 163. Ce changement partagé peut venir
d'un autre champ ; il ne prouve pas une évolution de CA.

Dans les 256 octets suivant chaque paire, une recherche **hypothétique** de
deux valeurs adjacentes entre 1 et 200, avec PA ≥ CA et PA constant dans les
quatre sauvegardes, laisse 15 positions possibles si chaque valeur occupe un
octet et aucune si chacune occupe deux octets. Beaucoup des 15 positions
contiennent manifestement des marqueurs ou d'autres petits nombres. Cette
recherche ne tenait pas compte de la position de l'enregistrement joueur par
rapport aux noms. Elle n'a donc pas identifié CA et PA.

Des captures de l'éditeur FM27 fournissent maintenant deux références :
`Brice Maubleu`, ID 34 001 981, CA 112, PA 120 ; `Mathys Tourraine`,
ID 49 054 454, CA 119, PA 129. L'utilisateur confirme que les captures
concernent `Binbin Carpenter - Pau FC.fm`. Les deux ID apparaissent chacun
deux fois dans la trame 3, à 360 octets de la paire de noms pour Brice et
368 octets pour Mathys, dans les quatre sauvegardes comparées. Ceci confirme
le rattachement des paires de noms aux ID de joueur.

Les premières recherches autour des ID affichés ont échoué parce que ces ID
se trouvent dans un en-tête qui **ferme** le bloc du joueur. Le CA et le PA
sont au début du bloc, environ 3 KiB avant cet en-tête dans les deux exemples.
Les ID réapparaissent aussi dans d'autres sections ; aucune occurrence isolée
dans ces sections ne constitue un décodage CA/PA. La correspondance concluante
est détaillée plus bas avec l'en-tête de fermeture, les réputations et la taille.

## Expérience de réécriture isolée

À la demande de l'utilisateur, une copie `PauScout_essai_nom.fm` a été créée
sans modifier la sauvegarde d'origine. Le seul octet décompressé changé dans
la trame 3 est le dernier caractère de `Maubleu` à l'offset 84 559 920 :
`u` devient `y`. La trame 3 a été réencodée en blocs Zstandard non compressés,
ce qui porte le fichier expérimental à 266 577 111 octets. La comparaison
intégrale des contenus décompressés confirme exactement une différence ; le
lecteur Go retrouve 1 287 trames et la version attendue. La copie placée
dans le dossier `games` a le même SHA-256 que celle du projet :
`1E0A9D3EBECEDB15AD5914F40E11ED468D5166325E62669DE7917B334E1F7B75`.
La compatibilité réelle avec FM27 et l'éventuelle validation de fin de
fichier restent à confirmer par un chargement dans le jeu. **FM27 a refusé de
charger cette première copie.** La trame 3 d'origine commence par l'en-tête
Zstandard `28 b5 2f fd 04 50` (fenêtre explicite et somme de contrôle),
alors que la copie commence par `28 b5 2f fd e0` (segment unique, sans somme
de contrôle). Le lecteur Go accepte les deux formes, mais le jeu peut imposer
des contraintes supplémentaires. La cause exacte de l'échec reste inconnue.
Cette expérience ne valide pas la méthode de réécriture ni les champs CA/PA.

Une seconde copie `PauScout_essai_nom_v2.fm` reprend l'en-tête de la trame 3
originale (`28 b5 2f fd 04 50`) et inclut les 32 bits de poids faible d'un
xxHash64 recalculé à partir du contenu modifié. Le calcul a d'abord été vérifié
contre la somme de contrôle originale (`3bf31cd3`) ; celle de la copie vaut
`46e74050`. Le contenu décompressé de cette seconde copie diffère encore de
l'original d'un seul octet, à 84 559 920. Le lecteur reconnaît toujours
1 287 trames. FM27 a également refusé la v2. Son écran de sélection
affichait `0,0 Mo`, une date en jeu au 1er janvier 1900 et aucun entraîneur
ou club. La v2 gardait dans l'en-tête la longueur originale du flux principal
alors que la trame 3 réencodée l'avait agrandi.

Une troisième copie `PauScout_essai_nom_v3.fm` porte la longueur corrigée
266 558 423. Son contenu à partir de l'octet 17 est identique à la v2 ; le
lecteur Go reconnaît toujours 1 287 trames. FM27 a affiché correctement le
résumé de la v3, puis a interrompu le chargement à 3 % avec le message
« La partie sauvegardée n'a pu être chargée ».

Une quatrième copie `PauScout_essai_nom_v4.fm` utilise la bibliothèque
Zstandard installée avec Git sur le PC de développement pour produire une
trame 3 réellement compressée de 61 225 165 octets, proche des 62 990 572
octets de l'original. Elle conserve l'en-tête `28 b5 2f fd 04 50`, la somme
de contrôle du contenu et la longueur FMF corrigée (131 759 957). La
comparaison intégrale des contenus décompressés confirme une seule différence
à l'offset 84 559 920 ; le lecteur retrouve 1 287 trames. La copie mesure
131 778 642 octets. FM27 a interrompu son chargement à 93 %.

Une cinquième copie `PauScout_essai_nom_v5.fm` corrige dans le répertoire la
taille de `game_db.dat` et décale les offsets suivants. Le suffixe a été
recompressé ; il mesure 18 577 octets et le fichier entier 131 778 552
octets. Les 136 entrées correspondent aux trames réelles dans le lecteur Go.
FM27 refuse encore de charger cette copie : d'autres champs du suffixe ou
des vérifications internes restent à identifier.

La v5 s'arrête encore à 93 %. Elle ne corrigeait que les 136 premières
entrées. Une sixième copie `PauScout_essai_nom_v6.fm` corrige aussi les
1 151 entrées du groupe `rgman`. Le suffixe recompressé mesure 18 557
octets et le fichier complet 131 778 532 octets. Les 1 287 positions et
tailles du répertoire ont été vérifiées contre les trames réelles ; le flux
principal est identique à celui de la v4. **FM27 charge cette v6 et affiche
« Maubley » pour Brice**, comme confirmé par l'utilisateur. Cela valide la
réécriture du conteneur pour ce changement de nom précis ; les champs CA/PA
restent à identifier et à valider séparément.

## Vérification des offsets CA/PA publiés

Les projets open source [fmsave](https://github.com/rhiever/fmsave) et
[FM27 Scout](https://github.com/shlk20/fm27-scout-code/blob/main/docs/fm27-layout.md)
décrivent, pour le schéma `game_db` 4000, un marqueur `01 00 6c 07` à
`record + 102`, puis CA à `record + 0` (`u16`) et PA à `record + 2` (`i16`).
FM27 Scout indique explicitement couvrir le build `27.0.1+2503222` de la
sauvegarde locale. Une sonde Go en lecture seule a décompressé `game_db` et
appliqué les contraintes de marqueur, identifiants doublés, positions et
attributs décrites dans ces sources. Elle trouve 74 134 candidats valides,
contre `ACTUAL_PLAYER = 76 150` dans l'index, ce qui laisse notamment des
enregistrements sans marqueur à étudier.

Le lien entre l'ID affiché et l'enregistrement joueur passe par un **en-tête
de fermeture** situé après les données de personne. Cet en-tête contient
`pindex + 1`, puis l'ID affiché deux fois. Il ne faut pas le confondre avec
l'en-tête de l'enregistrement suivant, qui commence immédiatement après.

| Joueur | Début d'enregistrement dans `game_db` | `pindex` | UID de l'en-tête | CA / PA | Réputation domicile / actuelle / mondiale | Taille | En-tête de fermeture (`pindex + 1`, ID × 2) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Brice Maubleu | 103 827 967 | 18 459 | 34 001 882 | 112 / 120 | 5 788 / 5 949 / 3 344 | 192 cm | offset 103 830 995 ; 18 460, 34 001 981 × 2 |
| Mathys Tourraine | 110 891 144 | 26 887 | 49 054 433 | 119 / 129 | 5 559 / 5 876 / 2 945 | 176 cm | offset 110 894 448 ; 26 888, 49 054 454 × 2 |

Ces CA/PA, trois réputations et tailles correspondent tous aux captures de
l'éditeur fournies par l'utilisateur. Les positions des noms dans les mêmes
fenêtres corroborent l'association. Les offsets CA/PA publiés sont donc
confirmés pour ces deux joueurs ; il reste à valider la détection exhaustive
des autres formes d'enregistrements avant de les exposer dans l'interface.

Une copie distincte `PauScout_essai_CA_Brice_v7.fm` reprend la v6 et change
uniquement l'octet décompressé à `game_db + 103 831 014`, de `76` à `100`,
avec compression, longueur FMF et les 1 287 offsets du répertoire recalculés.
Le lecteur retrouve 1 287 trames et la différence intégrale du `game_db`
décompressé avec la v6 est exactement cet octet. La copie est disponible dans
le dossier `games` ; ses 1 287 entrées de répertoire correspondent aux trames
réelles. **FM27 charge la v7, mais l'éditeur affiche toujours CA 112, PA 120
et 192 cm pour Brice Maubley**, d'après la capture fournie par l'utilisateur.
L'octet modifié appartenait à l'enregistrement **suivant** celui de Brice :
cette v7 a donc testé le mauvais joueur et ne réfute pas l'emplacement CA à
`record + 0`. Elle reste une copie expérimentale à ne pas utiliser comme
partie principale.

Une huitième copie `PauScout_essai_CA_Brice_v8.fm` repart de la v6 et change
uniquement `game_db + 103 827 967` de `112` à `113`. La comparaison intégrale
des données décompressées montre exactement cet octet différent et les 1 287
entrées du répertoire ont été vérifiées. **FM27 charge la v8 et l'éditeur
affiche CA 113 pour Brice Maubley**, confirmé par l'utilisateur. Cela valide
le champ CA de cet enregistrement dans le jeu.

Une neuvième copie `PauScout_essai_PA_Brice_v9.fm` repart aussi de la v6 et
change uniquement `game_db + 103 827 969` de `120` à `121`, sans toucher au
CA. Le lecteur voit CA 112 et PA 121, la comparaison décompressée ne montre
qu'un octet changé et les 1 287 entrées du répertoire sont cohérentes. Le
jeu charge la v9 et l'éditeur affiche **PA 121 pour Brice Maubley**, confirmé
par l'utilisateur. Les champs CA et PA de cet enregistrement sont donc tous
deux validés par des modifications isolées et un chargement réel dans FM27.

## Correspondance des attributs sur l'échelle 1 à 20

Le bloc d'un joueur contient 54 octets d'attributs à `record + 39` : 52
attributs et deux octets de pied gauche/droit aux positions brutes 24 et 25.
Pour l'attribut numéro `i` dans l'ordre de `reader/attributes.go`, la position
brute est `i` si `i < 24`, sinon `i + 2`. La valeur affichée est
`max(1, (valeur_brute + 2) / 5)` avec division entière. Des valeurs d'origine
visibles sur les captures (par exemple détente aérienne 14 et prise de balle
12 pour Brice, centres 13 et première touche 11 pour Mathys) correspondent
à ce calcul.

La copie `PauScout_essai_attributs_v10.fm`, issue de la v6, code les 11
attributs visibles seulement chez le gardien avec les valeurs 1 à 11 chez
Brice, et les 11 visibles seulement chez le joueur de champ avec les valeurs
1 à 11 chez Mathys. Les 30 attributs communs reçoivent des couples de valeurs
uniques entre les deux joueurs (Brice 12 à 15, Mathys 12 à 20). Le pied gauche,
le pied droit, les huit valeurs de personnalité, le CA et le PA ne changent
pas. Le manifeste détaillé se trouve dans `.research/attribute_probe_v10.csv`.
Le `game_db` décompressé diffère de la v6 sur exactement 79 octets, correspondant
aux changements prévus ; les 1 287 entrées du répertoire ont été vérifiées.
**FM27 charge la v10 et les deux captures fournies montrent les valeurs codées
sur les attributs visibles**, avec CA/PA inchangés. Cela valide la correspondance
des 52 positions d'attributs avec leurs libellés, en réunissant les deux vues.

`reader.ReadAttributesByID` expose ces 52 valeurs de façon ciblée pour un ID
dont l'enregistrement et l'en-tête de fermeture correspondent sans ambiguïté.
L'énumération complète et le rattachement fiable des noms restent à faire.

## Noms et couverture de la sauvegarde mise à jour

La sauvegarde locale `Binbin Carpenter - Pau FC.fm` a été mise à jour depuis
les premières mesures : elle mesure maintenant 140 408 478 octets et contient
1 295 trames, 34 noms de résumé et `ACTUAL_PLAYER = 80 217`. Le lecteur Go
localise les trois tables de noms de `game_db` et rattache l'en-tête de
fermeture à la fiche de personne. `reader.ReadIdentityByID` retrouve
`Brice Maubleu` pour l'ID 34 001 981 et `Mathys Tourraine` pour l'ID
49 054 454 sur cette sauvegarde.

Un balayage exploratoire des blocs structurés trouve 77 880 joueurs avec le
marqueur connu et 2 101 autres par la séquence positions/attributs, soit
79 981 candidats distincts. L'index annonce 80 217 joueurs : il manque 236
cas ou une validation complémentaire. La sauvegarde a ensuite été réenregistrée
et mesure 140 931 722 octets ; elle compte 79 980 candidats pour toujours
80 217 entrées `ACTUAL_PLAYER`. Le rapprochement des en-têtes de fermeture,
y compris ceux dont l'UID temporaire décroît, donne un ID à ces 79 980 fiches.
Le contrôle des huit valeurs de personnalité élimine les faux blocs de noms :
79 303 fiches ont un nom vérifié, 677 restent sans nom décodable. Ces dernières
apparaissent sous `Joueur ID …` et sont signalées séparément dans la couverture.
Une sonde des enregistrements rejetés a trouvé deux blocs nommés (Mirko Pelaia
et Matteo Bevere) avec CA à zéro et des zones d'attributs hors du format
validé. Ils restent exclus du catalogue : leurs autres champs ne peuvent pas
être présentés comme des attributs joueur fiables.

## Date, postes et clubs

La sauvegarde réenregistrée indique le 4 juin 2027 comme date en jeu. L'âge
est calculé à cette date à partir du jour et de l'année codés dans le bloc de
personne validé. Les 15 octets à `record + 24` donnent les notes de poste dans
l'ordre GK, SW, DL, DC, DR, DM, ML, MC, MR, AML, AMC, AMR, STC, WBL, WBR.
Les postes notés au moins 15 sont affichés du plus fort au plus faible.

L'ID d'équipe à `record + 16` est relié aux listes d'équipes des blocs de
clubs. Un club n'est affiché que si l'ID est relié sans ambiguïté à un seul
club. Sur la sauvegarde actuelle, 75 660 fiches ont un club résolu parmi
6 379 clubs. Brice Maubleu et Mathys Tourraine portent tous deux l'ID
d'équipe 1 651, relié à Pau FC ; leurs âges (37 et 26 ans) et postes (GK,
DR/WBR) concordent avec les captures de l'éditeur fournies par l'utilisateur.

## Genre des joueurs

Le bloc d'identité attaché à l'en-tête de fermeture contient un octet de
drapeaux à `closing_header - 7`. Le bit `0x10` marque une femme. Sur la
sauvegarde actuelle, ce contrôle répartit les 79 980 fiches validées en
65 964 hommes et 14 016 femmes. Brice Maubleu et Mathys Tourraine sont
classés hommes ; Sam Kerr et plusieurs joueuses connues sont classées femmes.
L'interface sélectionne « Hommes » à l'import `.fm` et permet aussi « Femmes »
et « Tous les joueurs ». Les imports CSV/JSON sans champ de genre restent
affichés quand « Tous les joueurs » est sélectionné.

## Rattachement des clubs aux ligues

Le répertoire FMF contient un groupe `rgman` avec des sections `comp_<ID>`.
L'ID du nom de section est celui que montre l'éditeur du jeu : `comp_17`
correspond à Ligue 2 BKT et `comp_11` à Premier League. Leurs contenus
décompressés contiennent de nombreuses occurrences des ID d'équipes premières
qui jouent dans la compétition. Sur la sauvegarde testée, les 18 clubs de
Ligue 2 et les 20 de Premier League concordent exactement avec les classements
fournis en captures par l'utilisateur. Pau FC porte l'ID d'équipe 1 651 ; cet
ID apparaît 131 fois dans `comp_17`, et n'apparaît pas dans `comp_11`.

Une simple recherche d'un entier dans `game_db` produisait des faux positifs :
les octets de petites valeurs peuvent se trouver dans d'autres champs. Le
lecteur utilise désormais les sections de compétition, regroupe les ID
d'équipes premières d'un même pays dont les fréquences sont comparables et
retient la ligue dominante pour chaque club. Il rattache ainsi 485 clubs à
29 compétitions dans la sauvegarde actuelle. Les cas insuffisamment étayés
restent « Ligue non identifiée ». Les noms de compétitions proviennent de la
table sous licence MIT incluse dans `reader/data/`.

Les sections de calendrier peuvent conserver des données de la saison passée.
Le lecteur privilégie donc désormais l'entrée de championnat identifiée dans la
fiche actuelle du club : son identifiant interne est relié à l'identifiant de
base de données via les enregistrements correspondants de `game_db`. Il conserve
la méthode par fréquence comme solution de repli quand cette entrée est absente.
Sur la sauvegarde de juillet 2027, Pau et Metz sont ainsi en Ligue 1 et Caen en
Ligue 2 ; dans la sauvegarde précédente, Pau reste en Ligue 2. Pour les sections
de calendrier, le lecteur accepte un groupe à partir de 25 occurrences par
équipe et une plage de fréquences de 1 à 1,6. La deuxième division néerlandaise
est également reconnue grâce au terme « Divisie ».
Les clubs des pays non chargés peuvent ne pas contenir assez d'informations pour
associer un championnat : notamment, la section de première division albanaise
est absente de cet échantillon et les fiches de clubs albanais et sud-africains
examinées sont courtes. Ne pas attribuer automatiquement un championnat à tous
les clubs d'un pays sur cette seule base.

Le dossier d'installation fourni par l'utilisateur,
`E:\SteamLibrary\steamapps\common\Football Manager 27`, contient la base
`shared/data/database/db/2700/2700_fm/` et d'autres fichiers FMF. Ces
fichiers ont été inspectés uniquement en lecture ; leur décodage reste une
piste pour compléter les noms et les données financières.

## Limites actuelles

Le lecteur ne sait pas encore parcourir tous les joueurs. Le catalogue affiché
par `/api/charger` contient uniquement les fiches dont la structure et l'ID
ont passé les contrôles ; la réponse donne `coverage.with_id`,
`coverage.with_name` et `coverage.indexed` pour rendre la limite visible.
Les noms absents sont explicitement signalés. La version de jeu testée est
`27.0.1+2503222` ; les autres versions ne sont pas extrapolées.

`reader.ReadAbilityByID` peut lire de façon ciblée CA, PA, taille et trois
réputations d'un ID de base de données lorsque le marqueur et l'en-tête de
fermeture correspondent sans ambiguïté. La décompression de `game_db` est
limitée à 256 MiB et cette API ne prétend pas énumérer tous les joueurs.

`reader.Read` parcourt au plus 4 096 trames et ne décompresse que la trame 2,
avec une limite de 8 MiB pour ce résumé. Il s'arrête au premier en-tête de
trame inconnu. `reader.ReadDirectory`, appelé à la fin de `reader.Read`,
valide la position du suffixe, sa trame Zstandard, les 1 287 entrées et leur correspondance exacte
avec les positions et tailles compressées des trames du flux principal, sans
décompresser `game_db`. Les tailles décompressées déclarées dans le répertoire
ne sont pas encore contrôlées par une décompression de chaque section.

## Reproduire le test d'intégration

Définir `FM27_SAMPLE` sur le chemin d'une **copie** de sauvegarde et lancer
`go test ./reader -run TestProvidedSample -v`. Les tailles des quatre
premières trames et le nombre total de trames sont contrôlés pour les deux
sauvegardes ci-dessus. Le test vérifie aussi que le lecteur ne fabrique
aucun enregistrement joueur.

## Écriture des joueurs dans la v0.11

`reader.WritePlayerEdits` localise les fiches par ID et n'autorise que CA, PA
et les 52 attributs dont l'offset a été vérifié. Il recompresse `game_db` avec
une trame Zstandard de la forme acceptée lors des essais v6–v10, décale tous
les offsets des sections suivantes dans le répertoire FMF, puis valide le
fichier temporaire complet avec `ReadDirectory`, `Read` et une comparaison des
octets décompressés de `game_db`. Sous Windows, `ReplaceFileW` remplace le
fichier `.fm` ouvert à son emplacement initial. Le test
`TestWritePlayerEditsOnCopy` exerce CA, PA, un attribut et le professionnalisme sur une copie
temporaire de `FM27_SAMPLE` et vérifie que seuls les octets prévus changent.
L'utilisateur a confirmé le chargement dans FM27 après modification.

## Personnalité des joueurs et staff dans la v0.13

Le bloc de personne validé par `findPersonIdentity` contient huit notes directes
à `birth + 22` : adaptabilité, ambition, loyauté, pression, professionnalisme,
esprit sportif, tempérament, controverse. Le catalogue les expose sous leurs
noms canoniques ; `WritePlayerEdits` modifie uniquement les octets demandés.
Le test sur copie modifie aussi le professionnalisme, contrôle les octets et
relit la valeur par le catalogue. Le libellé synthétique de personnalité reste
inconnu.

Le catalogue staff repère les blocs de membre du staff par leur en-tête, leur
fermeture, leur contrat de club et leur bloc de personne. Il expose CA/PA,
21 notes et, quand le bloc est non ambigu, 15 aptitudes aux postes.
`WriteStaffEdits` utilise la même validation et le même remplacement du `.fm`
que les joueurs. `TestWriteStaffEditsOnCopy` vérifie sur copie CA, PA, autorité,
aptitude au recrutement, les octets modifiés et la relecture du catalogue.
Le chargement de cette copie modifiée dans FM27 reste à confirmer.
