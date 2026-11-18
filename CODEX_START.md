# Brief à donner à Codex

Je développe **PauScout FM27**, un lecteur/scout Windows local pour Football Manager 2027. Le dépôt contient les sources Go 1.23 (backend HTTP + lecteur FMF/Zstandard intégré), `web/index.html` (front-end vanilla JS) et un exécutable Windows compilé. La sauvegarde personnelle de test reste dans le dossier `games` de FM27 ; aucune copie `.fm` n'est conservée dans ce projet.

**État actuel v0.13 :** pour le build vérifié `27.0.1+2503222`, `/api/charger` lit 79 980 fiches liées à un ID sur 80 217 entrées `ACTUAL_PLAYER` annoncées. Il retrouve 79 303 noms, 75 660 clubs, la date en jeu, l'âge quand la personne est identifiée, les postes, CA/PA et 52 attributs. Le drapeau de genre répartit les fiches en 65 964 hommes et 14 016 femmes ; l'interface sélectionne les hommes par défaut à l'import FM. Les fiches sans nom portent le libellé explicite `Joueur ID …`. Brice Maubleu et Mathys Tourraine ont servi de témoins vérifiés avec des captures de l'éditeur. Les CSV/JSON sont aussi acceptés. La navigation suit pays → ligue → club → équipe ; les équipes du Pau FC sont séparées par leur ID. Le lecteur rattache 485 clubs à 29 compétitions dans la sauvegarde testée ; les classements complets de Ligue 2 et Premier League concordent avec les captures de l'éditeur. L'interface peut ouvrir un `.fm` local, modifier CA/PA et 52 attributs, puis réécrire le fichier existant. L'écrivain valide la structure et les octets sur une copie temporaire avant remplacement. Le test automatisé sur copie passe ; l'utilisateur a confirmé le chargement dans FM27 après modification.

Évolution v0.13 : les huit valeurs de personnalité des joueurs sont affichées et modifiables quand le bloc de personne est identifié. Un onglet Staff séparé lit et modifie CA/PA, 21 attributs et jusqu’à 15 aptitudes aux postes. Les tests d’écriture passent sur copie temporaire ; le chargement dans FM27 après une modification du staff reste à vérifier dans le jeu. Le libellé synthétique de personnalité et les autres données du staff (licence, réputation, tactique) restent à décoder.

Évolution v0.14 : recherche globale des joueurs et du staff par nom ou ID, nationalité, genre, poste ou fonction, âge, fourchettes CA/PA et plusieurs attributs cumulés. La valeur des joueurs est enregistrée en livres ; l'interface permet de la lire, filtrer et modifier en livres ou en euros, avec le taux présent dans la sauvegarde. Les valeurs en euros de Yoon Sungjun et Kosei Tani ont été vérifiées avec l'utilisateur. Le salaire est disponible dans les imports qui le fournissent. Le grand panneau de diagnostic « Sauvegarde reconnue » a été retiré de l'interface.

Suite à travailler :
L'interface v0.13 utilise des menus en cascade obligatoires (pays → ligue → club → équipe), des libellés français et six catégories d'attributs. Conserver ces repères dans les évolutions suivantes.

1. Comprendre les 237 entrées d'écart entre le compteur `ACTUAL_PLAYER` et les fiches structurées. Ne pas fabriquer des joueurs pour atteindre artificiellement le compteur.
2. Examiner les 677 fiches sans nom et les 4 320 fiches sans club résolu ; conserver des libellés honnêtes si le lien manque.
3. Valider la lecture sur d'autres sauvegardes et builds avant d'étendre la compatibilité.
4. Garder les tests de non-régression et la compilation Windows sans dépendance runtime.
5. Étendre la résolution des ligues encore inconnues et des catégories des équipes des autres clubs à partir des compétitions et calendriers de la sauvegarde ; valider sur d'autres parties avant de généraliser.
6. Ajouter les finances du club (budgets, salaires et autres montants), avec le même contrôle de format et des tests sur copie.

Contraintes : ne jamais écrire dans la sauvegarde réelle pendant les tests ; l'écriture utilisateur remplace uniquement le `.fm` explicitement ouvert, après validation d'une copie temporaire. Pas de dépendance au service web fmsave, aucune installation de Python chez l'utilisateur, interface en français, gestion d'erreurs explicite, ne pas faire passer les noms du résumé pour la base complète. Conserver les licences `reader/zstd/LICENSE` et `reader/data/ZSTD_LICENSE`.

Fichiers : `main.go`, `reader/catalog.go`, `reader/names.go`, `reader/clubs.go`, `reader/dates.go`, `reader/positions.go`, `reader/reader_test.go`, `reader/zstd/*`, `web/index.html`. Les preuves et limites détaillées sont dans `FMF_FORMAT.md`.
