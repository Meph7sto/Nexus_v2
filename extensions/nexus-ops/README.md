# Nexus Ops

Independent maintenance module in the Nexus repository.

## Status

The official Pi coding-agent package is a local, version-pinned SDK dependency.
This module uses the published package rather than a copied source tree or a
nested Git checkout. The maintenance agent, bridge, and deployment services are
not implemented yet; installing Pi does not connect it to Nexus.

## Dependencies and Checks

Run from `extensions/nexus-ops`:

```sh
pnpm install --frozen-lockfile --ignore-scripts
pnpm exec node --version
pnpm run pi --version
pnpm test
```

The dependency version is pinned in `package.json`, and transitive dependencies
are locked in `pnpm-lock.yaml`. Pi's published package requires Node >=22.19.0.
The module pins Node 22.23.2 using pnpm's `devEngines.runtime` support so module
scripts do not depend on or replace the machine's global Node installation.

The smoke test imports the SDK without starting an agent session, calling a
model, or accessing Nexus. The `pi` script exposes the upstream CLI, not a
permission-restricted maintenance assistant. Do not give it Nexus administrator
credentials; the bridge and restricted tool configuration still need to be built.

Official references: [Pi](https://pi.dev/),
[SDK](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/sdk.md),
[pnpm runtime management](https://pnpm.io/11.x/package_json#devenginesruntime).

## Layout

| Directory | Intended contents |
| --- | --- |
| `agent/` | Pi integration, tool definitions, and session management |
| `bridge/` | Nexus HTTP client, authorization, approvals, and response redaction |
| `contracts/` | Request and response contracts shared by agent and bridge |
| `skills/` | Maintenance workflows for inspection and incident diagnosis |
| `tests/` | Contract, authorization, and integration tests |

Empty directories contain `.gitkeep` files so Git can preserve the scaffold.

## Implementation Boundaries

- Keep dependencies, builds, and deployment configuration local to this module.
- Run agent and bridge separately from the Nexus backend.
- Access Nexus through its HTTP APIs, not internal Go packages or the business database.
- Keep Nexus administrator credentials in the bridge, not the agent or browser.
- Enforce allowed operations and approvals in the bridge, not only in prompts.
- Keep the module optional: stopping it must not stop Nexus request forwarding.

Existing admin API tooling is documented in
[`skills/nexus-admin/SKILL.md`](../../skills/nexus-admin/SKILL.md).
