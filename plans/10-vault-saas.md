# Shuttle Vault — SaaS Secrets Manager

## Vision

Un vault cloud léger, conçu pour les développeurs VPS — pas Hashicorp (overkill), pas des `.env` sur disque (insecure). Un middle-ground qui s'intègre nativement avec `shuttle`.

```
shuttle vault set DB_PASSWORD "xxx"
shuttle vault pull
shuttle deploy
→ Les secrets sont injectés depuis le vault, jamais sur le disque
```

## Pourquoi

| Solution | Problème |
|---|---|
| `.env` sur disque | En clair, visible dans docker inspect, backups |
| Docker Secrets Swarm | Bien mais local au cluster, pas de sync multi-serveur, pas d'audit trail |
| Hashicorp Vault | $$$, complexe, overkill pour du VPS |
| AWS Secrets Manager / GCP KMS | Vendor lock-in, pas fait pour du self-hosted |

**Shuttle Vault** = un service cloud minimaliste qui stocke et distribue des secrets chiffrés, avec un CLI natif dans `shuttle`.

## Architecture

```
╔══════════════════════════════════════════════════════════════════╗
║  ARCHITECTURE SHUTTLE VAULT                                      ║
╠══════════════════════════════════════════════════════════════════╣
║                                                                  ║
║  ┌─────────────────┐         ┌────────────────────────────────┐ ║
║  │  CLI (shuttle)   │────────▶│  Vault API (vault.shuttle.dev) │ ║
║  │  vault set/pull  │  HTTPS │  Hono + Bun / Go               │ ║
║  │  deploy          │◀───────│  PostgreSQL (chiffré au repos)  │ ║
║  └─────────────────┘         └────────────────────────────────┘ ║
║         │                              │                         ║
║         │ SSH                          │ Chiffrement             ║
║         ▼                              ▼                         ║
║  ┌─────────────────┐         ┌────────────────────────────────┐ ║
║  │  VPS             │         │  Modèle de chiffrement         │ ║
║  │  Docker Secrets   │         │                                │ ║
║  │  ou /run/secrets  │         │  Client-side: XChaCha20-Poly  │ ║
║  │  (RAM only)       │         │  Server-side: AES-256-GCM     │ ║
║  └─────────────────┘         │  At-rest: PG + pgcrypto         │ ║
║                               │                                │ ║
║                               │  Le serveur ne voit JAMAIS     │ ║
║                               │  les secrets en clair           │ ║
║                               └────────────────────────────────┘ ║
║                                                                  ║
╚══════════════════════════════════════════════════════════════════╝
```

## Modèle de chiffrement (zero-knowledge)

### Principe
Le serveur ne stocke et ne voit JAMAIS les secrets en clair. Tout est chiffré côté client avant d'être envoyé.

### Clés

```
Master Key (MK)
├── Dérivée du passphrase utilisateur via Argon2id
├── N'est JAMAIS envoyée au serveur
└── Stockée nulle part — dérivée à chaque session

Envelope Key (EK)
├── AES-256 random, générée à la création du projet
├── Chiffrée avec MK → stockée sur le serveur (enveloppe chiffrée)
├── Déchiffrée côté client avec MK
└── Utilisée pour chiffrer/déchiffrer chaque secret

Secret Value
├── Chiffré avec EK + XChaCha20-Poly1305
├── Nonce unique par secret + version
└── Stocké chiffré dans PostgreSQL
```

### Flow : écriture d'un secret

```
1. CLI dérive MK depuis le passphrase (Argon2id, local)
2. CLI récupère l'enveloppe chiffrée (EK_encrypted) depuis l'API
3. CLI déchiffre EK = decrypt(EK_encrypted, MK)
4. CLI chiffre la valeur : ciphertext = encrypt(value, EK, nonce)
5. CLI envoie (key, ciphertext, nonce) à l'API
6. API stocke dans PostgreSQL (jamais vu le plaintext)
```

### Flow : lecture d'un secret

