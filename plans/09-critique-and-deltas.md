# Critique du PRD v2 et plan delta

Source du PRD pivoté: voir `01-product-prd.md` à `07-docs-and-implementation-prompt.md`.

Ce document n'invalide pas le PRD : il en cible les zones faibles et fixe le
plan d'exécution pour les 4 prochaines semaines, en s'appuyant sur ce qui est
déjà livré (cf. `08-execution-tracker.md`).

## 1. Ce qui est solide dans le PRD

- Repositionnement défendable : "production readiness" est plus net que
  "yet another deploy CLI" et différencie sans frontalité avec Coolify/Dokploy.
- Hook `doctor --target` démontrable en 30 secondes.
- Catalogue de checks granulaire avec sévérité + auto-fix.
- CLI-first, cloud-optionnel : la confiance se gagne avant le compte.
- Score déterministe + `--fail-below` : c'est le levier viral CI.
- Aligné avec l'implémentation Go existante (21 checks, `harden` dry-run+apply).

## 2. Faiblesses à corriger

### 2.1 Distribution non spécifiée

Le PRD vise "100 installs MVP" sans plan d'acquisition. Sans canal, le meilleur
CLI ne sort pas de zéro. À ajouter au plan d'exécution :

- GitHub Action publique (`makfly/deploy-shuttle-action@v1`) — multiplicateur CI.
- Articles SEO long-tail : "Top 10 VPS production mistakes",
  "How to audit a Hetzner VPS", "Ploi vs Forge vs DeployShuttle".
- Lancement Hacker News + r/selfhosted + r/devops avec un rapport HTML
  publiquement partageable comme proof.
- Badge `production-ready: 92/100` à coller dans les README.

### 2.2 Moat non verbalisé

Les checks bash sont copiables en un week-end. Les vrais moats :

1. **Base de remediation curée et testée sur de vrais VPS** (pas du LLM-slop).
2. **Rapports HTML/PDF brandés** pour handoff client (white-label en Agency).
3. **Historique cloud + scheduled scans** (verrou récurrent une fois adopté).
4. **Profils stack-aware** (Laravel, Next.js, Docker Swarm) avec faux positifs
   bas — exigeant un travail terrain difficile à reproduire.

### 2.3 Pricing à recalibrer

Hypothèse PRD : 19€ one-shot / 9€ Solo / 29€ Pro / 79€ Agency.

Problèmes :

- **19€ one-shot** : faible willingness-to-pay quand le CLI est gratuit et que
  le rapport HTML local est déjà disponible. À garder comme ancre psychologique
  mais pas comme moteur de revenu.
- **9€ Solo** : probablement mort-né. Les indie hackers ne paient pas pour
  surveiller un seul VPS qu'ils maîtrisent déjà.
- **79€ Agency** : c'est ici que se trouve la vraie willingness-to-pay
  (white-label PDF + multi-clients). C'est ce tier qu'il faut polir en
  priorité dès qu'on construit le cloud.

Reframe proposé :

```
CLI                 Free / open-core
Pro      29€/mo     scans planifiés, alertes email, 5 serveurs, html/pdf
Agency   99€/mo     white-label PDF, workspaces clients, 25 serveurs
```

Pas de tier 9€. Les rapports one-shot servent de funnel vers Pro, pas de SKU.

### 2.4 Sécurité de `harden` sous-spécifiée

Auto-modifier UFW en SSH sur un VPS de prod = un bug et le serveur est coupé.
Manque dans le PRD :

- Mode "génère un script bash signé, je l'exécute moi-même" en plus du dry-run.
- Snapshot/rollback de la configuration UFW avant tout `ufw deny`.
- Refus systématique d'exécuter si la session SSH passe par le port qu'on
  s'apprête à fermer.

Déjà fait : allow-list stricte (chmod 600 .env, ufw deny `<port>/tcp`).

### 2.5 Faux positifs non gérés systématiquement

Un check qui flag à tort un setup volontaire (Postgres exposé pour réplication
cross-region) détruit la crédibilité. Acquis : `.deployshuttle.yml` supporte
`checks.ignore` + `docker.allowDockerSocket` + `workerServices`.

Manque encore :

- Champ `justification` obligatoire pour chaque ignore, rendu dans le rapport
  comme "accepted risk with rationale" plutôt que masqué.
- Profils stack (`profile: laravel`, `profile: nextjs`) qui pré-ignorent
  les checks non pertinents.

### 2.6 Cloudflare via API token = friction

Le PRD impose un token CF pour les checks Cloudflare. Beaucoup d'utilisateurs
ne fourniront pas le token au premier essai. Prévoir un mode `manual:` avec
5 questions interactives, qui produit les mêmes findings qu'un check API.

### 2.7 Acceptance criteria MVP périmés

Le PRD demande "15 checks". On en a 21. La barre MVP est passée. La nouvelle
barre devrait être :

- 30 checks couvrant 8 catégories au moins.
- HTML report publique partageable (hosté ou statique).
- GitHub Action publiée.
- `init --preset` pour Next.js, Laravel, Node API.
- 50 utilisateurs CLI avec ≥ 2 scans en 30 jours.

## 3. Deltas check par check (catalogue 56 vs implémenté 21)

Implémenté (21) :

