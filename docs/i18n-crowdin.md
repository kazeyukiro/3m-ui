# Crowdin (panel UI i18n)

3m-ui keeps **TypeScript** locale modules for the app (`frontend/src/i18n/locales/*.ts`) and uses **JSON** as the Crowdin exchange format (`frontend/src/i18n/crowdin/*.json`).

## One-time setup

1. Create a project on [Crowdin](https://crowdin.com/) (source language: **English**).
2. Add target languages matching repo codes: `zh-CN`, `zh-TW`, `ja`, `ko`, `es`, `fr`, `de`, `ru`, `pt-BR`, `vi`, `id`, `th`, `tr`, `ar`, `hi`, `pl`, `uk`.
3. Create a **Personal Access Token** (Crowdin account → Settings → API).
4. In the GitHub repo → **Settings → Secrets and variables → Actions** → tab **Secrets** → **Repository secrets** (not Environment secrets), click **New repository secret**, add:
   - `CROWDIN_PROJECT_ID` — numeric project id
   - `CROWDIN_PERSONAL_TOKEN` — API token
5. (Optional) Install the [Crowdin GitHub app](https://github.com/apps/crowdin) on the repo for UI integration; Actions below work with the token alone.

## Local commands

```bash
# TS → JSON (before upload)
node scripts/i18n/ts-to-json.mjs

# After Crowdin download into crowdin/*.json → TS
node scripts/i18n/json-to-ts.mjs

# Ensure every locale has the same keys as en
node scripts/i18n/check-keys.mjs
```

## GitHub Actions

Workflow: **Crowdin** (`.github/workflows/crowdin.yml`)

| Trigger | Behaviour |
|---------|-----------|
| Push to `main` changing `en.ts` | Upload English sources to Crowdin |
| Manual **Run workflow** → `upload` / `download` / `both` | Upload and/or download; download opens a PR branch `l10n_crowdin_translations` |

Until the two secrets exist, the job is skipped (`if: secrets…`).

## App code

Do not edit `frontend/src/i18n/crowdin/` by hand for long-term source of truth — edit `locales/*.ts` (especially `en.ts`), export JSON, translate on Crowdin, import back.
