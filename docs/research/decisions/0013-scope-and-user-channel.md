# 0013: Scope and the user channel

## Context

A Windows management tree has a device scope and a user scope. The server prefixes a LocURI with
`./Device` (the default when absent) or `./User`, and a user setting can only be applied when a
user is signed in. Fleet issues #50196 and #48931 and the Learn known-issues page record what
happens otherwise: a `./User` command sent before sign-in fails with 500 or 507, and if it was
inside an Atomic the whole Atomic rolls back (Fleet #40713 mixed device and user commands in one
Atomic). On an Entra-joined device, `./User` provisioning fails entirely unless the signed-in
user is an Entra user. Azure Virtual Desktop adds a third case: the client runs a device session
or a user session, and a server that sends the wrong scope gets Status 405.

## Decision

Scope is a property of a command, derived from its LocURIs by the builders: a URI under `./User`
is user scope, everything else is device scope (matching the client's default). The command
builders refuse a Get that mixes scopes and an Atomic or Sequence that mixes device and user
commands, so a scope-crossing batch cannot be constructed. The engine gates delivery on the
package-1 facts: a user-scoped command is held in the queue until the session's LoginStatus
reports `user`, and in an AVD device session (SyncType `device`) user commands are skipped while
in a user session (SyncType `user`) device commands are skipped, which is the server side of the
405 rule. A held command stays `pending` and is delivered on a later session once a user is
present.

The Entra-joined `./User` limitation and the add-work-account and Entra-joined unenroll
limitations from the known-issues page are documented on the API rather than enforced: the
engine delivers the user command once LoginStatus allows it, and whether the signed-in user is
an Entra user is something only the device knows and reports through the command's own Status.

## Rationale

Refusing a scope-crossing Atomic at construction time makes the most damaging version of the
pitfall impossible: a batch that would roll back device settings because a user was not signed
in cannot be built. Holding user commands on LoginStatus rather than sending and hoping is the
fix for the 500/507-then-rollback failure. Deriving scope from the URI keeps the caller from
having to declare it and keeps it consistent with what the client infers. Leaving the
Entra-user refinement to the device's Status, rather than guessing on the server, avoids
encoding a rule the server cannot actually evaluate.

## Constraints

The engine reads scope from the LocURI prefix; a caller that hand-builds a command with a
mismatched declared scope is caught by `Validate`, but the URI is the source of truth. AVD
multi-user parallel sessions are not run (decision record 0011); only the single SyncType of a
session is honoured. Whether a `./User` command will succeed on a given Entra-joined device is
not predicted; the command is delivered and its Status reports the outcome.

## Verification

`mdmprotocol/mdm` builder tests refuse a mixed-scope Get, Atomic and Sequence and an Add
then Replace on one node inside an Atomic. The engine tests hold a `./User` command through a
no-user session and deliver it after a sign-in, and the AVD device-session and user-session
scope gates are unit-tested in `scopeAllows`. The `simulator` package runs the hold-and-release
scenario end to end.

## References

- [mdmprotocol/mdm](../../../mdmprotocol/mdm) (`command.go` scope rules, `deliver.go` `scopeAllows`)
- [Research store](../../research.md), section 7 (pitfall "User-scope commands before sign-in"), section 1.4 (User vs device scope, Known issues)
- Microsoft, OMA DM protocol support, user vs device targeting: <https://learn.microsoft.com/en-us/windows/client-management/oma-dm-protocol-support>
- Microsoft, [MS-MDM] 3.2.5.1 (AVD device and user sessions): <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mdm/33769a92-ac31-47ef-ae7b-dc8501f7104f>
- Fleet issues #50196, #48931 and #40713