```
system   : os_supported, disk_space_low, swap_missing, time_sync_inactive,
           unattended_upgrades_inactive, fail2ban_inactive
ssh      : root_login_enabled, password_auth_enabled
firewall : ufw_inactive, database_port_public
docker   : not_installed, service_not_enabled, containers_without_restart_policy,
           containers_without_healthcheck, containers_running_as_root, sock_exposed
caddy    : not_installed, admin_exposed
adminer  : ip_restriction_missing
secrets  : env_in_git, env_world_readable
```

Manquant (35), avec priorité (P0 = bloquant pour le MVP "credibility floor 30",
P1 = utile, P2 = nice-to-have) :

```
P0 — utiles immédiatement, faible complexité
  system.updates_pending          (apt list --upgradable | wc -l)
  system.memory_low               (free -m parsing)
  ssh.fail2ban_missing            (déjà couvert par system.fail2ban_inactive — fusionner ou renommer)
  ssh.port_default                (sshd_config Port == 22)
  firewall.docker_published_sensitive_ports (docker ps + 0.0.0.0)
  caddy.no_security_headers       (grep Caddyfile pour HSTS/X-Content-Type-Options)
  caddy.invalid_config            (caddy validate)
  tls.cert_missing                (curl -vI sur le domaine)
  tls.cert_expiring_soon          (openssl x509 -enddate)
  secrets.weak_file_permissions   (find . -name '.env*' -perm /o+r)
  monitoring.no_health_endpoint   (curl <domain>/health)

P1 — nécessitent un peu d'inférence
  compose.missing_prod_file
  compose.env_file_missing
  compose.latest_tag_used
  compose.no_resource_limits
  compose.bind_mount_sensitive_paths
  caddy.no_access_logs
  tls.hsts_missing                (header probe)
  dns.domain_not_pointing_to_server
  db.redis_public                 (extension de database_port_public)
  db.no_backup_detected           (heuristique cron + dump.sql)
  db.backup_not_recent
  db.volume_not_persistent        (docker inspect sur les services DB)
  monitoring.no_log_rotation
  backup.no_strategy

P2 — Cloudflare (token requis) + monitoring fin
  cloudflare.ssl_flexible
  cloudflare.proxy_disabled
  cloudflare.always_https_disabled
  cloudflare.waf_disabled
  cloudflare.dns_missing
  cloudflare.origin_exposed
  monitoring.no_uptime_check
  monitoring.no_alerting
  backup.no_restore_test
  backup.local_only
  backup.no_retention_policy
  docker.unused_images_large
  secrets.example_missing
```

P0 ajoute 11 checks → 32 au total. C'est l'objectif credibility floor.

## 4. Top 3 livrables prochains (avant le dashboard)

Critère : ce qui débloque la distribution sans construire de Next.js.

### 4.1 GitHub Action publique

Action composite à `action.yml` du repo, utilisable comme :

```yaml
- uses: makfly/deploy-shuttle-action@v1
  with:
    target: deploy@server.example.com
    fail-below: 75
```

Fait : tag `v1`, README avec exemple, badge "production readiness" généré.

### 4.2 Pack P0 checks (11 checks)

Lever la barre à 32 checks. Catégories renforcées : tls, monitoring,
compose, ssh.port_default, firewall.docker_published_sensitive_ports.

### 4.3 `init --preset`

Génération de `.deployshuttle.yml` opinionné par stack, qui pré-ignore les
checks non pertinents :

```
deploy-shuttle init --preset nextjs
deploy-shuttle init --preset laravel
deploy-shuttle init --preset node-api
deploy-shuttle init --preset docker-swarm
```

C'est ce qui fait passer un nouveau projet de "ça flag 15 trucs faux" à
"ça flag 3 trucs réels" en 10 secondes — le déclic d'adoption.

## 5. Dashboard Next.js : critères de déclenchement

Ne pas construire avant d'avoir, simultanément :

- ≥ 50 personnes lancent `doctor` au moins 2 fois sur 30 jours (rétention).
- ≥ 5 freelances ou agences demandent le white-label PDF.
- ≥ 3 paiements one-shot encaissés sur le rapport hosté.

Tant que ces 3 signaux ne sont pas réunis ensemble, le dashboard est une
distraction de 4 à 6 semaines pendant lesquelles le CLI ne progresse pas.

Quand on le construit, ordre minimal :

1. Auth + upload de rapport JSON depuis CLI (`deploy-shuttle report --push`).
2. Vue rapport hosté + lien partageable (la seule feature qui justifie un compte).
3. Historique multi-scan d'un même serveur.
4. Scheduled scans (cron côté serveur qui SSH vers les VPS).
5. Multi-server + équipes en dernier.

Piège classique : commencer par auth + organisations + RBAC pendant 3 semaines
avant d'avoir une feature qui justifie un compte. Inverse : "upload + lien
public partageable" sans auth d'abord.

## 6. Risques résiduels

- **Distribution** : le succès dépend autant du contenu (articles SEO,
  démos vidéo) que du code. Sans ça, la GitHub Action reste invisible.
- **Concurrence** : si Coolify ajoute un onglet "readiness", la fenêtre se
  ferme. Il faut occuper le terrain rapidement avec du contenu.
- **Support** : `harden --apply` distant peut détruire un serveur client.
  Garder le rate-limit allow-list strict tant qu'il n'y a pas de snapshot.
