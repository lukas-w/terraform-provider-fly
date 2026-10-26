# Changelog

## Unreleased

This project is based on
[fly-apps/terraform-provider-fly](https://github.com/fly-apps/terraform-provider-fly) v0.0.20.
The following changes were made since then:

### Breaking Changes

- `fly_certificate` resource:
  - Renamed
    - `dnsvalidationinstructions` → `dns_validation_instructions`
    - `dnsvalidationtarget` → `dns_validation_target`
    - `dnsvalidationhostname` → `dns_validation_hostname`
- `fly_machine` resource:
  - Moved & renamed guest config attributes:
    - `memorymb` → `guest.memory_mb`
    - `cpus` → `guest.cpus`
    - `cputype` → `guest.cpu_kind`
  - Nested attributes `env`, `processes`, `image`, `metadata`, `mounts`, `restart`, `services`,
    `guest`, `metrics` & `checks` in new `config` attribute
  - Nested attributes `exec`, `entrypoint`, `cmd` in new `init` attribute
  - Made attributes `encrypted` and `size_gb` computed as they are not documented to be settable.
    Use the corresponding attributes in the `fly_volume` resource instead.

### Enhancements

- `fly_app` resource:
  - Added `secrets` attribute
  - Added `role_id` attribute
  - Added `enable_consul` attribute
  - Added `preferred_region` attribute
  - Support changing `name` by replacing
  - Support for changing `org` by moving the app to the new organization
- `fly_certificate` resource:
  - Support changing `app` and `hostname` by replacing
- `fly_volume` resource:
  - Added `encrypted` attribute
  - Support increasing volume size
- `fly_ip` resource:
  - Support changing `app`, `type` and `region` via replacement
- `fly_machine` resource:
  - Added attribute `guest.kernel_args`
  - Added attribute `services[].ports[].force_https`
  - Added attribute `services[].concurrency`
  - Added attribute `processes`
  - Added attribute `restart`
  - Added attribute `metrics`
  - Added attribute `checks`
  - Added attribute `metadata`
  - Added attribute `schedule`
  - Make waiting for machine start-up optional via the new `wait` attribute
  - Added support for waiting for health checks to pass using the new `wait_for_checks` attribute
- Added `fly_deploy_token` resource
- Added `fly_organization` resource
- Added `fly_organization` data source
- Added `fly_secret` resource
- Added `fly_deployment` resource
- Added `fly_redis` resource
- Added `fly_volume` data source

### Fixes

- `fly_app` resource:
  - Fixed an error when app was deleted externally
- `fly_volume` resource:
  - Fixed an error when volume was deleted externally
- Many bug-fixes

### Internal Changes

- Upgraded from `terraform-plugin-framework` 0.11 to 1.13
- Added acceptance tests for `fly_app`, `fly_ip` & `fly_volume` resources
- Reduce custom API client code by using official `fly-go` & `flyctl` where possible
- Various refactorings to reduce duplicated code
- Remove generated code & docs from version control
- Removed the need for manually creating a Fly app for acceptance testing
- Added test sweeper to delete stray apps
- `fly_machine`: Use new flaps API client from `fly-go` instead of custom implementation