```
1. CLI dérive MK (Argon2id)
2. CLI récupère EK_encrypted + (key, ciphertext, nonce) depuis l'API
3. CLI déchiffre EK
4. CLI déchiffre value = decrypt(ciphertext, EK, nonce)
5. CLI injecte dans Docker Secret ou /run/secrets/
```

## API

### Endpoints

```
POST   /api/v1/auth/login          → JWT token
POST   /api/v1/auth/register       → Créer un compte

POST   /api/v1/projects            → Créer un projet (+ generate EK)
GET    /api/v1/projects             → Lister les projets
DELETE /api/v1/projects/:id         → Supprimer un projet

GET    /api/v1/projects/:id/envelope   → Récupérer EK_encrypted
POST   /api/v1/projects/:id/envelope   → Stocker EK_encrypted (setup initial)

GET    /api/v1/projects/:id/secrets             → Lister les secrets (keys + ciphertext)
PUT    /api/v1/projects/:id/secrets/:key        → Créer/MAJ un secret
DELETE /api/v1/projects/:id/secrets/:key        → Supprimer un secret
GET    /api/v1/projects/:id/secrets/:key/history → Historique de versions

POST   /api/v1/projects/:id/pull    → Récupérer tous les secrets (bulk, pour deploy)
```

### Auth
- JWT (access 15min + refresh 7j)
- API key pour CI/CD : `SHUTTLE_VAULT_TOKEN=svt_xxx`
- Le token donne accès au pull mais PAS au déchiffrement (le client a besoin du passphrase)

### Rate limiting
- 100 req/min par token
- Pull bulk : 10 req/min (anti-scraping)

## Base de données

```sql
-- Projets
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    envelope BYTEA NOT NULL,        -- EK chiffré avec MK (client-side)
    envelope_salt BYTEA NOT NULL,   -- Salt Argon2id pour dériver MK
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Secrets (chiffrés client-side)
CREATE TABLE secrets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id),
    key TEXT NOT NULL,               -- Nom du secret (en clair, c'est juste le nom)
    ciphertext BYTEA NOT NULL,       -- Valeur chiffrée (XChaCha20-Poly1305)
    nonce BYTEA NOT NULL,            -- Nonce unique
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, key)
);

-- Historique (audit trail)
CREATE TABLE secret_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    secret_id UUID NOT NULL REFERENCES secrets(id),
    ciphertext BYTEA NOT NULL,
    nonce BYTEA NOT NULL,
    version INT NOT NULL,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Audit log
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    user_id UUID,
    action TEXT NOT NULL,            -- 'secret.read', 'secret.write', 'secret.delete', 'pull'
    key TEXT,                        -- Quel secret (NULL pour les actions projet)
    ip INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

## CLI — Commandes vault

```bash
# Auth
shuttle vault login                     # Login interactif
shuttle vault login --token svt_xxx     # Login par API key (CI)

# Projet
shuttle vault init                      # Lier le projet courant au vault
shuttle vault status                    # Afficher l'état de connexion

# Secrets
shuttle vault set DB_PASSWORD "xxx"     # Chiffre + envoie au vault
shuttle vault get DB_PASSWORD           # Récupère + déchiffre
shuttle vault list                      # Liste les clés (pas les valeurs)
shuttle vault rm DB_PASSWORD            # Supprime du vault

# Sync avec le VPS
shuttle vault pull                      # Récupère tous les secrets → Docker Secrets
shuttle vault pull --env-file           # Récupère → écrit .env.secrets (fallback compose)

# Intégration deploy
shuttle deploy                          # Auto-pull les secrets avant deploy
  → Si vault configuré : pull + inject Docker Secrets
  → Si pas de vault : fallback .env.secrets classique

# Historique
shuttle vault history DB_PASSWORD       # Versions précédentes
shuttle vault rollback DB_PASSWORD 3    # Revenir à la version 3

# Rotation
shuttle vault rotate DB_PASSWORD        # Génère une nouvelle valeur + redeploy
```

## Configuration

### shuttle.yml
```yaml
vault:
  url: https://vault.shuttle.dev     # ou self-hosted
  project: prj_xxx                    # ID projet
  auto_pull: true                     # Pull automatique au deploy
