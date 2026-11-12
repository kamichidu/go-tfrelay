# Golden Set

The Golden Set records the contracts of the existing Source, Transform, and Sink separately. It does not add JSON input support or change the relay data model.

## Layout

| Path | Contents | Purpose |
| --- | --- | --- |
| `testdata/golden/cases/` | Small Terraform HCL configurations and optional import manifests | Inputs used to produce plans |
| `testdata/golden/environments/` | Exact Terraform version, provider constraints, and `.terraform.lock.hcl` | Reproducible provider installation |
| `testdata/golden/snapshots/<environment>/<case>/` | Canonical Terraform `plan.json` or state `state.json`, plus Source `expected.json` | Fixed plan/state and Source contract |
| `testdata/golden/snapshots/transforms/` | Transform input and expected relay JSON | Terraform-independent Transform contract |
| `testdata/golden/snapshots/sinks/` | Sink input, template, and expected rendered output | Terraform-independent Sink contract |

The committed `plan.json` is the Terraform show JSON input fixture. Golden checks normalize it directly and never invoke Terraform, initialize providers, or contact a registry. The Source command itself invokes `terraform show -json` according to its selected CLI mode.

## Fixture environments

Each environment records the Terraform and provider versions used to create its committed JSON fixtures. Golden checks do not install providers or access a backend.

The initial environment uses Terraform `1.17.1`, `hashicorp/random` `3.7.2`, and `hashicorp/time` `0.13.1`. The create case covers declared values, provider-computed values, the provider default for `random_password.defaulted.upper` (the HCL omits `upper`; the plan resolves it to `true`), sensitive output, and a partially unknown dependent object. The update case imports three stable `terraform_data` instances into local state, then changes their inputs to produce update actions without credentials or remote infrastructure.

## Running tests and updating snapshots

1. Run `go test ./...` to check the Source, Transform, and Sink contracts.
2. Run `go run ./cmd/golden -check` to normalize committed Terraform show JSON and compare both plan and RelayData snapshots.
3. For a reviewed intentional fixture change, run `go run ./cmd/golden -update`, inspect all changed snapshots, and commit those changes explicitly.

The update command is never run by CI. CI is read-only with respect to golden files and fails when generated output differs.

## Comparison rules

JSON is decoded into objects before comparison, so object key order has no meaning and numbers are retained without floating-point conversion. The plan comparator replaces only the generated top-level `timestamp` with `null` and sorts resource-set arrays by address. It preserves configuration, defaults, unknown values, sensitive markers, prior values, and action details. The relay comparator replaces only generated `__id__` values and sorts resource and instance collections by stable keys. No attribute values or sensitivity metadata are removed.

Changes to a fixture, toolchain, provider schema, or output must be reviewed together with the resulting `plan.json` and `expected.json`. The golden files are executable compatibility evidence, not an automatic approval of changed behavior.
