# How-To: Contacts

This guide explains how to test contact lifecycle operations using the EPP Test Framework. It is anchored to the `contact_lifecycle` conformance scenario (`scenarios/conformance/contact_lifecycle.yaml`), which exercises RFC 5733 §3.2 operations: create, info, update, and delete, including domain association to verify referential integrity.

See [Getting Started](getting-started.md) for setup instructions and [Config Reference](config-reference.md) for credential configuration.

## Registrar Interface Methods

The `Registrar` interface exposes contact operations through `ContactManager`:

| Method | Description |
|--------|-------------|
| `CheckContact(ctx, ids...)` | Reports whether one or more contact IDs are available |
| `InfoContact(ctx, id)` | Returns the full contact object for the given ID |
| `CreateContact(ctx, req)` | Registers a new contact |
| `UpdateContact(ctx, req)` | Modifies an existing contact (email, postal address, etc.) |
| `DeleteContact(ctx, id)` | Removes a contact object |

All methods accept `context.Context` as the first argument.

`ContactCreateRequest` carries: `ID`, `Name`, `Org`, `Email`, `Phone`, `Fax`, `Street`, `City`, `StateProvince`, `PostalCode`, `CountryCode`, `AuthInfo`, and `Extensions` for registrar-specific data.

## Conformance Scenario

The `contact_lifecycle` scenario (`scenarios/conformance/contact_lifecycle.yaml`) runs against `internetx`, `nicat`, and `eurid`. After creating and updating a contact, it associates the contact with a domain as its registrant, then deletes the domain before the contact to satisfy referential integrity:

```yaml
name: contact_lifecycle
rfc: "RFC 5733 §3.2"
matrix: [internetx, nicat, eurid]

steps:
  - name: create_contact
    op: create_contact
    params:
      name: "Test User"
      org: "Test Org"
      email: "test@example.com"
      street: ["1 Test St"]
      city: "Vienna"
      state_province: "W"
      postal_code: "1010"
      country_code: "AT"
      phone: "+43.1234567"
      auth_info: "cAuth1"
    expect:
      code: 1000

  - name: info_contact
    op: info_contact
    params:
      id: "${create_contact.ID}"
    expect:
      code: 1000
      fields:
        Email: "test@example.com"

  - name: update_contact
    op: update_contact
    params:
      id: "${create_contact.ID}"
      email: "new@example.com"
    expect:
      code: 1000

  # Associate the contact with a domain as its registrant (CONF-02: contact-domain association).
  - name: create_domain
    op: create_domain
    params:
      name: contact-assoc.example
      registrant: "${create_contact.ID}"
      period: 1
      auth_info: "domAuth01"
    expect:
      code: 1000

  # Remove the domain before its registrant contact (referential integrity).
  - name: delete_domain
    op: delete_domain
    params:
      name: contact-assoc.example
    expect:
      code: 1000

  - name: delete_contact
    op: delete_contact
    params:
      id: "${create_contact.ID}"
    expect:
      code: 1000
```

### Variable Interpolation

`${create_contact.ID}` references the contact handle assigned by the server after creation. The runner captures the `ContactResult` from the `create_contact` step; subsequent steps can reference any field using `${step_name.FieldName}`.

### Referential Integrity Order

EPP registrars reject `DeleteContact` when the contact is still linked to a domain as registrant or technical/admin contact. The scenario explicitly deletes the domain (`delete_domain`) before the contact (`delete_contact`). The runner's cleanup stack also respects this order — cleanup closures run in reverse creation order.

## What to Assert

**`create_contact`** — Assert `code: 1000`. The runner registers a cleanup to call `DeleteContact` at test end. The server assigns a contact handle (ROID-derived ID); capture it with `${create_contact.ID}`.

**`info_contact`** — Assert `code: 1000` and verify specific fields: `Email`, `Name`, `ID`. The `Email` field assertion (`Email: "test@example.com"`) confirms the stored value matches what was sent at create time.

**`update_contact`** — Assert `code: 1000`. Changing `email` to `"new@example.com"` is a minimal smoke test for the update operation. Follow with an `info_contact` step if you want to assert the updated value was persisted.

**`delete_contact`** — Assert `code: 1000`. Attempting to delete a contact that still has active domain associations returns EPP code `2305` (object association prohibits operation).

## Running the Scenario

```sh
# Unit test (mock server, offline)
go test -tags unit -run TestContactLifecycle ./scenarios/conformance/

# Integration test against NiCAT (requires credentials)
EPP_REGISTRARS_NICAT_PASSWORD=secret \
  go test -tags integration -run TestContactLifecycle/nicat ./scenarios/conformance/
```