```

### Variables d'environnement (CI)
```
SHUTTLE_VAULT_URL=https://vault.shuttle.dev
SHUTTLE_VAULT_TOKEN=svt_xxx
SHUTTLE_VAULT_PASSPHRASE=xxx         # Pour déchiffrer les secrets
```

## Pricing

| Tier | Prix | Secrets | Projets | Audit |
|---|---|---|---|---|
| Free | 0€ | 10 secrets | 1 projet | 7 jours |
| Pro | 9€/mois | Illimité | 10 projets | 90 jours |
| Team | 29€/mois | Illimité | Illimité | 1 an |
| Self-hosted | Open-source | Illimité | Illimité | Illimité |

## Stack technique

### Option A : Bun + Hono (cohérent avec le license-server existant)
- Runtime : Bun
- Framework : Hono
- DB : PostgreSQL (Neon ou Supabase)
- Auth : JWT (jose)
- Crypto : Web Crypto API (browser-compatible)
- Deploy : Fly.io ou Cloudflare Workers

### Option B : Go (cohérent avec le CLI)
- Framework : net/http + chi
- DB : PostgreSQL (pgx)
- Auth : JWT (golang-jwt)
- Crypto : golang.org/x/crypto (Argon2id, XChaCha20-Poly1305)
- Deploy : single binary sur Fly.io

### Recommandation : Go
- Même langage que le CLI (shared crypto code)
- Single binary, facile à self-host
- Performance native pour le crypto
- Pas de runtime dependency

## Roadmap

### Phase 1 — MVP (2 semaines)
- [ ] API Go : auth, projets, CRUD secrets, pull bulk
- [ ] CLI : `vault login/set/get/list/pull`
- [ ] Chiffrement zero-knowledge (Argon2id + XChaCha20)
- [ ] PostgreSQL schema + migrations
- [ ] Deploy sur Fly.io
- [ ] Intégration `shuttle deploy --vault`

### Phase 2 — Production (2 semaines)
- [ ] Audit trail (qui a lu/écrit quoi, quand)
- [ ] Historique de versions + rollback
- [ ] API keys pour CI/CD
- [ ] Rate limiting
- [ ] Dashboard web minimal (lister projets/secrets, pas voir les valeurs)
- [ ] Documentation

### Phase 3 — Growth
- [ ] Team management (inviter des membres, RBAC)
- [ ] Rotation automatique (webhook vers le VPS)
- [ ] Alertes (secret non-rotaté depuis X jours)
- [ ] Self-hosted version (Docker image)
- [ ] Intégration GitHub Actions (inject secrets dans les workflows)
- [ ] SDK JS/PHP pour lire les secrets au runtime (alternative aux env vars)

## Sécurité — Threat model

| Menace | Protection |
|---|---|
| Serveur vault compromis | Zero-knowledge : secrets chiffrés client-side |
| BDD PostgreSQL leakée | Chiffrement at-rest (pgcrypto + client-side encryption) |
| Man-in-the-middle | HTTPS + certificate pinning dans le CLI |
| Brute force passphrase | Argon2id (time=3, memory=64MB, parallelism=4) |
| Token volé | Le token donne accès au ciphertext, pas au plaintext (besoin du passphrase) |
| Insider threat (nous) | Zero-knowledge : on ne peut PAS lire les secrets même avec accès DB |
| Replay attack | Nonce unique par secret + version |
| VPS compromis | Secrets en RAM (/run/secrets/), pas sur disque ; rotation rapide via vault |

## Avantage compétitif vs alternatives

| | Shuttle Vault | Hashicorp Vault | Doppler | Infisical |
|---|---|---|---|---|
| Zero-knowledge | ✓ | ✗ (serveur déchiffre) | ✗ | ✓ |
| Intégration CLI deploy | Natif | Plugin | SDK | SDK |
| Self-hostable | ✓ | ✓ | ✗ | ✓ |
| Complexité setup | 0 (shuttle vault init) | Élevée | Faible | Moyenne |
| Prix entry | Free | Free (OSS) / $$$ (cloud) | Free / 18$/mois | Free / 8$/mois |
| Cible | Devs VPS | Enterprise | Startups | Startups |
