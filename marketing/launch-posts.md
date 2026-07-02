# Launch posts — DeployShuttle v1.0

Drafts ready to copy/paste. Edit the screenshot URLs and your handles before
publishing. Order suggested: HN first (Tuesday morning EST), then Reddit
within the hour, LinkedIn the same day.

---

## 1. Hacker News (Show HN)

**Title** (under 80 chars, no emoji, no marketing words):

```
Show HN: DeployShuttle – CLI that audits VPS production-readiness in 30s
```

**Body** (~150 words, the HN sweet spot):

```
I run a few client apps on cheap VPS (Hetzner, Contabo, OVH). Every time
I deploy I forget something — public Postgres port, no UFW, .env
world-readable, Caddy admin API exposed, no backups. Eventually a client
asks "is this production-ready?" and I have no answer.

DeployShuttle is the answer: one command, 43 checks, deterministic 0–100
score, exits non-zero on critical findings so you can wire it into CI.

  curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh
  shuttle doctor --target root@server

Covers system, ssh, docker, firewall, secrets, reverse-proxy, TLS, DNS,
monitoring, backups, compose, cloudflare. Each finding maps to a
remediation + a `harden` plan you can dry-run.

CLI is open-core. Hosted reports + scheduled scans are the paid tier
(WIP). Github: https://github.com/MakFly/deploy-shuttle

Curious what you'd want flagged that I'm missing.
```

**Why this works**:
- Personal pain story (not "we built", "I run")
- Concrete commands above the fold
- 43 checks = quantifiable
- Asks for feedback at the end (drives comments)
- One link only (HN penalizes link-heavy posts)

**Posting hygiene**:
- Tuesday or Wednesday, 9–11am EST
- Post yourself, don't ask others to upvote
- Reply to every comment in the first 2 hours
- If it stalls under 5 points after 1h, don't repost — it's burned

---

## 2. Reddit r/selfhosted

**Title**:

```
I built a CLI that scans your VPS for production risks (Docker, Caddy, firewall, backups)
```

**Body**:

```
Hi r/selfhosted, sharing a tool I made to scratch my own itch.

Background: I host a few side projects + client apps on Hetzner CAX
(arm64) and Contabo. After the third "oh no, Postgres was on 0.0.0.0
the whole time" moment I built DeployShuttle.

What it does:
- SSH into your VPS, runs 43 checks, gives a 0-100 score
- Findings cover: docker security, UFW, exposed DB ports, Cloudflare
  SSL mode (no more Flexible-by-mistake), TLS cert expiry, secret
  file perms, missing backups, compose latest tags, etc.
- Generates a markdown report you can paste into Notion/Obsidian
- Has a `harden` command with a dry-run that shows you what it'd fix

Demo on my dev box (real output, real findings):

  Score: 0/100 — Not Production Ready
  Critical:
    [x] Public sensitive database listeners detected: 5432, 7700, 6379
  High:
    [x] 3 sensitive Docker port mapping(s) on 0.0.0.0
    [x] UFW is missing or inactive
    [x] No recent backup artifacts and no pg_dump cron entry detected
  Medium:
    [x] 57 pending package update(s)
    [x] sshd defaults to port 22

Install:
  curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh

Repo: https://github.com/MakFly/deploy-shuttle

Free, MIT-friendly for the CLI. Let me know what checks I should add.
What do you wish someone had flagged on your VPS before it bit you?
```

**Why this works**:
- "I built ... for myself" framing (anti-marketing)
- Real screenshot of failures (proves the tool actually finds stuff)
- Mentions Hetzner CAX explicitly (subreddit knows it = trust signal)
- Open-ended question at the end

**Posting hygiene**:
- Avoid weekends (less engagement on selfhosted)
- Don't crosspost to r/devops the same day; wait 48h
- If a moderator removes for "self-promotion", DM them with proof of effort
  (this draft, the README, the test coverage)

---

## 3. LinkedIn (FR)

**Public**: freelances/agences francophones (Paris, Lyon, Bruxelles, Geneve, MTL).

**Post (~200 mots)**:

```
J'ai shippe DeployShuttle v1.

Le pitch en une ligne : un CLI qui audite ton VPS et te dit s'il est
pret pour la prod, en 30 secondes.

Pourquoi je l'ai construit : chaque fois que je livre une app a un
client sur un VPS Hetzner ou Contabo, j'oublie un truc. Postgres
expose sur 0.0.0.0. Caddy admin API joignable. .env en mode 644. Pas
de backup. Pas de fail2ban. Tu sais que ca marche, mais tu ne sais
pas si c'est pro.

DeployShuttle fait passer 43 checks (docker, firewall, TLS, Cloudflare,
backups, compose, secrets...) et te sort un score 0-100. Si t'es
sous 75 le CI plante. Le rapport markdown se colle dans la passation
au client, et tu sais quoi expliquer.

Une commande, pas de cloud, pas de compte :

curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh
shuttle doctor --target root@ton-serveur

Open-source pour le CLI. Les rapports HTML/PDF brandes pour les
livrables client sont dans le tier Pro (199 EUR une fois, licence a vie).

Lien : https://github.com/MakFly/deploy-shuttle

Si t'es freelance ou agence, ca m'interesse de savoir ce que tu
verifies a la main aujourd'hui pour ton workflow de livraison.

#devops #freelance #vps #docker
```

**Why this works**:
- Pas d'anglicisme inutile (LinkedIn FR conservateur)
- "j'ai shippe" en titre = tu sors un truc, pas une etude
- Question ouverte qui appelle des reponses concretes (algo LinkedIn aime)
- Pas de shilling, pas de "revolutionary", pas de "je suis fier"
- Mention du tier Pro sans lien direct = teasing sans agressivite

**Posting hygiene**:
- Mardi ou jeudi, 8h30 ou 13h CET
- Reponds dans la premiere heure a tous les commentaires
- Si quelqu'un demande le lien Stripe, donne en DM (pas en commentaire
  public — l'algo LinkedIn pénalise les liens de paiement)
- Tagge 0 personne dans le post (l'algo deteste)

---

## 4. Tweet (optionnel, court)

```
Shipped DeployShuttle v1.

CLI that audits a VPS for production-readiness in 30s.
43 checks. Deterministic score. Exits non-zero in CI on critical findings.

  curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh
  shuttle doctor --target root@server

https://github.com/MakFly/deploy-shuttle
```

---

## 5. Suivi post-lancement

**Première semaine**

- [ ] Repondre a chaque commentaire HN sous 1h pendant les 6 premieres heures
- [ ] Repondre a chaque comment Reddit + DM
- [ ] Noter les checks demandes -> issue GitHub avec label `from-launch`
- [ ] Ajouter "As seen on Hacker News" en haut du README si HN > 50 points

**Funnel de vente (pricing decide 02-07-2026 : Pro unique 199 EUR one-time)**

- Stripe Payment Link "DeployShuttle Pro - 199 EUR one-time" branche sur
  la pricing page (`PUBLIC_STRIPE_PAYMENT_LINK` au build du docs-site).
  Cible : 2 ventes la premiere semaine = validation que les gens paient
  pour le tier. Pas d'offre Early Bird separee, pas d'audit manuel a
  99 EUR sur la page — le prix EST l'offre.

**Si HN/Reddit > 50 upvotes total**

Article SEO long-form sur ton blog ou dev.to :
- "I scanned 100 VPS and here's what I found" (rejoue les findings agreges)
- "Why your $5 VPS is leaking your database to the internet"
- "How to make Hetzner CAX production-ready in 5 minutes"

Chaque article = 1 lien vers la landing + 1 vers le repo, pas plus.
