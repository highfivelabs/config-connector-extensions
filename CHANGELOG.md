# Changelog

What changes for someone who installs config-connector-extensions. Nothing
has been released yet; each release moves its entries out of
"Unreleased".

## Unreleased

### Added

- **The slot guard, opt-in** (`docs/design.md` section 2.5). A manager
  given `--slot=<name>` (or `HFL_SLOT`; through the operator,
  `--manager-arg=--slot=<name>`) labels its objects `hfl.dev/slot`, labels
  the cloud resources that have labels `hfl-dev-slot`, and refuses to
  write or delete a resource another slot holds (`Ready` False, reason
  `SlotMismatch`, and a Warning event). An object annotated
  `hfl.dev/slot-takeover: <slot>` may claim that slot's resources, which
  is how a resource moves between two installs. Without a slot nothing
  changes. `AddToManager` takes a new `controller.Options` argument.

### Changed

- **`IAMBinding` is `iam.gcp.hfl.dev/v1beta1`** (`docs/design.md`
  decision 24). Its schema is unchanged; only the version moved. The CRD
  no longer serves `v1alpha1`, so a manifest that says
  `apiVersion: iam.gcp.hfl.dev/v1alpha1` with `kind: IAMBinding` is
  refused: change the `apiVersion`. `IAMMember` stays `v1alpha1`.

  To upgrade a cluster that has `v1alpha1` `IAMBinding` objects (the API
  server will not apply a CRD that drops a version it has stored):

  1. Annotate each one `cnrm.cloud.google.com/deletion-policy: abandon`
     and delete it; the bindings stay in Google Cloud.
  2. Delete the `iambindings.iam.gcp.hfl.dev` CRD and install the new one.
  3. Apply the same objects at `v1beta1`. Each takes over the binding it
     names (the role, under its condition, on its resource) and sets its
     members, as the old one did.
