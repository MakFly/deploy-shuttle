# Versioning & Release

## Version actuelle

Toujours vérifier le dernier tag avant de release :
```bash
git tag -l 'v*' --sort=-v:refname | head -1
```

## Utilisation

```bash
sh scripts/release.sh patch   # v2.0.0 → v2.0.1
sh scripts/release.sh minor   # v2.0.1 → v2.1.0
sh scripts/release.sh major   # v2.1.0 → v3.0.0
sh scripts/release.sh v2.5.0  # version explicite
```

## Ce que fait le script

1. Vérifie que le working tree est propre
2. Lance `go vet` + `go test`
3. Calcule la prochaine version depuis le dernier git tag
4. Build le binaire avec la version injectée via ldflags
5. Installe dans `~/.local/bin/shuttle`
6. Crée le git tag annoté
7. Affiche la commande push à exécuter

## Publication

```bash
git push origin main && git push origin <tag>
```

Cela déclenche `.github/workflows/release.yml` qui :
- Cross-compile pour linux/darwin × x64/arm64
- Crée une GitHub Release avec les binaires + `checksums.txt`
- Rend `curl -fsSL .../install.sh | sh` fonctionnel pour tous

## Auto-update

- `shuttle` vérifie GitHub toutes les 24h au lancement (non-bloquant, 3s timeout)
- Affiche "nouvelle version dispo → shuttle update" si une release est plus récente
- `shuttle update` → télécharge le binaire depuis GitHub Releases et remplace l'actuel
- `shuttle uninstall` → supprime le binaire + `~/.shuttle/`

## Règles de bump

- **patch** : bugfix, typo, correction mineure sans changement d'API
- **minor** : nouvelle feature, nouveau check doctor, nouvelle commande (rétrocompatible)
- **major** : breaking change (rename CLI, changement d'env vars, changement de format config)

## Convention de commits

Les commits doivent utiliser conventional commits pour que `generate_release_notes` dans le workflow produise un changelog lisible :
- `feat(scope):` → nouvelle feature
- `fix(scope):` → correction de bug
- `refactor:` → restructuration sans changement de comportement
- `ci:` → changement CI/workflows
- `docs:` → documentation

## Ne JAMAIS faire

- Ne pas push un tag sans avoir run `scripts/release.sh` d'abord (il valide les tests)
- Ne pas éditer la version manuellement dans le code — elle est injectée par ldflags au build
- Ne pas créer de tag lightweight (`git tag v1.0.0`) — utiliser le script qui fait un tag annoté
- Ne pas supprimer ou déplacer des tags déjà publiés sur GitHub (casse les installs existantes)
