# Allowed removals

`EXPORTED_IDENTIFIERS.lock` records the exported identifiers the schema generator
produces. Regeneration adds identifiers automatically. `make verify` fails when a
locked constant, function or variable is no longer generated and its removal has
not been recorded here, so a DDF drop that drops a node cannot silently remove an
identifier a consumer imports.

To authorize an intentional removal, add an entry below in the form
"- `csp/dmclient/Identifier` reason", then run `make generate`. The identifier
leaves the lock and verification accepts its absence. A rename requires an entry
for the removed name; the new name is added automatically. Retain entries to
document API compatibility changes.

## Entries

<!-- - `policy/speakforme/Root` area removed in the February 2026 drop -->
