# Reprise — reify

## À quoi sert le projet

Reify lit les fichiers de contexte d'agents de code (`CLAUDE.md`, `copilot-instructions.md`, `.cursorrules`…) et les compile vers d'autres harnais, ou les génère depuis une source YAML unique (skills et agents avec contrats `consumes`/`produces`, garde-fous, sécurité). Il analyse aussi les fichiers existants : `classify` range chaque instruction dans l'une des 5 facettes (context, strategy, guardrails, observability, security), `doctor` détecte les manques structurels, `check` estime quelles instructions risquent d'être ignorées par harnais. `import` décompose un fichier de contexte en skills réutilisables. CLI en Go (Cobra), providers LLM : Ollama (local), Anthropic, OpenRouter. Le dépôt se compile lui-même (`make dogfood`).

## État au 2026-11-02

- Branche principale : `main` (GitHub `mirandaguillaume/reify`, public), dernier commit `a919c3f` du 2026-06-04 « feat(aider): aider target … (#11) ». Arbre propre, aucun stash, pas de branche `wip/sauvegarde-*`.
- README : « pre-1.0 », CLI opérationnelle de bout en bout sur des projets OSS ; le framework d'évaluation runtime est volontairement absent de cette version.
- Cibles de build livrées : `claude`, `copilot`, `cursor`, `agents` (AGENTS.md), `aider`, `reify` (runtime Go autonome) ; portage inter-harnais avec fidélité par pilier et avertissement quand un champ MCP non modélisé est perdu (PR #8 à #11). Tout le code source de `pkg/` est public et importable.
- Batterie de calibration du classifieur : binaire séparé `reify-calibrate` (sous-commandes d'échantillonnage stratifié, `judge`, `score` avec précision/rappel/F1/kappa, `explore`) ; résultats et rubrique dans `docs/calibration/` (`findings.md`, `rubric.md`, `REGENERATING.md`).
- Branches distantes et locales encore ouvertes (toutes poussées, identiques à leur copie GitHub) :
  - `feat/eval-harness` (27 commits d'avance, PR #7) : binaire `cmd/reify-eval`, mesure de la fidélité des instructions (extraction des appels d'outils, fixtures d'intention (secrets, dépendances, suppression, test), prompts « lost in the middle », `eval-run-005` formulation × longueur × position).
  - `feat/calibrate-fewshot` (6 commits, runs 7 et 8 : ancrage few-shot) et `feat/calibrate-selfconsistency` (3 commits, run 7), correspondant à la PR #6.
  - `docs/vision-shibboleth-moat` (3 commits) : note de direction « le fossé = la conformité », spec du chantier A (eval harness, phase 1) et son plan.
  - Les autres branches distantes (`dependabot/*`, `chore/*`, `feat/aider-target`…) sont des branches de PR déjà fusionnées (aider, harness-targets, dogfood…) ou de mises à jour de dépendances.

## Installer et lancer

Prérequis : Go 1.25 (`go.mod`), `make`. Optionnel : `gremlins` (mutation).

```bash
go build ./cmd/reify                  # ou make build  → ./reify
go build ./cmd/reify-calibrate        # make build-calibrate
make test            # go test ./...
make test-race       # avec détecteur de courses (c'est ce que fait la CI)
make cover           # couverture ; make cover-html
make vet
make validate        # tests -tags validation : schéma officiel des réglages Claude Code + vrai CLI `claude mcp` (sautés si absents)
make dogfood         # compile skills/ et agents/ du dépôt vers tous les harnais (sortie ignorée par git)
make mutation        # gremlins sur les paquets purs (nightly en CI)

reify init
reify build --target claude|copilot|cursor|agents|aider|reify
reify classify <fichier> --provider anthropic --model claude-haiku-4-5
reify doctor <fichier> ; reify check <fichier> ; reify import <fichier>
```

CI : `ci.yml` (go mod tidy, vet, tests `-race` + Codecov, build, dogfood) et `mutation.yml` (nightly, gremlins).

## Où j'en étais et prochaines étapes

1. PR #7 (`feat/eval-harness`) : corriger le blocage du balayage LITM (un cell à 1500 tokens a tourné ~77 min sans sortie) en ajoutant un timeout par cell/tentative, puis finir les longueurs 1500 et 3000 ; fusionner ensuite, car c'est ce qui fonde l'argument « fossé de conformité » (résultat de `eval-run-005` : formulation douce qui s'effondre sous remplissage, formulation forte robuste).
2. PR #6 (calibrate : auto-cohérence + few-shot) : relire et fusionner.
3. Cible `windsurf` : non faite (interface graphique sans validation headless, valeur faible, fidélité documentaire seulement).
4. Validation externe des générateurs hors Claude : seul `claude` est validé contre une vérité externe (schéma SchemaStore et vrai `claude mcp`) ; cursor/copilot reposent sur la documentation faute de CLI installé et de schéma MCP publié ; étendre les tests `//go:build validation` quand un outil ou schéma apparaît.
5. Robustesse de l'import : `ImportProjectConfig` est peu testé sur les hooks (matchers regex, autres types d'événements).
6. Provider Anthropic : ajouter un retry avec backoff sur 429/529 (aujourd'hui toute réponse non-200 est fatale), ajouter un réglage de température (absent de l'interface `llm.Provider`), afficher le texte d'erreur dans `selfconsistency`/`judge`.
7. `ROADMAP.md` (dogfooding sur des projets OSS, phase 2 durcissement, phase 3 batterie de calibration avec gate de F1 minimale par facette dans la CI) : les cases de la phase 3 sont encore décochées ; hors périmètre noté : refonte de `internal/cmd/doctor.go`.

## À fournir hors dépôt

- Variables d'environnement : `REIFY_PROVIDER`, `REIFY_MODEL`, `REIFY_API_KEY`, `ANTHROPIC_API_KEY`, `ANTHROPIC_REIFY_API_KEY`, `OPENROUTER_API_KEY`, `OPENROUTER_REIFY_API_KEY`, `REIFY_DEBUG`. Le fichier local `.envrc` (ignoré) définit `ANTHROPIC_FORGENT_API_KEY` et `OPENROUTER_FORGENT_API_KEY` : ces noms ne sont pas ceux lus par reify, il faut les rebrancher (`export ANTHROPIC_API_KEY="$ANTHROPIC_FORGENT_API_KEY"`).
- Secret CI : `CODECOV_TOKEN`. Ollama local optionnel (aucune clé).
- Artefacts de calibration non versionnés : les `classify.log` et corpus `*.jsonl` (volumineux, dérivés d'appels LLM) sont à régénérer avec `docs/calibration/REGENERATING.md` ; l'échantillonnage est déterministe (`--seed 42`), les étiquettes non. Compter environ 1000 appels pour la batterie complète (classify 206 fichiers, judge 200, explore tag ×3 modèles, cluster).
- Fichiers locaux ignorés par git, sans intérêt pour la reprise : sorties de `make dogfood` (`.claude/`, `.github/skills|agents`, `.cursor/`, `AGENTS.md`, `CONVENTIONS.md`, `.aider.conf.yml`), sauvegardes `*.bak`, binaires `reify`, `reify-calibrate`, `coverage.out`.

## Pièges connus

- `selectProvider` teste Ollama avant les variables Anthropic : si `ollama.service` tourne, `reify classify` sans `--provider` part silencieusement sur le modèle local et corrompt le corpus. Toujours passer `--provider anthropic --model claude-haiku-4-5` (valable pour `classify` ; `explore` utilise `--provider/--model`, `judge` utilise `--judge-provider/--judge-model`, avec un modèle par défaut périmé à surcharger).
- `/tmp` est purgé entre sessions : tout ce qui vit là (logs de classify, corpus) disparaît.
- `reify classify` est sériel et n'écrit son JSON qu'à la fin : lancer un processus par dépôt en parallèle, sinon un timeout perd tout le dépôt.
- `anthropicMaxTok` (`internal/llm/anthropic.go`) est une constante (16384 aujourd'hui) : `explore cluster` sur ~970 tags a déjà tronqué un JSON à 8192.
- Un modèle surchargé (HTTP 529) fait échouer tous ses appels sans retry ; relancer plus tard ou exclure ce modèle.
- Tous les fichiers committés dans ce dépôt doivent rester en anglais, y compris plans et notes (règle du projet) ; ce fichier de reprise fait exception à la demande explicite de l'auteur ou doit être traduit.
- Pour la mutation : n'ajouter au pilote que des paquets déterministes à forte couverture ; le code qui touche au LLM produit trop de mutants non tuables.
