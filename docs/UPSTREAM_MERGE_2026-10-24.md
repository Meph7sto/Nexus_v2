# Upstream merge: 2026-10-24

## Revisions and method

- Nexus base: `352ad27bb3ca38c1a716bcdb26d08e7964ce8be1`.
- Upstream: `Wei-Shaw/sub2api`, `37067e8e340e9440d173a92b4faa05e8ce58368f`
  (main after v0.2.18, including scoped administrator API keys).
- Simulated with `git merge-tree --write-tree --name-only HEAD upstream/main`.
- Reviewed 45 conflicted paths, then resolved the merge without replacing the
  Nexus frontend or selecting either side wholesale.

## Preserved custom behavior

- Nexus PNG branding, global CSS, Tailwind configuration, navigation and layouts.
- Monthly ledger and email workflows, OpenAI quota summary, readable interaction
  records, administrator permission model and deployment naming.
- Existing removal of onboarding/compliance prompts and the driver.js dependency.
- New upstream UI uses the existing light presentation and permission gates.

## Integration decisions

- Interaction capture follows both successful and interrupted stream usage
  submission, and is also attached to the new System One gateway path.
- Ordinary administrators retain interaction availability in usage responses;
  scoped administrator keys receive the upstream restricted response.
- Scoped keys use a machine principal, accept super-admin creators, and remain
  bounded by the current Nexus permissions of limited administrator creators.
- Key lifecycle management requires a human super-admin. Scoped keys remain
  subject to rate limits; the existing human super-admin exemption is retained.
- Wire was regenerated. Cleanup tolerates an absent monthly-ledger service.
- Upstream database migrations and merged Ent output are included. No migration
  was executed against a live database.

## Verification

- Passed frontend type checking, production build and ESLint for changed TS/Vue
  files. The build reported only existing chunk-size/Browserslist warnings.
- Initial full Vitest run covered 397 files and 3,023 tests. All merge-related
  failures were repaired and retested; seven affected suites passed 27 tests.
- Final payment/settings/localization checks passed another 63 tests.
- Thirty frontend failures were reproduced on the original Nexus revision:
  AccountUsageCell (15), UseKeyModal (2), KeysView (9), and
  ProxiesView.credentials (4). The full frontend suite is therefore not green.
- Passed Go unit tests for services, handlers, server/routes/middleware, setup,
  Ent schemas, repositories, packages, domain and payment providers.
- Passed embedded-resource, startup/cleanup, configuration and migration tests;
  built the server with `go build -tags=embed`.
- golangci-lint v2.14.0 reported zero issues with `--new-from-rev=HEAD` against
  the pre-merge Nexus base. The older installed linter could not read Go 1.27.3
  export data, so this check used the version specified by upstream CI.
- Browser smoke checks used mocked APIs at 1440x900 and 390x844: 18 current-page
  captures had no page errors, broken images or horizontal overflow. Baseline
  captures and source comparisons confirmed the retained Nexus presentation.
- No production deployment, live provider calls or database-backed integration
  run was performed. Ent regeneration hit a Windows mapped-file lock; its
  partial generated-file write was restored from the merged index, and the
  merged Ent code compiled and passed its schema checks.

Local diagnostic logs, baseline checkout, screenshots and executable are under
ignored `tmp/merge-20261024-*` paths and are not part of the commit.
